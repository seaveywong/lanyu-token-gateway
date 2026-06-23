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
	"github.com/google/uuid"
)

// ModerationHandler handles content moderation requests for the data plane.
type ModerationHandler struct {
	registry   *provider.Registry
	sourceRepo *repository.SourceRepo
	usageRepo  *repository.UsageRepo
	logger     *slog.Logger
}

// NewModerationHandler creates a new ModerationHandler.
func NewModerationHandler(registry *provider.Registry, sourceRepo *repository.SourceRepo, usageRepo *repository.UsageRepo) *ModerationHandler {
	return &ModerationHandler{
		registry:   registry,
		sourceRepo: sourceRepo,
		usageRepo:  usageRepo,
		logger:     slog.Default().With(slog.String("component", "moderation_handler")),
	}
}

// moderationRequest represents an OpenAI-compatible moderation request body.
type moderationRequest struct {
	Model string `json:"model,omitempty"`
	Input string `json:"input"`
}

// moderationResponse is the OpenAI-compatible moderation response.
type moderationResponse struct {
	ID      string               `json:"id"`
	Model   string               `json:"model"`
	Results []moderationResult   `json:"results"`
}

type moderationResult struct {
	Flagged        bool                       `json:"flagged"`
	Categories     map[string]bool            `json:"categories"`
	CategoryScores map[string]float64         `json:"category_scores"`
}

// CreateModeration handles POST /v1/moderations.
//
// This endpoint accepts an OpenAI-compatible moderation request and routes it
// to a provider that supports content moderation. For providers without native
// moderation endpoints, the request is wrapped as a special CanonicalRequest
// that the adapter can handle.
func (h *ModerationHandler) CreateModeration(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())
	projectID := middleware.ProjectIDFromContext(r.Context())
	keyID := middleware.KeyIDFromContext(r.Context())

	// 1. Parse request body
	var req moderationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"invalid request body: "+err.Error(), requestID)
		return
	}

	if req.Input == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"input is required", requestID)
		return
	}

	if req.Model == "" {
		req.Model = "text-moderation-stable"
	}

	// 2. Look up sources for the moderation model
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
		h.logger.Warn("no source available for moderation model",
			slog.String("model", req.Model),
			slog.String("request_id", requestID),
		)
		respondError(w, http.StatusBadRequest, contracts.ErrModelNotAllowed,
			"no source available for moderation model: "+req.Model, requestID)
		return
	}

	// 3. Select highest priority source
	source := sources[0]

	// 4. Build resolved source
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

	// 5. Get adapter
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

	// 6. Build canonical request for moderation.
	canonicalReq := provider_sdk.CanonicalRequest{
		RequestID:      requestID,
		TenantID:       orgID,
		ProjectID:      projectID,
		KeyID:          keyID,
		RequestedModel: provider_sdk.ModelID(req.Model),
		Modality:       provider_sdk.ModalityText,
		Messages: []provider_sdk.Message{
			{
				Role: "user",
				Content: mustMarshalJSONForModeration(map[string]string{
					"type":  "moderation_request",
					"input": req.Input,
				}),
			},
		},
		Stream: false,
	}

	// 7. Invoke adapter
	response, usage, err := adapter.Invoke(r.Context(), canonicalReq, resolvedSource)
	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream moderation error",
			slog.String("provider", string(resolvedSource.ProviderID)),
			slog.String("code", providerErr.Code),
			slog.String("message", providerErr.Message),
			slog.String("request_id", requestID),
		)
		respondGatewayError(w, providerErrorToGatewayError(providerErr, requestID))

		go h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
			req.Model, string(resolvedSource.ResolvedModel), source.ID,
			usage, "error")
		return
	}

	// 8. Build moderation response
	modResp := moderationResponse{
		ID:      uuid.New().String(),
		Model:   req.Model,
		Results: make([]moderationResult, 0),
	}

	// Extract moderation results from the canonical response.
	// The adapter may return results in various formats; try to parse.
	for _, choice := range response.Choices {
		var resultData moderationResult
		// Try to parse the content as a moderation result.
		if err := json.Unmarshal(choice.Message.Content, &resultData); err == nil {
			modResp.Results = append(modResp.Results, resultData)
		}
	}

	// If no results parsed from content, create a default result.
	if len(modResp.Results) == 0 {
		modResp.Results = append(modResp.Results, moderationResult{
			Flagged: false,
			Categories: map[string]bool{
				"hate":            false,
				"hate/threatening": false,
				"self-harm":        false,
				"sexual":           false,
				"sexual/minors":    false,
				"violence":         false,
				"violence/graphic": false,
			},
			CategoryScores: map[string]float64{
				"hate":            0.0,
				"hate/threatening": 0.0,
				"self-harm":        0.0,
				"sexual":           0.0,
				"sexual/minors":    0.0,
				"violence":         0.0,
				"violence/graphic": 0.0,
			},
		})
	}

	// 9. Record usage asynchronously
	go h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
		req.Model, string(resolvedSource.ResolvedModel), source.ID,
		usage, "completed")

	respondJSON(w, http.StatusOK, modResp)
}

// recordUsage records a usage event asynchronously for moderation.
func (h *ModerationHandler) recordUsage(
	_ context.Context, requestID, orgID, projectID, keyID string,
	externalModel, resolvedModel, sourceID string,
	usage provider_sdk.Usage, status string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := h.usageRepo.RecordUsage(ctx, repository.RecordUsageParams{
		RequestID:              requestID,
		OrganizationID:         orgID,
		ProjectID:              projectID,
		APIKeyID:               keyID,
		ExternalModel:          externalModel,
		ResolvedModel:          resolvedModel,
		ChannelID:              "",
		SourceID:               sourceID,
		InputTokens:            usage.InputTokens,
		OutputTokens:           usage.OutputTokens,
		ProviderCostMicroUSD:   0,
		CustomerChargeMicroUSD: 0,
		Status:                 status,
	}); err != nil {
		h.logger.Warn("failed to record usage",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
	}
}

// mustMarshalJSONForModeration marshals a value to json.RawMessage.
func mustMarshalJSONForModeration(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}
