package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/service"
)

// WebhookHandler handles webhook endpoint management for customers and admins.
type WebhookHandler struct {
	webhookService *service.WebhookService
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(webhookService *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

// CreateEndpoint handles POST /portal-api/webhooks.
func (h *WebhookHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())

	var req struct {
		URL       string   `json:"url"`
		Secret    string   `json:"secret"`
		Events    []string `json:"events"`
		ProjectID *string  `json:"project_id,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}

	if req.URL == "" || req.Secret == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "url and secret are required", requestID(r))
		return
	}

	ep, err := h.webhookService.CreateEndpoint(r.Context(), orgID, userID, req.ProjectID, req.URL, req.Secret, req.Events)
	if err != nil {
		slog.Error("webhook create failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, ep)
}

// ListEndpoints handles GET /portal-api/webhooks.
func (h *WebhookHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())

	eps, err := h.webhookService.ListEndpoints(r.Context(), orgID)
	if err != nil {
		slog.Error("webhook list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  eps,
		"total": len(eps),
	})
}

// UpdateEndpoint handles PUT /portal-api/webhooks/{id}.
func (h *WebhookHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		URL      string   `json:"url"`
		Events   []string `json:"events"`
		IsActive *bool    `json:"is_active"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}

	isActive := req.IsActive != nil && *req.IsActive
	if err := h.webhookService.UpdateEndpoint(r.Context(), id, req.URL, req.Events, isActive); err != nil {
		slog.Error("webhook update failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "updated"})
}

// DeleteEndpoint handles DELETE /portal-api/webhooks/{id}.
func (h *WebhookHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.webhookService.DeleteEndpoint(r.Context(), id); err != nil {
		slog.Error("webhook delete failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// AdminListDeliveries handles GET /admin-api/webhooks/deliveries.
func (h *WebhookHandler) AdminListDeliveries(w http.ResponseWriter, r *http.Request) {
	// This is a stub — delivery listing is primarily handled by the async worker.
	// For admin visibility, we return the repository list capabilities.
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  []repository.WebhookDelivery{},
		"total": 0,
	})
}
