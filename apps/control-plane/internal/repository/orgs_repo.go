package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Organization represents a row in the organizations table.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OrgRepo provides CRUD operations on the organizations table.
type OrgRepo struct {
	pool *pgxpool.Pool
}

// NewOrgRepo returns an OrgRepo backed by the given connection pool.
func NewOrgRepo(pool *pgxpool.Pool) *OrgRepo {
	return &OrgRepo{pool: pool}
}

// Create inserts a new organization and returns it.
func (r *OrgRepo) Create(ctx context.Context, name, slug string) (*Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug)
		 VALUES ($1, $2)
		 RETURNING id, name, slug, created_at, updated_at`,
		name, slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert organization: %w", err)
	}
	return &o, nil
}

// FindByID looks up an organization by UUID.
func (r *OrgRepo) FindByID(ctx context.Context, id string) (*Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at, updated_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find org by id: %w", err)
	}
	return &o, nil
}

// FindBySlug looks up an organization by its unique slug.
func (r *OrgRepo) FindBySlug(ctx context.Context, slug string) (*Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, created_at, updated_at
		 FROM organizations WHERE slug = $1`, slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find org by slug: %w", err)
	}
	return &o, nil
}

// ListByUserID returns all organizations the user is a member of.
func (r *OrgRepo) ListByUserID(ctx context.Context, userID string) ([]Organization, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.id, o.name, o.slug, o.created_at, o.updated_at
		 FROM organizations o
		 INNER JOIN organization_members om ON om.organization_id = o.id
		 WHERE om.user_id = $1
		 ORDER BY o.created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list orgs by user: %w", err)
	}
	defer rows.Close()

	var orgs []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan org: %w", err)
		}
		orgs = append(orgs, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter orgs: %w", err)
	}
	return orgs, nil
}

// Update changes the organization name.
func (r *OrgRepo) Update(ctx context.Context, id, name string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations SET name = $2, updated_at = NOW() WHERE id = $1`,
		id, name,
	)
	if err != nil {
		return fmt.Errorf("update org: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update org: organization %s not found", id)
	}
	return nil
}

// Delete removes an organization by ID (hard delete).
func (r *OrgRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM organizations WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("delete org: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete org: organization %s not found", id)
	}
	return nil
}
