-- +goose Up
-- +goose StatementBegin

CREATE TABLE payment_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    order_no VARCHAR(64) NOT NULL UNIQUE,           -- 平台订单号 PO_xxx
    payment_method VARCHAR(50) NOT NULL,             -- alipay | wechat_pay
    amount_micro_usd BIGINT NOT NULL,                -- 金额
    amount_yuan INTEGER NOT NULL,                    -- 人民币分 (1元=100)
    status VARCHAR(50) NOT NULL DEFAULT 'created',   -- created|pending|paid|credited|expired|failed|refunded|manual_review
    provider_trade_no VARCHAR(128),                  -- 支付渠道交易号
    provider_callback_raw TEXT,                      -- 原始回调数据
    credited_at TIMESTAMPTZ,                         -- 入账时间
    expires_at TIMESTAMPTZ NOT NULL,                 -- 订单过期时间
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payment_webhooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_order_id UUID REFERENCES payment_orders(id),
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,                -- payment.success | payment.failed | refund.success
    raw_body TEXT NOT NULL,                          -- 原始回调 body
    signature_verified BOOLEAN NOT NULL DEFAULT FALSE,
    idempotency_key VARCHAR(128) NOT NULL UNIQUE,   -- 回调幂等键
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE refund_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_order_id UUID NOT NULL REFERENCES payment_orders(id),
    amount_micro_usd BIGINT NOT NULL,
    amount_yuan INTEGER NOT NULL,
    reason TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',   -- pending|approved|processing|refunded|rejected
    approved_by UUID REFERENCES users(id),
    ledger_entry_id UUID,                            -- 关联的冲正账本分录
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE reconciliation_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_date DATE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'running',   -- running|completed|failed
    total_provider_charges BIGINT DEFAULT 0,
    total_platform_records BIGINT DEFAULT 0,
    total_ledger_entries BIGINT DEFAULT 0,
    discrepancy_count INTEGER DEFAULT 0,
    discrepancy_micro_usd BIGINT DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE reconciliation_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES reconciliation_runs(id),
    discrepancy_type VARCHAR(50) NOT NULL,            -- delayed_bill | meter_diff | duplicate | unknown_charge | missing_charge
    request_id VARCHAR(64),
    provider_amount BIGINT,
    platform_amount BIGINT,
    ledger_amount BIGINT,
    diff_micro_usd BIGINT NOT NULL,
    resolution_status VARCHAR(50) DEFAULT 'open',     -- open|investigating|accepted|rejected|corrected
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_orders_org ON payment_orders(organization_id, created_at);
CREATE INDEX idx_payment_orders_no ON payment_orders(order_no);
CREATE INDEX idx_payment_orders_status ON payment_orders(status);
CREATE INDEX idx_payment_webhooks_order ON payment_webhooks(payment_order_id);
CREATE INDEX idx_refund_requests_order ON refund_requests(payment_order_id);
CREATE INDEX idx_reconciliation_runs_date ON reconciliation_runs(run_date);
CREATE INDEX idx_reconciliation_items_run ON reconciliation_items(run_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS reconciliation_items CASCADE;
DROP TABLE IF EXISTS reconciliation_runs CASCADE;
DROP TABLE IF EXISTS refund_requests CASCADE;
DROP TABLE IF EXISTS payment_webhooks CASCADE;
DROP TABLE IF EXISTS payment_orders CASCADE;
-- +goose StatementEnd
