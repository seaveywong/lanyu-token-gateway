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

// AudioHandler handles audio generation and transcription requests for the
// data plane.
type AudioHandler struct {
	registry   *provider.Registry
	sourceRepo *repository.SourceRepo
	usageRepo  *repository.UsageRepo
	logger     *slog.Logger
}

// NewAudioHandler creates a new AudioHandler.
func NewAudioHandler(registry *provider.Registry, sourceRepo *repository.SourceRepo, usageRepo *repository.UsageRepo) *AudioHandler {
	return &AudioHandler{
		registry:   registry,
		sourceRepo: sourceRepo,
		usageRepo:  usageRepo,
		logger:     slog.Default().With(slog.String("component", "audio_handler")),
	}
}

// ---------------------------------------------------------------------------
// Text-to-Speech (POST /v1/audio/speech)
// ---------------------------------------------------------------------------

// speechRequest represents an OpenAI-compatible text-to-speech request body.
type speechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"` // "mp3", "opus", "aac", "flac", "wav", "pcm"
}

// CreateSpeech handles POST /v1/audio/speech (text-to-speech).
//
// This endpoint accepts an OpenAI TTS-compatible request and returns audio
// binary data. Audio binary response handling is implemented with the
// adapter returning encoded audio in the canonical response.
func (h *AudioHandler) CreateSpeech(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())
	projectID := middleware.ProjectIDFromContext(r.Context())
	keyID := middleware.KeyIDFromContext(r.Context())

	var req speechRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"invalid request body: "+err.Error(), requestID)
		return
	}

	if req.Model == "" {
		req.Model = "tts-1"
	}
	if req.Input == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"input text is required", requestID)
		return
	}
	if req.Voice == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"voice is required", requestID)
		return
	}

	// Default values.
	if req.Speed <= 0 {
		req.Speed = 1.0
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "mp3"
	}

	// Look up sources.
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
		respondError(w, http.StatusBadRequest, contracts.ErrModelNotAllowed,
			"no source available for TTS model: "+req.Model, requestID)
		return
	}

	source := sources[0]

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

	adapter, err := h.registry.Get(resolvedSource.ProviderID)
	if err != nil {
		h.logger.Error("no adapter for provider",
			slog.String("provider_id", string(resolvedSource.ProviderID)),
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusBadGateway, contracts.ErrProviderUnavailable,
			"no adapter for provider", requestID)
		return
	}

	// Check if provider supports audio modality.
	caps := adapter.Capabilities(req.Model)
	if caps.SupportsAudioInput == provider_sdk.SupportUnsupported {
		respondError(w, http.StatusBadRequest, contracts.ErrFeatureNotSupported,
			"speech generation not supported for model: "+req.Model, requestID)
		return
	}

	// Build canonical request with audio modality.
	canonicalReq := provider_sdk.CanonicalRequest{
		RequestID:      requestID,
		TenantID:       orgID,
		ProjectID:      projectID,
		KeyID:          keyID,
		RequestedModel: provider_sdk.ModelID(req.Model),
		Modality:       provider_sdk.ModalityAudio,
		Messages: []provider_sdk.Message{
			{
				Role: "user",
				Content: mustMarshalJSONForAudio(map[string]interface{}{
					"type":            "tts_request",
					"input":           req.Input,
					"voice":           req.Voice,
					"speed":           req.Speed,
					"response_format": req.ResponseFormat,
				}),
			},
		},
		Stream: false,
	}

	response, usage, err := adapter.Invoke(r.Context(), canonicalReq, resolvedSource)
	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream TTS error",
			slog.String("provider", string(resolvedSource.ProviderID)),
			slog.String("code", providerErr.Code),
			slog.String("message", providerErr.Message),
		)
		respondGatewayError(w, providerErrorToGatewayError(providerErr, requestID))

		h.recordAudioUsage(requestID, orgID, projectID, keyID,
			req.Model, string(resolvedSource.ResolvedModel), source.ID,
			usage, "error")
		return
	}

	// Extract audio data from response and stream as binary.
	var audioData []byte
	for _, choice := range response.Choices {
		var content map[string]interface{}
		if err := json.Unmarshal(choice.Message.Content, &content); err == nil {
			if b64, ok := content["audio_b64"]; ok {
				if b64Str, ok := b64.(string); ok {
					audioData = []byte(b64Str)
				}
			}
			if raw, ok := content["audio_bytes"]; ok {
				if rawStr, ok := raw.(string); ok {
					audioData = []byte(rawStr)
				}
			}
		} else {
			audioData = choice.Message.Content
		}
	}

	// Determine Content-Type based on format.
	contentType := "audio/mpeg"
	switch req.ResponseFormat {
	case "opus":
		contentType = "audio/opus"
	case "aac":
		contentType = "audio/aac"
	case "flac":
		contentType = "audio/flac"
	case "wav":
		contentType = "audio/wav"
	case "pcm":
		contentType = "audio/l16"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)

	if len(audioData) > 0 {
		w.Write(audioData)
	}

	h.recordAudioUsage(requestID, orgID, projectID, keyID,
		req.Model, string(resolvedSource.ResolvedModel), source.ID,
		usage, "completed")
}

// ---------------------------------------------------------------------------
// Speech-to-Text (POST /v1/audio/transcriptions)
// ---------------------------------------------------------------------------

// transcriptionResponse represents an OpenAI-compatible transcription
// response.
type transcriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// CreateTranscription handles POST /v1/audio/transcriptions (speech-to-text).
//
// This endpoint accepts multipart/form-data with an audio file and returns
// transcription text. It parses the multipart upload, wraps the audio data in
// a canonical request, and routes it to the appropriate provider adapter.
func (h *AudioHandler) CreateTranscription(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.RequestIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())
	projectID := middleware.ProjectIDFromContext(r.Context())
	keyID := middleware.KeyIDFromContext(r.Context())

	// Parse multipart form (max 25MB upload).
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"failed to parse multipart form: "+err.Error(), requestID)
		return
	}

	model := r.FormValue("model")
	if model == "" {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"model is required", requestID)
		return
	}

	language := r.FormValue("language")

	// Get the uploaded file.
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, contracts.ErrInvalidRequest,
			"audio file is required: "+err.Error(), requestID)
		return
	}
	defer file.Close()

	h.logger.Info("received transcription request",
		slog.String("model", model),
		slog.String("filename", header.Filename),
		slog.Int64("size", header.Size),
		slog.String("language", language),
	)

	// Look up sources.
	sources, err := h.sourceRepo.ListHealthyByModel(r.Context(), model)
	if err != nil {
		h.logger.Error("failed to list sources for model",
			slog.String("model", model),
			slog.String("error", err.Error()),
		)
		respondError(w, http.StatusInternalServerError, contracts.ErrInternalError,
			"routing lookup failed", requestID)
		return
	}
	if len(sources) == 0 {
		respondError(w, http.StatusBadRequest, contracts.ErrModelNotAllowed,
			"no source available for transcription model: "+model, requestID)
		return
	}

	source := sources[0]

	resolvedSource := provider_sdk.ResolvedSource{
		SourceID:      source.ID,
		SourceType:    source.SourceType,
		ResolvedModel: provider_sdk.ModelID(model),
	}
	if source.ProviderID != nil {
		resolvedSource.ProviderID = provider_sdk.ProviderID(*source.ProviderID)
	}
	if source.Endpoint != nil {
		resolvedSource.EndpointURL = *source.Endpoint
	}
	resolvedSource.Credential = json.RawMessage(source.CredentialCiphertext)

	adapter, err := h.registry.Get(resolvedSource.ProviderID)
	if err != nil {
		h.logger.Error("no adapter for provider",
			slog.String("provider_id", string(resolvedSource.ProviderID)),
		)
		respondError(w, http.StatusBadGateway, contracts.ErrProviderUnavailable,
			"no adapter for provider", requestID)
		return
	}

	// Check capabilities.
	caps := adapter.Capabilities(model)
	if caps.SupportsAudioInput == provider_sdk.SupportUnsupported {
		respondError(w, http.StatusBadRequest, contracts.ErrFeatureNotSupported,
			"transcription not supported for model: "+model, requestID)
		return
	}

	// Read file content into memory.
	fileBytes := make([]byte, header.Size)
	n, err := file.Read(fileBytes)
	if err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusInternalServerError, contracts.ErrInternalError,
			"failed to read audio file: "+err.Error(), requestID)
		return
	}
	fileBytes = fileBytes[:n]

	// Detect content type from header or filename.
	mimeType := "audio/mpeg"
	if header.Header.Get("Content-Type") != "" {
		mimeType = header.Header.Get("Content-Type")
	}

	// Build canonical request.
	canonicalReq := provider_sdk.CanonicalRequest{
		RequestID:      requestID,
		TenantID:       orgID,
		ProjectID:      projectID,
		KeyID:          keyID,
		RequestedModel: provider_sdk.ModelID(model),
		Modality:       provider_sdk.ModalityAudio,
		Messages: []provider_sdk.Message{
			{
				Role: "user",
				Content: mustMarshalJSONForAudio(map[string]interface{}{
					"type":      "transcription_request",
					"audio":     string(fileBytes),
					"mime_type": mimeType,
					"language":  language,
					"filename":  header.Filename,
				}),
			},
		},
		Stream: false,
	}

	response, usage, err := adapter.Invoke(r.Context(), canonicalReq, resolvedSource)
	if err != nil {
		providerErr := adapter.NormalizeError(err)
		h.logger.Error("upstream transcription error",
			slog.String("provider", string(resolvedSource.ProviderID)),
			slog.String("code", providerErr.Code),
		)
		respondGatewayError(w, providerErrorToGatewayError(providerErr, requestID))

		h.recordAudioUsage(requestID, orgID, projectID, keyID,
			model, string(resolvedSource.ResolvedModel), source.ID,
			usage, "error")
		return
	}

	// Extract text from response.
	text := ""
	for _, choice := range response.Choices {
		var contentStr string
		if err := json.Unmarshal(choice.Message.Content, &contentStr); err == nil {
			text = contentStr
			break
		}
	}

	h.recordAudioUsage(requestID, orgID, projectID, keyID,
		model, string(resolvedSource.ResolvedModel), source.ID,
		usage, "completed")

	respondJSON(w, http.StatusOK, transcriptionResponse{
		Text:     text,
		Language: language,
	})
}

// recordAudioUsage records a usage event asynchronously for audio handlers.
func (h *AudioHandler) recordAudioUsage(
	requestID, orgID, projectID, keyID string,
	externalModel, resolvedModel, sourceID string,
	usage provider_sdk.Usage, status string,
) {
	go func() {
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
	}()
}

// mustMarshalJSONForAudio marshals a value to json.RawMessage.
func mustMarshalJSONForAudio(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return b
}
