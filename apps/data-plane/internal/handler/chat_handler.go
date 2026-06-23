package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	provider_sdk "github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/cache"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/provider"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/repository"
	"github.com/seaveywong/lanyu-token-gateway/packages/contracts"
)

// ChatHandler handles chat completion requests for the data plane.
type ChatHandler struct {
	registry   *provider.Registry
	sourceRepo *repository.SourceRepo
	usageRepo  *repository.UsageRepo
	cache      *cache.ExactCache
	logger     *slog.Logger
}

// DefaultCacheTTL is the default TTL for cached responses.
const DefaultCacheTTL = 1 * time.Hour

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(registry *provider.Registry, sourceRepo *repository.SourceRepo, usageRepo *repository.UsageRepo, exactCache *cache.ExactCache) *ChatHandler {
	return &ChatHandler{
		registry:   registry,
		sourceRepo: sourceRepo,
		usageRepo:  usageRepo,
		cache:      exactCache,
		logger:     slog.Default().With(slog.String("component", "chat_handler")),
	}
}

// chatCompletionRequest is the OpenAI-compatible chat completion request body.
type chatCompletionRequest struct {
	Model            string                          `json:"model"`
	Messages         []provider_sdk.Message           `json:"messages"`
	Stream           bool                            `json:"stream,omitempty"`
	Temperature      *float64                        `json:"temperature,omitempty"`
	TopP             *float64                        `json:"top_p,omitempty"`
	TopK             *int                            `json:"top_k,omitempty"`
	MaxTokens        *int                            `json:"max_tokens,omitempty"`
	Stop             []string                        `json:"stop,omitempty"`
	FrequencyPenalty *float64                        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64                        `json:"presence_penalty,omitempty"`
	Seed             *int64                          `json:"seed,omitempty"`
	Tools            []provider_sdk.ToolDefinition    `json:"tools,omitempty"`
	ToolChoice       json.RawMessage                 `json:"tool_choice,omitempty"`
	ResponseFormat   *provider_sdk.ResponseFormat     `json:"response_format,omitempty"`
	User             string                          `json:"user,omitempty"`
}

// chatCompletionChoice is an OpenAI-compatible completion choice.
type chatCompletionChoice struct {
	Index        int                  `json:"index"`
	Message      provider_sdk.Message `json:"message"`
	FinishReason string               `json:"finish_reason"`
}

// chatCompletionResponse is the OpenAI-compatible chat completion response.
type chatCompletionResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []chatCompletionChoice  `json:"choices"`
	Usage   usageInfo               `json:"usage"`
}

// usageInfo mirrors OpenAI's usage format.
type usageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// streamChunk is an SSE delta chunk in OpenAI-compatible format.
type streamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []streamChunkChoice  `json:"choices"`
}

type streamChunkChoice struct {
	Index        int                    `json:"index"`
	Delta        streamDelta            `json:"delta"`
	FinishReason *string                `json:"finish_reason,omitempty"`
}

type streamDelta struct {
	Role    string          `json:"role,omitempty"`
	Content string          `json:"content,omitempty"`
}

// CreateChatCompletion handles POST /v1/chat/completions.
//
// Full flow:
// 1. Parse the OpenAI-format request body
// 2. Convert to CanonicalRequest
// 3. Look up model routing — which source supports this model?
// 4. Resolve the source credential
// 5. Select the ProviderAdapter from registry
// 6. If streaming: SSE with heartbeat
// 7. If non-streaming: invoke and return OpenAI-format JSON
// 8. Record usage event
func (h *ChatHandler) CreateChatCompletion(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())
	projectID := middleware.ProjectIDFromContext(r.Context())
	keyID := middleware.KeyIDFromContext(r.Context())

	// 1. Parse request body
	var req chatCompletionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"invalid request body: "+err.Error(), requestID)
		return
	}

	if req.Model == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"model is required", requestID)
		return
	}

	// 2. Convert to CanonicalRequest
	canonicalReq := h.buildCanonicalRequest(requestID, orgID, projectID, keyID, req)

	// 3. Look up model routing — find all sources that support this model
	sources, err := h.sourceRepo.ListHealthyByModel(r.Context(), req.Model)
	if err != nil {
		h.logger.Error("failed to list sources for model",
			slog.String("model", req.Model),
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusInternalServerError, contracts.ErrInternalError,
			"routing lookup failed", requestID)
		return
	}
	if len(sources) == 0 {
		h.logger.Warn("no source available for model",
			slog.String("model", req.Model),
			slog.String("request_id", requestID),
		)
		respondError(w, http.StatusBadRequest, contracts.ErrModelNotAllowed,
			"no source available for model: "+req.Model, requestID)
		return
	}

	// Select the first (highest priority) source
	source := sources[0]

	// 4. Resolve source credential
	resolvedSource := h.buildResolvedSource(source, req.Model)

	// 5. Select adapter from registry
	adapter, err := h.registry.Get(resolvedSource.ProviderID)
	if err != nil {
		h.logger.Error("no adapter for provider",
			slog.String("provider_id", string(resolvedSource.ProviderID)),
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusBadGateway, contracts.ErrProviderUnavailable,
			"no adapter for provider: "+string(resolvedSource.ProviderID), requestID)
		return
	}

	// 6. Check exact cache before invoking adapter (non-streaming only).
	if !req.Stream && cache.IsCacheable(canonicalReq) {
		cacheKey := cache.CacheKey(canonicalReq, "v1")
		if cached, err := h.cache.Get(r.Context(), cacheKey); err == nil && cached != nil {
			h.logger.Debug("cache hit",
				slog.String("cache_key", cacheKey),
				slog.String("request_id", requestID),
			)
			// Record usage as a cache hit.
			h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
				req.Model, string(resolvedSource.ResolvedModel),
				"", source.ID, 0, 0, "cache_hit")

			openAIResp := chatCompletionResponse{
				ID:      "chatcmpl-" + requestID,
				Object:  "chat.completion",
				Created: cached.Created,
				Model:   cached.Model,
			}
			if err := json.Unmarshal(cached.Choices, &openAIResp.Choices); err != nil {
				h.logger.Warn("failed to unmarshal cached choices, falling through to adapter",
					slog.String("error", err.Error()),
				)
			} else {
				if err := json.Unmarshal(cached.Usage, &openAIResp.Usage); err != nil {
					h.logger.Warn("failed to unmarshal cached usage",
						slog.String("error", err.Error()),
					)
				}
				respondJSON(w, http.StatusOK, openAIResp)
				return
			}
		}
	}

	// 7/8. Execute based on streaming preference
	if req.Stream {
		h.handleStream(w, r, adapter, canonicalReq, resolvedSource, source, req, requestID, orgID, projectID, keyID)
	} else {
		h.handleNonStream(w, r, adapter, canonicalReq, resolvedSource, source, req, requestID, orgID, projectID, keyID)
	}
}

// buildCanonicalRequest converts an OpenAI-format request into a CanonicalRequest.
func (h *ChatHandler) buildCanonicalRequest(requestID, orgID, projectID, keyID string, req chatCompletionRequest) provider_sdk.CanonicalRequest {
	cr := provider_sdk.CanonicalRequest{
		RequestID:      requestID,
		TenantID:       orgID,
		ProjectID:      projectID,
		KeyID:          keyID,
		RequestedModel: provider_sdk.ModelID(req.Model),
		Modality:       provider_sdk.ModalityText,
		Messages:       req.Messages,
		Tools:          req.Tools,
		ToolChoice:     req.ToolChoice,
		ResponseFormat: req.ResponseFormat,
		Stream:         req.Stream,
		GenerationParams: provider_sdk.GenerationParams{
			Stop: req.Stop,
		},
	}

	if req.Temperature != nil {
		cr.GenerationParams.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		cr.GenerationParams.TopP = *req.TopP
	}
	if req.TopK != nil {
		cr.GenerationParams.TopK = *req.TopK
	}
	if req.MaxTokens != nil {
		cr.GenerationParams.MaxTokens = *req.MaxTokens
	}
	if req.FrequencyPenalty != nil {
		cr.GenerationParams.FrequencyPenalty = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		cr.GenerationParams.PresencePenalty = *req.PresencePenalty
	}
	if req.Seed != nil {
		cr.GenerationParams.Seed = req.Seed
	}

	return cr
}

// buildResolvedSource builds a ResolvedSource from an AccountSource repository record.
func (h *ChatHandler) buildResolvedSource(source repository.AccountSource, model string) provider_sdk.ResolvedSource {
	resolved := provider_sdk.ResolvedSource{
		SourceID:      source.ID,
		SourceType:    source.SourceType,
		ResolvedModel: provider_sdk.ModelID(model),
		Credential:    json.RawMessage(source.CredentialCiphertext),
	}

	if source.ProviderID != nil {
		resolved.ProviderID = provider_sdk.ProviderID(*source.ProviderID)
	}
	if source.Endpoint != nil {
		resolved.EndpointURL = *source.Endpoint
	}

	return resolved
}

// handleNonStream handles a non-streaming chat completion request.
func (h *ChatHandler) handleNonStream(
	w http.ResponseWriter, r *http.Request,
	adapter provider_sdk.ProviderAdapter,
	canonicalReq provider_sdk.CanonicalRequest,
	resolvedSource provider_sdk.ResolvedSource,
	source repository.AccountSource,
	req chatCompletionRequest,
	requestID, orgID, projectID, keyID string,
) {
	// Call adapter
	response, usage, err := adapter.Invoke(r.Context(), canonicalReq, resolvedSource)
	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream invoke error",
			slog.String("provider", string(resolvedSource.ProviderID)),
			slog.String("code", providerErr.Code),
			slog.String("message", providerErr.Message),
			slog.String("request_id", requestID),
		)
		respondGatewayError(w, providerErrorToGatewayError(providerErr, requestID))
		// Record error usage event
		h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
			req.Model, string(resolvedSource.ResolvedModel),
			"", source.ID, 0, 0, "error")
		return
	}

	// Convert CanonicalResponse to OpenAI format
	openAIResp := h.canonicalToOpenAI(response, req.Model, usage)

	// Record usage event asynchronously
	h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
		req.Model, string(resolvedSource.ResolvedModel),
		"", source.ID, usage.InputTokens, usage.OutputTokens, "completed")

	// Store in exact cache if request is cacheable
	if cache.IsCacheable(canonicalReq) {
		cacheKey := cache.CacheKey(canonicalReq, "v1")
		choicesJSON, _ := json.Marshal(openAIResp.Choices)
		usageJSON, _ := json.Marshal(openAIResp.Usage)
		cached := &cache.CachedResponse{
			Model:   openAIResp.Model,
			Choices: json.RawMessage(choicesJSON),
			Usage:   json.RawMessage(usageJSON),
			Created: time.Now().Unix(),
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.cache.Set(ctx, cacheKey, cached, DefaultCacheTTL); err != nil {
				h.logger.Warn("failed to cache response",
					slog.String("cache_key", cacheKey),
					slog.String("error", err.Error()),
				)
			}
		}()
	}

	respondJSON(w, http.StatusOK, openAIResp)
}

// handleStream handles a streaming chat completion request using SSE.
func (h *ChatHandler) handleStream(
	w http.ResponseWriter, r *http.Request,
	adapter provider_sdk.ProviderAdapter,
	canonicalReq provider_sdk.CanonicalRequest,
	resolvedSource provider_sdk.ResolvedSource,
	source repository.AccountSource,
	req chatCompletionRequest,
	requestID, orgID, projectID, keyID string,
) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-ID", requestID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("streaming not supported by ResponseWriter")
		respondError(w, http.StatusInternalServerError, contracts.ErrInternalError,
			"streaming not supported", requestID)
		return
	}

	// Create the SSE writer
	sse := &sseWriter{
		w:       w,
		flusher: flusher,
		logger:  h.logger,
	}

	// Heartbeat: send a comment every 15 seconds to keep the connection alive
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			case <-heartbeatDone:
				return
			}
		}
	}()

	// Call adapter.Stream with EventSink that writes SSE to the ResponseWriter
	usage, err := adapter.Stream(r.Context(), canonicalReq, resolvedSource,
		func(event provider_sdk.StreamEvent) error {
			return sse.Emit(event)
		},
	)

	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream stream error",
			slog.String("provider", string(resolvedSource.ProviderID)),
			slog.String("code", providerErr.Code),
			slog.String("message", providerErr.Message),
			slog.String("request_id", requestID),
		)
		// Write error as SSE event
		errJSON, _ := json.Marshal(map[string]string{
			"error": providerErr.Message,
			"code":  providerErr.Code,
		})
		fmt.Fprintf(w, "data: %s\n\n", string(errJSON))
		flusher.Flush()

		// Record error usage
		h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
			req.Model, string(resolvedSource.ResolvedModel),
			"", source.ID, 0, 0, "error")
		return
	}

	// Send the [DONE] signal
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Record usage event asynchronously
	h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
		req.Model, string(resolvedSource.ResolvedModel),
		"", source.ID, usage.InputTokens, usage.OutputTokens, "completed")
}

// sseWriter implements SSE event writing to http.ResponseWriter.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	logger  *slog.Logger
}

// Emit writes a StreamEvent as an SSE data event and flushes.
func (s *sseWriter) Emit(event provider_sdk.StreamEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		s.logger.Error("failed to marshal stream event data", slog.String("error", err.Error()))
		return err
	}

	_, err = fmt.Fprintf(s.w, "data: %s\n\n", string(data))
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// canonicalToOpenAI converts a CanonicalResponse to an OpenAI-compatible chat completion response.
func (h *ChatHandler) canonicalToOpenAI(resp provider_sdk.CanonicalResponse, model string, usage provider_sdk.Usage) chatCompletionResponse {
	choices := make([]chatCompletionChoice, 0, len(resp.Choices))
	for _, c := range resp.Choices {
		choices = append(choices, chatCompletionChoice{
			Index:        c.Index,
			Message:      c.Message,
			FinishReason: c.FinishReason,
		})
	}

	return chatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
		Usage: usageInfo{
			PromptTokens:     usage.InputTokens,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      usage.TotalTokens,
		},
	}
}

// recordUsage records a usage event asynchronously (best-effort, non-blocking).
func (h *ChatHandler) recordUsage(
	_ context.Context, requestID, orgID, projectID, keyID string,
	externalModel, resolvedModel, channelID, sourceID string,
	inputTokens, outputTokens int, status string,
) {
	params := repository.RecordUsageParams{
		RequestID:              requestID,
		OrganizationID:         orgID,
		ProjectID:              projectID,
		APIKeyID:               keyID,
		ExternalModel:          externalModel,
		ResolvedModel:          resolvedModel,
		ChannelID:              channelID,
		SourceID:               sourceID,
		InputTokens:            inputTokens,
		OutputTokens:           outputTokens,
		ProviderCostMicroUSD:   0,
		CustomerChargeMicroUSD: 0,
		Status:                 status,
	}
	// Use a background context with timeout since the request context may be cancelled
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := h.usageRepo.RecordUsage(ctx, params); err != nil {
			h.logger.Warn("failed to record usage",
				slog.String("request_id", requestID),
				slog.String("error", err.Error()),
			)
		}
	}()
}
