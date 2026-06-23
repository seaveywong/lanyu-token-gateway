package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ApprovalAction constants define the actions that require four-eyes approval.
const (
	ActionPaymentRefund = "payment.refund"   // Refund request — default 2 approvers
	ActionOrgTransfer   = "org.transfer"     // Organization ownership transfer
	ActionKeyExport     = "key.export"       // API key export
	ActionSourceDisable = "source.disable"   // Account source / channel disable
)

// DefaultApprovalExpiry is the default lifetime of an approval request.
const DefaultApprovalExpiry = 24 * time.Hour

// RequiredApprovalsByAction returns the minimum number of approvals required for an action.
// This implements the four-eyes principle: sensitive operations need at least 2 approvers.
func RequiredApprovalsByAction(action string) int {
	switch action {
	case ActionPaymentRefund:
		return 2
	case ActionOrgTransfer:
		return 2
	case ActionKeyExport:
		return 2
	case ActionSourceDisable:
		return 1 // single approval for disable (can be escalated)
	default:
		return 2
	}
}

// ApprovalService manages the four-eyes approval workflow for sensitive operations.
type ApprovalService struct {
	approvals *repository.ApprovalRepo
	audit     *repository.AuditRepo
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(approvals *repository.ApprovalRepo, audit *repository.AuditRepo) *ApprovalService {
	return &ApprovalService{
		approvals: approvals,
		audit:     audit,
	}
}

// CreateApproval creates an approval request for a sensitive action.
// The request must gather the required number of approvals before the action can proceed.
func (s *ApprovalService) CreateApproval(
	ctx context.Context,
	requesterID, orgID, action, resourceType, resourceID string,
	payload json.RawMessage,
	requiredApprovals int,
) (*repository.ApprovalRequest, error) {
	if requiredApprovals <= 0 {
		requiredApprovals = RequiredApprovalsByAction(action)
	}

	req := &repository.ApprovalRequest{
		OrganizationID:    orgID,
		RequesterID:       requesterID,
		Action:            action,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Payload:           payload,
		RequiredApprovals: requiredApprovals,
		ExpiresAt:         time.Now().Add(DefaultApprovalExpiry),
	}

	result, err := s.approvals.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("approval service: create: %w", err)
	}

	// Record audit trail
	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        requesterID,
		Action:         "approval.create",
		ResourceType:   "approval_request",
		ResourceID:     result.ID,
		MetadataJSON:   fmt.Sprintf(`{"action":"%s","resource_type":"%s","resource_id":"%s"}`, action, resourceType, resourceID),
	})

	return result, nil
}

// Approve records a single approval. If the required number of approvals is met,
// the status transitions to 'approved' and the caller should proceed to execute
// the underlying action.
func (s *ApprovalService) Approve(ctx context.Context, approverID, requestID string) (*repository.ApprovalRequest, error) {
	// Load the request to get org context
	req, err := s.approvals.FindByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("approval service: approve: find: %w", err)
	}
	if req == nil {
		return nil, fmt.Errorf("approval request %s not found", requestID)
	}

	// Prevent self-approval
	if req.RequesterID == approverID {
		return nil, fmt.Errorf("self-approval is not allowed: approver %s is the requester", approverID)
	}

	result, err := s.approvals.AddApproval(ctx, requestID, approverID)
	if err != nil {
		return nil, fmt.Errorf("approval service: approve: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("approval request %s is not in a pending state or has already been resolved", requestID)
	}

	// Record audit trail
	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: req.OrganizationID,
		ActorID:        approverID,
		Action:         "approval.approve",
		ResourceType:   "approval_request",
		ResourceID:     requestID,
		MetadataJSON:   fmt.Sprintf(`{"new_status":"%s","approvals_count":%d,"required":%d}`, result.Status, len(result.ApprovedBy), result.RequiredApprovals),
	})

	return result, nil
}

// Reject rejects an approval request with a reason.
func (s *ApprovalService) Reject(ctx context.Context, rejecterID, requestID, reason string) error {
	req, err := s.approvals.FindByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("approval service: reject: find: %w", err)
	}
	if req == nil {
		return fmt.Errorf("approval request %s not found", requestID)
	}

	// Prevent self-rejection
	if req.RequesterID == rejecterID {
		return fmt.Errorf("self-rejection is not allowed: rejecter %s is the requester", rejecterID)
	}

	if err := s.approvals.Reject(ctx, requestID, rejecterID, reason); err != nil {
		return fmt.Errorf("approval service: reject: %w", err)
	}

	// Record audit trail
	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: req.OrganizationID,
		ActorID:        rejecterID,
		Action:         "approval.reject",
		ResourceType:   "approval_request",
		ResourceID:     requestID,
		MetadataJSON:   fmt.Sprintf(`{"reason":"%s"}`, reason),
	})

	return nil
}

// Cancel cancels a pending approval request. Only the requester can cancel.
func (s *ApprovalService) Cancel(ctx context.Context, requesterID, requestID string) error {
	if err := s.approvals.Cancel(ctx, requestID, requesterID); err != nil {
		return fmt.Errorf("approval service: cancel: %w", err)
	}
	return nil
}

// ListPending returns pending approval requests for an organization.
func (s *ApprovalService) ListPending(ctx context.Context, orgID string, page, pageSize int) ([]repository.ApprovalRequest, int, error) {
	offset := (page - 1) * pageSize
	reqs, err := s.approvals.ListPending(ctx, orgID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("approval service: list pending: %w", err)
	}
	// We return count as len(reqs) for simplicity; a full impl would do a COUNT query.
	return reqs, len(reqs), nil
}

// ListHistory returns the approval history (all statuses) for an organization.
func (s *ApprovalService) ListHistory(ctx context.Context, orgID string, page, pageSize int) ([]repository.ApprovalRequest, int, error) {
	offset := (page - 1) * pageSize
	reqs, err := s.approvals.ListHistory(ctx, orgID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("approval service: list history: %w", err)
	}
	return reqs, len(reqs), nil
}
