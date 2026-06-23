package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SourceRepo handles account source lookups for model routing in the data plane.
type SourceRepo struct {
	pool *pgxpool.Pool
}

// NewSourceRepo creates a new SourceRepo backed by the given connection pool.
func NewSourceRepo(pool *pgxpool.Pool) *SourceRepo {
	return &SourceRepo{pool: pool}
}

// AccountSource represents an account source record used for routing.
type AccountSource struct {
	ID                   string
	Name                 string
	SourceType           string // official_api_key | official_oauth | subscription_pool | upstream_api
	ProviderID           *string
	Endpoint             *string
	CredentialCiphertext string
	Priority             int
	Weight               int
	MaxConcurrency       int
	DailyBudgetMicroUSD  int64
	Status               string
	HealthState          string
}

// ListHealthyByModel returns all active sources that support the given model,
// ordered by routing priority (official_api_key first, then official_oauth,
// then subscription_pool, then upstream_api).
func (r *SourceRepo) ListHealthyByModel(ctx context.Context, model string) ([]AccountSource, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT
			s.id, s.name, s.source_type, s.provider_id, s.endpoint,
			s.credential_ciphertext, s.priority, s.weight,
			s.max_concurrency, s.daily_budget_micro_usd, s.status, s.health_state
		 FROM account_sources s
		 JOIN model_catalog mc ON s.provider_id = mc.provider_id
		 WHERE mc.model_name = $1
		   AND s.status = 'active'
		   AND s.health_state NOT IN ('dead', 'circuit_open')
		 ORDER BY
		   CASE s.source_type
		     WHEN 'official_api_key' THEN 1
		     WHEN 'official_oauth' THEN 2
		     WHEN 'subscription_pool' THEN 3
		     WHEN 'upstream_api' THEN 4
		     ELSE 5
		   END,
		   s.priority ASC`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []AccountSource
	for rows.Next() {
		var s AccountSource
		if err := rows.Scan(
			&s.ID, &s.Name, &s.SourceType, &s.ProviderID, &s.Endpoint,
			&s.CredentialCiphertext, &s.Priority, &s.Weight,
			&s.MaxConcurrency, &s.DailyBudgetMicroUSD, &s.Status, &s.HealthState,
		); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}
