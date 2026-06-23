-- +goose Up
-- +goose StatementBegin

-- Pricing versions: versioned price lists. Only one version is active at a time.
CREATE TABLE pricing_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Pricing rules: per-model pricing within a version.
-- Prices are in micro_usd (1e-6 USD). e.g. 1500 = $0.0015
CREATE TABLE pricing_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    version_id UUID NOT NULL REFERENCES pricing_versions(id) ON DELETE CASCADE,
    model_name VARCHAR(255) NOT NULL,
    input_price_micro_usd BIGINT NOT NULL DEFAULT 0,
    output_price_micro_usd BIGINT NOT NULL DEFAULT 0,
    cached_price_micro_usd BIGINT NOT NULL DEFAULT 0,
    image_price_micro_usd BIGINT NOT NULL DEFAULT 0,
    audio_price_micro_usd BIGINT NOT NULL DEFAULT 0,
    UNIQUE(version_id, model_name)
);

CREATE INDEX idx_pricing_versions_active ON pricing_versions(is_active);
CREATE INDEX idx_pricing_rules_version ON pricing_rules(version_id);
CREATE INDEX idx_pricing_rules_model ON pricing_rules(model_name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pricing_rules CASCADE;
DROP TABLE IF EXISTS pricing_versions CASCADE;
-- +goose StatementEnd
