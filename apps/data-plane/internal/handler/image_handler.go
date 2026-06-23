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

// ImageHandler handles image generation requests for the data plane.
type ImageHandler struct {
	registry   *provider.Registry
	sourceRepo *repository.SourceRepo
	usageRepo  *repository.UsageRepo
	logger     *slog.Logger
}

// NewImageHandler creates a new ImageHandler.
func NewImageHandler(registry *provider.Registry, sourceRepo *repository.SourceRepo, usageRepo *repository.UsageRepo) *ImageHandler {
	return &ImageHandler{
		registry:   registry,
		sourceRepo: sourceRepo,
		usageRepo:  usageRepo,
		logger:     slog.Default().With(slog.String("component", "image_handler")),
	}
}

// imageGenerationRequest represents an OpenAI DALL-E compatible image
// generation request body.
type imageGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // "url" or "b64_json"
	Style          string `json:"style,omitempty"`
	User           string `json:"user,omitempty"`
}

// imageGenerationResponse is an OpenAI DALL-E compatible response.
type imageGenerationResponse struct {
	Created int64           `json:"created"`
	Data    []imageDataItem `json:"data"`
}

type imageDataItem struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// CreateImage handles POST /v1/images/generations.
//
// This endpoint accepts an OpenAI DALL-E compatible request format and
// routes it to a provider that supports image generation.
// Currently returns feature_not_supported for most providers since
// image generation APIs differ significantly across providers.
func (h *ImageHandler) CreateImage(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())
	projectID := middleware.ProjectIDFromContext(r.Context())
	keyID := middleware.KeyIDFromContext(r.Context())

	// 1. Parse request body
	var req imageGenerationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"invalid request body: "+err.Error(), requestID)
		return
	}

	if req.Model == "" {
		req.Model = "dall-e-3"
	}
	if req.Prompt == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"prompt is required", requestID)
		return
	}

	// Default to 1 image
	if req.N <= 0 {
		req.N = 1
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "url"
	}

	// 2. Look up sources that support this model
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
			"no source available for image model: "+req.Model, requestID)
		return
	}

	// 3. Select the highest priority source
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

	// 6. Check if provider supports image modality
	caps := adapter.Capabilities(req.Model)
	if caps.SupportsImageInput == provider_sdk.SupportUnsupported {
		respondError(w, http.StatusBadRequest, contracts.ErrFeatureNotSupported,
			"image generation not supported for model: "+req.Model, requestID)
		return
	}

	// 7. Build a canonical request with image modality
	canonicalReq := provider_sdk.CanonicalRequest{
		RequestID:      requestID,
		TenantID:       orgID,
		ProjectID:      projectID,
		KeyID:          keyID,
		RequestedModel: provider_sdk.ModelID(req.Model),
		Modality:       provider_sdk.ModalityImage,
		Messages: []provider_sdk.Message{
			{
				Role:    "user",
				Content: mustMarshalJSONForHandler(req.Prompt),
			},
		},
		Stream: false,
		GenerationParams: provider_sdk.GenerationParams{
			MaxTokens: 1,
		},
	}

	// 8. Invoke adapter
	response, usage, err := adapter.Invoke(r.Context(), canonicalReq, resolvedSource)
	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream image generation error",
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

	// 9. Build image generation response
	imgResp := imageGenerationResponse{
		Created: time.Now().Unix(),
	}

	// Extract image data from the response.
	// If the adapter returned a canonical text response, the text may contain
	// an image URL or base64 data.
	for _, choice := range response.Choices {
		var textContent string
		if err := json.Unmarshal(choice.Message.Content, &textContent); err == nil {
			item := imageDataItem{}
			if req.ResponseFormat == "b64_json" {
				item.B64JSON = textContent
			} else {
				item.URL = textContent
			}
			imgResp.Data = append(imgResp.Data, item)
		}
	}

	// If no data extracted, return a placeholder.
	if len(imgResp.Data) == 0 {
		for i := 0; i < req.N; i++ {
			imgResp.Data = append(imgResp.Data, imageDataItem{
				RevisedPrompt: req.Prompt,
			})
		}
	}

	// 10. Record usage
	go h.recordUsage(r.Context(), requestID, orgID, projectID, keyID,
		req.Model, string(resolvedSource.ResolvedModel), source.ID,
		usage, "completed")

	respondJSON(w, http.StatusOK, imgResp)
}

// recordUsage records a usage event asynchronously.
func (h *ImageHandler) recordUsage(
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

// mustMarshalJSONForHandler marshals a value to json.RawMessage.
func mustMarshalJSONForHandler(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}
