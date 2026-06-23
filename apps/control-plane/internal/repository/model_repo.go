package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ModelCatalog represents a row in the model_catalog table.
type ModelCatalog struct {
	ID                string  `json:"id"`
	ProviderID        string  `json:"provider_id"`
	ModelName         string  `json:"model_name"`
	DisplayName       *string `json:"display_name"`
	Modality          string  `json:"modality"`
	InputTokenLimit   *int    `json:"input_token_limit"`
	OutputTokenLimit  *int    `json:"output_token_limit"`
	SupportsStreaming bool    `json:"supports_streaming"`
	SupportsTools     bool    `json:"supports_tools"`
	SupportsVision    bool    `json:"supports_vision"`
	SupportsAudio     bool    `json:"supports_audio"`
	CreatedAt         time.Time `json:"created_at"`
}

// CreateModelCatalogParams holds the data needed to insert a model catalog entry.
type CreateModelCatalogParams struct {
	ProviderID        string
	ModelName         string
	DisplayName       *string
	Modality          string
	InputTokenLimit   *int
	OutputTokenLimit  *int
	SupportsStreaming bool
	SupportsTools     bool
	SupportsVision    bool
	SupportsAudio     bool
}

// ModelMapping represents a row in the model_mappings table.
type ModelMapping struct {
	ID             string  `json:"id"`
	ExternalModel  string  `json:"external_model"`
	ChannelID      *string `json:"channel_id"`
	NativeModel    string  `json:"native_model"`
	CostMultiplier float64 `json:"cost_multiplier"`
	CreatedAt      time.Time `json:"created_at"`
}

// RouteRule represents a row in the route_rules table.
type RouteRule struct {
	ID             string  `json:"id"`
	OrganizationID *string `json:"organization_id"`
	ProjectID      *string `json:"project_id"`
	ModelName      *string `json:"model_name"`
	ChannelID      *string `json:"channel_id"`
	Priority       int     `json:"priority"`
	Weight         int     `json:"weight"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateRouteRuleParams holds the data needed to create a route rule.
type CreateRouteRuleParams struct {
	OrganizationID *string
	ProjectID      *string
	ModelName      *string
	ChannelID      *string
	Priority       int
	Weight         int
}

// ModelRepo provides CRUD operations on model_catalog, model_mappings, and
// route_rules tables.
type ModelRepo struct {
	pool *pgxpool.Pool
}

// NewModelRepo returns a ModelRepo backed by the given connection pool.
func NewModelRepo(pool *pgxpool.Pool) *ModelRepo {
	return &ModelRepo{pool: pool}
}

// --- Model Catalog ---

// CreateCatalog inserts a new model catalog entry and returns the created row.
func (r *ModelRepo) CreateCatalog(ctx context.Context, params CreateModelCatalogParams) (*ModelCatalog, error) {
	var m ModelCatalog
	err := r.pool.QueryRow(ctx,
		`INSERT INTO model_catalog (provider_id, model_name, display_name, modality,
		                            input_token_limit, output_token_limit,
		                            supports_streaming, supports_tools,
		                            supports_vision, supports_audio)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, provider_id, model_name, display_name, modality,
		           input_token_limit, output_token_limit, supports_streaming,
		           supports_tools, supports_vision, supports_audio, created_at`,
		params.ProviderID, params.ModelName, params.DisplayName, params.Modality,
		params.InputTokenLimit, params.OutputTokenLimit,
		params.SupportsStreaming, params.SupportsTools,
		params.SupportsVision, params.SupportsAudio,
	).Scan(
		&m.ID, &m.ProviderID, &m.ModelName, &m.DisplayName, &m.Modality,
		&m.InputTokenLimit, &m.OutputTokenLimit, &m.SupportsStreaming,
		&m.SupportsTools, &m.SupportsVision, &m.SupportsAudio, &m.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert model catalog: %w", err)
	}
	return &m, nil
}

// ListCatalog returns all model catalog entries.
func (r *ModelRepo) ListCatalog(ctx context.Context) ([]ModelCatalog, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, provider_id, model_name, display_name, modality,
		        input_token_limit, output_token_limit, supports_streaming,
		        supports_tools, supports_vision, supports_audio, created_at
		 FROM model_catalog ORDER BY provider_id, model_name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list model catalog: %w", err)
	}
	defer rows.Close()

	return scanModelCatalogs(rows)
}

// FindCatalogByModel looks up a model catalog entry by its model name.
// Returns nil if not found.
func (r *ModelRepo) FindCatalogByModel(ctx context.Context, modelName string) (*ModelCatalog, error) {
	var m ModelCatalog
	err := r.pool.QueryRow(ctx,
		`SELECT id, provider_id, model_name, display_name, modality,
		        input_token_limit, output_token_limit, supports_streaming,
		        supports_tools, supports_vision, supports_audio, created_at
		 FROM model_catalog WHERE model_name = $1`, modelName,
	).Scan(
		&m.ID, &m.ProviderID, &m.ModelName, &m.DisplayName, &m.Modality,
		&m.InputTokenLimit, &m.OutputTokenLimit, &m.SupportsStreaming,
		&m.SupportsTools, &m.SupportsVision, &m.SupportsAudio, &m.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find model catalog by model: %w", err)
	}
	return &m, nil
}

// --- Model Mappings ---

// CreateMapping inserts a new model mapping and returns the created row.
// channelID may be empty, in which case NULL is stored.
func (r *ModelRepo) CreateMapping(ctx context.Context, externalModel, channelID, nativeModel string, costMultiplier float64) (*ModelMapping, error) {
	var channelIDPtr any
	if channelID == "" {
		channelIDPtr = nil
	} else {
		channelIDPtr = channelID
	}

	var m ModelMapping
	err := r.pool.QueryRow(ctx,
		`INSERT INTO model_mappings (external_model, channel_id, native_model, cost_multiplier)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, external_model, channel_id, native_model, cost_multiplier, created_at`,
		externalModel, channelIDPtr, nativeModel, costMultiplier,
	).Scan(&m.ID, &m.ExternalModel, &m.ChannelID, &m.NativeModel, &m.CostMultiplier, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert model mapping: %w", err)
	}
	return &m, nil
}

// ListMappings returns all model mappings.
func (r *ModelRepo) ListMappings(ctx context.Context) ([]ModelMapping, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, external_model, channel_id, native_model, cost_multiplier, created_at
		 FROM model_mappings ORDER BY external_model, created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list model mappings: %w", err)
	}
	defer rows.Close()

	return scanModelMappings(rows)
}

// FindMapping returns all model mappings for a given external model name.
func (r *ModelRepo) FindMapping(ctx context.Context, externalModel string) ([]ModelMapping, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, external_model, channel_id, native_model, cost_multiplier, created_at
		 FROM model_mappings WHERE external_model = $1
		 ORDER BY created_at`, externalModel,
	)
	if err != nil {
		return nil, fmt.Errorf("find model mapping: %w", err)
	}
	defer rows.Close()

	return scanModelMappings(rows)
}

// --- Route Rules ---

// CreateRouteRule inserts a new route rule and returns the created row.
func (r *ModelRepo) CreateRouteRule(ctx context.Context, params CreateRouteRuleParams) (*RouteRule, error) {
	var rr RouteRule
	err := r.pool.QueryRow(ctx,
		`INSERT INTO route_rules (organization_id, project_id, model_name, channel_id,
		                          priority, weight)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, organization_id, project_id, model_name, channel_id,
		           priority, weight, created_at`,
		params.OrganizationID, params.ProjectID, params.ModelName, params.ChannelID,
		params.Priority, params.Weight,
	).Scan(
		&rr.ID, &rr.OrganizationID, &rr.ProjectID, &rr.ModelName, &rr.ChannelID,
		&rr.Priority, &rr.Weight, &rr.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert route rule: %w", err)
	}
	return &rr, nil
}

// ListRouteRules returns all route rules ordered by priority.
func (r *ModelRepo) ListRouteRules(ctx context.Context) ([]RouteRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, project_id, model_name, channel_id,
		        priority, weight, created_at
		 FROM route_rules ORDER BY priority ASC, created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list route rules: %w", err)
	}
	defer rows.Close()

	return scanRouteRules(rows)
}

// --- Scan helpers ---

func scanModelCatalogs(rows pgx.Rows) ([]ModelCatalog, error) {
	var entries []ModelCatalog
	for rows.Next() {
		var m ModelCatalog
		if err := rows.Scan(
			&m.ID, &m.ProviderID, &m.ModelName, &m.DisplayName, &m.Modality,
			&m.InputTokenLimit, &m.OutputTokenLimit, &m.SupportsStreaming,
			&m.SupportsTools, &m.SupportsVision, &m.SupportsAudio, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model catalog: %w", err)
		}
		entries = append(entries, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter model catalog: %w", err)
	}
	if entries == nil {
		entries = []ModelCatalog{}
	}
	return entries, nil
}

func scanModelMappings(rows pgx.Rows) ([]ModelMapping, error) {
	var mappings []ModelMapping
	for rows.Next() {
		var m ModelMapping
		if err := rows.Scan(
			&m.ID, &m.ExternalModel, &m.ChannelID, &m.NativeModel, &m.CostMultiplier, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter model mappings: %w", err)
	}
	if mappings == nil {
		mappings = []ModelMapping{}
	}
	return mappings, nil
}

func scanRouteRules(rows pgx.Rows) ([]RouteRule, error) {
	var rules []RouteRule
	for rows.Next() {
		var rr RouteRule
		if err := rows.Scan(
			&rr.ID, &rr.OrganizationID, &rr.ProjectID, &rr.ModelName, &rr.ChannelID,
			&rr.Priority, &rr.Weight, &rr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan route rule: %w", err)
		}
		rules = append(rules, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter route rules: %w", err)
	}
	if rules == nil {
		rules = []RouteRule{}
	}
	return rules, nil
}
