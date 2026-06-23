package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// ApprovalServiceInterface is the subset of ApprovalService used by the middleware.
type ApprovalServiceInterface interface {
	CreateApproval(ctx context.Context, requesterID, orgID, action, resourceType, resourceID string, payload json.RawMessage, requiredApprovals int) (interface{ ID() string }, error)
}

// RequireApproval wraps a handler with four-eyes approval enforcement.
// When the action requires approval (and the request is not pre-approved),
// it creates an approval request and returns 202 Accepted instead of executing
// the handler. The caller must poll or listen for the approval to complete.
//
// Currently this is a skeleton that returns 202 with information about how to
// proceed. Full implementation requires:
//   - A way to identify whether a request has already been approved (e.g., an
//     X-Approval-ID header)
//   - Executing the handler once the approval status is verified
//
// Actions that require approval: payment.refund, org.transfer, key.export, source.disable
func RequireApproval(action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this request already carries an approval token
			approvalID := r.Header.Get("X-Approval-ID")
			if approvalID != "" {
				// Verification that the approval is complete happens downstream.
				// For now, pass through with the approval context.
				ctx := context.WithValue(r.Context(), contextKeyApprovalID, approvalID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Check if the action requires approval at all
			if !requiresApproval(action) {
				next.ServeHTTP(w, r)
				return
			}

			// Action requires approval but no approval ID present.
			// Return 202 Accepted with instructions.
			slog.Info("approval required for action",
				slog.String("action", action),
				slog.String("path", r.URL.Path))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    "approval_required",
				"message": "This action requires approval. Submit an approval request via POST /admin-api/approvals.",
				"action":  action,
			})
		})
	}
}

// requiresApproval returns true for actions that must pass the four-eyes check.
func requiresApproval(action string) bool {
	switch action {
	case "payment.refund", "org.transfer", "key.export", "source.disable":
		return true
	default:
		return false
	}
}

const contextKeyApprovalID contextKey = "approval_id"
