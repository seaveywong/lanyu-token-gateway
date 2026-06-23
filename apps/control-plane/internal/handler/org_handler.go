package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
)

// OrgHandler handles organization management endpoints for both portal and admin APIs.
type OrgHandler struct {
	orgService OrgService
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(orgService OrgService) *OrgHandler {
	return &OrgHandler{orgService: orgService}
}

// Create handles POST /portal-api/orgs.
// Creates a new organization with the authenticated user as the first member
// (org_owner role).
func (h *OrgHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required", requestID(r))
		return
	}

	org, err := h.orgService.Create(r.Context(), userID, req.Name)
	if err != nil {
		slog.Error("org create failed", slog.String("error", err.Error()), slog.String("user_id", userID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, org)
}

// List handles GET /portal-api/orgs.
// Returns all organizations the authenticated user belongs to.
func (h *OrgHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	orgs, err := h.orgService.ListByUser(r.Context(), userID)
	if err != nil {
		slog.Error("org list failed", slog.String("error", err.Error()), slog.String("user_id", userID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, orgs)
}

// Get handles GET /portal-api/orgs/:orgId.
// Returns a single organization by ID (tenant-scoped).
func (h *OrgHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	org, err := h.orgService.GetByID(r.Context(), orgID)
	if err != nil {
		slog.Error("org get failed", slog.String("error", err.Error()), slog.String("org_id", orgID))
		respondError(w, http.StatusNotFound, "not_found", "organization not found", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, org)
}

// AdminList handles GET /admin-api/orgs.
// Returns all organizations (paginated) for platform administrators.
// TODO: requires orgService.ListAll which will be added to the service layer.
func (h *OrgHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	page, pageSize := getPageParams(r)

	orgs, total, err := h.orgService.ListAll(r.Context(), page, pageSize)
	if err != nil {
		slog.Error("admin org list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      orgs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}
