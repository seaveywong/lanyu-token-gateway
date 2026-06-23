package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApprovalRequest represents a row in the approval_requests table.
type ApprovalRequest struct {
	ID                string         `json:"id"`
	OrganizationID    string         `json:"organization_id"`
	RequesterID       string         `json:"requester_id"`
	Action            string         `json:"action"`
	ResourceType      string         `json:"resource_type"`
	ResourceID        string         `json:"resource_id"`
	Payload           json.RawMessage `json:"payload"`
	Status            string         `json:"status"`
	RequiredApprovals int            `json:"required_approvals"`
	ApprovedBy        []string       `json:"approved_by"`
	RejectedBy        *string        `json:"rejected_by,omitempty"`
	RejectionReason   *string        `json:"rejection_reason,omitempty"`
	ExpiresAt         time.Time      `json:"expires_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// ApprovalRepo provides CRUD operations on the approval_requests table.
type ApprovalRepo struct {
	pool *pgxpool.Pool
}

// NewApprovalRepo returns an ApprovalRepo backed by the given connection pool.
func NewApprovalRepo(pool *pgxpool.Pool) *ApprovalRepo {
	return &ApprovalRepo{pool: pool}
}

// Create inserts a new approval request and returns the created row.
func (r *ApprovalRepo) Create(ctx context.Context, req *ApprovalRequest) (*ApprovalRequest, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO approval_requests (organization_id, requester_id, action,
		                                 resource_type, resource_id, payload,
		                                 required_approvals, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, status, approved_by, created_at, updated_at`,
		req.OrganizationID, req.RequesterID, req.Action,
		req.ResourceType, req.ResourceID, req.Payload,
		req.RequiredApprovals, req.ExpiresAt,
	)

	var result ApprovalRequest
	result = *req // copy input fields
	if err := row.Scan(&result.ID, &result.Status, &result.ApprovedBy,
		&result.CreatedAt, &result.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert approval request: %w", err)
	}
	return &result, nil
}

// FindByID retrieves an approval request by its ID.
func (r *ApprovalRepo) FindByID(ctx context.Context, id string) (*ApprovalRequest, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, requester_id, action, resource_type, resource_id,
		        payload, status, required_approvals, approved_by, rejected_by,
		        rejection_reason, expires_at, created_at, updated_at
		 FROM approval_requests WHERE id = $1`, id)

	var req ApprovalRequest
	var rejectedBy, rejectionReason *string
	err := row.Scan(
		&req.ID, &req.OrganizationID, &req.RequesterID, &req.Action,
		&req.ResourceType, &req.ResourceID, &req.Payload,
		&req.Status, &req.RequiredApprovals, &req.ApprovedBy,
		&rejectedBy, &rejectionReason,
		&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find approval request by id: %w", err)
	}
	req.RejectedBy = rejectedBy
	req.RejectionReason = rejectionReason
	return &req, nil
}

// AddApproval appends an approver to the approved_by list. If the required number of
// approvals is reached, status is set to 'approved'. Returns the updated row.
func (r *ApprovalRepo) AddApproval(ctx context.Context, id, approverID string) (*ApprovalRequest, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE approval_requests
		 SET approved_by = array_append(approved_by, $2),
		     updated_at = NOW()
		 WHERE id = $1
		   AND status = 'pending'
		   AND NOT ($2 = ANY(approved_by))
		   AND expires_at > NOW()
		 RETURNING id, organization_id, requester_id, action, resource_type, resource_id,
		           payload, status, required_approvals, approved_by, rejected_by,
		           rejection_reason, expires_at, created_at, updated_at`,
		id, approverID)

	var req ApprovalRequest
	var rejectedBy, rejectionReason *string
	err := row.Scan(
		&req.ID, &req.OrganizationID, &req.RequesterID, &req.Action,
		&req.ResourceType, &req.ResourceID, &req.Payload,
		&req.Status, &req.RequiredApprovals, &req.ApprovedBy,
		&rejectedBy, &rejectionReason,
		&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // already approved/rejected/expired, or duplicate approver
		}
		return nil, fmt.Errorf("add approval: %w", err)
	}
	req.RejectedBy = rejectedBy
	req.RejectionReason = rejectionReason

	// Check if threshold met — update status if so.
	if len(req.ApprovedBy) >= req.RequiredApprovals {
		_, err := r.pool.Exec(ctx,
			`UPDATE approval_requests SET status = 'approved', updated_at = NOW() WHERE id = $1`, id)
		if err != nil {
			return nil, fmt.Errorf("mark approved: %w", err)
		}
		req.Status = "approved"
	}

	return &req, nil
}

// Reject sets the status to 'rejected' with a reason.
func (r *ApprovalRepo) Reject(ctx context.Context, id, rejecterID, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE approval_requests
		 SET status = 'rejected', rejected_by = $2, rejection_reason = $3, updated_at = NOW()
		 WHERE id = $1 AND status = 'pending' AND expires_at > NOW()`,
		id, rejecterID, reason,
	)
	if err != nil {
		return fmt.Errorf("reject approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("approval request not found or already resolved")
	}
	return nil
}

// Cancel sets the status to 'cancelled'. Only the requester can cancel.
func (r *ApprovalRepo) Cancel(ctx context.Context, id, requesterID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE approval_requests
		 SET status = 'cancelled', updated_at = NOW()
		 WHERE id = $1 AND requester_id = $2 AND status = 'pending'`,
		id, requesterID,
	)
	if err != nil {
		return fmt.Errorf("cancel approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("approval request not found or already resolved")
	}
	return nil
}

// ListPending returns pending approval requests for an organization, ordered by
// creation time (oldest first so they get attention first).
func (r *ApprovalRepo) ListPending(ctx context.Context, orgID string, limit, offset int) ([]ApprovalRequest, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, requester_id, action, resource_type, resource_id,
		        payload, status, required_approvals, approved_by, rejected_by,
		        rejection_reason, expires_at, created_at, updated_at
		 FROM approval_requests
		 WHERE organization_id = $1 AND status = 'pending' AND expires_at > NOW()
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	defer rows.Close()
	return scanApprovalRequests(rows)
}

// ListByRequester returns approval requests created by a specific user.
func (r *ApprovalRepo) ListByRequester(ctx context.Context, requesterID string, limit, offset int) ([]ApprovalRequest, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, requester_id, action, resource_type, resource_id,
		        payload, status, required_approvals, approved_by, rejected_by,
		        rejection_reason, expires_at, created_at, updated_at
		 FROM approval_requests
		 WHERE requester_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, requesterID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list by requester: %w", err)
	}
	defer rows.Close()
	return scanApprovalRequests(rows)
}

// ListHistory returns all approval requests (any status) for an organization.
func (r *ApprovalRepo) ListHistory(ctx context.Context, orgID string, limit, offset int) ([]ApprovalRequest, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, requester_id, action, resource_type, resource_id,
		        payload, status, required_approvals, approved_by, rejected_by,
		        rejection_reason, expires_at, created_at, updated_at
		 FROM approval_requests
		 WHERE organization_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list approval history: %w", err)
	}
	defer rows.Close()
	return scanApprovalRequests(rows)
}

func scanApprovalRequests(rows pgx.Rows) ([]ApprovalRequest, error) {
	var reqs []ApprovalRequest
	for rows.Next() {
		var req ApprovalRequest
		var rejectedBy, rejectionReason *string
		if err := rows.Scan(
			&req.ID, &req.OrganizationID, &req.RequesterID, &req.Action,
			&req.ResourceType, &req.ResourceID, &req.Payload,
			&req.Status, &req.RequiredApprovals, &req.ApprovedBy,
			&rejectedBy, &rejectionReason,
			&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan approval request: %w", err)
		}
		req.RejectedBy = rejectedBy
		req.RejectionReason = rejectionReason
		reqs = append(reqs, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter approval requests: %w", err)
	}
	return reqs, nil
}
