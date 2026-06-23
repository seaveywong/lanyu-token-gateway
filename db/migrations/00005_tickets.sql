-- +goose Up
-- +goose StatementBegin
CREATE TABLE support_tickets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    created_by UUID NOT NULL REFERENCES users(id),
    subject VARCHAR(500) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    priority VARCHAR(20) NOT NULL DEFAULT 'low',
    assigned_to UUID REFERENCES users(id),
    related_request_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ticket_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    is_internal BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_support_tickets_org ON support_tickets(organization_id, status);
CREATE INDEX idx_support_tickets_assignee ON support_tickets(assigned_to, status);
CREATE INDEX idx_ticket_messages_ticket ON ticket_messages(ticket_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ticket_messages CASCADE;
DROP TABLE IF EXISTS support_tickets CASCADE;
-- +goose StatementEnd
