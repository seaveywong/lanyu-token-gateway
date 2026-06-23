// Package anthropic implements the ProviderAdapter for the Anthropic (Claude)
// Messages API. It translates CanonicalRequest into Anthropic-compatible HTTP
// calls and normalises responses back into the gateway's canonical types.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	provider_sdk "github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
)

// DefaultAPIBase is the default Anthropic API base URL.
const DefaultAPIBase = "https://api.anthropic.com/v1"

// ---------------------------------------------------------------------------
// Anthropic wire types
// ---------------------------------------------------------------------------

// anthropicRequest is the JSON body sent to POST /v1/messages.
type anthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []anthropicMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	System      string              `json:"system,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	TopP        *float64            `json:"top_p,omitempty"`
	TopK        *int                `json:"top_k,omitempty"`
	Stream      bool                `json:"stream"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool     `json:"tools,omitempty"`
	ToolChoice  interface{}         `json:"tool_choice,omitempty"`
	Metadata    map[string]string   `json:"metadata,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type         string           `json:"type"`
	Text         string           `json:"text,omitempty"`
	Source       *anthropicSource `json:"source,omitempty"`
	ToolUseID    string           `json:"tool_use_id,omitempty"`
	Content      json.RawMessage  `json:"content,omitempty"`       // for tool_result type
	Name         string           `json:"name,omitempty"`
	ID           string           `json:"id,omitempty"`
	Input        json.RawMessage  `json:"input,omitempty"`         // for tool_use type
}

type anthropicSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg", "image/png", "image/gif", "image/webp"
	Data      string `json:"data"`       // base64-encoded
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicResponse is the JSON body returned by POST /v1/messages.
type anthropicResponse struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"` // "message"
	Role       string              `json:"role"`
	Model      string              `json:"model"`
	Content    []anthropicContent  `json:"content"`
	StopReason string              `json:"stop_reason"`
	StopSequence string            `json:"stop_sequence,omitempty"`
	Usage      anthropicUsage      `json:"usage"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// anthropicErrorResponse is the error body returned by Anthropic.
type anthropicErrorResponse struct {
	Type  string              `json:"type"` // "error"
	Error anthropicErrorDetail `json:"error"`
}

type anthropicErrorDetail struct {
	Type    string `json:"type"`    // e.g. "invalid_request_error", "authentication_error"
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// Anthropic SSE streaming types
// ---------------------------------------------------------------------------

// anthropicSSEEvent represents a parsed SSE event from the Anthropic stream.
type anthropicSSEEvent struct {
	Event string          `json:"-"` // the "event:" line value
	Data  json.RawMessage `json:"-"` // the "data:" line value
}

// anthropicMessageStart is the data payload for event:message_start.
type anthropicMessageStart struct {
	Type    string              `json:"type"`
	Message anthropicStreamMsg  `json:"message"`
}

type anthropicStreamMsg struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Usage   *anthropicUsage `json:"usage,omitempty"`
}

// anthropicContentBlockStart is the data payload for event:content_block_start.
type anthropicContentBlockStart struct {
	Type         string          `json:"type"`
	Index        int             `json:"index"`
	ContentBlock json.RawMessage `json:"content_block"`
}

// anthropicContentBlockDelta is the data payload for event:content_block_delta.
type anthropicContentBlockDelta struct {
	Type  string          `json:"type"`
	Index int             `json:"index"`
	Delta json.RawMessage `json:"delta"`
}

// anthropicTextDelta is the delta.text content.
type anthropicTextDelta struct {
	Type string `json:"type"` // "text_delta"
	Text string `json:"text"`
}

// anthropicMessageDelta is the data payload for event:message_delta.
type anthropicMessageDelta struct {
	Type  string              `json:"type"`
	Delta anthropicDeltaBody  `json:"delta"`
	Usage anthropicUsageData  `json:"usage"`
}

type anthropicDeltaBody struct {
	StopReason  string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type anthropicUsageData struct {
	OutputTokens int `json:"output_tokens"`
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

// Adapter implements provider_sdk.ProviderAdapter for Anthropic.
type Adapter struct {
	httpClient *http.Client
	apiBase    string

	capabilitiesMu    sync.RWMutex
	capabilitiesCache map[string]provider_sdk.ModelCapabilities
}

// NewAdapter creates a new Anthropic adapter with the given API base URL.
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
	return "anthropic"
}

// Validate checks whether the source credential is valid by making a
// lightweight API call to list models.
func (a *Adapter) Validate(ctx context.Context, source provider_sdk.SourceCredential) provider_sdk.ValidationResult {
	apiKey, err := extractAPIKey(source.Credential)
	if err != nil {
		return provider_sdk.ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("failed to extract API key: %v", err),
			ErrorCode: "invalid_credential",
		}
	}

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
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
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

// DiscoverModels returns the models available from this provider.
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
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := make([]provider_sdk.ProviderModel, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, provider_sdk.ProviderModel{
			ID:           provider_sdk.ModelID(m.ID),
			ProviderID:   "anthropic",
			DisplayName:  m.DisplayName,
			Modality:     provider_sdk.ModalityText,
			Capabilities: a.Capabilities(m.ID),
		})
	}
	return models
}

// Capabilities returns the capability matrix for the named model.
func (a *Adapter) Capabilities(model string) provider_sdk.ModelCapabilities {
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

// Estimate returns a pre-flight cost estimate for a canonical request.
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

// Invoke executes a synchronous (non-streaming) request and returns the
// complete response along with usage.
func (a *Adapter) Invoke(ctx context.Context, req provider_sdk.CanonicalRequest, source provider_sdk.ResolvedSource) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
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

	return a.messagesCall(ctx, req, apiKey)
}

// Stream executes a streaming request, emitting each chunk via the EventSink.
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

	return a.streamMessages(ctx, req, apiKey, emit)
}

// NormalizeError converts a raw upstream error into a structured ProviderError.
func (a *Adapter) NormalizeError(err error) provider_sdk.ProviderError {
	if err == nil {
		return provider_sdk.ProviderError{
			Code:    "unknown",
			Message: "nil error",
		}
	}

	// If already a ProviderError, return it.
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

	// Attempt to parse as an Anthropic API error.
	if anthErr := parseAnthropicError(err); anthErr != nil {
		return *anthErr
	}

	return provider_sdk.ProviderError{
		Code:       "anthropic_error",
		Message:    err.Error(),
		Retryable:  true,
		StatusCode: 0,
		Raw:        err,
	}
}

// Health performs a health check against the upstream.
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
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
// Request conversion: CanonicalRequest → Anthropic Messages API JSON
// ---------------------------------------------------------------------------

// convertRequest converts a CanonicalRequest into an Anthropic Messages API
// request body.
func convertRequest(req provider_sdk.CanonicalRequest) ([]byte, error) {
	// Extract system message and convert remaining messages.
	var systemContent string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Anthropic puts system prompt as a top-level field.
			systemContent = extractStringContent(msg.Content)
			continue
		}

		// Convert message content to Anthropic content blocks.
		content := convertMessageContent(msg)
		if len(content) > 0 {
			messages = append(messages, anthropicMessage{
				Role:    convertRole(msg.Role),
				Content: content,
			})
		}
	}

	// Determine max_tokens.
	maxTokens := req.GenerationParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	ar := anthropicRequest{
		Model:        string(req.RequestedModel),
		Messages:     messages,
		MaxTokens:    maxTokens,
		System:       systemContent,
		Stream:       false,
		StopSequences: req.GenerationParams.Stop,
	}

	if req.GenerationParams.Temperature > 0 {
		ar.Temperature = &req.GenerationParams.Temperature
	}
	if req.GenerationParams.TopP > 0 {
		ar.TopP = &req.GenerationParams.TopP
	}
	if req.GenerationParams.TopK > 0 {
		ar.TopK = &req.GenerationParams.TopK
	}

	// Convert tools.
	if len(req.Tools) > 0 {
		ar.Tools = convertTools(req.Tools)
	}

	// Convert tool_choice.
	if len(req.ToolChoice) > 0 {
		var tc interface{}
		if err := json.Unmarshal(req.ToolChoice, &tc); err == nil {
			ar.ToolChoice = tc
		}
	}

	// Metadata for idempotency and privacy.
	if req.IdempotencyKey != "" || req.PrivacyPolicy.DisableTraining {
		ar.Metadata = make(map[string]string)
		if req.IdempotencyKey != "" {
			ar.Metadata["idempotency_key"] = req.IdempotencyKey
		}
	}

	return json.Marshal(ar)
}

// convertMessageContent converts a Message's Content (json.RawMessage, which
// could be a plain string or a multimodal array) into an Anthropic content
// block array.
func convertMessageContent(msg provider_sdk.Message) []anthropicContent {
	// Try to parse as a plain string first.
	var textStr string
	if err := json.Unmarshal(msg.Content, &textStr); err == nil {
		return []anthropicContent{{Type: "text", Text: textStr}}
	}

	// Try to parse as a multimodal content array.
	var multiContent []map[string]json.RawMessage
	if err := json.Unmarshal(msg.Content, &multiContent); err != nil {
		// Fallback: treat as string.
		return []anthropicContent{{Type: "text", Text: string(msg.Content)}}
	}

	var blocks []anthropicContent
	for _, item := range multiContent {
		typeStr, _ := extractFieldString(item, "type")

		switch typeStr {
		case "text":
			text, _ := extractFieldString(item, "text")
			blocks = append(blocks, anthropicContent{Type: "text", Text: text})
		case "image_url":
			// Extract image from image_url format.
			if imgURL, ok := item["image_url"]; ok {
				url, _ := extractFieldString(item, "image_url")
				_ = url
				// Try to get base64 data from the image_url object.
				var imgObj map[string]json.RawMessage
				if json.Unmarshal(imgURL, &imgObj) == nil {
					u, _ := extractFieldString(imgObj, "url")
					if strings.HasPrefix(u, "data:") {
						// data:image/jpeg;base64,<data>
						mediaType, b64Data := parseDataURL(u)
						if b64Data != "" {
							blocks = append(blocks, anthropicContent{
								Type: "image",
								Source: &anthropicSource{
									Type:      "base64",
									MediaType: mediaType,
									Data:      b64Data,
								},
							})
						}
					}
				}
			}
		case "image":
			// Direct image block.
			sourceData, hasSource := item["source"]
			if hasSource {
				var src anthropicSource
				if json.Unmarshal(sourceData, &src) == nil {
					blocks = append(blocks, anthropicContent{
						Type:   "image",
						Source: &src,
					})
				}
			}
		case "tool_use":
			var ac anthropicContent
			if err := json.Unmarshal(flattenRawMessage(item), &ac); err == nil {
				ac.Type = "tool_use"
				blocks = append(blocks, ac)
			}
		case "tool_result":
			toolUseID, _ := extractFieldString(item, "tool_use_id")
			contentRaw := item["content"]
			blocks = append(blocks, anthropicContent{
				Type:      "tool_result",
				ToolUseID: toolUseID,
				Content:   contentRaw,
			})
		default:
			// Unknown type; skip or treat as text.
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, anthropicContent{Type: "text", Text: string(msg.Content)})
	}
	return blocks
}

// convertRole converts a canonical role to an Anthropic role.
func convertRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "user":
		return "user"
	default:
		return "user"
	}
}

// convertTools converts canonical tool definitions to Anthropic tool format.
func convertTools(tools []provider_sdk.ToolDefinition) []anthropicTool {
	result := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		var funcDef struct {
			Name        string          `json:"name"`
			Description string          `json:"description,omitempty"`
			Parameters  json.RawMessage `json:"parameters,omitempty"`
		}
		if err := json.Unmarshal(t.Function, &funcDef); err != nil {
			continue
		}
		result = append(result, anthropicTool{
			Name:        funcDef.Name,
			Description: funcDef.Description,
			InputSchema: funcDef.Parameters,
		})
	}
	return result
}

// ---------------------------------------------------------------------------
// Response conversion: Anthropic JSON → CanonicalResponse
// ---------------------------------------------------------------------------

// convertResponse converts an Anthropic Messages API response body into a
// CanonicalResponse.
func convertResponse(body []byte, model string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	var ar anthropicResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	responseID := ar.ID
	if responseID == "" {
		responseID = uuid.New().String()
	}

	// Build a message from the content blocks.
	var textContent string
	for _, block := range ar.Content {
		if block.Type == "text" {
			textContent += block.Text
		}
	}

	choice := provider_sdk.ResponseChoice{
		Index: 0,
		Message: provider_sdk.Message{
			Role:    "assistant",
			Content: mustMarshalJSON(textContent),
		},
		FinishReason: mapFinishReason(ar.StopReason),
	}

	usage := provider_sdk.Usage{
		InputTokens:     ar.Usage.InputTokens,
		OutputTokens:    ar.Usage.OutputTokens,
		CacheReadTokens: ar.Usage.CacheReadInputTokens,
		CacheWriteTokens: ar.Usage.CacheCreationInputTokens,
		TotalTokens:     ar.Usage.InputTokens + ar.Usage.OutputTokens,
	}

	return provider_sdk.CanonicalResponse{
		ID:           responseID,
		Model:        provider_sdk.ModelID(model),
		Choices:      []provider_sdk.ResponseChoice{choice},
		Usage:        usage,
		FinishReason: mapFinishReason(ar.StopReason),
	}, usage, nil
}

// mapFinishReason maps Anthropic stop_reason to canonical finish reasons.
func mapFinishReason(anthropicReason string) string {
	switch anthropicReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}

// ---------------------------------------------------------------------------
// Synchronous invocation
// ---------------------------------------------------------------------------

// messagesCall sends a non-streaming request to the Anthropic Messages API.
func (a *Adapter) messagesCall(ctx context.Context, req provider_sdk.CanonicalRequest, apiKey string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
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

	respBody, statusCode, err := a.doAnthropicRequest(ctx, http.MethodPost, a.apiBase+"/messages", body, apiKey)
	if err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{}, err
	}
	_ = statusCode

	return convertResponse(respBody, string(req.RequestedModel))
}

// ---------------------------------------------------------------------------
// Streaming invocation
// ---------------------------------------------------------------------------

// streamMessages sends a streaming request to the Anthropic Messages API and
// emits events via the EventSink.
func (a *Adapter) streamMessages(
	ctx context.Context,
	req provider_sdk.CanonicalRequest,
	apiKey string,
	emit provider_sdk.EventSink,
) (provider_sdk.Usage, error) {
	body, err := convertRequest(req)
	if err != nil {
		return provider_sdk.Usage{},
			&provider_sdk.ProviderError{
				Code:    "request_conversion_error",
				Message: fmt.Sprintf("failed to convert request: %v", err),
				Fatal:   true,
				Raw:     err,
			}
	}

	// Patch stream:false → stream:true.
	body = bytes.Replace(body, []byte(`"stream":false`), []byte(`"stream":true`), 1)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.apiBase+"/messages", bytes.NewReader(body))
	if err != nil {
		return provider_sdk.Usage{}, err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return provider_sdk.Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		pe := parseAnthropicErrorBody(resp.StatusCode, respBody)
		return provider_sdk.Usage{}, pe
	}

	return a.readAnthropicSSEStream(ctx, resp.Body, string(req.RequestedModel), emit)
}

// readAnthropicSSEStream reads Anthropic SSE events and converts them to StreamEvents.
func (a *Adapter) readAnthropicSSEStream(
	ctx context.Context,
	body io.Reader,
	modelID string,
	emit provider_sdk.EventSink,
) (provider_sdk.Usage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		currentEvent string
		totalUsage   provider_sdk.Usage
		inputTokens  int
	)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	lines := make(chan string, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(lines)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- scanner.Text():
			}
		}
		if err := scanner.Err(); err != nil {
			errs <- err
		}
		close(errs)
	}()

	for {
		select {
		case <-ctx.Done():
			return totalUsage, ctx.Err()

		case err := <-errs:
			if err != nil {
				return totalUsage, fmt.Errorf("sse read error: %w", err)
			}
			return totalUsage, nil

		case line, ok := <-lines:
			if !ok {
				select {
				case err := <-errs:
					if err != nil {
						return totalUsage, fmt.Errorf("sse read error: %w", err)
					}
				default:
				}
				return totalUsage, nil
			}

			trimmed := strings.TrimSpace(line)

			// Empty line — process the accumulated event.
			if trimmed == "" && currentEvent != "" {
				currentEvent = ""
				continue
			}

			// event: line
			if strings.HasPrefix(trimmed, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
				continue
			}

			// data: line
			if strings.HasPrefix(trimmed, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))

				switch currentEvent {
				case "message_start":
					var ms anthropicMessageStart
					if err := json.Unmarshal([]byte(data), &ms); err == nil {
						if ms.Message.Usage != nil {
							inputTokens = ms.Message.Usage.InputTokens
						}
						// Emit a start event.
						startData, _ := json.Marshal(map[string]interface{}{
							"id":    ms.Message.ID,
							"model": ms.Message.Model,
							"role":  ms.Message.Role,
						})
						_ = emit(provider_sdk.StreamEvent{
							EventType: "delta",
							Data:      startData,
						})
					}

				case "content_block_delta":
					var delta anthropicContentBlockDelta
					if err := json.Unmarshal([]byte(data), &delta); err != nil {
						continue
					}

					// Try to extract text from the delta.
					var textDelta anthropicTextDelta
					if err := json.Unmarshal(delta.Delta, &textDelta); err == nil && textDelta.Type == "text_delta" {
						chunkData, _ := json.Marshal(map[string]interface{}{
							"index":   delta.Index,
							"content": textDelta.Text,
						})
						_ = emit(provider_sdk.StreamEvent{
							EventType: "delta",
							Data:      chunkData,
						})
					}

				case "message_delta":
					var md anthropicMessageDelta
					if err := json.Unmarshal([]byte(data), &md); err != nil {
						continue
					}
					totalUsage = provider_sdk.Usage{
						InputTokens:  inputTokens,
						OutputTokens: md.Usage.OutputTokens,
						TotalTokens:  inputTokens + md.Usage.OutputTokens,
					}

					// Emit done event with finish reason.
					doneData, _ := json.Marshal(map[string]interface{}{
						"status":        "done",
						"stop_reason":   md.Delta.StopReason,
						"output_tokens": md.Usage.OutputTokens,
						"input_tokens":  inputTokens,
					})
					_ = emit(provider_sdk.StreamEvent{
						EventType: "done",
						Data:      doneData,
					})

				default:
					// Unknown event type; emit as raw.
					_ = emit(provider_sdk.StreamEvent{
						EventType: "delta",
						Data:      json.RawMessage(data),
					})
				}
			}

		case <-heartbeat.C:
			_ = emit(provider_sdk.StreamEvent{
				EventType: "heartbeat",
				Data:      json.RawMessage(`": heartbeat"`),
			})
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// doAnthropicRequest sends an HTTP request with Anthropic-specific headers.
func (a *Adapter) doAnthropicRequest(ctx context.Context, method, url string, body []byte, apiKey string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
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
		return respBody, resp.StatusCode, parseAnthropicErrorBody(resp.StatusCode, respBody)
	}

	return respBody, resp.StatusCode, nil
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

// anthropicError wraps an Anthropic API error.
type anthropicError struct {
	StatusCode int
	ErrorType  string
	Message    string
	Raw        []byte
}

func (e *anthropicError) Error() string {
	return fmt.Sprintf("anthropic http %d: [%s] %s", e.StatusCode, e.ErrorType, e.Message)
}

// parseAnthropicErrorBody parses an Anthropic error response body.
func parseAnthropicErrorBody(statusCode int, body []byte) *provider_sdk.ProviderError {
	var ar anthropicErrorResponse
	if json.Unmarshal(body, &ar) == nil && ar.Error.Message != "" {
		code := mapAnthropicErrorCode(statusCode, ar.Error.Type)
		return &provider_sdk.ProviderError{
			Code:       code,
			Message:    ar.Error.Message,
			StatusCode: statusCode,
			Retryable:  isRetryableStatus(statusCode),
			Fatal:      statusCode == 401 || statusCode == 403,
			Raw: &anthropicError{
				StatusCode: statusCode,
				ErrorType:  ar.Error.Type,
				Message:    ar.Error.Message,
				Raw:        body,
			},
		}
	}

	// Fallback: treat as generic error.
	return &provider_sdk.ProviderError{
		Code:       fmt.Sprintf("http_%d", statusCode),
		Message:    string(body),
		StatusCode: statusCode,
		Retryable:  isRetryableStatus(statusCode),
	}
}

// parseAnthropicError attempts to parse a raw error as an Anthropic API error.
func parseAnthropicError(err error) *provider_sdk.ProviderError {
	var ae *anthropicError
	if AsAnthropicError(err, &ae) {
		return &provider_sdk.ProviderError{
			Code:       ae.ErrorType,
			Message:    ae.Message,
			StatusCode: ae.StatusCode,
			Retryable:  isRetryableStatus(ae.StatusCode),
			Fatal:      ae.StatusCode == 401 || ae.StatusCode == 403,
			Raw:        err,
		}
	}
	return nil
}

// mapAnthropicErrorCode maps Anthropic error types to gateway error codes.
func mapAnthropicErrorCode(statusCode int, errType string) string {
	switch errType {
	case "invalid_request_error":
		return "invalid_request"
	case "authentication_error":
		return "invalid_api_key"
	case "permission_error":
		return "permission_denied"
	case "not_found_error":
		return "not_found"
	case "rate_limit_error":
		return "rate_limit"
	case "api_error":
		return "provider_unavailable"
	case "overloaded_error":
		return "provider_overloaded"
	default:
		if statusCode == 429 {
			return "rate_limit"
		}
		if statusCode >= 500 {
			return "provider_unavailable"
		}
		return fmt.Sprintf("http_%d", statusCode)
	}
}

// AsAnthropicError checks if err is an *anthropicError and extracts it.
func AsAnthropicError(err error, target **anthropicError) bool {
	if t, ok := err.(*anthropicError); ok {
		*target = t
		return true
	}
	return false
}

// AsProviderError checks if err is a *provider_sdk.ProviderError and extracts it.
func AsProviderError(err error, target **provider_sdk.ProviderError) bool {
	if t, ok := err.(*provider_sdk.ProviderError); ok {
		*target = t
		return true
	}
	return false
}

// isRetryableStatus returns true for HTTP status codes that indicate a
// transient error worth retrying.
func isRetryableStatus(status int) bool {
	return status == 429 || status == 500 || status == 502 || status == 503 || status == 504
}

// ---------------------------------------------------------------------------
// Model capabilities
// ---------------------------------------------------------------------------

// resolveCapabilities returns the capability matrix for a given model name.
func (a *Adapter) resolveCapabilities(model string) provider_sdk.ModelCapabilities {
	lower := strings.ToLower(model)

	// Claude Opus 4
	if strings.Contains(lower, "opus") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportUnsupported,
			SupportsVideoInput:   provider_sdk.SupportUnsupported,
			SupportsJSONMode:     provider_sdk.SupportBestEffort,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportNative,
			MaxContextTokens:     200000,
			MaxOutputTokens:      32000,
		}
	}

	// Claude Sonnet 4
	if strings.Contains(lower, "sonnet") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportUnsupported,
			SupportsVideoInput:   provider_sdk.SupportUnsupported,
			SupportsJSONMode:     provider_sdk.SupportBestEffort,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportNative,
			MaxContextTokens:     200000,
			MaxOutputTokens:      16384,
		}
	}

	// Claude Haiku
	if strings.Contains(lower, "haiku") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportUnsupported,
			SupportsAudioInput:   provider_sdk.SupportUnsupported,
			SupportsVideoInput:   provider_sdk.SupportUnsupported,
			SupportsJSONMode:     provider_sdk.SupportBestEffort,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportUnsupported,
			MaxContextTokens:     200000,
			MaxOutputTokens:      8192,
		}
	}

	// Default for unknown Anthropic models.
	return provider_sdk.ModelCapabilities{
		SupportsStreaming:    provider_sdk.SupportNative,
		SupportsToolUse:      provider_sdk.SupportNative,
		SupportsImageInput:   provider_sdk.SupportNative,
		SupportsAudioInput:   provider_sdk.SupportUnsupported,
		SupportsVideoInput:   provider_sdk.SupportUnsupported,
		SupportsJSONMode:     provider_sdk.SupportBestEffort,
		SupportsSystemPrompt: provider_sdk.SupportNative,
		SupportsMultiTurn:    provider_sdk.SupportNative,
		SupportsReasoning:    provider_sdk.SupportNative,
		MaxContextTokens:     200000,
		MaxOutputTokens:      8192,
	}
}

// modelPricing returns the pricing for a given Anthropic model.
func modelPricing(model string) provider_sdk.ModelPricing {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "opus"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  15000, // $15.00/1M input tokens
			OutputPriceMicroUSD: 75000, // $75.00/1M output tokens
			Currency:            "USD",
		}
	case strings.Contains(lower, "sonnet"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  3000,  // $3.00/1M input tokens
			OutputPriceMicroUSD: 15000, // $15.00/1M output tokens
			Currency:            "USD",
		}
	case strings.Contains(lower, "haiku"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  250,  // $0.25/1M input tokens
			OutputPriceMicroUSD: 1250, // $1.25/1M output tokens
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

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

// extractAPIKey extracts the API key from a json.RawMessage credential.
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

// extractStringContent extracts a plain string from json.RawMessage content.
func extractStringContent(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return string(raw)
}

// extractFieldString extracts a string field from a map.
func extractFieldString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", fmt.Errorf("key %q not found", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

// parseDataURL parses a data URL (data:image/jpeg;base64,<data>) into media
// type and base64 data.
func parseDataURL(dataURL string) (mediaType, data string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", dataURL
	}
	// data:image/jpeg;base64,ABC123
	parts := strings.SplitN(dataURL[5:], ",", 2)
	if len(parts) != 2 {
		return "", dataURL
	}
	meta := parts[0]
	// Strip ";base64" suffix.
	mediaType = strings.TrimSuffix(meta, ";base64")
	return mediaType, parts[1]
}

// flattenRawMessage converts a map[string]json.RawMessage to json.RawMessage.
func flattenRawMessage(m map[string]json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(m)
	return b
}

// mustMarshalJSON marshals a value to json.RawMessage, returning empty on
// error.
func mustMarshalJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}

// estimateInputTokens provides a rough estimate of input tokens from the
// request.
func estimateInputTokens(req provider_sdk.CanonicalRequest) int {
	total := 0
	for _, msg := range req.Messages {
		// Each content character is ~4 characters per token.
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
	return 256
}
