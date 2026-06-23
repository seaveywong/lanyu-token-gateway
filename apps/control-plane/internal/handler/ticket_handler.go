package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/middleware"
	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/service"
)

// TicketHandler handles support ticket operations for customers and admins.
type TicketHandler struct {
	ticketService *service.TicketService
}

// NewTicketHandler creates a new TicketHandler.
func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

// CreateTicket handles POST /portal-api/tickets.
func (h *TicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := middleware.OrgIDFromContext(r.Context())

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}

	if req.Subject == "" || req.Body == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "subject and body are required", requestID(r))
		return
	}

	ticket, err := h.ticketService.CreateTicket(r.Context(), orgID, userID, req.Subject, req.Body)
	if err != nil {
		slog.Error("ticket create failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, ticket)
}

// ListMyTickets handles GET /portal-api/tickets.
func (h *TicketHandler) ListMyTickets(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	page, pageSize := getPageParams(r)
	status := r.URL.Query().Get("status")

	tickets, total, err := h.ticketService.ListTicketsByOrg(r.Context(), orgID, status, page, pageSize)
	if err != nil {
		slog.Error("ticket list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      tickets,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// GetTicket handles GET /portal-api/tickets/{id}.
func (h *TicketHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgIDFromContext(r.Context())
	ticketID := chi.URLParam(r, "id")

	ticket, messages, err := h.ticketService.GetTicket(r.Context(), ticketID, orgID, false)
	if err != nil {
		slog.Error("ticket get failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	if ticket == nil {
		respondError(w, http.StatusNotFound, "not_found", "ticket not found", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ticket":   ticket,
		"messages": messages,
	})
}

// AddMessage handles POST /portal-api/tickets/{id}/messages.
func (h *TicketHandler) AddMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	ticketID := chi.URLParam(r, "id")

	var req struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}

	if req.Body == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "body is required", requestID(r))
		return
	}

	msg, err := h.ticketService.AddMessage(r.Context(), ticketID, userID, req.Body, false)
	if err != nil {
		slog.Error("ticket message add failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, msg)
}

// AdminListTickets handles GET /admin-api/tickets.
func (h *TicketHandler) AdminListTickets(w http.ResponseWriter, r *http.Request) {
	page, pageSize := getPageParams(r)
	status := r.URL.Query().Get("status")
	priority := r.URL.Query().Get("priority")

	tickets, total, err := h.ticketService.ListAllTickets(r.Context(), status, priority, page, pageSize)
	if err != nil {
		slog.Error("admin ticket list failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":      tickets,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// AdminGetTicket handles GET /admin-api/tickets/{id}.
func (h *TicketHandler) AdminGetTicket(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	ticket, messages, err := h.ticketService.GetTicket(r.Context(), ticketID, "", true)
	if err != nil {
		slog.Error("admin ticket get failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}
	if ticket == nil {
		respondError(w, http.StatusNotFound, "not_found", "ticket not found", requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ticket":   ticket,
		"messages": messages,
	})
}

// AdminUpdateTicket handles PATCH /admin-api/tickets/{id}.
func (h *TicketHandler) AdminUpdateTicket(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	var req struct {
		Status     *string `json:"status"`
		Priority   *string `json:"priority"`
		AssignedTo *string `json:"assigned_to"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}

	ticket, err := h.ticketService.UpdateTicket(r.Context(), ticketID, req.Status, req.Priority, req.AssignedTo)
	if err != nil {
		slog.Error("admin ticket update failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusOK, ticket)
}

// AdminAddMessage handles POST /admin-api/tickets/{id}/messages.
func (h *TicketHandler) AdminAddMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	ticketID := chi.URLParam(r, "id")

	var req struct {
		Body       string `json:"body"`
		IsInternal bool   `json:"is_internal"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid request body: "+err.Error(), requestID(r))
		return
	}

	if req.Body == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "body is required", requestID(r))
		return
	}

	msg, err := h.ticketService.AddMessage(r.Context(), ticketID, userID, req.Body, req.IsInternal)
	if err != nil {
		slog.Error("admin ticket message add failed", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error(), requestID(r))
		return
	}

	respondJSON(w, http.StatusCreated, msg)
}
