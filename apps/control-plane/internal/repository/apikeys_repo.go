package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIKey represents a row in the api_keys table.
type APIKey struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organization_id"`
	ProjectID        string     `json:"project_id"`
	Name             string     `json:"name"`
	Environment      string     `json:"environment"`
	KeyPrefix        string     `json:"key_prefix"`
	KeyHash          string     `json:"-"`
	ScopesJSON       string     `json:"scopes"`
	ModelPolicyJSON  string     `json:"model_policy"`
	IPAllowlistJSON  string     `json:"ip_allowlist"`
	RateLimitPolicyID *string   `json:"rate_limit_policy_id"`
	ExpiresAt        *time.Time `json:"expires_at"`
	LastUsedAt       *time.Time `json:"last_used_at"`
	RevokedAt        *time.Time `json:"revoked_at"`
	CreatedBy        *string    `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
}

// CreateAPIKeyParams holds the data needed to insert a new API key.
type CreateAPIKeyParams struct {
	OrganizationID  string
	ProjectID       string
	Name            string
	Environment     string
	KeyPrefix       string
	KeyHash         string
	ScopesJSON      string
	ModelPolicyJSON string
	IPAllowlistJSON string
	ExpiresAt       *time.Time
	CreatedBy       string
}

// APIKeyRepo provides CRUD operations on the api_keys table.
type APIKeyRepo struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepo returns an APIKeyRepo backed by the given connection pool.
func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{pool: pool}
}

// Create inserts a new API key and returns the created row.
func (r *APIKeyRepo) Create(ctx context.Context, params CreateAPIKeyParams) (*APIKey, error) {
	var k APIKey
	err := r.pool.QueryRow(ctx,
		`INSERT INTO api_keys (organization_id, project_id, name, environment,
		                       key_prefix, key_hash, scopes_json,
		                       model_policy_json, ip_allowlist_json,
		                       expires_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, organization_id, project_id, name, environment,
		           key_prefix, key_hash, scopes_json, model_policy_json,
		           ip_allowlist_json, rate_limit_policy_id, expires_at,
		           last_used_at, revoked_at, created_by, created_at`,
		params.OrganizationID, params.ProjectID, params.Name, params.Environment,
		params.KeyPrefix, params.KeyHash, params.ScopesJSON,
		params.ModelPolicyJSON, params.IPAllowlistJSON,
		params.ExpiresAt, params.CreatedBy,
	).Scan(
		&k.ID, &k.OrganizationID, &k.ProjectID, &k.Name, &k.Environment,
		&k.KeyPrefix, &k.KeyHash, &k.ScopesJSON, &k.ModelPolicyJSON,
		&k.IPAllowlistJSON, &k.RateLimitPolicyID, &k.ExpiresAt,
		&k.LastUsedAt, &k.RevokedAt, &k.CreatedBy, &k.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	return &k, nil
}

// FindByHash looks up an API key by its key_hash (unique).
func (r *APIKeyRepo) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	var k APIKey
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id, name, environment,
		        key_prefix, key_hash, scopes_json, model_policy_json,
		        ip_allowlist_json, rate_limit_policy_id, expires_at,
		        last_used_at, revoked_at, created_by, created_at
		 FROM api_keys WHERE key_hash = $1`, hash,
	).Scan(
		&k.ID, &k.OrganizationID, &k.ProjectID, &k.Name, &k.Environment,
		&k.KeyPrefix, &k.KeyHash, &k.ScopesJSON, &k.ModelPolicyJSON,
		&k.IPAllowlistJSON, &k.RateLimitPolicyID, &k.ExpiresAt,
		&k.LastUsedAt, &k.RevokedAt, &k.CreatedBy, &k.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find api key by hash: %w", err)
	}
	return &k, nil
}

// FindByPrefix returns all API keys matching a given key_prefix.
func (r *APIKeyRepo) FindByPrefix(ctx context.Context, prefix string) ([]APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, project_id, name, environment,
		        key_prefix, key_hash, scopes_json, model_policy_json,
		        ip_allowlist_json, rate_limit_policy_id, expires_at,
		        last_used_at, revoked_at, created_by, created_at
		 FROM api_keys WHERE key_prefix = $1
		 ORDER BY created_at DESC`, prefix,
	)
	if err != nil {
		return nil, fmt.Errorf("find api keys by prefix: %w", err)
	}
	defer rows.Close()

	return scanAPIKeys(rows)
}

// ListByProject returns all API keys belonging to a project.
func (r *APIKeyRepo) ListByProject(ctx context.Context, projectID string) ([]APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, project_id, name, environment,
		        key_prefix, key_hash, scopes_json, model_policy_json,
		        ip_allowlist_json, rate_limit_policy_id, expires_at,
		        last_used_at, revoked_at, created_by, created_at
		 FROM api_keys WHERE project_id = $1
		 ORDER BY created_at DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys by project: %w", err)
	}
	defer rows.Close()

	return scanAPIKeys(rows)
}

// FindByID looks up an API key by its primary key UUID.
func (r *APIKeyRepo) FindByID(ctx context.Context, id string) (*APIKey, error) {
	var k APIKey
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id, name, environment,
		        key_prefix, key_hash, scopes_json, model_policy_json,
		        ip_allowlist_json, rate_limit_policy_id, expires_at,
		        last_used_at, revoked_at, created_by, created_at
		 FROM api_keys WHERE id = $1`, id,
	).Scan(
		&k.ID, &k.OrganizationID, &k.ProjectID, &k.Name, &k.Environment,
		&k.KeyPrefix, &k.KeyHash, &k.ScopesJSON, &k.ModelPolicyJSON,
		&k.IPAllowlistJSON, &k.RateLimitPolicyID, &k.ExpiresAt,
		&k.LastUsedAt, &k.RevokedAt, &k.CreatedBy, &k.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find api key by id: %w", err)
	}
	return &k, nil
}

// Revoke marks an API key as revoked by setting revoked_at to NOW().
func (r *APIKeyRepo) Revoke(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("revoke api key: key %s not found or already revoked", id)
	}
	return nil
}

// UpdateLastUsed sets the last_used_at timestamp to NOW().
func (r *APIKeyRepo) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}

// scanAPIKeys reads API key rows into a slice.
func scanAPIKeys(rows pgx.Rows) ([]APIKey, error) {
	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(
			&k.ID, &k.OrganizationID, &k.ProjectID, &k.Name, &k.Environment,
			&k.KeyPrefix, &k.KeyHash, &k.ScopesJSON, &k.ModelPolicyJSON,
			&k.IPAllowlistJSON, &k.RateLimitPolicyID, &k.ExpiresAt,
			&k.LastUsedAt, &k.RevokedAt, &k.CreatedBy, &k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter api keys: %w", err)
	}
	return keys, nil
}
