# Database Migrations

## Quick Start

```bash
# Set connection string
export DATABASE_URL=postgres://token:token_dev@localhost:5432/token_gateway?sslmode=disable

# Run migrations
cd db && goose postgres "$DATABASE_URL" up

# Check status
goose postgres "$DATABASE_URL" status

# Create new migration
goose create add_new_table sql
```

## Migration Files

Migrations follow goose naming: `NNNNN_description.sql` with `-- +goose Up` and `-- +goose Down` blocks.

## Queries

SQL query templates for `sqlc` code generation. Run `sqlc generate` from the `db/` directory to generate Go code.

## Seed Data

Development seed data in `seed/seed.sql`. Apply with:
```bash
psql "$DATABASE_URL" -f seed/seed.sql
```
