// Package migrate provides embedded database migrations using goose.
// Migrations are embedded from the ../migrations directory at compile time.
package migrate

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed ../migrations/*.sql
var migrations embed.FS

// Up runs all pending migrations against the database.
func Up(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(db, "../migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// Down rolls back the last migration.
func Down(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Down(db, "../migrations"); err != nil {
		return fmt.Errorf("rollback migration: %w", err)
	}
	return nil
}

// Version returns the current database version.
func Version(db *sql.DB) (int64, error) {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return 0, fmt.Errorf("set dialect: %w", err)
	}
	current, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("get version: %w", err)
	}
	return current, nil
}

// Status prints migration status to stdout (for CLI use).
func Status(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	return goose.Status(db, "../migrations")
}
