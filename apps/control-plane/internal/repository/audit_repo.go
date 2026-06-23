package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditEntry represents a row in the audit_logs table.
type AuditEntry struct {
	ID             string     `json:"id"`
	OrganizationID *string    `json:"organization_id"`
	ActorID        *string    `json:"actor_id"`
	Action         string     `json:"action"`
	ResourceType   *string    `json:"resource_type"`
	ResourceID     *string    `json:"resource_id"`
	MetadataJSON   string     `json:"metadata"`
	IPAddress      *string    `json:"ip_address"`
	UserAgent      *string    `json:"user_agent"`
	TraceID        *string    `json:"trace_id"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CreateAuditParams holds the data needed to write an audit log entry.
type CreateAuditParams struct {
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	MetadataJSON   string
	IPAddress      string
	UserAgent      string
	TraceID        string
}

// AuditRepo provides write and read operations on the audit_logs table.
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo returns an AuditRepo backed by the given connection pool.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// Create inserts a new audit log entry.
func (r *AuditRepo) Create(ctx context.Context, params CreateAuditParams) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_logs (organization_id, actor_id, action,
		                         resource_type, resource_id, metadata_json,
		                         ip_address, user_agent, trace_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		params.OrganizationID, params.ActorID, params.Action,
		params.ResourceType, params.ResourceID, params.MetadataJSON,
		params.IPAddress, params.UserAgent, params.TraceID,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// ListByOrg returns audit entries for an organization, paginated.
func (r *AuditRepo) ListByOrg(ctx context.Context, orgID string, limit, offset int) ([]AuditEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, actor_id, action, resource_type, resource_id,
		        metadata_json, ip_address, user_agent, trace_id, created_at
		 FROM audit_logs
		 WHERE organization_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, orgID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit by org: %w", err)
	}
	defer rows.Close()

	return scanAuditEntries(rows)
}

// ListByActor returns audit entries by a specific actor, paginated.
func (r *AuditRepo) ListByActor(ctx context.Context, actorID string, limit, offset int) ([]AuditEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, actor_id, action, resource_type, resource_id,
		        metadata_json, ip_address, user_agent, trace_id, created_at
		 FROM audit_logs
		 WHERE actor_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, actorID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit by actor: %w", err)
	}
	defer rows.Close()

	return scanAuditEntries(rows)
}

// ListByResource returns audit entries for a specific resource, paginated.
func (r *AuditRepo) ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]AuditEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, actor_id, action, resource_type, resource_id,
		        metadata_json, ip_address, user_agent, trace_id, created_at
		 FROM audit_logs
		 WHERE resource_type = $1 AND resource_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`, resourceType, resourceID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit by resource: %w", err)
	}
	defer rows.Close()

	return scanAuditEntries(rows)
}

// scanAuditEntries reads audit log rows into a slice.
func scanAuditEntries(rows pgx.Rows) ([]AuditEntry, error) {
	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(
			&e.ID, &e.OrganizationID, &e.ActorID, &e.Action,
			&e.ResourceType, &e.ResourceID, &e.MetadataJSON,
			&e.IPAddress, &e.UserAgent, &e.TraceID, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter audit entries: %w", err)
	}
	return entries, nil
}
