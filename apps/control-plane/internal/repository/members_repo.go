package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Member represents a row in the organization_members table, with optional
// organization name joined from the organizations table.
type Member struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organization_id"`
	UserID          string    `json:"user_id"`
	Role            string    `json:"role"`
	OrgName         string    `json:"org_name"` // joined from organizations
	CreatedAt       time.Time `json:"created_at"`
}

// MemberRepo provides CRUD operations on the organization_members table.
type MemberRepo struct {
	pool *pgxpool.Pool
}

// NewMemberRepo returns a MemberRepo backed by the given connection pool.
func NewMemberRepo(pool *pgxpool.Pool) *MemberRepo {
	return &MemberRepo{pool: pool}
}

// Add inserts a new organization member.
func (r *MemberRepo) Add(ctx context.Context, orgID, userID, role string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO organization_members (organization_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (organization_id, user_id) DO UPDATE SET role = $3`,
		orgID, userID, role,
	)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// Remove deletes an organization member.
func (r *MemberRepo) Remove(ctx context.Context, orgID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM organization_members
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("remove member: member not found in org %s", orgID)
	}
	return nil
}

// GetRole returns the role for a specific member.
func (r *MemberRepo) GetRole(ctx context.Context, orgID, userID string) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM organization_members
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("get role: member not found")
		}
		return "", fmt.Errorf("get role: %w", err)
	}
	return role, nil
}

// ListByOrg returns all members of an organization.
func (r *MemberRepo) ListByOrg(ctx context.Context, orgID string) ([]Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT om.id, om.organization_id, om.user_id, om.role,
		        o.name AS org_name, om.created_at
		 FROM organization_members om
		 INNER JOIN organizations o ON o.id = om.organization_id
		 WHERE om.organization_id = $1
		 ORDER BY om.created_at ASC`, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members by org: %w", err)
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role,
			&m.OrgName, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter members: %w", err)
	}
	return members, nil
}

// ListByUser returns all memberships for a user, with org name joined.
func (r *MemberRepo) ListByUser(ctx context.Context, userID string) ([]Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT om.id, om.organization_id, om.user_id, om.role,
		        o.name AS org_name, om.created_at
		 FROM organization_members om
		 INNER JOIN organizations o ON o.id = om.organization_id
		 WHERE om.user_id = $1
		 ORDER BY om.created_at ASC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members by user: %w", err)
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role,
			&m.OrgName, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iter members: %w", err)
	}
	return members, nil
}

// UpdateRole changes the role of an organization member.
func (r *MemberRepo) UpdateRole(ctx context.Context, orgID, userID, role string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organization_members SET role = $3
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID, role,
	)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update member role: member not found")
	}
	return nil
}
