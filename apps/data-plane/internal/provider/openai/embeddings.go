package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/seaveywong/lanyu-token-gateway/packages/provider_sdk"
)

// ---------------------------------------------------------------------------
// OpenAI Embeddings API wire types
// ---------------------------------------------------------------------------

// openAIEmbeddingRequest is the wire format sent to POST /v1/embeddings.
type openAIEmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	User           string   `json:"user,omitempty"`
}

// openAIEmbeddingResponse is the wire format returned by POST /v1/embeddings.
type openAIEmbeddingResponse struct {
	Object string                  `json:"object"`
	Data   []openAIEmbeddingData   `json:"data"`
	Model  string                  `json:"model"`
	Usage  openAIEmbeddingUsage    `json:"usage"`
}

// openAIEmbeddingData represents a single embedding vector.
type openAIEmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// openAIEmbeddingUsage captures token usage from the embeddings response.
type openAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ---------------------------------------------------------------------------
// Embedding conversion
// ---------------------------------------------------------------------------

// createEmbedding converts a CanonicalRequest into an OpenAI Embeddings API
// call and returns the canonical response with usage.
func (a *Adapter) createEmbedding(
	ctx context.Context,
	req provider_sdk.CanonicalRequest,
	apiKey string,
) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	// Extract the input text(s) from the request messages.
	inputs := extractEmbeddingInputs(req)

	embReq := openAIEmbeddingRequest{
		Model:          string(req.RequestedModel),
		Input:          inputs,
		EncodingFormat: "float",
		User:           req.TenantID,
	}

	body, err := json.Marshal(embReq)
	if err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	respBody, statusCode, err := a.doRequest(ctx, http.MethodPost,
		a.apiBase+"/embeddings", body, apiKey)
	if err != nil {
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

	return convertEmbeddingResponse(respBody, string(req.RequestedModel))
}

// convertEmbeddingResponse converts an OpenAI Embeddings API response into a
// CanonicalResponse.
func convertEmbeddingResponse(body []byte, model string) (provider_sdk.CanonicalResponse, provider_sdk.Usage, error) {
	var embResp openAIEmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return provider_sdk.CanonicalResponse{}, provider_sdk.Usage{},
			fmt.Errorf("failed to parse embedding response: %w", err)
	}

	responseID := uuid.New().String()

	// Embedding responses are represented as choices in the canonical model.
	choices := make([]provider_sdk.ResponseChoice, 0, len(embResp.Data))
	for _, d := range embResp.Data {
		embeddingJSON, _ := json.Marshal(d.Embedding)
		choices = append(choices, provider_sdk.ResponseChoice{
			Index: d.Index,
			Message: provider_sdk.Message{
				Role:    "assistant",
				Content: embeddingJSON,
			},
		})
	}

	usage := provider_sdk.Usage{
		InputTokens: embResp.Usage.PromptTokens,
		OutputTokens: 0, // embeddings have no output tokens in the LLM sense
		TotalTokens:  embResp.Usage.TotalTokens,
	}

	return provider_sdk.CanonicalResponse{
		ID:      responseID,
		Model:   provider_sdk.ModelID(model),
		Choices: choices,
		Usage:   usage,
	}, usage, nil
}

// extractEmbeddingInputs extracts the input strings from the canonical
// request messages for embedding generation.
func extractEmbeddingInputs(req provider_sdk.CanonicalRequest) []string {
	inputs := make([]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		// Attempt to unmarshal as a string.
		var text string
		if err := json.Unmarshal(msg.Content, &text); err == nil && text != "" {
			inputs = append(inputs, text)
			continue
		}

		// Attempt to unmarshal as a string array (batch embedding).
		var texts []string
		if err := json.Unmarshal(msg.Content, &texts); err == nil && len(texts) > 0 {
			inputs = append(inputs, texts...)
			continue
		}

		// Fallback: treat raw content as string.
		if len(msg.Content) > 0 {
			inputs = append(inputs, string(msg.Content))
		}
	}
	return inputs
}
