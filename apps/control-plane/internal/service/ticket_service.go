package service

import (
	"context"
	"fmt"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// TicketService manages support ticket lifecycle.
type TicketService struct {
	tickets *repository.TicketRepo
	audit   *repository.AuditRepo
}

// NewTicketService returns a TicketService with the given repositories.
func NewTicketService(tickets *repository.TicketRepo, audit *repository.AuditRepo) *TicketService {
	return &TicketService{tickets: tickets, audit: audit}
}

// CreateTicket creates a new support ticket and its initial message.
func (s *TicketService) CreateTicket(ctx context.Context, orgID, userID, subject, body string) (*repository.SupportTicket, error) {
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if body == "" {
		return nil, fmt.Errorf("body is required")
	}

	ticket, err := s.tickets.CreateTicket(ctx, orgID, userID, subject)
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	// Add the initial message
	_, err = s.tickets.CreateMessage(ctx, ticket.ID, userID, body, false)
	if err != nil {
		return nil, fmt.Errorf("create initial message: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        userID,
		Action:         "ticket.created",
		ResourceType:   "support_ticket",
		ResourceID:     ticket.ID,
	})
	return ticket, nil
}

// GetTicket returns a ticket by ID with its messages.
func (s *TicketService) GetTicket(ctx context.Context, ticketID, viewerOrgID string, viewerIsStaff bool) (*repository.SupportTicket, []repository.TicketMessage, error) {
	ticket, err := s.tickets.FindTicketByID(ctx, ticketID)
	if err != nil {
		return nil, nil, fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return nil, nil, nil
	}

	// Authorization: only staff or the ticket's organization can view
	if !viewerIsStaff && ticket.OrganizationID != viewerOrgID {
		return nil, nil, fmt.Errorf("access denied")
	}

	messages, err := s.tickets.ListMessagesByTicket(ctx, ticketID, viewerIsStaff)
	if err != nil {
		return nil, nil, fmt.Errorf("list messages: %w", err)
	}
	if messages == nil {
		messages = []repository.TicketMessage{}
	}

	return ticket, messages, nil
}

// ListTicketsByOrg returns tickets for an organization.
func (s *TicketService) ListTicketsByOrg(ctx context.Context, orgID string, status string, page, pageSize int) ([]repository.SupportTicket, int, error) {
	tickets, total, err := s.tickets.ListTicketsByOrg(ctx, orgID, status, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if tickets == nil {
		tickets = []repository.SupportTicket{}
	}
	return tickets, total, nil
}

// ListAllTickets returns all tickets (admin).
func (s *TicketService) ListAllTickets(ctx context.Context, status, priority string, page, pageSize int) ([]repository.SupportTicket, int, error) {
	tickets, total, err := s.tickets.ListAllTickets(ctx, status, priority, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if tickets == nil {
		tickets = []repository.SupportTicket{}
	}
	return tickets, total, nil
}

// UpdateTicket updates a ticket's metadata (status, priority, assignee).
func (s *TicketService) UpdateTicket(ctx context.Context, ticketID string, status, priority, assignedTo *string) (*repository.SupportTicket, error) {
	ticket, err := s.tickets.UpdateTicket(ctx, ticketID, status, priority, assignedTo)
	if err != nil {
		return nil, fmt.Errorf("update ticket: %w", err)
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket %s not found", ticketID)
	}
	return ticket, nil
}

// AddMessage adds a message to a ticket.
func (s *TicketService) AddMessage(ctx context.Context, ticketID, senderID, body string, isInternal bool) (*repository.TicketMessage, error) {
	if body == "" {
		return nil, fmt.Errorf("message body is required")
	}

	// Verify ticket exists
	ticket, err := s.tickets.FindTicketByID(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("find ticket: %w", err)
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket %s not found", ticketID)
	}

	msg, err := s.tickets.CreateMessage(ctx, ticketID, senderID, body, isInternal)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}
	return msg, nil
}
