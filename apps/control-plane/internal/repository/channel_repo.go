package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Channel represents a row in the channels table.
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChannelRepo provides CRUD operations on the channels table and manages the
// channel_sources join table.
type ChannelRepo struct {
	pool *pgxpool.Pool
}

// NewChannelRepo returns a ChannelRepo backed by the given connection pool.
func NewChannelRepo(pool *pgxpool.Pool) *ChannelRepo {
	return &ChannelRepo{pool: pool}
}

// Create inserts a new channel and returns the created row.
func (r *ChannelRepo) Create(ctx context.Context, name, description string) (*Channel, error) {
	var c Channel
	var desc *string
	if description != "" {
		desc = &description
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO channels (name, description)
		 VALUES ($1, $2)
		 RETURNING id, name, description, status, created_at, updated_at`,
		name, desc,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert channel: %w", err)
	}
	return &c, nil
}

// FindByID looks up a channel by its primary key UUID.
// Returns nil if not found.
func (r *ChannelRepo) FindByID(ctx context.Context, id string) (*Channel, error) {
	var c Channel
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, status, created_at, updated_at
		 FROM channels WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find channel by id: %w", err)
	}
	return &c, nil
}

// List returns all channels ordered by creation time descending.
func (r *ChannelRepo) List(ctx context.Context) ([]Channel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, description, status, created_at, updated_at
		 FROM channels ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	return scanChannels(rows)
}

// AddSource associates an account source with a channel.
func (r *ChannelRepo) AddSource(ctx context.Context, channelID, sourceID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO channel_sources (channel_id, source_id)
		 VALUES ($1, $2)
		 ON CONFLICT (channel_id, source_id) DO NOTHING`,
		channelID, sourceID,
	)
	if err != nil {
		return fmt.Errorf("add source to channel: %w", err)
	}
	return nil
}

// RemoveSource dissociates an account source from a channel.
func (r *ChannelRepo) RemoveSource(ctx context.Context, channelID, sourceID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM channel_sources WHERE channel_id = $1 AND source_id = $2`,
		channelID, sourceID,
	)
	if err != nil {
		return fmt.Errorf("remove source from channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("remove source from channel: association not found")
	}
	return nil
}

// ListSources returns all account sources associated with a channel, ordered
// by priority.
func (r *ChannelRepo) ListSources(ctx context.Context, channelID string) ([]AccountSource, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT acs.id, acs.name, acs.source_type, acs.provider_id, acs.endpoint,
		        acs.credential_ciphertext, acs.credential_key_version, acs.credential_fingerprint,
		        acs.model_policy_json, acs.priority, acs.weight, acs.max_concurrency,
		        acs.daily_budget_micro_usd, acs.subscription_accounts_count,
		        acs.arkose_solver_config_json, acs.status, acs.health_state,
		        acs.last_validated_at, acs.last_used_at, acs.created_by, acs.created_at, acs.updated_at
		 FROM account_sources acs
		 JOIN channel_sources cs ON cs.source_id = acs.id
		 WHERE cs.channel_id = $1
		 ORDER BY acs.priority ASC, acs.created_at DESC`, channelID,
	)
	if err != nil {
		return nil, fmt.Errorf("list channel sources: %w", err)
	}
	defer rows.Close()

	sources, _, err := scanAccountSources(rows, -1)
	return sources, err
}

// scanChannels reads channel rows into a slice.
func scanChannels(rows pgx.Rows) ([]Channel, error) {
	var channels []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &c.Status, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter channels: %w", err)
	}
	if channels == nil {
		channels = []Channel{}
	}
	return channels, nil
}
