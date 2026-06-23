// Package openai implements the ProviderAdapter for the OpenAI API.
// It translates Canonical IR requests into OpenAI-compatible HTTP calls and
// normalises responses back into the gateway's canonical types.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
)

// DefaultAPIBase is the default OpenAI API base URL.
const DefaultAPIBase = "https://api.openai.com/v1"

// Adapter implements provider_sdk.ProviderAdapter for OpenAI.
type Adapter struct {
	httpClient *http.Client
	apiBase    string

	// capabilitiesMu protects the capabilities cache.
	capabilitiesMu sync.RWMutex
	// capabilitiesCache holds per-model capabilities to avoid recomputation.
	capabilitiesCache map[string]provider_sdk.ModelCapabilities
}

// NewAdapter creates a new OpenAI adapter with the given API base URL.
// If apiBase is empty, DefaultAPIBase is used.
func NewAdapter(apiBase string) *Adapter {
	if apiBase == "" {
		apiBase = DefaultAPIBase
	}
	apiBase = strings.TrimRight(apiBase, "/")

	return &Adapter{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		apiBase:           apiBase,
		capabilitiesCache: make(map[string]provider_sdk.ModelCapabilities),
	}
}

// ---------------------------------------------------------------------------
// ProviderAdapter interface implementation
// ---------------------------------------------------------------------------

// Provider returns the provider identifier.
func (a *Adapter) Provider() provider_sdk.ProviderID {
	return "openai"
}

// Validate checks whether the source credential is valid by making a
// lightweight API call (list models with limit=1).
func (a *Adapter) Validate(ctx context.Context, source provider_sdk.SourceCredential) provider_sdk.ValidationResult {
	apiKey, err := extractAPIKey(source.Credential)
	if err != nil {
		return provider_sdk.ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("failed to extract API key: %v", err),
			ErrorCode: "invalid_credential",
		}
	}

	// Use a lightweight models list call with context timeout.
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.apiBase+"/models", nil)
	if err != nil {
		return provider_sdk.ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("failed to create validation request: %v", err),
			ErrorCode: "internal_error",
		}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return provider_sdk.ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("validation request failed: %v", err),
			ErrorCode: "network_error",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return provider_sdk.ValidationResult{Valid: true}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return provider_sdk.ValidationResult{
		Valid:     false,
		Message:   fmt.Sprintf("upstream returned %d: %s", resp.StatusCode, string(body)),
		ErrorCode: fmt.Sprintf("http_%d", resp.StatusCode),
	}
}

// DiscoverModels calls GET /v1/models and returns the model list.
func (a *Adapter) DiscoverModels(ctx context.Context, source provider_sdk.SourceCredential) []provider_sdk.ProviderModel {
	apiKey, err := extractAPIKey(source.Credential)
	if err != nil {
		return nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.apiBase+"/models", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := make([]provider_sdk.ProviderModel, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, provider_sdk.ProviderModel{
			ID:           provider_sdk.ModelID(m.ID),
			ProviderID:   "openai",
			DisplayName:  m.ID,
			Modality:     provider_sdk.ModalityText,
			Capabilities: a.Capabilities(m.ID),
		})
	}
	return models
}

// Capabilities returns the capability matrix for the named model.
// It uses a hardcoded mapping for well-known models and falls back to a
// sensible default for unknown models.
func (a *Adapter) Capabilities(model string) provider_sdk.ModelCapabilities {
	// Check cache first.
	a.capabilitiesMu.RLock()
	if caps, ok := a.capabilitiesCache[model]; ok {
		a.capabilitiesMu.RUnlock()
		return caps
	}
	a.capabilitiesMu.RUnlock()

	caps := a.resolveCapabilities(model)

	a.capabilitiesMu.Lock()
	a.capabilitiesCache[model] = caps
	a.capabilitiesMu.Unlock()

	return caps
}

// Estimate returns a pre-flight cost estimate based on the model and token
// estimates derived from the request.
func (a *Adapter) Estimate(ctx context.Context, req provider_sdk.CanonicalRequest) provider_sdk.CostEstimate {
	inputTokens := estimateInputTokens(req)
	outputTokens := estimateOutputTokens(req)

	pricing := modelPricing(string(req.RequestedModel))

	return provider_sdk.CostEstimate{
		EstimatedCostMicroUSD: pricing.InputPriceMicroUSD*int64(inputTokens) +
			pricing.OutputPriceMicroUSD*int64(outputTokens),
		EstimatedInputTokens:  inputTokens,
		EstimatedOutputTokens: outputTokens,
		Currency:              "USD",
	}
}

// Invoke executes a synchronous request and returns the complete response
// with usage information.
func (a *Adapter) Invoke(ctx context.Context, req provider_sdk.CanonicalRequest, source provider_sdk.ResolvedSource) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	// Check for context cancellation.
	select {
	case <-ctx.Done():
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{}, ctx.Err()
	default:
	}

	apiKey, err := extractAPIKey(source.Credential)
	if err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			&provider_sdk.ProviderError{
				Code:    "invalid_credential",
				Message: "failed to extract API key",
				Fatal:   true,
				Raw:     err,
			}
	}

	switch req.Modality {
	case provider_sdk.ModalityText, provider_sdk.ModalityMultimodal:
		return a.chatCompletion(ctx, req, apiKey)
	default:
		// For embeddings or other modalities, route accordingly.
		return a.chatCompletion(ctx, req, apiKey)
	}
}

// Stream executes a streaming request, emitting each chunk via the EventSink.
// It returns the cumulative usage when the stream completes.
func (a *Adapter) Stream(ctx context.Context, req provider_sdk.CanonicalRequest, source provider_sdk.ResolvedSource, emit provider_sdk.EventSink) (provider_sdk.Usage, error) {
	select {
	case <-ctx.Done():
		return provider_sdk.Usage{}, ctx.Err()
	default:
	}

	apiKey, err := extractAPIKey(source.Credential)
	if err != nil {
		return provider_sdk.Usage{},
			&provider_sdk.ProviderError{
				Code:    "invalid_credential",
				Message: "failed to extract API key",
				Fatal:   true,
				Raw:     err,
			}
	}

	return a.streamChatCompletion(ctx, req, apiKey, emit)
}

// NormalizeError converts a raw upstream error into a structured ProviderError.
func (a *Adapter) NormalizeError(err error) provider_sdk.ProviderError {
	if err == nil {
		return provider_sdk.ProviderError{
			Code:    "unknown",
			Message: "nil error",
		}
	}

	// If it is already a ProviderError, return it.
	var pe *provider_sdk.ProviderError
	if AsProviderError(err, &pe) {
		return *pe
	}

	// Check for context errors.
	if err == context.Canceled {
		return provider_sdk.ProviderError{
			Code:       "context_canceled",
			Message:    "request was canceled",
			Retryable:  true,
			StatusCode: 499,
			Raw:        err,
		}
	}
	if err == context.DeadlineExceeded {
		return provider_sdk.ProviderError{
			Code:       "context_deadline_exceeded",
			Message:    "request timed out",
			Retryable:  true,
			StatusCode: 504,
			Raw:        err,
		}
	}

	// Attempt to parse as an OpenAI API error.
	if oaiErr := parseOpenAIError(err); oaiErr != nil {
		return *oaiErr
	}

	// Default: treat as a transient network error.
	return provider_sdk.ProviderError{
		Code:       "network_error",
		Message:    err.Error(),
		Retryable:  true,
		StatusCode: 0,
		Raw:        err,
	}
}

// Health performs a health check against the upstream by making a lightweight
// API call to list models.
func (a *Adapter) Health(ctx context.Context, source provider_sdk.ResolvedSource) provider_sdk.HealthResult {
	apiKey, err := extractAPIKey(source.Credential)
	if err != nil {
		return provider_sdk.HealthResult{
			Healthy:      false,
			ErrorMessage: fmt.Sprintf("invalid credential: %v", err),
			LastChecked:  time.Now(),
		}
	}

	start := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.apiBase+"/models", nil)
	if err != nil {
		return provider_sdk.HealthResult{
			Healthy:      false,
			ErrorMessage: fmt.Sprintf("failed to create health check request: %v", err),
			LastChecked:  time.Now(),
		}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := a.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return provider_sdk.HealthResult{
			Healthy:      false,
			Latency:      latency,
			ErrorMessage: err.Error(),
			LastChecked:  time.Now(),
		}
	}
	defer resp.Body.Close()

	return provider_sdk.HealthResult{
		Healthy:     resp.StatusCode == http.StatusOK,
		Latency:     latency,
		LastChecked: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// extractAPIKey extracts the API key from a json.RawMessage credential.
// The credential can be:
//   - A raw string: "sk-..."
//   - A JSON object: {"api_key": "sk-..."}
func extractAPIKey(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("empty credential")
	}

	// Try as a raw string.
	if raw[0] == '"' {
		var key string
		if err := json.Unmarshal(raw, &key); err == nil && key != "" {
			return key, nil
		}
	}

	// Try as a JSON object with api_key field.
	var obj struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("unable to parse credential: %w", err)
	}
	if obj.APIKey == "" {
		return "", fmt.Errorf("credential does not contain api_key")
	}
	return obj.APIKey, nil
}

// doRequest is a helper that sends an HTTP request, checks status, and
// returns the response body.
func (a *Adapter) doRequest(ctx context.Context, method, url string, body []byte, apiKey string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, newOpenAIError(resp.StatusCode, respBody)
	}

	return respBody, resp.StatusCode, nil
}

// chatCompletion sends a synchronous chat completion request to OpenAI.
func (a *Adapter) chatCompletion(ctx context.Context, req provider_sdk.CanonicalRequest, apiKey string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	body, err := convertRequest(req)
	if err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			&provider_sdk.ProviderError{
				Code:    "request_conversion_error",
				Message: fmt.Sprintf("failed to convert request: %v", err),
				Fatal:   true,
				Raw:     err,
			}
	}

	respBody, statusCode, err := a.doRequest(ctx, http.MethodPost, a.apiBase+"/chat/completions", body, apiKey)
	if err != nil {
		// Check if this is already an OpenAI error.
		var oaiErr *openAIError
		if AsOpenAIError(err, &oaiErr) {
			return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
				&provider_sdk.ProviderError{
					Code:       oaiErr.ErrorCode,
					Message:    oaiErr.Message,
					StatusCode: oaiErr.StatusCode,
					Retryable:  isRetryableStatus(oaiErr.StatusCode),
					Raw:        err,
				}
		}
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			&provider_sdk.ProviderError{
				Code:       "upstream_error",
				Message:    err.Error(),
				StatusCode: statusCode,
				Retryable:  true,
				Raw:        err,
			}
	}

	return convertResponse(respBody, string(req.RequestedModel))
}

// resolveCapabilities returns the capability matrix for a given model name.
func (a *Adapter) resolveCapabilities(model string) provider_sdk.ModelCapabilities {
	// GPT-4 family models.
	if strings.HasPrefix(model, "gpt-4") || strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportNative,
			SupportsVideoInput:   provider_sdk.SupportBestEffort,
			SupportsJSONMode:     provider_sdk.SupportNative,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportNative,
			MaxContextTokens:     128000,
			MaxOutputTokens:      16384,
		}
	}

	// GPT-3.5 family.
	if strings.HasPrefix(model, "gpt-3.5") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportUnsupported,
			SupportsAudioInput:   provider_sdk.SupportUnsupported,
			SupportsVideoInput:   provider_sdk.SupportUnsupported,
			SupportsJSONMode:     provider_sdk.SupportNative,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportUnsupported,
			MaxContextTokens:     16385,
			MaxOutputTokens:      4096,
		}
	}

	// Embedding models.
	if strings.Contains(model, "embedding") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportUnsupported,
			SupportsToolUse:      provider_sdk.SupportUnsupported,
			SupportsImageInput:   provider_sdk.SupportUnsupported,
			SupportsAudioInput:   provider_sdk.SupportUnsupported,
			SupportsVideoInput:   provider_sdk.SupportUnsupported,
			SupportsJSONMode:     provider_sdk.SupportUnsupported,
			SupportsSystemPrompt: provider_sdk.SupportUnsupported,
			SupportsMultiTurn:    provider_sdk.SupportUnsupported,
			SupportsReasoning:    provider_sdk.SupportUnsupported,
			MaxContextTokens:     8192,
			MaxOutputTokens:      1,
		}
	}

	// Default for unknown models: assume basic chat capabilities.
	return provider_sdk.ModelCapabilities{
		SupportsStreaming:    provider_sdk.SupportNative,
		SupportsToolUse:      provider_sdk.SupportNative,
		SupportsImageInput:   provider_sdk.SupportBestEffort,
		SupportsAudioInput:   provider_sdk.SupportUnsupported,
		SupportsVideoInput:   provider_sdk.SupportUnsupported,
		SupportsJSONMode:     provider_sdk.SupportBestEffort,
		SupportsSystemPrompt: provider_sdk.SupportNative,
		SupportsMultiTurn:    provider_sdk.SupportNative,
		SupportsReasoning:    provider_sdk.SupportUnsupported,
		MaxContextTokens:     128000,
		MaxOutputTokens:      4096,
	}
}

// modelPricing returns the pricing for a given model.
func modelPricing(model string) provider_sdk.ModelPricing {
	// Pricing in micro-USD (millionths of a USD).
	switch {
	case strings.HasPrefix(model, "gpt-4o"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  2500,  // $2.50/1M input tokens
			OutputPriceMicroUSD: 10000, // $10.00/1M output tokens
			Currency:            "USD",
		}
	case strings.HasPrefix(model, "gpt-4"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  30000, // $30.00/1M input tokens
			OutputPriceMicroUSD: 60000, // $60.00/1M output tokens
			Currency:            "USD",
		}
	case strings.HasPrefix(model, "o1"), strings.HasPrefix(model, "o3"), strings.HasPrefix(model, "o4"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  15000, // $15.00/1M input tokens
			OutputPriceMicroUSD: 60000, // $60.00/1M output tokens
			Currency:            "USD",
		}
	case strings.HasPrefix(model, "gpt-3.5"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  500,  // $0.50/1M input tokens
			OutputPriceMicroUSD: 1500, // $1.50/1M output tokens
			Currency:            "USD",
		}
	default:
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  0,
			OutputPriceMicroUSD: 0,
			Currency:            "USD",
		}
	}
}

// estimateInputTokens provides a rough estimate of input tokens from the request.
func estimateInputTokens(req provider_sdk.CanonicalRequest) int {
	total := 0
	for _, msg := range req.Messages {
		// Rough estimation: ~4 characters per token.
		contentStr := string(msg.Content)
		total += len(contentStr) / 4
	}
	if total < 1 {
		total = 1
	}
	return total
}

// estimateOutputTokens provides a rough estimate of output tokens.
func estimateOutputTokens(req provider_sdk.CanonicalRequest) int {
	if req.GenerationParams.MaxTokens > 0 {
		return req.GenerationParams.MaxTokens / 2
	}
	// Default estimate.
	return 256
}

// isRetryableStatus returns true for HTTP status codes that indicate a
// transient error worth retrying.
func isRetryableStatus(status int) bool {
	return status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}
