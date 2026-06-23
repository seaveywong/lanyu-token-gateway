// Package repository provides direct PostgreSQL data access for the control plane.
// Each repository wraps a *pgxpool.Pool and uses parameterized SQL for all queries.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents a row in the users table.
type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	DisplayName     *string    `json:"display_name"`
	AvatarURL       *string    `json:"avatar_url"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	MFAEnabled      bool       `json:"mfa_enabled"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateUserParams holds the data needed to insert a new user.
type CreateUserParams struct {
	Email        string
	PasswordHash string
	DisplayName  string
}

// UserRepo provides CRUD operations on the users table.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo returns a UserRepo backed by the given connection pool.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// CreateUser inserts a new user and returns the created row.
func (r *UserRepo) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, display_name, avatar_url,
		           email_verified_at, mfa_enabled, created_at, updated_at`,
		params.Email, params.PasswordHash, params.DisplayName,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName,
		&u.AvatarURL, &u.EmailVerifiedAt, &u.MFAEnabled,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

// FindByEmail looks up a user by email. Returns nil if not found.
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, display_name, avatar_url,
		        email_verified_at, mfa_enabled, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName,
		&u.AvatarURL, &u.EmailVerifiedAt, &u.MFAEnabled,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &u, nil
}

// FindByID looks up a user by UUID.
func (r *UserRepo) FindByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, display_name, avatar_url,
		        email_verified_at, mfa_enabled, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName,
		&u.AvatarURL, &u.EmailVerifiedAt, &u.MFAEnabled,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return &u, nil
}

// UpdateMFA enables or disables MFA for the user. The totpSecret is stored
// externally (e.g. Redis) — this method only toggles the mfa_enabled flag
// in the database.
func (r *UserRepo) UpdateMFA(ctx context.Context, id, totpSecret string, enabled bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET mfa_enabled = $2, updated_at = NOW() WHERE id = $1`,
		id, enabled,
	)
	if err != nil {
		return fmt.Errorf("update mfa: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update mfa: user %s not found", id)
	}
	return nil
}

// UpdatePassword changes a user's password hash.
func (r *UserRepo) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`,
		id, passwordHash,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update password: user %s not found", id)
	}
	return nil
}

// VerifyEmail marks the user's email as verified.
func (r *UserRepo) VerifyEmail(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET email_verified_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("verify email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("verify email: user %s not found", id)
	}
	return nil
}
