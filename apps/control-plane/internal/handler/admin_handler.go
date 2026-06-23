package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
)

// AdminHandler aggregates admin-only operations. It delegates to the
// individual entity handlers for the actual business logic.
type AdminHandler struct {
	orgHandler           *OrgHandler
	accountSourceService AccountSourceService
	auditService         AuditService
	userService          UserService
}

// NewAdminHandler creates a new AdminHandler with the required services.
func NewAdminHandler(
	orgHandler *OrgHandler,
	accountSourceService AccountSourceService,
	auditService AuditService,
	userService UserService,
) *AdminHandler {
	return &AdminHandler{
		orgHandler:           orgHandler,
		accountSourceService: accountSourceService,
		auditService:         auditService,
		userService:          userService,
	}
}

// ListUsers handles GET /admin-api/users.
// Returns a paginated list of all users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := getPageParams(r)

	users, total, err := h.userService.ListAll(r.Context(), page, pageSize)
	if err != nil {
		slog.Error("admin user list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      users,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// ListOrgs handles GET /admin-api/orgs.
// Delegates to OrgHandler.AdminList.
func (h *AdminHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	h.orgHandler.AdminList(w, r)
}

// ListAccountSources handles GET /admin-api/account-sources.
// Returns paginated account sources for platform admins.
func (h *AdminHandler) ListAccountSources(w http.ResponseWriter, r *http.Request) {
	page, pageSize := getPageParams(r)

	sources, total, err := h.accountSourceService.List(r.Context(), page, pageSize)
	if err != nil {
		slog.Error("admin account source list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      sources,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// CreateAccountSource handles POST /admin-api/account-sources.
// Registers a new account source (provider key, OAuth, or subscription pool).
func (h *AdminHandler) CreateAccountSource(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Name          string `json:"name"`
		SourceType    string `json:"source_type"`
		ProviderID    string `json:"provider_id"`
		CredentialRaw string `json:"credential_raw"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Name == "" || req.SourceType == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "name and source_type are required", requestID(r))
		return
	}

	source, err := h.accountSourceService.Create(r.Context(), req.Name, req.SourceType, req.ProviderID, []byte(req.CredentialRaw), userID)
	if err != nil {
		slog.Error("admin account source create failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, source)
}

// ListAuditLogs handles GET /admin-api/audit-logs.
// Returns paginated audit log entries.
func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId") // optional filter
	page, pageSize := getPageParams(r)

	logs, total, err := h.auditService.ListByOrg(r.Context(), orgID, page, pageSize)
	if err != nil {
		slog.Error("admin audit log list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      logs,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}
