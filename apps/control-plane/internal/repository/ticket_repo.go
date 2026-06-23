package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupportTicket represents a row in the support_tickets table.
type SupportTicket struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organization_id"`
	CreatedBy        string    `json:"created_by"`
	Subject          string    `json:"subject"`
	Status           string    `json:"status"`
	Priority         string    `json:"priority"`
	AssignedTo       *string   `json:"assigned_to,omitempty"`
	RelatedRequestID *string   `json:"related_request_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TicketMessage represents a row in the ticket_messages table.
type TicketMessage struct {
	ID         string    `json:"id"`
	TicketID   string    `json:"ticket_id"`
	SenderID   string    `json:"sender_id"`
	Body       string    `json:"body"`
	IsInternal bool      `json:"is_internal"`
	CreatedAt  time.Time `json:"created_at"`
}

// TicketRepo provides CRUD operations on support tickets and messages.
type TicketRepo struct {
	pool *pgxpool.Pool
}

// NewTicketRepo returns a TicketRepo backed by the given connection pool.
func NewTicketRepo(pool *pgxpool.Pool) *TicketRepo {
	return &TicketRepo{pool: pool}
}

// CreateTicket inserts a new support ticket and returns it.
func (r *TicketRepo) CreateTicket(ctx context.Context, orgID, createdBy, subject string) (*SupportTicket, error) {
	var t SupportTicket
	err := r.pool.QueryRow(ctx,
		`INSERT INTO support_tickets (organization_id, created_by, subject)
		 VALUES ($1, $2, $3)
		 RETURNING id, organization_id, created_by, subject, status, priority, assigned_to, related_request_id, created_at, updated_at`,
		orgID, createdBy, subject,
	).Scan(&t.ID, &t.OrganizationID, &t.CreatedBy, &t.Subject, &t.Status, &t.Priority, &t.AssignedTo, &t.RelatedRequestID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert support ticket: %w", err)
	}
	return &t, nil
}

// FindTicketByID looks up a support ticket by UUID.
func (r *TicketRepo) FindTicketByID(ctx context.Context, id string) (*SupportTicket, error) {
	var t SupportTicket
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, created_by, subject, status, priority, assigned_to, related_request_id, created_at, updated_at
		 FROM support_tickets WHERE id = $1`, id,
	).Scan(&t.ID, &t.OrganizationID, &t.CreatedBy, &t.Subject, &t.Status, &t.Priority, &t.AssignedTo, &t.RelatedRequestID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find ticket by id: %w", err)
	}
	return &t, nil
}

// ListTicketsByOrg returns tickets for an organization, with optional status filter.
func (r *TicketRepo) ListTicketsByOrg(ctx context.Context, orgID string, status string, page, pageSize int) ([]SupportTicket, int, error) {
	// Count total
	var total int
	countSQL := `SELECT COUNT(*) FROM support_tickets WHERE organization_id = $1`
	args := []interface{}{orgID}
	if status != "" {
		countSQL += ` AND status = $2`
		args = append(args, status)
	}
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	// List with pagination
	offset := (page - 1) * pageSize
	listSQL := `SELECT id, organization_id, created_by, subject, status, priority, assigned_to, related_request_id, created_at, updated_at
		 FROM support_tickets
		 WHERE organization_id = $1`
	listArgs := []interface{}{orgID}
	argIdx := 2
	if status != "" {
		listSQL += fmt.Sprintf(` AND status = $%d`, argIdx)
		listArgs = append(listArgs, status)
		argIdx++
	}
	listSQL += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	listArgs = append(listArgs, pageSize, offset)

	rows, err := r.pool.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	var tickets []SupportTicket
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.CreatedBy, &t.Subject, &t.Status, &t.Priority, &t.AssignedTo, &t.RelatedRequestID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return tickets, total, rows.Err()
}

// ListAllTickets returns all tickets across organizations with filters (admin).
func (r *TicketRepo) ListAllTickets(ctx context.Context, status, priority string, page, pageSize int) ([]SupportTicket, int, error) {
	var total int
	countSQL := `SELECT COUNT(*) FROM support_tickets WHERE 1=1`
	args := []interface{}{}
	argIdx := 1
	if status != "" {
		countSQL += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if priority != "" {
		countSQL += fmt.Sprintf(` AND priority = $%d`, argIdx)
		args = append(args, priority)
		argIdx++
	}
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count all tickets: %w", err)
	}

	offset := (page - 1) * pageSize
	args = []interface{}{}
	argIdx = 1
	listSQL := `SELECT id, organization_id, created_by, subject, status, priority, assigned_to, related_request_id, created_at, updated_at
		 FROM support_tickets WHERE 1=1`
	if status != "" {
		listSQL += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if priority != "" {
		listSQL += fmt.Sprintf(` AND priority = $%d`, argIdx)
		args = append(args, priority)
		argIdx++
	}
	listSQL += fmt.Sprintf(` ORDER BY
		 CASE priority
		   WHEN 'urgent' THEN 1
		   WHEN 'high' THEN 2
		   WHEN 'medium' THEN 3
		   WHEN 'low' THEN 4
		 END, created_at DESC
		 LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list all tickets: %w", err)
	}
	defer rows.Close()

	var tickets []SupportTicket
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.CreatedBy, &t.Subject, &t.Status, &t.Priority, &t.AssignedTo, &t.RelatedRequestID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return tickets, total, rows.Err()
}

// UpdateTicket updates a ticket's status, priority, and/or assignee.
func (r *TicketRepo) UpdateTicket(ctx context.Context, id string, status, priority, assignedTo *string) (*SupportTicket, error) {
	var t SupportTicket
	err := r.pool.QueryRow(ctx,
		`UPDATE support_tickets
		 SET status = COALESCE($2, status),
		     priority = COALESCE($3, priority),
		     assigned_to = COALESCE($4, assigned_to),
		     updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, organization_id, created_by, subject, status, priority, assigned_to, related_request_id, created_at, updated_at`,
		id, status, priority, assignedTo,
	).Scan(&t.ID, &t.OrganizationID, &t.CreatedBy, &t.Subject, &t.Status, &t.Priority, &t.AssignedTo, &t.RelatedRequestID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("update ticket: %w", err)
	}
	return &t, nil
}

// CreateMessage adds a message to a ticket.
func (r *TicketRepo) CreateMessage(ctx context.Context, ticketID, senderID, body string, isInternal bool) (*TicketMessage, error) {
	var m TicketMessage
	err := r.pool.QueryRow(ctx,
		`INSERT INTO ticket_messages (ticket_id, sender_id, body, is_internal)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, ticket_id, sender_id, body, is_internal, created_at`,
		ticketID, senderID, body, isInternal,
	).Scan(&m.ID, &m.TicketID, &m.SenderID, &m.Body, &m.IsInternal, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert ticket message: %w", err)
	}
	return &m, nil
}

// ListMessagesByTicket returns all messages for a ticket, ordered by creation time.
// If viewerIsStaff is false, internal notes are excluded.
func (r *TicketRepo) ListMessagesByTicket(ctx context.Context, ticketID string, viewerIsStaff bool) ([]TicketMessage, error) {
	query := `SELECT id, ticket_id, sender_id, body, is_internal, created_at
		 FROM ticket_messages
		 WHERE ticket_id = $1`
	if !viewerIsStaff {
		query += ` AND is_internal = FALSE`
	}
	query += ` ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("list ticket messages: %w", err)
	}
	defer rows.Close()

	var msgs []TicketMessage
	for rows.Next() {
		var m TicketMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.SenderID, &m.Body, &m.IsInternal, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ticket message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
