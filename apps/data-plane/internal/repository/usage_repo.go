package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UsageRepo handles usage event recording for the data plane.
type UsageRepo struct {
	pool *pgxpool.Pool
}

// NewUsageRepo creates a new UsageRepo backed by the given connection pool.
func NewUsageRepo(pool *pgxpool.Pool) *UsageRepo {
	return &UsageRepo{pool: pool}
}

// RecordUsageParams contains all fields needed to record a usage event.
type RecordUsageParams struct {
	RequestID              string
	OrganizationID         string
	ProjectID              string
	APIKeyID               string
	ExternalModel          string
	ResolvedModel          string
	ChannelID              string
	SourceID               string
	InputTokens            int
	OutputTokens           int
	ProviderCostMicroUSD   int64
	CustomerChargeMicroUSD int64
	Status                 string
}

// RecordUsage inserts a usage event into the usage_events table.
func (r *UsageRepo) RecordUsage(ctx context.Context, params RecordUsageParams) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO usage_events (
			request_id, organization_id, project_id, api_key_id,
			external_model, resolved_model, channel_id, source_id,
			input_tokens, output_tokens,
			provider_cost_micro_usd, customer_charge_micro_usd,
			status, started_at, completed_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10,
			$11, $12,
			$13, NOW(), NOW()
		)`,
		params.RequestID,
		params.OrganizationID,
		params.ProjectID,
		params.APIKeyID,
		params.ExternalModel,
		params.ResolvedModel,
		params.ChannelID,
		params.SourceID,
		params.InputTokens,
		params.OutputTokens,
		params.ProviderCostMicroUSD,
		params.CustomerChargeMicroUSD,
		params.Status,
	)
	return err
}
