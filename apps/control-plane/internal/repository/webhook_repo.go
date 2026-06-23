package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WebhookEndpoint represents a row in the webhook_endpoints table.
type WebhookEndpoint struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	ProjectID      *string   `json:"project_id,omitempty"`
	URL            string    `json:"url"`
	Events         []string  `json:"events"`
	Secret         string    `json:"secret"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WebhookDelivery represents a row in the webhook_deliveries table.
type WebhookDelivery struct {
	ID              string     `json:"id"`
	EndpointID      string     `json:"endpoint_id"`
	EventType       string     `json:"event_type"`
	Payload         string     `json:"payload"`
	ResponseStatus  *int       `json:"response_status,omitempty"`
	ResponseBody    *string    `json:"response_body,omitempty"`
	AttemptCount    int        `json:"attempt_count"`
	MaxAttempts     int        `json:"max_attempts"`
	NextRetryAt     *time.Time `json:"next_retry_at,omitempty"`
	Status          string     `json:"status"`
	IdempotencyKey  string     `json:"idempotency_key"`
	CreatedAt       time.Time  `json:"created_at"`
}

// WebhookRepo provides CRUD operations on webhook endpoints and deliveries.
type WebhookRepo struct {
	pool *pgxpool.Pool
}

// NewWebhookRepo returns a WebhookRepo backed by the given connection pool.
func NewWebhookRepo(pool *pgxpool.Pool) *WebhookRepo {
	return &WebhookRepo{pool: pool}
}

// CreateEndpoint inserts a new webhook endpoint.
func (r *WebhookRepo) CreateEndpoint(ctx context.Context, orgID string, projectID *string, url, secret string, events []string) (*WebhookEndpoint, error) {
	var ep WebhookEndpoint
	err := r.pool.QueryRow(ctx,
		`INSERT INTO webhook_endpoints (organization_id, project_id, url, secret, events)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, organization_id, project_id, url, events, secret, is_active, created_at, updated_at`,
		orgID, projectID, url, secret, events,
	).Scan(&ep.ID, &ep.OrganizationID, &ep.ProjectID, &ep.URL, &ep.Events, &ep.Secret, &ep.IsActive, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert webhook endpoint: %w", err)
	}
	return &ep, nil
}

// ListByOrg returns all webhook endpoints for an organization.
func (r *WebhookRepo) ListByOrg(ctx context.Context, orgID string) ([]WebhookEndpoint, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, project_id, url, events, secret, is_active, created_at, updated_at
		 FROM webhook_endpoints
		 WHERE organization_id = $1
		 ORDER BY created_at DESC`, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer rows.Close()

	var eps []WebhookEndpoint
	for rows.Next() {
		var ep WebhookEndpoint
		if err := rows.Scan(&ep.ID, &ep.OrganizationID, &ep.ProjectID, &ep.URL, &ep.Events, &ep.Secret, &ep.IsActive, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook endpoint: %w", err)
		}
		eps = append(eps, ep)
	}
	return eps, rows.Err()
}

// FindEndpointByID looks up a webhook endpoint by UUID.
func (r *WebhookRepo) FindEndpointByID(ctx context.Context, id string) (*WebhookEndpoint, error) {
	var ep WebhookEndpoint
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id, url, events, secret, is_active, created_at, updated_at
		 FROM webhook_endpoints WHERE id = $1`, id,
	).Scan(&ep.ID, &ep.OrganizationID, &ep.ProjectID, &ep.URL, &ep.Events, &ep.Secret, &ep.IsActive, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find webhook endpoint: %w", err)
	}
	return &ep, nil
}

// UpdateEndpoint updates a webhook endpoint's URL, events, and active status.
func (r *WebhookRepo) UpdateEndpoint(ctx context.Context, id, url string, events []string, isActive bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE webhook_endpoints SET url = $2, events = $3, is_active = $4, updated_at = NOW()
		 WHERE id = $1`,
		id, url, events, isActive,
	)
	if err != nil {
		return fmt.Errorf("update webhook endpoint: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook endpoint %s not found", id)
	}
	return nil
}

// DeleteEndpoint removes a webhook endpoint by ID.
func (r *WebhookRepo) DeleteEndpoint(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM webhook_endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook endpoint %s not found", id)
	}
	return nil
}

// CreateDelivery inserts a new webhook delivery record.
func (r *WebhookRepo) CreateDelivery(ctx context.Context, endpointID, eventType, payload, idempotencyKey string) (*WebhookDelivery, error) {
	var d WebhookDelivery
	err := r.pool.QueryRow(ctx,
		`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload, idempotency_key)
		 VALUES ($1, $2, $3::jsonb, $4)
		 RETURNING id, endpoint_id, event_type, payload::text, response_status, response_body,
		           attempt_count, max_attempts, next_retry_at, status, idempotency_key, created_at`,
		endpointID, eventType, payload, idempotencyKey,
	).Scan(&d.ID, &d.EndpointID, &d.EventType, &d.Payload, &d.ResponseStatus, &d.ResponseBody,
		&d.AttemptCount, &d.MaxAttempts, &d.NextRetryAt, &d.Status, &d.IdempotencyKey, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert webhook delivery: %w", err)
	}
	return &d, nil
}

// ListPendingDeliveries returns deliveries that are pending or failed and due for retry.
func (r *WebhookRepo) ListPendingDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, endpoint_id, event_type, payload::text, response_status, response_body,
		        attempt_count, max_attempts, next_retry_at, status, idempotency_key, created_at
		 FROM webhook_deliveries
		 WHERE status IN ('pending', 'failed')
		   AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		 ORDER BY created_at ASC
		 LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		if err := rows.Scan(&d.ID, &d.EndpointID, &d.EventType, &d.Payload, &d.ResponseStatus, &d.ResponseBody,
			&d.AttemptCount, &d.MaxAttempts, &d.NextRetryAt, &d.Status, &d.IdempotencyKey, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

// UpdateDeliveryStatus updates a delivery's status, response details, and retry info.
func (r *WebhookRepo) UpdateDeliveryStatus(ctx context.Context, id string, status string, responseStatus *int, responseBody string, nextRetryAt *time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE webhook_deliveries
		 SET status = $2, response_status = $3, response_body = $4,
		     next_retry_at = $5, attempt_count = attempt_count + 1
		 WHERE id = $1`,
		id, status, responseStatus, responseBody, nextRetryAt,
	)
	if err != nil {
		return fmt.Errorf("update webhook delivery status: %w", err)
	}
	return nil
}

// FindDeliveryByIdempotencyKey checks if a delivery with the given idempotency key already exists.
func (r *WebhookRepo) FindDeliveryByIdempotencyKey(ctx context.Context, key string) (*WebhookDelivery, error) {
	var d WebhookDelivery
	err := r.pool.QueryRow(ctx,
		`SELECT id, endpoint_id, event_type, payload::text, response_status, response_body,
		        attempt_count, max_attempts, next_retry_at, status, idempotency_key, created_at
		 FROM webhook_deliveries WHERE idempotency_key = $1`, key,
	).Scan(&d.ID, &d.EndpointID, &d.EventType, &d.Payload, &d.ResponseStatus, &d.ResponseBody,
		&d.AttemptCount, &d.MaxAttempts, &d.NextRetryAt, &d.Status, &d.IdempotencyKey, &d.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find delivery by idempotency key: %w", err)
	}
	return &d, nil
}
