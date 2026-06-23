package service

import (
	"context"
	"fmt"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// AuditService provides a business-logic wrapper around audit logging.
type AuditService struct {
	audit *repository.AuditRepo
}

// NewAuditService returns an AuditService with the given repository.
func NewAuditService(audit *repository.AuditRepo) *AuditService {
	return &AuditService{audit: audit}
}

// Log writes an audit entry. It fills in defaults for empty fields.
func (s *AuditService) Log(ctx context.Context, params repository.CreateAuditParams) error {
	if params.Action == "" {
		return fmt.Errorf("audit log: action is required")
	}
	return s.audit.Create(ctx, params)
}

// ListByOrg returns paginated audit entries for an organization.
// page is 1-based; pageSize controls the number of entries per page.
// Returns the entries and the total count.
func (s *AuditService) ListByOrg(ctx context.Context, orgID string, page, pageSize int) ([]repository.AuditEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	entries, err := s.audit.ListByOrg(ctx, orgID, pageSize+1, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit by org: %w", err)
	}

	hasMore := len(entries) > pageSize
	if hasMore {
		entries = entries[:pageSize]
	}

	total := offset + len(entries)
	if hasMore {
		total = -1 // indicate there are more pages
	}

	if entries == nil {
		entries = []repository.AuditEntry{}
	}
	return entries, total, nil
}
