package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PricingVersion represents a row in the pricing_versions table.
type PricingVersion struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	EffectiveAt time.Time `json:"effective_at"`
	CreatedBy   *string   `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// PricingRule represents a row in the pricing_rules table.
type PricingRule struct {
	ID                   string `json:"id"`
	VersionID            string `json:"version_id"`
	ModelName            string `json:"model_name"`
	InputPriceMicroUSD   int64  `json:"input_price_micro_usd"`
	OutputPriceMicroUSD  int64  `json:"output_price_micro_usd"`
	CachedPriceMicroUSD  int64  `json:"cached_price_micro_usd"`
	ImagePriceMicroUSD   int64  `json:"image_price_micro_usd"`
	AudioPriceMicroUSD   int64  `json:"audio_price_micro_usd"`
}

// CreatePricingVersionParams holds the data to create a new pricing version.
type CreatePricingVersionParams struct {
	Name        string
	Description *string
	CreatedBy   *string
}

// PricingRepo provides CRUD operations on pricing_versions and pricing_rules.
type PricingRepo struct {
	pool *pgxpool.Pool
}

// NewPricingRepo returns a PricingRepo backed by the given connection pool.
func NewPricingRepo(pool *pgxpool.Pool) *PricingRepo {
	return &PricingRepo{pool: pool}
}

// CreateVersion creates a new pricing version (inactive by default).
func (r *PricingRepo) CreateVersion(ctx context.Context, params CreatePricingVersionParams) (*PricingVersion, error) {
	var v PricingVersion
	err := r.pool.QueryRow(ctx,
		`INSERT INTO pricing_versions (name, description, created_by)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, description, is_active, effective_at, created_by, created_at`,
		params.Name, params.Description, params.CreatedBy,
	).Scan(&v.ID, &v.Name, &v.Description, &v.IsActive, &v.EffectiveAt, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create pricing version: %w", err)
	}
	return &v, nil
}

// GetCurrentVersion returns the currently active pricing version.
func (r *PricingRepo) GetCurrentVersion(ctx context.Context) (*PricingVersion, error) {
	var v PricingVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, is_active, effective_at, created_by, created_at
		 FROM pricing_versions
		 WHERE is_active = TRUE
		 ORDER BY effective_at DESC
		 LIMIT 1`,
	).Scan(&v.ID, &v.Name, &v.Description, &v.IsActive, &v.EffectiveAt, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current pricing version: %w", err)
	}
	return &v, nil
}

// GetVersionByID returns a specific pricing version by ID (for historical lookup).
func (r *PricingRepo) GetVersionByID(ctx context.Context, id string) (*PricingVersion, error) {
	var v PricingVersion
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, is_active, effective_at, created_by, created_at
		 FROM pricing_versions
		 WHERE id = $1`,
		id,
	).Scan(&v.ID, &v.Name, &v.Description, &v.IsActive, &v.EffectiveAt, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get pricing version by id: %w", err)
	}
	return &v, nil
}

// CreateRule adds a pricing rule to a version.
func (r *PricingRepo) CreateRule(ctx context.Context, versionID, modelName string, inputPrice, outputPrice, cachedPrice, imagePrice, audioPrice int64) (*PricingRule, error) {
	var rule PricingRule
	err := r.pool.QueryRow(ctx,
		`INSERT INTO pricing_rules (version_id, model_name,
		     input_price_micro_usd, output_price_micro_usd,
		     cached_price_micro_usd, image_price_micro_usd, audio_price_micro_usd)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (version_id, model_name)
		 DO UPDATE SET
		     input_price_micro_usd = EXCLUDED.input_price_micro_usd,
		     output_price_micro_usd = EXCLUDED.output_price_micro_usd,
		     cached_price_micro_usd = EXCLUDED.cached_price_micro_usd,
		     image_price_micro_usd = EXCLUDED.image_price_micro_usd,
		     audio_price_micro_usd = EXCLUDED.audio_price_micro_usd
		 RETURNING id, version_id, model_name,
		     input_price_micro_usd, output_price_micro_usd,
		     cached_price_micro_usd, image_price_micro_usd, audio_price_micro_usd`,
		versionID, modelName, inputPrice, outputPrice, cachedPrice, imagePrice, audioPrice,
	).Scan(&rule.ID, &rule.VersionID, &rule.ModelName,
		&rule.InputPriceMicroUSD, &rule.OutputPriceMicroUSD,
		&rule.CachedPriceMicroUSD, &rule.ImagePriceMicroUSD, &rule.AudioPriceMicroUSD)
	if err != nil {
		return nil, fmt.Errorf("create pricing rule: %w", err)
	}
	return &rule, nil
}

// FindRulesByModel returns all pricing rules for a model across versions that
// are active (joined with pricing_versions where is_active = TRUE).
func (r *PricingRepo) FindRulesByModel(ctx context.Context, modelName string) ([]PricingRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT pr.id, pr.version_id, pr.model_name,
		        pr.input_price_micro_usd, pr.output_price_micro_usd,
		        pr.cached_price_micro_usd, pr.image_price_micro_usd, pr.audio_price_micro_usd
		 FROM pricing_rules pr
		 INNER JOIN pricing_versions pv ON pv.id = pr.version_id
		 WHERE pr.model_name = $1 AND pv.is_active = TRUE
		 ORDER BY pv.effective_at DESC`,
		modelName,
	)
	if err != nil {
		return nil, fmt.Errorf("find rules by model: %w", err)
	}
	defer rows.Close()

	var rules []PricingRule
	for rows.Next() {
		var rl PricingRule
		if err := rows.Scan(
			&rl.ID, &rl.VersionID, &rl.ModelName,
			&rl.InputPriceMicroUSD, &rl.OutputPriceMicroUSD,
			&rl.CachedPriceMicroUSD, &rl.ImagePriceMicroUSD, &rl.AudioPriceMicroUSD,
		); err != nil {
			return nil, fmt.Errorf("scan pricing rule: %w", err)
		}
		rules = append(rules, rl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter pricing rules: %w", err)
	}
	return rules, nil
}

// ActivateVersion deactivates all other versions and activates the specified one.
// This is done in a transaction to ensure at most one version is active.
func (r *PricingRepo) ActivateVersion(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("activate version: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Deactivate all versions
	_, err = tx.Exec(ctx,
		`UPDATE pricing_versions SET is_active = FALSE WHERE is_active = TRUE`)
	if err != nil {
		return fmt.Errorf("activate version: deactivate all: %w", err)
	}

	// Activate the target version
	tag, err := tx.Exec(ctx,
		`UPDATE pricing_versions SET is_active = TRUE, effective_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("activate version: activate target: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("activate version: version %s not found", id)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("activate version: commit: %w", err)
	}
	return nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique violation (code 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
