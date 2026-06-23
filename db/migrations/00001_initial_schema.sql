-- +goose Up
-- +goose StatementBegin

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users & Identity
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    avatar_url TEXT,
    email_verified_at TIMESTAMPTZ,
    mfa_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE organization_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(50) NOT NULL DEFAULT 'developer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, user_id)
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    daily_budget_micro_usd BIGINT DEFAULT 0,
    monthly_budget_micro_usd BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    project_id UUID NOT NULL REFERENCES projects(id),
    name VARCHAR(255) NOT NULL,
    environment VARCHAR(20) NOT NULL DEFAULT 'production',
    key_prefix VARCHAR(20) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    scopes_json JSONB DEFAULT '[]',
    model_policy_json JSONB DEFAULT '{}',
    ip_allowlist_json JSONB DEFAULT '[]',
    rate_limit_policy_id UUID,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Account Sources & Channels
CREATE TABLE providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    adapter_type VARCHAR(100) NOT NULL,
    official_api_base_url TEXT,
    oauth_endpoint TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE account_sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN ('official_api_key', 'official_oauth', 'upstream_api', 'subscription_pool')),
    provider_id UUID REFERENCES providers(id),
    endpoint TEXT,
    credential_ciphertext TEXT NOT NULL,
    credential_key_version INTEGER NOT NULL DEFAULT 1,
    credential_fingerprint VARCHAR(64) NOT NULL,
    model_policy_json JSONB DEFAULT '{}',
    priority INTEGER NOT NULL DEFAULT 10,
    weight INTEGER NOT NULL DEFAULT 1,
    max_concurrency INTEGER NOT NULL DEFAULT 10,
    daily_budget_micro_usd BIGINT DEFAULT 0,
    subscription_accounts_count INTEGER DEFAULT 0,
    arkose_solver_config_json JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    health_state VARCHAR(50) NOT NULL DEFAULT 'unknown',
    last_validated_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE subscription_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES account_sources(id) ON DELETE CASCADE,
    account_label VARCHAR(255),
    credential_type VARCHAR(50) NOT NULL CHECK (credential_type IN ('session_token', 'refresh_token', 'access_token', 'cookie_jar', 'har_archive')),
    credential_ciphertext TEXT NOT NULL,
    credential_key_version INTEGER NOT NULL DEFAULT 1,
    credential_fingerprint VARCHAR(64) NOT NULL,
    refresh_ciphertext TEXT,
    refresh_key_version INTEGER DEFAULT 1,
    cookie_jar_json JSONB,
    proxy_binding_json JSONB,
    token_expires_at TIMESTAMPTZ,
    quota_limit_per_window INTEGER DEFAULT 0,
    quota_remaining INTEGER DEFAULT 0,
    quota_window_seconds INTEGER DEFAULT 10800,
    cooldown_until TIMESTAMPTZ,
    consecutive_failures INTEGER DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cooldown', 'exhausted', 'dead', 'manual_disabled', 'manual_intervention')),
    last_used_at TIMESTAMPTZ,
    last_refreshed_at TIMESTAMPTZ,
    last_error_code VARCHAR(50),
    last_error_message TEXT,
    refresh_count INTEGER DEFAULT 0,
    total_refresh_failures INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE channel_sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    source_id UUID NOT NULL REFERENCES account_sources(id) ON DELETE CASCADE,
    UNIQUE(channel_id, source_id)
);

-- Model Catalog
CREATE TABLE model_catalog (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id UUID NOT NULL REFERENCES providers(id),
    model_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    modality VARCHAR(100) NOT NULL DEFAULT 'text',
    input_token_limit INTEGER,
    output_token_limit INTEGER,
    supports_streaming BOOLEAN DEFAULT TRUE,
    supports_tools BOOLEAN DEFAULT FALSE,
    supports_vision BOOLEAN DEFAULT FALSE,
    supports_audio BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE model_mappings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    external_model VARCHAR(255) NOT NULL,
    channel_id UUID REFERENCES channels(id),
    native_model VARCHAR(255) NOT NULL,
    cost_multiplier REAL NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE route_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES organizations(id),
    project_id UUID REFERENCES projects(id),
    model_name VARCHAR(255),
    channel_id UUID REFERENCES channels(id),
    priority INTEGER NOT NULL DEFAULT 10,
    weight INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Usage & Billing
CREATE TABLE usage_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id VARCHAR(64) NOT NULL UNIQUE,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    project_id UUID NOT NULL REFERENCES projects(id),
    api_key_id UUID REFERENCES api_keys(id),
    external_model VARCHAR(255),
    resolved_model VARCHAR(255),
    channel_id UUID REFERENCES channels(id),
    source_id UUID REFERENCES account_sources(id),
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cached_tokens INTEGER DEFAULT 0,
    modality_units JSONB DEFAULT '{}',
    provider_cost_micro_usd BIGINT DEFAULT 0,
    customer_charge_micro_usd BIGINT DEFAULT 0,
    pricing_version_id UUID,
    status VARCHAR(50) DEFAULT 'completed',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    project_id UUID REFERENCES projects(id),
    balance_micro_usd BIGINT NOT NULL DEFAULT 0,
    frozen_micro_usd BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'))
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    amount_micro_usd BIGINT NOT NULL,
    entry_type VARCHAR(50) NOT NULL,
    description TEXT,
    request_id VARCHAR(64),
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit & Events
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES organizations(id),
    actor_id UUID REFERENCES users(id),
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(100),
    resource_id UUID,
    metadata_json JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    trace_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE account_source_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES account_sources(id),
    account_id UUID REFERENCES subscription_accounts(id),
    event_type VARCHAR(100) NOT NULL,
    metadata_json JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type VARCHAR(255) NOT NULL,
    aggregate_id UUID NOT NULL,
    payload_json JSONB NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 10,
    next_retry_at TIMESTAMPTZ,
    status VARCHAR(50) DEFAULT 'pending',
    trace_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_org_members_org ON organization_members(organization_id);
CREATE INDEX idx_org_members_user ON organization_members(user_id);
CREATE INDEX idx_projects_org ON projects(organization_id);
CREATE INDEX idx_api_keys_org ON api_keys(organization_id);
CREATE INDEX idx_api_keys_project ON api_keys(project_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_account_sources_type ON account_sources(source_type);
CREATE INDEX idx_account_sources_status ON account_sources(status);
CREATE INDEX idx_subscription_accounts_source ON subscription_accounts(source_id);
CREATE INDEX idx_subscription_accounts_status ON subscription_accounts(status);
CREATE INDEX idx_usage_events_org ON usage_events(organization_id, started_at);
CREATE INDEX idx_usage_events_request ON usage_events(request_id);
CREATE INDEX idx_ledger_entries_org ON ledger_entries(organization_id);
CREATE INDEX idx_ledger_entries_wallet ON ledger_entries(wallet_id);
CREATE INDEX idx_audit_logs_org ON audit_logs(organization_id, created_at);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_id);
CREATE INDEX idx_outbox_events_status ON outbox_events(status, next_retry_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outbox_events CASCADE;
DROP TABLE IF EXISTS account_source_events CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS ledger_entries CASCADE;
DROP TABLE IF EXISTS wallets CASCADE;
DROP TABLE IF EXISTS usage_events CASCADE;
DROP TABLE IF EXISTS route_rules CASCADE;
DROP TABLE IF EXISTS model_mappings CASCADE;
DROP TABLE IF EXISTS model_catalog CASCADE;
DROP TABLE IF EXISTS channel_sources CASCADE;
DROP TABLE IF EXISTS channels CASCADE;
DROP TABLE IF EXISTS subscription_accounts CASCADE;
DROP TABLE IF EXISTS account_sources CASCADE;
DROP TABLE IF EXISTS providers CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS organization_members CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS users CASCADE;
-- +goose StatementEnd
