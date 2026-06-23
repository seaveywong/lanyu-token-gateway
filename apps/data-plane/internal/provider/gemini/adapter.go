// Package gemini implements the ProviderAdapter for the Google Gemini API.
// It translates CanonicalRequest into Gemini-compatible HTTP calls and
// normalises responses back into the gateway's canonical types.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	provider_sdk "github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
)

// DefaultAPIBase is the default Gemini API base URL.
const DefaultAPIBase = "https://generativelanguage.googleapis.com/v1beta"

// ---------------------------------------------------------------------------
// Gemini wire types
// ---------------------------------------------------------------------------

// geminiRequest is the JSON body sent to POST /models/{model}:generateContent.
type geminiRequest struct {
	Contents          []geminiContent     `json:"contents"`
	SystemInstruction *geminiContent      `json:"systemInstruction,omitempty"`
	Tools             []geminiTool        `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig   `json:"toolConfig,omitempty"`
	GenerationConfig  *geminiGenConfig    `json:"generationConfig,omitempty"`
	SafetySettings    []geminiSafetySetting `json:"safetySettings,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"` // "user" or "model"
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string           `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
	FileData   *geminiFileData   `json:"fileData,omitempty"`
	FunctionCall *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

type geminiFileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

// geminiTool represents a tool definition for Gemini.
type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *geminiFuncCallConfig `json:"functionCallingConfig,omitempty"`
}

type geminiFuncCallConfig struct {
	Mode string `json:"mode"` // "AUTO", "ANY", "NONE"
}

type geminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
	ResponseSchema  json.RawMessage `json:"responseSchema,omitempty"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// geminiResponse is the JSON body returned by generateContent.
type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsageMeta  `json:"usageMetadata,omitempty"`
	ModelVersion  string            `json:"modelVersion,omitempty"`
}

type geminiCandidate struct {
	Content       geminiContent       `json:"content"`
	FinishReason  string              `json:"finishReason,omitempty"`
	SafetyRatings []geminiSafetyRating `json:"safetyRatings,omitempty"`
	Index         int                 `json:"index"`
}

type geminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type geminiUsageMeta struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// geminiErrorResponse is the error body returned by Gemini.
type geminiErrorResponse struct {
	Error geminiErrorDetail `json:"error"`
}

type geminiErrorDetail struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Status  string           `json:"status"`
	Details []json.RawMessage `json:"details,omitempty"`
}

// ---------------------------------------------------------------------------
// Gemini SSE streaming types
// ---------------------------------------------------------------------------

// geminiSSEChunk represents a single SSE chunk from the Gemini stream.
type geminiSSEChunk struct {
	Candidates    []geminiCandidate `json:"candidates,omitempty"`
	UsageMetadata *geminiUsageMeta  `json:"usageMetadata,omitempty"`
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

// Adapter implements provider_sdk.ProviderAdapter for Gemini.
type Adapter struct {
	httpClient *http.Client
	apiBase    string

	capabilitiesMu    sync.RWMutex
	capabilitiesCache map[string]provider_sdk.ModelCapabilities
}

// NewAdapter creates a new Gemini adapter with the given API base URL.
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
	return "gemini"
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

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.apiBase+"/models?key="+url.QueryEscape(apiKey), nil)
	if err != nil {
		return provider_sdk.ValidationResult{
			Valid:     false,
			Message:   fmt.Sprintf("failed to create validation request: %v", err),
			ErrorCode: "internal_error",
		}
	}
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

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.apiBase+"/models?key="+url.QueryEscape(apiKey), nil)
	if err != nil {
		return nil
	}
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
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := make([]provider_sdk.ProviderModel, 0, len(result.Models))
	for _, m := range result.Models {
		// Strip "models/" prefix from name.
		modelID := strings.TrimPrefix(m.Name, "models/")
		models = append(models, provider_sdk.ProviderModel{
			ID:           provider_sdk.ModelID(modelID),
			ProviderID:   "gemini",
			DisplayName:  m.DisplayName,
			Modality:     provider_sdk.ModalityMultimodal,
			Capabilities: a.Capabilities(modelID),
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

// Invoke executes a synchronous (non-streaming) request.
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

	return a.generateContent(ctx, req, apiKey)
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

	return a.streamGenerateContent(ctx, req, apiKey, emit)
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

	// Attempt to parse as a Gemini API error.
	if gErr := parseGeminiError(err); gErr != nil {
		return *gErr
	}

	return provider_sdk.ProviderError{
		Code:       "gemini_error",
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

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, a.apiBase+"/models?key="+url.QueryEscape(apiKey), nil)
	if err != nil {
		return provider_sdk.HealthResult{
			Healthy:      false,
			ErrorMessage: fmt.Sprintf("failed to create health check request: %v", err),
			LastChecked:  time.Now(),
		}
	}
	req.Header.Set("Content-Type", "application/json")

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
// Request conversion: CanonicalRequest → Gemini generateContent JSON
// ---------------------------------------------------------------------------

// convertRequest converts a CanonicalRequest into a Gemini generateContent
// request body.
func convertRequest(req provider_sdk.CanonicalRequest) ([]byte, error) {
	// Split messages: system instruction vs conversation turns.
	var systemInstruction *geminiContent
	contents := make([]geminiContent, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			parts := convertMessageContentToParts(msg.Content)
			systemInstruction = &geminiContent{
				Parts: parts,
			}
			continue
		}

		gc := geminiContent{
			Role:  convertRole(msg.Role),
			Parts: convertMessageContentToParts(msg.Content),
		}
		contents = append(contents, gc)
	}

	// Build generation config.
	maxTokens := req.GenerationParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	genConfig := &geminiGenConfig{
		MaxOutputTokens: &maxTokens,
		StopSequences:   req.GenerationParams.Stop,
	}

	if req.GenerationParams.Temperature > 0 {
		genConfig.Temperature = &req.GenerationParams.Temperature
	}
	if req.GenerationParams.TopP > 0 {
		genConfig.TopP = &req.GenerationParams.TopP
	}
	if req.GenerationParams.TopK > 0 {
		genConfig.TopK = &req.GenerationParams.TopK
	}

	// Response format (JSON mode).
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		genConfig.ResponseMimeType = "application/json"
	}
	if req.ResponseFormat != nil && len(req.ResponseFormat.JSONSchema) > 0 {
		genConfig.ResponseMimeType = "application/json"
		genConfig.ResponseSchema = req.ResponseFormat.JSONSchema
	}

	gr := geminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		GenerationConfig:  genConfig,
	}

	// Convert tools.
	if len(req.Tools) > 0 {
		gr.Tools = convertTools(req.Tools)
	}

	// Convert tool_choice.
	if len(req.ToolChoice) > 0 {
		gr.ToolConfig = convertToolChoice(req.ToolChoice)
	}

	body, err := json.Marshal(gr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	// Remove empty arrays/objects that might cause Gemini validation errors.
	// Gemini is strict about empty contents, tools, etc.
	body = cleanEmptyArrays(body)

	return body, nil
}

// convertMessageContentToParts converts a Message's Content (json.RawMessage)
// into a slice of geminiPart.
func convertMessageContentToParts(content json.RawMessage) []geminiPart {
	// Try plain string first.
	var textStr string
	if err := json.Unmarshal(content, &textStr); err == nil {
		return []geminiPart{{Text: textStr}}
	}

	// Try multimodal content array.
	var multiContent []map[string]json.RawMessage
	if err := json.Unmarshal(content, &multiContent); err != nil {
		return []geminiPart{{Text: string(content)}}
	}

	var parts []geminiPart
	for _, item := range multiContent {
		typeVal, _ := extractFieldString(item, "type")

		switch typeVal {
		case "text":
			text, _ := extractFieldString(item, "text")
			parts = append(parts, geminiPart{Text: text})
		case "image_url":
			if imgData, ok := item["image_url"]; ok {
				var imgObj struct {
					URL    string `json:"url"`
					Detail string `json:"detail,omitempty"`
				}
				if json.Unmarshal(imgData, &imgObj) == nil {
					if strings.HasPrefix(imgObj.URL, "data:") {
						mediaType, b64 := parseDataURL(imgObj.URL)
						if b64 != "" {
							parts = append(parts, geminiPart{
								InlineData: &geminiInlineData{
									MimeType: mediaType,
									Data:     b64,
								},
							})
						}
					}
				}
			}
		case "image":
			// Direct anthropic-style source.
			if srcData, ok := item["source"]; ok {
				var src struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				}
				if json.Unmarshal(srcData, &src) == nil && src.Type == "base64" {
					parts = append(parts, geminiPart{
						InlineData: &geminiInlineData{
							MimeType: src.MediaType,
							Data:     src.Data,
						},
					})
				}
			}
		case "input_audio":
			if audioData, ok := item["input_audio"]; ok {
				var aud struct {
					Data   string `json:"data"`
					Format string `json:"format"`
				}
				if json.Unmarshal(audioData, &aud) == nil {
					parts = append(parts, geminiPart{
						InlineData: &geminiInlineData{
							MimeType: "audio/" + aud.Format,
							Data:     aud.Data,
						},
					})
				}
			}
		default:
			// Unknown type, skip.
		}
	}

	if len(parts) == 0 {
		parts = append(parts, geminiPart{Text: string(content)})
	}
	return parts
}

// cleanEmptyArrays removes empty JSON arrays/objects that Gemini might reject.
// This is a simple pass: replace "contents":[] with removing the contents key,
// etc.
func cleanEmptyArrays(body []byte) []byte {
	// Simple replacements for known problematic patterns.
	body = bytes.Replace(body, []byte(`"contents":[]`), []byte(`"contents":null`), 1)
	body = bytes.Replace(body, []byte(`"tools":[]`), []byte(`"tools":null`), 1)
	return body
}

// convertRole converts a canonical role to a Gemini role.
func convertRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "user":
		return "user"
	default:
		return "user"
	}
}

// convertTools converts canonical tool definitions to Gemini functionDeclarations.
func convertTools(tools []provider_sdk.ToolDefinition) []geminiTool {
	declarations := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		var funcDef struct {
			Name        string          `json:"name"`
			Description string          `json:"description,omitempty"`
			Parameters  json.RawMessage `json:"parameters,omitempty"`
		}
		if err := json.Unmarshal(t.Function, &funcDef); err != nil {
			continue
		}
		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        funcDef.Name,
			Description: funcDef.Description,
			Parameters:  funcDef.Parameters,
		})
	}

	if len(declarations) == 0 {
		return nil
	}
	return []geminiTool{{FunctionDeclarations: declarations}}
}

// convertToolChoice converts a canonical tool_choice to Gemini toolConfig.
func convertToolChoice(toolChoice json.RawMessage) *geminiToolConfig {
	if len(toolChoice) == 0 {
		return nil
	}

	// Try to parse as a string tool choice like "auto", "none", or {"type": "function", "function": {...}}.
	var choiceStr string
	if json.Unmarshal(toolChoice, &choiceStr) == nil {
		switch choiceStr {
		case "auto":
			return &geminiToolConfig{
				FunctionCallingConfig: &geminiFuncCallConfig{Mode: "AUTO"},
			}
		case "none":
			return &geminiToolConfig{
				FunctionCallingConfig: &geminiFuncCallConfig{Mode: "NONE"},
			}
		case "required":
			return &geminiToolConfig{
				FunctionCallingConfig: &geminiFuncCallConfig{Mode: "ANY"},
			}
		}
	}

	// Default to AUTO.
	return &geminiToolConfig{
		FunctionCallingConfig: &geminiFuncCallConfig{Mode: "AUTO"},
	}
}

// ---------------------------------------------------------------------------
// Response conversion: Gemini JSON → CanonicalResponse
// ---------------------------------------------------------------------------

// convertResponse converts a Gemini generateContent response body into a
// CanonicalResponse.
func convertResponse(body []byte, model string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	var gr geminiResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	responseID := uuid.New().String()

	choices := make([]provider_sdk.ResponseChoice, 0, len(gr.Candidates))
	var finishReason string

	for i, cand := range gr.Candidates {
		// Extract text from parts.
		var textBuilder strings.Builder
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				textBuilder.WriteString(part.Text)
			}
		}

		textContent := textBuilder.String()
		if textContent == "" {
			textContent = "" // fallback empty
		}

		choice := provider_sdk.ResponseChoice{
			Index: i,
			Message: provider_sdk.Message{
				Role:    "assistant",
				Content: mustMarshalJSON(textContent),
			},
			FinishReason: mapFinishReason(cand.FinishReason),
		}
		if cand.FinishReason != "" {
			finishReason = mapFinishReason(cand.FinishReason)
		}
		choices = append(choices, choice)
	}

	usage := provider_sdk.Usage{}
	if gr.UsageMetadata != nil {
		usage = provider_sdk.Usage{
			InputTokens:  gr.UsageMetadata.PromptTokenCount,
			OutputTokens: gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  gr.UsageMetadata.TotalTokenCount,
		}
	}

	return provider_sdk.CanonicalResponse{
		ID:           responseID,
		Model:        provider_sdk.ModelID(model),
		Choices:      choices,
		Usage:        usage,
		FinishReason: finishReason,
	}, usage, nil
}

// mapFinishReason maps Gemini finishReason to canonical finish reasons.
func mapFinishReason(geminiReason string) string {
	switch geminiReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "MALFORMED_FUNCTION_CALL":
		return "tool_calls"
	default:
		return "stop"
	}
}

// ---------------------------------------------------------------------------
// Synchronous invocation
// ---------------------------------------------------------------------------

// generateContent sends a non-streaming request to Gemini generateContent.
func (a *Adapter) generateContent(ctx context.Context, req provider_sdk.CanonicalRequest, apiKey string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
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

	modelID := string(req.RequestedModel)
	url := fmt.Sprintf("%s/models/%s:generateContent", a.apiBase, modelID)

	respBody, statusCode, err := a.doGeminiRequest(ctx, http.MethodPost, url, body, apiKey)
	if err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{}, err
	}
	_ = statusCode

	return convertResponse(respBody, modelID)
}

// ---------------------------------------------------------------------------
// Streaming invocation
// ---------------------------------------------------------------------------

// streamGenerateContent sends a streaming request to Gemini
// :streamGenerateContent endpoint.
func (a *Adapter) streamGenerateContent(
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

	modelID := string(req.RequestedModel)
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", a.apiBase, modelID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return provider_sdk.Usage{}, err
	}
	httpReq.Header.Set("x-goog-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return provider_sdk.Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		pe := parseGeminiErrorBody(resp.StatusCode, respBody)
		return provider_sdk.Usage{}, pe
	}

	return a.readGeminiSSEStream(ctx, resp.Body, modelID, emit)
}

// readGeminiSSEStream reads Gemini SSE stream and emits StreamEvents.
func (a *Adapter) readGeminiSSEStream(
	ctx context.Context,
	body io.Reader,
	modelID string,
	emit provider_sdk.EventSink,
) (provider_sdk.Usage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var totalUsage provider_sdk.Usage

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

			// Skip empty lines and comments.
			if trimmed == "" || strings.HasPrefix(trimmed, ":") {
				continue
			}

			// data: {...}
			if !strings.HasPrefix(trimmed, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))

			// Parse the chunk.
			var chunk geminiSSEChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			// Extract text from candidates.
			for _, cand := range chunk.Candidates {
				var textBuilder strings.Builder
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						textBuilder.WriteString(part.Text)
					}
				}
				textContent := textBuilder.String()
				if textContent != "" {
					chunkData, _ := json.Marshal(map[string]interface{}{
						"index":   cand.Index,
						"content": textContent,
					})
					_ = emit(provider_sdk.StreamEvent{
						EventType: "delta",
						Data:      chunkData,
					})
				}
			}

			// Collect usage.
			if chunk.UsageMetadata != nil {
				totalUsage = provider_sdk.Usage{
					InputTokens:  chunk.UsageMetadata.PromptTokenCount,
					OutputTokens: chunk.UsageMetadata.CandidatesTokenCount,
					TotalTokens:  chunk.UsageMetadata.TotalTokenCount,
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

// doGeminiRequest sends an HTTP request with Gemini-specific auth.
// Gemini uses API key as query param (?key=) or x-goog-api-key header.
func (a *Adapter) doGeminiRequest(ctx context.Context, method, urlStr string, body []byte, apiKey string) ([]byte, int, error) {
	// Append API key as query param.
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, 0, err
	}
	q := parsedURL.Query()
	q.Set("key", apiKey)
	parsedURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
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
		return respBody, resp.StatusCode, parseGeminiErrorBody(resp.StatusCode, respBody)
	}

	return respBody, resp.StatusCode, nil
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

// geminiError wraps a Gemini API error.
type geminiError struct {
	StatusCode int
	ErrorCode  int
	Message    string
	Status     string
	Raw        []byte
}

func (e *geminiError) Error() string {
	return fmt.Sprintf("gemini http %d: [%d] %s", e.StatusCode, e.ErrorCode, e.Message)
}

// parseGeminiErrorBody parses a Gemini error response body.
func parseGeminiErrorBody(statusCode int, body []byte) *provider_sdk.ProviderError {
	var ger geminiErrorResponse
	if json.Unmarshal(body, &ger) == nil && ger.Error.Message != "" {
		code := mapGeminiErrorCode(statusCode, ger.Error.Status)
		return &provider_sdk.ProviderError{
			Code:       code,
			Message:    ger.Error.Message,
			StatusCode: statusCode,
			Retryable:  isRetryableStatus(statusCode),
			Fatal:      statusCode == 401 || statusCode == 403,
			Raw: &geminiError{
				StatusCode: statusCode,
				ErrorCode:  ger.Error.Code,
				Message:    ger.Error.Message,
				Status:     ger.Error.Status,
				Raw:        body,
			},
		}
	}

	return &provider_sdk.ProviderError{
		Code:       fmt.Sprintf("http_%d", statusCode),
		Message:    string(body),
		StatusCode: statusCode,
		Retryable:  isRetryableStatus(statusCode),
	}
}

// parseGeminiError attempts to parse a raw error as a Gemini API error.
func parseGeminiError(err error) *provider_sdk.ProviderError {
	var ge *geminiError
	if AsGeminiError(err, &ge) {
		return &provider_sdk.ProviderError{
			Code:       ge.Status,
			Message:    ge.Message,
			StatusCode: ge.StatusCode,
			Retryable:  isRetryableStatus(ge.StatusCode),
			Fatal:      ge.StatusCode == 401 || ge.StatusCode == 403,
			Raw:        err,
		}
	}
	return nil
}

// mapGeminiErrorCode maps Gemini error status strings to gateway error codes.
func mapGeminiErrorCode(statusCode int, status string) string {
	switch status {
	case "INVALID_ARGUMENT":
		return "invalid_request"
	case "PERMISSION_DENIED":
		return "invalid_api_key"
	case "UNAUTHENTICATED":
		return "invalid_api_key"
	case "RESOURCE_EXHAUSTED":
		return "rate_limit"
	case "UNAVAILABLE":
		return "provider_unavailable"
	case "INTERNAL":
		return "provider_unavailable"
	case "NOT_FOUND":
		return "not_found"
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

// AsGeminiError checks if err is a *geminiError and extracts it.
func AsGeminiError(err error, target **geminiError) bool {
	if t, ok := err.(*geminiError); ok {
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

	// Gemini 2.5 Pro
	if strings.Contains(lower, "gemini-2.5-pro") || strings.Contains(lower, "gemini-2.5-pro") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportNative,
			SupportsVideoInput:   provider_sdk.SupportNative,
			SupportsJSONMode:     provider_sdk.SupportNative,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportBestEffort,
			MaxContextTokens:     1048576,
			MaxOutputTokens:      65536,
		}
	}

	// Gemini 2.5 Flash
	if strings.Contains(lower, "gemini-2.5-flash") || strings.Contains(lower, "gemini-2.5-flash") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportNative,
			SupportsVideoInput:   provider_sdk.SupportNative,
			SupportsJSONMode:     provider_sdk.SupportNative,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportBestEffort,
			MaxContextTokens:     1048576,
			MaxOutputTokens:      8192,
		}
	}

	// Gemini 2.0 Flash
	if strings.Contains(lower, "gemini-2.0") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportNative,
			SupportsVideoInput:   provider_sdk.SupportBestEffort,
			SupportsJSONMode:     provider_sdk.SupportBestEffort,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportBestEffort,
			MaxContextTokens:     1048576,
			MaxOutputTokens:      8192,
		}
	}

	// Gemini 1.5 Pro
	if strings.Contains(lower, "gemini-1.5-pro") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportNative,
			SupportsVideoInput:   provider_sdk.SupportNative,
			SupportsJSONMode:     provider_sdk.SupportBestEffort,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportUnsupported,
			MaxContextTokens:     2097152,
			MaxOutputTokens:      8192,
		}
	}

	// Gemini 1.5 Flash
	if strings.Contains(lower, "gemini-1.5-flash") {
		return provider_sdk.ModelCapabilities{
			SupportsStreaming:    provider_sdk.SupportNative,
			SupportsToolUse:      provider_sdk.SupportNative,
			SupportsImageInput:   provider_sdk.SupportNative,
			SupportsAudioInput:   provider_sdk.SupportNative,
			SupportsVideoInput:   provider_sdk.SupportBestEffort,
			SupportsJSONMode:     provider_sdk.SupportBestEffort,
			SupportsSystemPrompt: provider_sdk.SupportNative,
			SupportsMultiTurn:    provider_sdk.SupportNative,
			SupportsReasoning:    provider_sdk.SupportUnsupported,
			MaxContextTokens:     1048576,
			MaxOutputTokens:      8192,
		}
	}

	// Default Gemini capabilities.
	return provider_sdk.ModelCapabilities{
		SupportsStreaming:    provider_sdk.SupportNative,
		SupportsToolUse:      provider_sdk.SupportNative,
		SupportsImageInput:   provider_sdk.SupportNative,
		SupportsAudioInput:   provider_sdk.SupportNative,
		SupportsVideoInput:   provider_sdk.SupportBestEffort,
		SupportsJSONMode:     provider_sdk.SupportBestEffort,
		SupportsSystemPrompt: provider_sdk.SupportNative,
		SupportsMultiTurn:    provider_sdk.SupportNative,
		SupportsReasoning:    provider_sdk.SupportBestEffort,
		MaxContextTokens:     1048576,
		MaxOutputTokens:      8192,
	}
}

// modelPricing returns the pricing for a given Gemini model.
func modelPricing(model string) provider_sdk.ModelPricing {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "gemini-2.5-pro"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  1250, // $1.25/1M input tokens
			OutputPriceMicroUSD: 10000, // $10.00/1M output tokens
			Currency:            "USD",
		}
	case strings.Contains(lower, "gemini-2.5-flash"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  75,  // $0.075/1M input tokens
			OutputPriceMicroUSD: 300, // $0.30/1M output tokens
			Currency:            "USD",
		}
	case strings.Contains(lower, "gemini-2.0"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  150, // $0.15/1M input tokens
			OutputPriceMicroUSD: 600, // $0.60/1M output tokens
			Currency:            "USD",
		}
	case strings.Contains(lower, "gemini-1.5-pro"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  1250, // $1.25/1M input tokens
			OutputPriceMicroUSD: 5000, // $5.00/1M output tokens
			Currency:            "USD",
		}
	case strings.Contains(lower, "gemini-1.5-flash"):
		return provider_sdk.ModelPricing{
			InputPriceMicroUSD:  75,  // $0.075/1M input tokens
			OutputPriceMicroUSD: 300, // $0.30/1M output tokens
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

// parseDataURL parses a data URL into media type and base64 data.
func parseDataURL(dataURL string) (mediaType, data string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", dataURL
	}
	parts := strings.SplitN(dataURL[5:], ",", 2)
	if len(parts) != 2 {
		return "", dataURL
	}
	meta := parts[0]
	mediaType = strings.TrimSuffix(meta, ";base64")
	return mediaType, parts[1]
}

// mustMarshalJSON marshals a value to json.RawMessage.
func mustMarshalJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}

// estimateInputTokens provides a rough estimate of input tokens.
func estimateInputTokens(req provider_sdk.CanonicalRequest) int {
	total := 0
	for _, msg := range req.Messages {
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
