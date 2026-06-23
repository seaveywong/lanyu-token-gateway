package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Project represents a row in the projects table.
type Project struct {
	ID                    string    `json:"id"`
	OrganizationID        string    `json:"organization_id"`
	Name                  string    `json:"name"`
	Description           *string   `json:"description"`
	Status                string    `json:"status"`
	DailyBudgetMicroUSD   int64     `json:"daily_budget_micro_usd"`
	MonthlyBudgetMicroUSD int64     `json:"monthly_budget_micro_usd"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// ProjectRepo provides CRUD operations on the projects table.
type ProjectRepo struct {
	pool *pgxpool.Pool
}

// NewProjectRepo returns a ProjectRepo backed by the given connection pool.
func NewProjectRepo(pool *pgxpool.Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

// Create inserts a new project and returns it.
func (r *ProjectRepo) Create(ctx context.Context, orgID, name, description string) (*Project, error) {
	var p Project
	err := r.pool.QueryRow(ctx,
		`INSERT INTO projects (organization_id, name, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, organization_id, name, description, status,
		           daily_budget_micro_usd, monthly_budget_micro_usd,
		           created_at, updated_at`,
		orgID, name, description,
	).Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Status,
		&p.DailyBudgetMicroUSD, &p.MonthlyBudgetMicroUSD,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	return &p, nil
}

// FindByID looks up a project by UUID.
func (r *ProjectRepo) FindByID(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, description, status,
		        daily_budget_micro_usd, monthly_budget_micro_usd,
		        created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Status,
		&p.DailyBudgetMicroUSD, &p.MonthlyBudgetMicroUSD,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find project by id: %w", err)
	}
	return &p, nil
}

// ListByOrg returns all projects belonging to an organization.
func (r *ProjectRepo) ListByOrg(ctx context.Context, orgID string) ([]Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, name, description, status,
		        daily_budget_micro_usd, monthly_budget_micro_usd,
		        created_at, updated_at
		 FROM projects
		 WHERE organization_id = $1
		 ORDER BY created_at DESC`, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects by org: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(
			&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Status,
			&p.DailyBudgetMicroUSD, &p.MonthlyBudgetMicroUSD,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter projects: %w", err)
	}
	return projects, nil
}

// Update changes the project name and description.
func (r *ProjectRepo) Update(ctx context.Context, id, name, description string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects SET name = $2, description = $3, updated_at = NOW()
		 WHERE id = $1`,
		id, name, description,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update project: project %s not found", id)
	}
	return nil
}

// UpdateBudget changes the daily and monthly budget limits for a project.
func (r *ProjectRepo) UpdateBudget(ctx context.Context, id string, dailyBudget, monthlyBudget int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects
		 SET daily_budget_micro_usd = $2,
		     monthly_budget_micro_usd = $3,
		     updated_at = NOW()
		 WHERE id = $1`,
		id, dailyBudget, monthlyBudget,
	)
	if err != nil {
		return fmt.Errorf("update project budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update project budget: project %s not found", id)
	}
	return nil
}

// Delete removes a project by ID (hard delete).
func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM projects WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete project: project %s not found", id)
	}
	return nil
}
