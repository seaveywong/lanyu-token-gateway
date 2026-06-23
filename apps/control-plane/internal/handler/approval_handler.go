package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ApprovalService defines the operations for the four-eyes approval workflow.
type ApprovalService interface {
	CreateApproval(ctx context.Context, requesterID, orgID, action, resourceType, resourceID string, payload json.RawMessage, requiredApprovals int) (*repository.ApprovalRequest, error)
	Approve(ctx context.Context, approverID, requestID string) (*repository.ApprovalRequest, error)
	Reject(ctx context.Context, rejecterID, requestID, reason string) error
	Cancel(ctx context.Context, requesterID, requestID string) error
	ListPending(ctx context.Context, orgID string, page, pageSize int) ([]repository.ApprovalRequest, int, error)
	ListHistory(ctx context.Context, orgID string, page, pageSize int) ([]repository.ApprovalRequest, int, error)
}

// ApprovalHandler handles HTTP requests for the four-eyes approval workflow.
type ApprovalHandler struct {
	approvalService ApprovalService
}

// NewApprovalHandler creates a new ApprovalHandler.
func NewApprovalHandler(approvalService ApprovalService) *ApprovalHandler {
	return &ApprovalHandler{approvalService: approvalService}
}

// Create handles POST /admin-api/approvals.
// Creates a new approval request for a sensitive action.
func (h *ApprovalHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())

	var req struct {
		Action            string          `json:"action"`
		ResourceType      string          `json:"resource_type"`
		ResourceID        string          `json:"resource_id"`
		Payload           json.RawMessage `json:"payload"`
		RequiredApprovals int             `json:"required_approvals"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Action == "" || req.ResourceType == "" || req.ResourceID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "action, resource_type and resource_id are required", requestID(r))
		return
	}

	approval, err := h.approvalService.CreateApproval(r.Context(), userID, orgID, req.Action, req.ResourceType, req.ResourceID, req.Payload, req.RequiredApprovals)
	if err != nil {
		slog.Error("create approval failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, approval)
}

// Approve handles POST /admin-api/approvals/{id}/approve.
// Records an approval from the current user.
func (h *ApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	approvalID := chi.URLParam(r, "id")

	result, err := h.approvalService.Approve(r.Context(), userID, approvalID)
	if err != nil {
		slog.Error("approve failed", slog.String("error", err.Error()))
		respondError(w, http.StatusBadRequest, "approval_failed", err.Error(), requestID(r))
		return
	}

	if result.Status == "approved" {
		slog.Info("approval request fully approved",
			slog.String("request_id", requestID(r)),
			slog.String("action", result.Action))
	}

	respondJSON(w, http.StatusOK, result)
}

// Reject handles POST /admin-api/approvals/{id}/reject.
// Rejects an approval request.
func (h *ApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	approvalID := chi.URLParam(r, "id")

	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}
	if req.Reason == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "rejection reason is required", requestID(r))
		return
	}

	if err := h.approvalService.Reject(r.Context(), userID, approvalID, req.Reason); err != nil {
		slog.Error("reject failed", slog.String("error", err.Error()))
		respondError(w, http.StatusBadRequest, "approval_failed", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// Cancel handles POST /admin-api/approvals/{id}/cancel.
// Cancels a pending approval request. Only the requester can cancel.
func (h *ApprovalHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	approvalID := chi.URLParam(r, "id")

	if err := h.approvalService.Cancel(r.Context(), userID, approvalID); err != nil {
		slog.Error("cancel failed", slog.String("error", err.Error()))
		respondError(w, http.StatusBadRequest, "cancel_failed", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ListPending handles GET /admin-api/approvals/pending.
// Returns pending approval requests for the organization.
func (h *ApprovalHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	page, pageSize := getPageParams(r)

	requests, total, err := h.approvalService.ListPending(r.Context(), orgID, page, pageSize)
	if err != nil {
		slog.Error("list pending approvals failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      requests,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// ListHistory handles GET /admin-api/approvals/history.
// Returns the approval history (all statuses).
func (h *ApprovalHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	page, pageSize := getPageParams(r)

	requests, total, err := h.approvalService.ListHistory(r.Context(), orgID, page, pageSize)
	if err != nil {
		slog.Error("list approval history failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      requests,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}
