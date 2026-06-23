# Token Gateway — AI Agent Development Guide

## Project Overview

Token Gateway is a multi-vendor AI API gateway. It provides a unified
OpenAI-compatible API to customers while managing routing, billing, and
operational control across multiple upstream providers.

Tech stack: Go 1.24 + PostgreSQL 15 + Redis 7 + React 19 + TypeScript + Vite 6.

## Repository Structure

```
Token/
  apps/
    data-plane/          Go — public API gateway (port 8080)
    control-plane/       Go — admin & portal API (port 8081)
    async-worker/        Go — background jobs
    admin-web/           React — operator admin console (port 5173)
    portal-web/          React — customer portal (port 5174)
    edge-gateway/        Cloudflare Worker — edge proxy
  packages/
    contracts/           OpenAPI 3.1 specs + shared error codes
    provider-sdk/        Go — Provider Adapter interfaces
    config/              Go — configuration loading & validation
    observability/       Go — OpenTelemetry setup + slog
  db/
    migrations/          goose SQL migrations
    queries/             sqlc query templates
    seed/                dev seed data
  deploy/
    compose/             Docker Compose for local dev
    runbooks/            Operational runbooks
  docs/                  Design & architecture documents
```

## Getting Started

### Prerequisites

- Go 1.24+
- Node.js 22+
- pnpm 9+
- Docker & Docker Compose
- goose (DB migrations): `go install github.com/pressly/goose/v3/cmd/goose@latest`
- sqlc (code generation): `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- gitleaks (secret scanning): `go install github.com/gitleaks/gitleaks/v8@latest`

### Quick Start

```bash
# Install all dependencies
make install

# Start development environment
make dev

# Run database migrations
export DATABASE_URL=postgres://token:token_dev@localhost:5432/token_gateway?sslmode=disable
make db-migrate

# Check everything works
curl http://localhost:8080/health
curl http://localhost:8081/health

# Start web UIs
pnpm dev:admin   # http://localhost:5173
pnpm dev:portal  # http://localhost:5174
```

## Key Design Rules (MUST)

### Security
- Never log API keys, tokens, authorization headers, or prompt content.
- Credentials are envelope-encrypted with AES-256-GCM at rest.
- Use parameterized SQL — never string concatenation for queries.
- Admin mutations require audit log entries.

### API
- Public API uses OpenAI-compatible format under `/v1`.
- All errors use the standard GatewayError envelope (see `packages/contracts/errors.go`).
- Never expose internal source IDs, upstream URLs, or channel names in API responses.

### Routing
- Route priority: official_api_key > official_oauth > subscription_pool > upstream_api.
- Within a subscription pool, select least-loaded + weighted round-robin.
- Never retry non-idempotent requests after upstream may have accepted them.

### Database
- All tables include `organization_id` for multi-tenant isolation.
- Monetary amounts use micro_usd (BIGINT integer micro-dollars) — never float.
- `usage_events`, `audit_logs` use monthly partitioning in production.
- Ledger is double-entry — every debit has a matching credit.

### Code Quality
- All Go code passes `golangci-lint` and `go vet`.
- All SQL changes go through goose migrations.
- New provider adapters must implement the full `ProviderAdapter` interface.
- Frontend never performs authorization decisions — that's the backend's job.

## Adding a New Provider

1. Create adapter in `apps/data-plane/internal/providers/<name>/`.
2. Implement `provider_sdk.ProviderAdapter` interface.
3. Register in the adapter registry.
4. Add test fixtures.
5. Add to `PROVIDER_CAPABILITY_MATRIX.md`.
6. Create migration if new credential types needed.

## Running Tests

```bash
# All Go tests
make test

# Single package
cd apps/data-plane && go test ./internal/...

# With race detection
go test -race ./...

# Frontend
pnpm lint
pnpm typecheck
```

## Before Committing

1. `make fmt` — format all code
2. `make lint` — no lint errors
3. `make test` — all tests pass
4. `make check-secrets` — no secrets in code
5. Ensure no `.env` files are staged
6. Write conventional commit messages
