package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/packages/provider_sdk"
)

// ---------------------------------------------------------------------------
// OpenAI SSE streaming wire types
// ---------------------------------------------------------------------------

// openAISSEChunk is a single "data: {...}" line from the OpenAI SSE stream.
type openAISSEChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *openAIUsage         `json:"usage,omitempty"` // only in final chunk
}

// openAIStreamChoice represents a single choice delta in a streaming chunk.
type openAIStreamChoice struct {
	Index        int         `json:"index"`
	Delta        openAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// openAIDelta is the incremental content in a streaming chunk.
type openAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ---------------------------------------------------------------------------
// Streaming implementation
// ---------------------------------------------------------------------------

// streamChatCompletion handles SSE streaming from the OpenAI Chat Completions
// endpoint. It reads the SSE stream line by line, parses each "data: ..."
// chunk, converts to StreamEvent, and calls emit on each chunk.
// Returns the cumulative usage from the final chunk.
func (a *Adapter) streamChatCompletion(
	ctx context.Context,
	req provider_sdk.CanonicalRequest,
	apiKey string,
	emit provider_sdk.EventSink,
) (provider_sdk.Usage, error) {
	// Convert request with stream=true.
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

	// Patch the JSON to set stream=true.
	body = setStreamTrue(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.apiBase+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return provider_sdk.Usage{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return provider_sdk.Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		oaiErr := newOpenAIError(resp.StatusCode, respBody)
		return provider_sdk.Usage{},
			&provider_sdk.ProviderError{
				Code:       oaiErr.ErrorCode,
				Message:    oaiErr.Message,
				StatusCode: oaiErr.StatusCode,
				Retryable:  isRetryableStatus(oaiErr.StatusCode),
				Raw:        oaiErr,
			}
	}

	return a.readSSEStream(ctx, resp.Body, string(req.RequestedModel), emit)
}

// readSSEStream reads an SSE stream from the response body, emitting events
// via the EventSink. It handles heartbeats by sending an SSE comment every
// 15 seconds of idle time.
func (a *Adapter) readSSEStream(
	ctx context.Context,
	body io.Reader,
	modelID string,
	emit provider_sdk.EventSink,
) (provider_sdk.Usage, error) {
	scanner := bufio.NewScanner(body)
	// Increase buffer for large chunks.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var finalUsage provider_sdk.Usage

	// Heartbeat ticker: send an SSE comment every 15 seconds to keep the
	// connection alive during idle periods.
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// Channel to signal that we have new data.
	lines := make(chan string, 32)
	errs := make(chan error, 1)

	// Read lines in a goroutine so we can multiplex with heartbeat.
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
			return finalUsage, ctx.Err()

		case err := <-errs:
			if err != nil {
				return finalUsage, fmt.Errorf("sse read error: %w", err)
			}
			// Scanner finished normally.
			return finalUsage, nil

		case line, ok := <-lines:
			if !ok {
				// Channel closed, check for errors.
				select {
				case err := <-errs:
					if err != nil {
						return finalUsage, fmt.Errorf("sse read error: %w", err)
					}
				default:
				}
				return finalUsage, nil
			}

			// Process the SSE line.
			usage, err := a.processSSELine(line, modelID, emit)
			if err != nil {
				return finalUsage, err
			}
			if usage != nil {
				finalUsage = *usage
			}

		case <-heartbeat.C:
			// Send SSE comment as heartbeat to keep connection alive.
			_ = emit(provider_sdk.StreamEvent{
				EventType: "heartbeat",
				Data:      json.RawMessage(`": heartbeat"`),
			})
		}
	}
}

// processSSELine processes a single line from the SSE stream.
// Returns the usage if the line contains final usage data.
func (a *Adapter) processSSELine(
	line string,
	modelID string,
	emit provider_sdk.EventSink,
) (*provider_sdk.Usage, error) {
	// Empty lines are ignored.
	if strings.TrimSpace(line) == "" {
		return nil, nil
	}

	// SSE comment lines start with ":".
	if strings.HasPrefix(line, ":") {
		return nil, nil
	}

	// Must start with "data: ".
	if !strings.HasPrefix(line, "data: ") {
		return nil, nil
	}

	data := strings.TrimPrefix(line, "data: ")

	// Stream termination signal.
	if data == "[DONE]" {
		_ = emit(provider_sdk.StreamEvent{
			EventType: "done",
			Data:      json.RawMessage(`{"status":"done"}`),
		})
		return nil, nil
	}

	// Parse the JSON chunk.
	var chunk openAISSEChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		// If we can't parse, skip this chunk but don't fail the stream.
		return nil, nil
	}

	// Convert to a stream event and emit.
	event := convertChunkToEvent(chunk, modelID)
	if err := emit(event); err != nil {
		return nil, fmt.Errorf("emit callback error: %w", err)
	}

	// If this chunk contains usage data, return it.
	if chunk.Usage != nil {
		return toCanonicalUsage(chunk.Usage), nil
	}

	return nil, nil
}

// convertChunkToEvent converts an OpenAI SSE chunk to a StreamEvent.
func convertChunkToEvent(chunk openAISSEChunk, modelID string) provider_sdk.StreamEvent {
	deltaBytes, _ := json.Marshal(chunk)

	eventType := "delta"
	if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
		eventType = "done"
	}

	return provider_sdk.StreamEvent{
		EventType: eventType,
		Data:      deltaBytes,
	}
}

// toCanonicalUsage converts openAIUsage to provider_sdk.Usage.
func toCanonicalUsage(u *openAIUsage) *provider_sdk.Usage {
	if u == nil {
		return nil
	}
	return &provider_sdk.Usage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		TotalTokens:  u.TotalTokens,
	}
}

// setStreamTrue patches the JSON body to set "stream":true for streaming
// requests.
func setStreamTrue(body []byte) []byte {
	// The body is valid JSON. We replace "stream":false with "stream":true.
	return bytes.Replace(body, []byte(`"stream":false`), []byte(`"stream":true`), 1)
}
