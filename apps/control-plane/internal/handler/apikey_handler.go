package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
)

// APIKeyHandler handles API key lifecycle endpoints.
type APIKeyHandler struct {
	apiKeyService APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(apiKeyService APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeyService: apiKeyService}
}

// Create handles POST /portal-api/projects/:projectId/keys.
// Generates a new API key for the given project and returns the plaintext
// key (only available at creation time).
func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "projectId")

	var req struct {
		Name        string `json:"name"`
		Environment string `json:"environment"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required", requestID(r))
		return
	}
	if req.Environment == "" {
		req.Environment = "production"
	}

	result, err := h.apiKeyService.Create(r.Context(), projectID, userID, req.Name, req.Environment)
	if err != nil {
		slog.Error("api key create failed", slog.String("error", err.Error()), slog.String("project_id", projectID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// ListByProject handles GET /portal-api/projects/:projectId/keys.
// Returns all API keys for the given project (plaintext keys are never
// included after creation).
func (h *APIKeyHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "projectId")

	keys, err := h.apiKeyService.ListByProject(r.Context(), projectID, userID)
	if err != nil {
		slog.Error("api key list failed", slog.String("error", err.Error()), slog.String("project_id", projectID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, keys)
}

// Revoke handles POST /portal-api/projects/:projectId/keys/:keyId/revoke.
// Revokes a specific API key so it can no longer be used.
func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	keyID := chi.URLParam(r, "keyId")

	if err := h.apiKeyService.Revoke(r.Context(), keyID, userID); err != nil {
		slog.Error("api key revoke failed", slog.String("error", err.Error()), slog.String("key_id", keyID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
