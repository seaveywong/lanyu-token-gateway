package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
)

// ProjectHandler handles project management endpoints.
type ProjectHandler struct {
	projectService ProjectService
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(projectService ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

// Create handles POST /portal-api/projects (within an org context).
// Creates a new project under the given organization.
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name is required", requestID(r))
		return
	}

	project, err := h.projectService.Create(r.Context(), orgID, userID, req.Name, req.Description)
	if err != nil {
		slog.Error("project create failed", slog.String("error", err.Error()), slog.String("org_id", orgID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, project)
}

// ListByOrg handles GET /portal-api/orgs/:orgId/projects.
// Returns all projects within a specific organization.
func (h *ProjectHandler) ListByOrg(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	projects, err := h.projectService.ListByOrg(r.Context(), orgID, userID)
	if err != nil {
		slog.Error("project list failed", slog.String("error", err.Error()), slog.String("org_id", orgID))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, projects)
}

// Get handles GET /portal-api/projects/:projectId.
// Returns a single project by ID.
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	project, err := h.projectService.GetByID(r.Context(), projectID)
	if err != nil {
		slog.Error("project get failed", slog.String("error", err.Error()), slog.String("project_id", projectID))
		respondError(w, http.StatusNotFound, "not_found", "project not found", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, project)
}
