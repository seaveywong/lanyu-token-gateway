package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountSource represents a row in the account_sources table.
type AccountSource struct {
	ID                       string     `json:"id"`
	Name                     string     `json:"name"`
	SourceType               string     `json:"source_type"`
	ProviderID               *string    `json:"provider_id"`
	Endpoint                 *string    `json:"endpoint"`
	CredentialCiphertext     string     `json:"-"`
	CredentialKeyVersion     int        `json:"credential_key_version"`
	CredentialFingerprint    string     `json:"credential_fingerprint"`
	ModelPolicyJSON          string     `json:"model_policy"`
	Priority                 int        `json:"priority"`
	Weight                   int        `json:"weight"`
	MaxConcurrency           int        `json:"max_concurrency"`
	DailyBudgetMicroUSD      int64      `json:"daily_budget_micro_usd"`
	SubscriptionAccountsCount int       `json:"subscription_accounts_count"`
	ArkoseSolverConfigJSON   *string    `json:"arkose_solver_config_json,omitempty"`
	Status                   string     `json:"status"`
	HealthState              string     `json:"health_state"`
	LastValidatedAt          *time.Time `json:"last_validated_at"`
	LastUsedAt               *time.Time `json:"last_used_at"`
	CreatedBy                *string    `json:"created_by"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

// CreateAccountSourceParams holds the data needed to insert a new account source.
type CreateAccountSourceParams struct {
	Name                 string
	SourceType           string
	ProviderID           *string
	Endpoint             *string
	CredentialCiphertext string
	CredentialFingerprint string
	ModelPolicyJSON      string
	Priority             int
	Weight               int
	MaxConcurrency       int
	DailyBudgetMicroUSD  int64
	CreatedBy            string
}

// UpdateAccountSourceParams holds the fields that can be updated on an account source.
type UpdateAccountSourceParams struct {
	Name               *string
	Priority           *int
	Weight             *int
	MaxConcurrency     *int
	DailyBudgetMicroUSD *int64
	ModelPolicyJSON    *string
}

// AccountSourceRepo provides CRUD operations on the account_sources table.
type AccountSourceRepo struct {
	pool *pgxpool.Pool
}

// NewAccountSourceRepo returns an AccountSourceRepo backed by the given connection pool.
func NewAccountSourceRepo(pool *pgxpool.Pool) *AccountSourceRepo {
	return &AccountSourceRepo{pool: pool}
}

// Create inserts a new account source and returns the created row.
func (r *AccountSourceRepo) Create(ctx context.Context, params CreateAccountSourceParams) (*AccountSource, error) {
	var s AccountSource
	err := r.pool.QueryRow(ctx,
		`INSERT INTO account_sources (name, source_type, provider_id, endpoint,
		                              credential_ciphertext, credential_fingerprint,
		                              model_policy_json, priority, weight,
		                              max_concurrency, daily_budget_micro_usd, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, name, source_type, provider_id, endpoint,
		           credential_ciphertext, credential_key_version, credential_fingerprint,
		           model_policy_json, priority, weight, max_concurrency,
		           daily_budget_micro_usd, subscription_accounts_count,
		           arkose_solver_config_json, status, health_state,
		           last_validated_at, last_used_at, created_by, created_at, updated_at`,
		params.Name, params.SourceType, params.ProviderID, params.Endpoint,
		params.CredentialCiphertext, params.CredentialFingerprint,
		params.ModelPolicyJSON, params.Priority, params.Weight,
		params.MaxConcurrency, params.DailyBudgetMicroUSD, params.CreatedBy,
	).Scan(
		&s.ID, &s.Name, &s.SourceType, &s.ProviderID, &s.Endpoint,
		&s.CredentialCiphertext, &s.CredentialKeyVersion, &s.CredentialFingerprint,
		&s.ModelPolicyJSON, &s.Priority, &s.Weight, &s.MaxConcurrency,
		&s.DailyBudgetMicroUSD, &s.SubscriptionAccountsCount,
		&s.ArkoseSolverConfigJSON, &s.Status, &s.HealthState,
		&s.LastValidatedAt, &s.LastUsedAt, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert account source: %w", err)
	}
	return &s, nil
}

// FindByID looks up an account source by its primary key UUID.
// Returns nil if not found.
func (r *AccountSourceRepo) FindByID(ctx context.Context, id string) (*AccountSource, error) {
	var s AccountSource
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, source_type, provider_id, endpoint,
		        credential_ciphertext, credential_key_version, credential_fingerprint,
		        model_policy_json, priority, weight, max_concurrency,
		        daily_budget_micro_usd, subscription_accounts_count,
		        arkose_solver_config_json, status, health_state,
		        last_validated_at, last_used_at, created_by, created_at, updated_at
		 FROM account_sources WHERE id = $1`, id,
	).Scan(
		&s.ID, &s.Name, &s.SourceType, &s.ProviderID, &s.Endpoint,
		&s.CredentialCiphertext, &s.CredentialKeyVersion, &s.CredentialFingerprint,
		&s.ModelPolicyJSON, &s.Priority, &s.Weight, &s.MaxConcurrency,
		&s.DailyBudgetMicroUSD, &s.SubscriptionAccountsCount,
		&s.ArkoseSolverConfigJSON, &s.Status, &s.HealthState,
		&s.LastValidatedAt, &s.LastUsedAt, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find account source by id: %w", err)
	}
	return &s, nil
}

// List returns account sources filtered by sourceType, paginated.
// If sourceType is empty, all types are returned.
func (r *AccountSourceRepo) List(ctx context.Context, sourceType string, page, pageSize int) ([]AccountSource, int, error) {
	offset := (page - 1) * pageSize

	// Count query
	var total int
	countQuery := `SELECT COUNT(*) FROM account_sources WHERE status != 'deleted'`
	countArgs := []any{}
	if sourceType != "" {
		countQuery += ` AND source_type = $1`
		countArgs = append(countArgs, sourceType)
	}
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count account sources: %w", err)
	}

	// Data query
	dataQuery := `SELECT id, name, source_type, provider_id, endpoint,
	                     credential_ciphertext, credential_key_version, credential_fingerprint,
	                     model_policy_json, priority, weight, max_concurrency,
	                     daily_budget_micro_usd, subscription_accounts_count,
	                     arkose_solver_config_json, status, health_state,
	                     last_validated_at, last_used_at, created_by, created_at, updated_at
	              FROM account_sources WHERE status != 'deleted'`
	dataArgs := []any{}
	argIdx := 1
	if sourceType != "" {
		dataQuery += fmt.Sprintf(` AND source_type = $%d`, argIdx)
		dataArgs = append(dataArgs, sourceType)
		argIdx++
	}
	dataQuery += fmt.Sprintf(` ORDER BY priority ASC, created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	dataArgs = append(dataArgs, pageSize, offset)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list account sources: %w", err)
	}
	defer rows.Close()

	return scanAccountSources(rows, total)
}

// Update modifies the updatable fields of an account source.
func (r *AccountSourceRepo) Update(ctx context.Context, id string, params UpdateAccountSourceParams) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE account_sources SET
			name = COALESCE($2, name),
			priority = COALESCE($3, priority),
			weight = COALESCE($4, weight),
			max_concurrency = COALESCE($5, max_concurrency),
			daily_budget_micro_usd = COALESCE($6, daily_budget_micro_usd),
			model_policy_json = COALESCE($7, model_policy_json),
			updated_at = NOW()
		 WHERE id = $1`,
		id, params.Name, params.Priority, params.Weight,
		params.MaxConcurrency, params.DailyBudgetMicroUSD, params.ModelPolicyJSON,
	)
	if err != nil {
		return fmt.Errorf("update account source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update account source: source %s not found", id)
	}
	return nil
}

// Disable marks an account source as disabled.
func (r *AccountSourceRepo) Disable(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE account_sources SET status = 'disabled', updated_at = NOW() WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("disable account source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("disable account source: source %s not found", id)
	}
	return nil
}

// UpdateHealth sets the health_state and last_validated_at for a source.
func (r *AccountSourceRepo) UpdateHealth(ctx context.Context, id, healthState string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE account_sources SET health_state = $2, last_validated_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id, healthState,
	)
	if err != nil {
		return fmt.Errorf("update account source health: %w", err)
	}
	return nil
}

// UpdateLastUsed sets the last_used_at timestamp to NOW().
func (r *AccountSourceRepo) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE account_sources SET last_used_at = NOW(), updated_at = NOW() WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("update account source last used: %w", err)
	}
	return nil
}

// ListRoutingCandidates returns active, healthy sources that support the given
// model, ordered by routing priority (priority ASC, weight DESC).
//
// The query joins account_sources → channel_sources → channels →
// model_mappings to find sources that are mapped to the given external model.
// Sources that are disabled, dead, or have a circuit open are excluded.
func (r *AccountSourceRepo) ListRoutingCandidates(ctx context.Context, model string) ([]AccountSource, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT acs.id, acs.name, acs.source_type, acs.provider_id, acs.endpoint,
		        acs.credential_ciphertext, acs.credential_key_version, acs.credential_fingerprint,
		        acs.model_policy_json, acs.priority, acs.weight, acs.max_concurrency,
		        acs.daily_budget_micro_usd, acs.subscription_accounts_count,
		        acs.arkose_solver_config_json, acs.status, acs.health_state,
		        acs.last_validated_at, acs.last_used_at, acs.created_by, acs.created_at, acs.updated_at
		 FROM account_sources acs
		 JOIN channel_sources cs ON cs.source_id = acs.id
		 JOIN channels ch ON ch.id = cs.channel_id AND ch.status = 'active'
		 JOIN model_mappings mm ON (mm.channel_id = ch.id OR mm.channel_id IS NULL)
		 WHERE acs.status = 'active'
		   AND acs.health_state NOT IN ('dead', 'circuit_open')
		   AND mm.external_model = $1
		 ORDER BY acs.priority ASC, acs.weight DESC`, model,
	)
	if err != nil {
		return nil, fmt.Errorf("list routing candidates: %w", err)
	}
	defer rows.Close()

	sources, _, err := scanAccountSources(rows, -1)
	return sources, err
}

// scanAccountSources reads account source rows into a slice.
func scanAccountSources(rows pgx.Rows, total int) ([]AccountSource, int, error) {
	var sources []AccountSource
	for rows.Next() {
		var s AccountSource
		if err := rows.Scan(
			&s.ID, &s.Name, &s.SourceType, &s.ProviderID, &s.Endpoint,
			&s.CredentialCiphertext, &s.CredentialKeyVersion, &s.CredentialFingerprint,
			&s.ModelPolicyJSON, &s.Priority, &s.Weight, &s.MaxConcurrency,
			&s.DailyBudgetMicroUSD, &s.SubscriptionAccountsCount,
			&s.ArkoseSolverConfigJSON, &s.Status, &s.HealthState,
			&s.LastValidatedAt, &s.LastUsedAt, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan account source: %w", err)
		}
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter account sources: %w", err)
	}
	if sources == nil {
		sources = []AccountSource{}
	}
	return sources, total, nil
}
