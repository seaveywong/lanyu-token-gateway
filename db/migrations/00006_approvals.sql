-- +goose Up
-- +goose StatementBegin
CREATE TABLE approval_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    requester_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(100) NOT NULL,                    -- payment.refund | org.transfer | key.export | source.disable
    resource_type VARCHAR(100) NOT NULL,
    resource_id UUID NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',             -- action-specific data
    status VARCHAR(50) NOT NULL DEFAULT 'pending',   -- pending | approved | rejected | cancelled
    required_approvals INTEGER NOT NULL DEFAULT 2,   -- number of distinct approvers needed
    approved_by UUID[] DEFAULT '{}',                 -- list of approver user IDs
    rejected_by UUID REFERENCES users(id),
    rejection_reason TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approvals_org_status ON approval_requests(organization_id, status);
CREATE INDEX idx_approvals_requester ON approval_requests(requester_id);
CREATE INDEX idx_approvals_expires ON approval_requests(expires_at) WHERE status = 'pending';
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS approval_requests CASCADE;
