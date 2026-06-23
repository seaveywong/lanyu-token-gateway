package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// APIKeyRepo handles read-only lookups on the api_keys table for the data plane.
type APIKeyRepo struct {
	pool *pgxpool.Pool
}

// NewAPIKeyRepo creates a new APIKeyRepo backed by the given connection pool.
func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{pool: pool}
}

// APIKey represents an API key record from the database.
type APIKey struct {
	ID               string
	OrganizationID   string
	ProjectID        string
	Name             string
	Environment      string
	KeyPrefix        string
	KeyHash          string
	ScopesJSON       string
	ModelPolicyJSON  string
	IPAllowlistJSON  string
	RateLimitPolicyID *string
	ExpiresAt        *time.Time
	LastUsedAt       *time.Time
	RevokedAt        *time.Time
}

// FindByHash looks up an API key by its HMAC hash. Returns the full key record.
// Used during request authentication.
func (r *APIKeyRepo) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id, name, environment,
		        key_prefix, key_hash, COALESCE(scopes_json::text, '[]'),
		        COALESCE(model_policy_json::text, '{}'),
		        COALESCE(ip_allowlist_json::text, '[]'),
		        rate_limit_policy_id, expires_at, last_used_at, revoked_at
		 FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL`, hash)

	var k APIKey
	err := row.Scan(
		&k.ID, &k.OrganizationID, &k.ProjectID, &k.Name, &k.Environment,
		&k.KeyPrefix, &k.KeyHash, &k.ScopesJSON, &k.ModelPolicyJSON,
		&k.IPAllowlistJSON, &k.RateLimitPolicyID, &k.ExpiresAt,
		&k.LastUsedAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// FindByPrefix looks up keys by prefix (ly_live_ / ly_test_).
func (r *APIKeyRepo) FindByPrefix(ctx context.Context, prefix string) ([]APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, project_id, name, environment,
		        key_prefix, key_hash, COALESCE(scopes_json::text, '[]'),
		        COALESCE(model_policy_json::text, '{}'),
		        COALESCE(ip_allowlist_json::text, '[]'),
		        rate_limit_policy_id, expires_at, last_used_at, revoked_at
		 FROM api_keys WHERE key_prefix = $1 AND revoked_at IS NULL LIMIT 10`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(
			&k.ID, &k.OrganizationID, &k.ProjectID, &k.Name, &k.Environment,
			&k.KeyPrefix, &k.KeyHash, &k.ScopesJSON, &k.ModelPolicyJSON,
			&k.IPAllowlistJSON, &k.RateLimitPolicyID, &k.ExpiresAt,
			&k.LastUsedAt, &k.RevokedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// UpdateLastUsed updates the last_used_at timestamp for the given API key.
func (r *APIKeyRepo) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}
