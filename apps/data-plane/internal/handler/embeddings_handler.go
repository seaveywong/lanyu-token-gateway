package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	provider_sdk "github.com/seaveywong/lanyu-token-gateway/packages/provider-sdk"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/provider"
	"github.com/seaveywong/lanyu-token-gateway/apps/data-plane/internal/repository"
	"github.com/seaveywong/lanyu-token-gateway/packages/contracts"
)

// EmbeddingsHandler handles embeddings creation requests.
type EmbeddingsHandler struct {
	registry   *provider.Registry
	sourceRepo *repository.SourceRepo
	usageRepo  *repository.UsageRepo
	logger     *slog.Logger
}

// NewEmbeddingsHandler creates a new EmbeddingsHandler.
func NewEmbeddingsHandler(registry *provider.Registry, sourceRepo *repository.SourceRepo, usageRepo *repository.UsageRepo) *EmbeddingsHandler {
	return &EmbeddingsHandler{
		registry:   registry,
		sourceRepo: sourceRepo,
		usageRepo:  usageRepo,
		logger:     slog.Default().With(slog.String("component", "embeddings_handler")),
	}
}

// embeddingsRequest is the OpenAI-compatible embeddings request body.
type embeddingsRequest struct {
	Model          string          `json:"model"`
	Input          json.RawMessage `json:"input"`
	EncodingFormat string          `json:"encoding_format,omitempty"`
	User           string          `json:"user,omitempty"`
}

// CreateEmbedding handles POST /v1/embeddings requests.
// Skeleton implementation — full adapter integration pending.
func (h *EmbeddingsHandler) CreateEmbedding(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())

	var req embeddingsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest, "invalid request body: "+err.Error(), requestID)
		return
	}

	if req.Model == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest, "model is required", requestID)
		return
	}

	// Find sources that support this model
	sources, err := h.sourceRepo.ListHealthyByModel(r.Context(), req.Model)
	if err != nil {
		h.logger.Error("failed to list sources for model",
			slog.String("model", req.Model),
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusInternalServerError, contracts.ErrInternalError, "routing lookup failed", requestID)
		return
	}
	if len(sources) == 0 {
		respondError(w, http.StatusBadRequest, contracts.ErrModelNotAllowed, "no source available for model: "+req.Model, requestID)
		return
	}

	// Select the first (highest priority) source
	source := sources[0]

	// Build canonical request
	canonicalReq := provider_sdk.CanonicalRequest{
		RequestID:      requestID,
		TenantID:       middleware.OrgIDFromContext(r.Context()),
		ProjectID:      middleware.ProjectIDFromContext(r.Context()),
		KeyID:          middleware.KeyIDFromContext(r.Context()),
		RequestedModel: provider_sdk.ModelID(req.Model),
		Modality:       provider_sdk.ModalityText,
		Stream:         false,
	}

	// Resolve source
	resolvedSource := provider_sdk.ResolvedSource{
		SourceID:      source.ID,
		SourceType:    source.SourceType,
		ResolvedModel: provider_sdk.ModelID(req.Model),
	}

	if source.ProviderID != nil {
		resolvedSource.ProviderID = provider_sdk.ProviderID(*source.ProviderID)
	}
	if source.Endpoint != nil {
		resolvedSource.EndpointURL = *source.Endpoint
	}
	resolvedSource.Credential = json.RawMessage(source.CredentialCiphertext)

	// Get adapter
	adapter, err := h.registry.Get(resolvedSource.ProviderID)
	if err != nil {
		h.logger.Error("no adapter for provider",
			slog.String("provider_id", string(resolvedSource.ProviderID)),
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusBadGateway, contracts.ErrProviderUnavailable, "no adapter for provider", requestID)
		return
	}

	// Call adapter (non-streaming)
	_, usage, err := adapter.Invoke(r.Context(), canonicalReq, resolvedSource)
	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream error",
			slog.String("code", providerErr.Code),
			slog.String("message", providerErr.Message),
		)
		respondGatewayError(w, providerErrorToGatewayError(providerErr, requestID))
		return
	}

	// Record usage asynchronously (use background context to avoid cancellation)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := h.usageRepo.RecordUsage(ctx, repository.RecordUsageParams{
			RequestID:              requestID,
			OrganizationID:         middleware.OrgIDFromContext(r.Context()),
			ProjectID:              middleware.ProjectIDFromContext(r.Context()),
			APIKeyID:               middleware.KeyIDFromContext(r.Context()),
			ExternalModel:          req.Model,
			ResolvedModel:          string(resolvedSource.ResolvedModel),
			ChannelID:              "",
			SourceID:               source.ID,
			InputTokens:            usage.InputTokens,
			OutputTokens:           usage.OutputTokens,
			ProviderCostMicroUSD:   0,
			CustomerChargeMicroUSD: 0,
			Status:                 "completed",
		}); err != nil {
			h.logger.Warn("failed to record usage",
				slog.String("request_id", requestID),
				slog.String("error", err.Error()),
			)
		}
	}()

	// Return placeholder response for now (full response conversion pending)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   []interface{}{},
		"model":  req.Model,
		"usage": map[string]int{
			"prompt_tokens": usage.InputTokens,
			"total_tokens":  usage.TotalTokens,
		},
	})
}
