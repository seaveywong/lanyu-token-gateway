package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// WebhookService manages webhook endpoint registration and event delivery.
type WebhookService struct {
	webhooks *repository.WebhookRepo
	audit    *repository.AuditRepo
}

// NewWebhookService returns a WebhookService with the given repositories.
func NewWebhookService(webhooks *repository.WebhookRepo, audit *repository.AuditRepo) *WebhookService {
	return &WebhookService{webhooks: webhooks, audit: audit}
}

// CreateEndpoint registers a new webhook endpoint for an organization.
func (s *WebhookService) CreateEndpoint(ctx context.Context, orgID, actorID string, projectID *string, url, secret string, events []string) (*repository.WebhookEndpoint, error) {
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	if len(events) == 0 {
		events = []string{}
	}

	ep, err := s.webhooks.CreateEndpoint(ctx, orgID, projectID, url, secret, events)
	if err != nil {
		return nil, fmt.Errorf("create webhook endpoint: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		Action:         "webhook.endpoint_created",
		ResourceType:   "webhook_endpoint",
		ResourceID:     ep.ID,
	})
	return ep, nil
}

// ListEndpoints returns all webhook endpoints for an organization.
func (s *WebhookService) ListEndpoints(ctx context.Context, orgID string) ([]repository.WebhookEndpoint, error) {
	eps, err := s.webhooks.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if eps == nil {
		eps = []repository.WebhookEndpoint{}
	}
	return eps, nil
}

// UpdateEndpoint modifies an existing webhook endpoint.
func (s *WebhookService) UpdateEndpoint(ctx context.Context, id, url string, events []string, isActive bool) error {
	if url == "" {
		return fmt.Errorf("url is required")
	}
	return s.webhooks.UpdateEndpoint(ctx, id, url, events, isActive)
}

// DeleteEndpoint removes a webhook endpoint.
func (s *WebhookService) DeleteEndpoint(ctx context.Context, id string) error {
	return s.webhooks.DeleteEndpoint(ctx, id)
}

// SignPayload computes the HMAC-SHA-256 signature for a webhook payload.
func SignPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks that a webhook request signature is valid.
func VerifySignature(secret string, body []byte, signature string) bool {
	expected := SignPayload(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ValidWebhookEvents is the list of recognized webhook event types.
var ValidWebhookEvents = []string{
	"usage.updated",
	"key.expired",
	"key.rotated",
	"payment.success",
	"payment.refunded",
	"project.budget.exceeded",
	"source.health.changed",
}
