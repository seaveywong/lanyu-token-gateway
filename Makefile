.PHONY: help build test lint clean dev docker-up docker-down db-migrate db-reset

# Default target
.DEFAULT_GOAL := help

# Variables
GO_MODULES := apps/data-plane apps/control-plane apps/async-worker packages/config packages/provider-sdk packages/observability packages/contracts
WEB_APPS := apps/admin-web apps/portal-web
COMPOSE_FILE := deploy/compose/docker-compose.yml

## help: Show this help
help:
	@grep -E '^##' Makefile | cut -c 4- | sort

## build: Build all Go services
build:
	@for mod in $(GO_MODULES); do \
		echo "Building $$mod..."; \
		cd $$mod && go build -o /dev/null ./... ; cd - > /dev/null; \
	done

## build-web: Build all web apps
build-web:
	@pnpm run build:admin
	@pnpm run build:portal

## test: Run all Go tests
test:
	@for mod in $(GO_MODULES); do \
		echo "Testing $$mod..."; \
		cd $$mod && go test -race -count=1 ./... ; cd - > /dev/null; \
	done

## lint: Run all linters
lint:
	@echo "Linting Go..."
	@for mod in $(GO_MODULES); do \
		cd $$mod && golangci-lint run ./... ; cd - > /dev/null; \
	done
	@echo "Linting web..."
	@pnpm run lint

## fmt: Format all code
fmt:
	@for mod in $(GO_MODULES); do \
		cd $$mod && go fmt ./... && go vet ./... ; cd - > /dev/null; \
	done
	@pnpm run --parallel format 2>/dev/null || true

## clean: Remove build artifacts
clean:
	@rm -rf apps/*/tmp packages/*/tmp
	@find . -name "*.test" -delete

## dev: Start development environment (Docker Compose)
dev:
	docker compose -f $(COMPOSE_FILE) up -d

## dev-down: Stop development environment
dev-down:
	docker compose -f $(COMPOSE_FILE) down

## dev-logs: Tail development logs
dev-logs:
	docker compose -f $(COMPOSE_FILE) logs -f

## db-migrate: Run database migrations
db-migrate:
	@echo "Running goose migrations..."
	@cd db && goose postgres "$$DATABASE_URL" up

## db-reset: Reset database (drop all, re-migrate, seed)
db-reset:
	@echo "Resetting database..."
	@cd db && goose postgres "$$DATABASE_URL" reset
	@cd db && goose postgres "$$DATABASE_URL" up
	@echo "Seeding..."
	@psql "$$DATABASE_URL" -f db/seed/seed.sql

## db-status: Show migration status
db-status:
	@cd db && goose postgres "$$DATABASE_URL" status

## db-new name=<name>: Create a new migration
db-new:
	@cd db && goose create $(name) sql

## install: Install all dependencies
install:
	@echo "Installing Go dependencies..."
	@for mod in $(GO_MODULES); do \
		cd $$mod && go mod tidy ; cd - > /dev/null; \
	done
	@echo "Installing Node dependencies..."
	@pnpm install

## docker-build: Build all Docker images
docker-build:
	docker compose -f $(COMPOSE_FILE) build

## check-secrets: Run secret scanner
check-secrets:
	gitleaks detect --source . --verbose

## all: Install, lint, test, build
all: install lint test build build-web

## gen: Generate code (sqlc, OpenAPI client)
gen:
	@echo "Generating sqlc..."
	@cd db && sqlc generate
	@echo "Generating OpenAPI..."
	# Placeholder for OpenAPI codegen
