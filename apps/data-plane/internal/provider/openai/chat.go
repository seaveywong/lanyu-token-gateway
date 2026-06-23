package openai

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/seaveywong/lanyu-token-gateway/packages/provider_sdk"
)

// ---------------------------------------------------------------------------
// OpenAI wire types for Chat Completions
// ---------------------------------------------------------------------------

// openAIRequest is the wire format sent to POST /v1/chat/completions.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  interface{}     `json:"tool_choice,omitempty"`
	User        string          `json:"user,omitempty"`
}

// openAIMessage represents a single chat message on the OpenAI wire.
// Content is json.RawMessage to handle both plain strings and multimodal arrays.
type openAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// openAIToolCall represents a tool call made by the model.
type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

// openAIFunctionCall represents a function call within a tool call.
type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAITool represents a tool definition on the OpenAI wire.
type openAITool struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function,omitempty"`
}

// openAIResponse is the wire format returned by POST /v1/chat/completions.
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

// openAIChoice represents a single completion choice.
type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// openAIUsage captures token usage from the OpenAI response.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ---------------------------------------------------------------------------
// OpenAI error types
// ---------------------------------------------------------------------------

// openAIErrorResponse is the error body returned by OpenAI on failure.
type openAIErrorResponse struct {
	Error openAIErrorDetail `json:"error"`
}

// openAIErrorDetail contains the details of an OpenAI API error.
type openAIErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

// openAIError wraps an OpenAI API error response.
type openAIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
	Raw        []byte
}

func (e *openAIError) Error() string {
	return fmt.Sprintf("openai http %d: [%s] %s", e.StatusCode, e.ErrorCode, e.Message)
}

// AsOpenAIError checks if err is an *openAIError and extracts it.
func AsOpenAIError(err error, target **openAIError) bool {
	if t, ok := err.(*openAIError); ok {
		*target = t
		return true
	}
	return false
}

// newOpenAIError creates an openAIError from an HTTP status and response body.
func newOpenAIError(statusCode int, body []byte) *openAIError {
	oe := &openAIError{
		StatusCode: statusCode,
		Raw:        body,
	}

	var apiErr openAIErrorResponse
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
		oe.ErrorCode = apiErr.Error.Code
		if oe.ErrorCode == "" {
			oe.ErrorCode = apiErr.Error.Type
		}
		oe.Message = apiErr.Error.Message
	} else {
		oe.ErrorCode = fmt.Sprintf("http_%d", statusCode)
		oe.Message = string(body)
	}
	return oe
}

// parseOpenAIError attempts to parse a raw error as an OpenAI API error.
func parseOpenAIError(err error) *provider_sdk.ProviderError {
	var oaiErr *openAIError
	if !AsOpenAIError(err, &oaiErr) {
		return nil
	}

	return &provider_sdk.ProviderError{
		Code:       oaiErr.ErrorCode,
		Message:    oaiErr.Message,
		StatusCode: oaiErr.StatusCode,
		Retryable:  isRetryableStatus(oaiErr.StatusCode),
		Fatal:      oaiErr.StatusCode == 401 || oaiErr.StatusCode == 403,
		Raw:        err,
	}
}

// AsProviderError checks if err is a *provider_sdk.ProviderError and extracts it.
func AsProviderError(err error, target **provider_sdk.ProviderError) bool {
	if t, ok := err.(*provider_sdk.ProviderError); ok {
		*target = t
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Request conversion: CanonicalRequest → OpenAI JSON
// ---------------------------------------------------------------------------

// convertRequest converts a CanonicalRequest into an OpenAI Chat Completions
// API JSON body.
func convertRequest(req provider_sdk.CanonicalRequest) ([]byte, error) {
	messages := make([]openAIMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		om := openAIMessage{
			Role:       msg.Role,
			Content:    msg.Content, // pass through as-is (string or multimodal array)
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		}

		// Convert tool calls if present.
		if len(msg.ToolCalls) > 0 {
			var toolCalls []openAIToolCall
			if err := json.Unmarshal(msg.ToolCalls, &toolCalls); err == nil {
				om.ToolCalls = toolCalls
			}
		}

		messages = append(messages, om)
	}

	// Build tools array.
	tools := make([]openAITool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, openAITool{
			Type:     t.Type,
			Function: t.Function,
		})
	}

	// Build the OpenAI request body.
	oaiReq := openAIRequest{
		Model:    string(req.RequestedModel),
		Messages: messages,
		Stream:   false,
		User:     req.TenantID,
	}

	// Generation params.
	if req.GenerationParams.Temperature > 0 {
		oaiReq.Temperature = &req.GenerationParams.Temperature
	}
	if req.GenerationParams.TopP > 0 {
		oaiReq.TopP = &req.GenerationParams.TopP
	}
	if req.GenerationParams.MaxTokens > 0 {
		oaiReq.MaxTokens = &req.GenerationParams.MaxTokens
	}

	// Tools.
	if len(tools) > 0 {
		oaiReq.Tools = tools
	}

	// Tool choice.
	if len(req.ToolChoice) > 0 {
		var tc interface{}
		if err := json.Unmarshal(req.ToolChoice, &tc); err == nil {
			oaiReq.ToolChoice = tc
		}
	}

	return json.Marshal(oaiReq)
}

// ---------------------------------------------------------------------------
// Response conversion: OpenAI JSON → CanonicalResponse
// ---------------------------------------------------------------------------

// convertResponse converts an OpenAI Chat Completions API response body into
// a CanonicalResponse.
func convertResponse(body []byte, model string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	var oaiResp openAIResponse
	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	// Use the response ID or generate one.
	responseID := oaiResp.ID
	if responseID == "" {
		responseID = uuid.New().String()
	}

	choices := make([]provider_sdk.ResponseChoice, 0, len(oaiResp.Choices))
	var finishReason string

	for _, ch := range oaiResp.Choices {
		if ch.FinishReason != "" {
			finishReason = ch.FinishReason
		}

		// Serialize tool calls if present.
		var toolCallsRaw json.RawMessage
		if len(ch.Message.ToolCalls) > 0 {
			toolCallsRaw, _ = json.Marshal(ch.Message.ToolCalls)
		}

		choices = append(choices, provider_sdk.ResponseChoice{
			Index: ch.Index,
			Message: provider_sdk.Message{
				Role:       ch.Message.Role,
				Content:    ch.Message.Content,
				Name:       ch.Message.Name,
				ToolCalls:  toolCallsRaw,
				ToolCallID: ch.Message.ToolCallID,
			},
			FinishReason: ch.FinishReason,
		})
	}

	usage := provider_sdk.Usage{
		InputTokens:  oaiResp.Usage.PromptTokens,
		OutputTokens: oaiResp.Usage.CompletionTokens,
		TotalTokens:  oaiResp.Usage.TotalTokens,
	}

	return provider_sdk.CanonicalResponse{
		ID:           responseID,
		Model:        provider_sdk.ModelID(model),
		Choices:      choices,
		Usage:        usage,
		FinishReason: finishReason,
	}, usage, nil
}
