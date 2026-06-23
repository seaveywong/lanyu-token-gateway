-- +goose Up
-- +goose StatementBegin
CREATE TABLE webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    project_id UUID REFERENCES projects(id),
    url TEXT NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    secret VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    response_status INTEGER,
    response_body TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMPTZ,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_endpoints_org ON webhook_endpoints(organization_id);
CREATE INDEX idx_webhook_deliveries_endpoint ON webhook_deliveries(endpoint_id, created_at);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status, next_retry_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_deliveries CASCADE;
DROP TABLE IF EXISTS webhook_endpoints CASCADE;
-- +goose StatementEnd
