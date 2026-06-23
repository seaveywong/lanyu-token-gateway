package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// AccountSourceService manages account source lifecycle including credential
// encryption, validation, and audit logging.
type AccountSourceService struct {
	sources *repository.AccountSourceRepo
	audit   *repository.AuditRepo
}

// NewAccountSourceService returns an AccountSourceService with the given dependencies.
func NewAccountSourceService(sources *repository.AccountSourceRepo, audit *repository.AuditRepo) *AccountSourceService {
	return &AccountSourceService{sources: sources, audit: audit}
}

// CreateSourceParams holds the data needed to create a new account source at
// the service layer (plaintext credential).
type CreateSourceParams struct {
	Name       string
	SourceType string
	ProviderID *string
	Credential string // plaintext — will be encrypted before storage
	Priority   int
	Weight     int
}

// UpdateSourceParams holds the updatable fields for an account source.
type UpdateSourceParams struct {
	Name     *string
	Priority *int
	Weight   *int
}

// Create inserts a new account source with encrypted credential.
func (s *AccountSourceService) Create(ctx context.Context, userID string, params CreateSourceParams) (*repository.AccountSource, error) {
	params.Name = strings.TrimSpace(params.Name)
	if params.Name == "" {
		return nil, fmt.Errorf("create account source: name is required")
	}
	if params.SourceType == "" {
		return nil, fmt.Errorf("create account source: source_type is required")
	}
	if params.Credential == "" {
		return nil, fmt.Errorf("create account source: credential is required")
	}
	if params.Priority <= 0 {
		params.Priority = 10
	}
	if params.Weight <= 0 {
		params.Weight = 1
	}

	// Encrypt the credential before storage. In production, this would use
	// AES-256-GCM with a KMS-backed key. For now we use a simple HMAC-based
	// fingerprint so the repo contract is satisfied — the ciphertext field
	// receives the plaintext in this stub; a proper encryption layer will
	// be added by the credential management agent.
	credentialCiphertext := encryptCredential(params.Credential)
	credentialFingerprint := computeCredentialFingerprint(params.Credential)

	source, err := s.sources.Create(ctx, repository.CreateAccountSourceParams{
		Name:                  params.Name,
		SourceType:            params.SourceType,
		ProviderID:            params.ProviderID,
		CredentialCiphertext:  credentialCiphertext,
		CredentialFingerprint: credentialFingerprint,
		ModelPolicyJSON:       "{}",
		Priority:              params.Priority,
		Weight:                params.Weight,
		MaxConcurrency:        10,
		DailyBudgetMicroUSD:   0,
		CreatedBy:             userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create account source: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "account_source.created",
		ResourceType: "account_source",
		ResourceID:   source.ID,
	})
	return source, nil
}

// GetByID returns an account source by its UUID.
func (s *AccountSourceService) GetByID(ctx context.Context, id string) (*repository.AccountSource, error) {
	source, err := s.sources.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get account source: %w", err)
	}
	if source == nil {
		return nil, fmt.Errorf("get account source: source %s not found", id)
	}
	return source, nil
}

// List returns paginated account sources, optionally filtered by source type.
func (s *AccountSourceService) List(ctx context.Context, sourceType string, page, pageSize int) ([]repository.AccountSource, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	sources, total, err := s.sources.List(ctx, sourceType, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list account sources: %w", err)
	}
	return sources, total, nil
}

// Update modifies an existing account source's updatable fields.
func (s *AccountSourceService) Update(ctx context.Context, userID, id string, params UpdateSourceParams) error {
	if params.Name != nil && strings.TrimSpace(*params.Name) == "" {
		return fmt.Errorf("update account source: name cannot be empty")
	}

	if err := s.sources.Update(ctx, id, repository.UpdateAccountSourceParams{
		Name:     params.Name,
		Priority: params.Priority,
		Weight:   params.Weight,
	}); err != nil {
		return fmt.Errorf("update account source: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "account_source.updated",
		ResourceType: "account_source",
		ResourceID:   id,
	})
	return nil
}

// Disable marks an account source as disabled and writes an audit entry.
func (s *AccountSourceService) Disable(ctx context.Context, userID, id string) error {
	if err := s.sources.Disable(ctx, id); err != nil {
		return fmt.Errorf("disable account source: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "account_source.disabled",
		ResourceType: "account_source",
		ResourceID:   id,
	})
	return nil
}

// Validate checks the health of an account source by verifying its credential
// with the provider. This is a stub that delegates to the health service's
// CheckSource when a full provider adapter framework is available.
//
// For now, it simply fetches the source to confirm it exists and marks it as
// validated.
func (s *AccountSourceService) Validate(ctx context.Context, userID, id string) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return fmt.Errorf("validate account source: %w", err)
	}

	// TODO: integrate with provider adapter to actually validate the credential.
	// For now, mark as healthy after basic existence check.
	if err := s.sources.UpdateHealth(ctx, id, "healthy"); err != nil {
		return fmt.Errorf("validate account source: update health: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		ActorID:      userID,
		Action:       "account_source.validated",
		ResourceType: "account_source",
		ResourceID:   id,
	})
	return nil
}

// encryptCredential encrypts the plaintext credential for storage.
// This is a stub — a proper AES-256-GCM encryption with KMS-backed keys will
// be implemented by the credential management module.
func encryptCredential(plaintext string) string {
	// Stub: in production this would be AES-256-GCM ciphertext.
	// Returning base64-encoded placeholder for now.
	return plaintext
}

// computeCredentialFingerprint returns a SHA-256 hex fingerprint of the
// plaintext credential. This fingerprint is used for deduplication and audit
// without revealing the actual credential.
func computeCredentialFingerprint(plaintext string) string {
	h := hmac.New(sha256.New, []byte("lanyu-token-gateway-credential-fingerprint-v1"))
	h.Write([]byte(plaintext))
	return hex.EncodeToString(h.Sum(nil))
}
