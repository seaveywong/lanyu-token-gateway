package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// GeneratedKey is returned when a new API key is created. The RawKey is shown
// only once; the caller is responsible for presenting it securely.
type GeneratedKey struct {
	ID        string
	RawKey    string
	Prefix    string
	ExpiresAt *time.Time
}

// APIKeyService manages API key creation, validation, and revocation.
type APIKeyService struct {
	apiKeys  *repository.APIKeyRepo
	projects *repository.ProjectRepo
	members  *repository.MemberRepo
	audit    *repository.AuditRepo
	pepper   []byte
	keyPrefix string
	keyByteLen int
	defaultExpiry time.Duration
}

// APIKeyServiceConfig holds configuration for the API key service.
type APIKeyServiceConfig struct {
	Pepper           []byte
	KeyPrefix        string
	KeyByteLength    int
	DefaultExpiry    time.Duration
}

// NewAPIKeyService returns an APIKeyService with the given dependencies.
func NewAPIKeyService(
	apiKeys *repository.APIKeyRepo,
	projects *repository.ProjectRepo,
	members *repository.MemberRepo,
	audit *repository.AuditRepo,
	cfg APIKeyServiceConfig,
) *APIKeyService {
	if cfg.KeyByteLength == 0 {
		cfg.KeyByteLength = 32
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "lt"
	}
	return &APIKeyService{
		apiKeys:       apiKeys,
		projects:      projects,
		members:       members,
		audit:         audit,
		pepper:        cfg.Pepper,
		keyPrefix:     cfg.KeyPrefix,
		keyByteLen:    cfg.KeyByteLength,
		defaultExpiry: cfg.DefaultExpiry,
	}
}

// Create generates a new API key for a project. The user must be a member of
// the project's parent organization. Returns a GeneratedKey with the raw key.
func (s *APIKeyService) Create(ctx context.Context, projectID, userID, name, env string) (*GeneratedKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("create api key: name is required")
	}
	if env == "" {
		env = "production"
	}

	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("create api key: project %s not found", projectID)
	}

	if _, err := s.members.GetRole(ctx, project.OrganizationID, userID); err != nil {
		return nil, fmt.Errorf("create api key: membership check: %w", err)
	}

	rawKey, err := generateRawKey(s.keyPrefix, s.keyByteLen)
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	keyHash := hashAPIKey(rawKey, s.pepper)
	prefix := rawKey[:strings.Index(rawKey, "_")]

	var expiresAt *time.Time
	if s.defaultExpiry > 0 {
		t := time.Now().UTC().Add(s.defaultExpiry)
		expiresAt = &t
	}

	created, err := s.apiKeys.Create(ctx, repository.CreateAPIKeyParams{
		OrganizationID:  project.OrganizationID,
		ProjectID:       projectID,
		Name:            name,
		Environment:     env,
		KeyPrefix:       prefix,
		KeyHash:         keyHash,
		ScopesJSON:      "[]",
		ModelPolicyJSON: "{}",
		IPAllowlistJSON: "[]",
		ExpiresAt:       expiresAt,
		CreatedBy:       userID,
	})
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: project.OrganizationID,
		ActorID:        userID,
		Action:         "apikey.created",
		ResourceType:   "api_key",
		ResourceID:     created.ID,
	})

	return &GeneratedKey{
		ID:        created.ID,
		RawKey:    rawKey,
		Prefix:    prefix,
		ExpiresAt: expiresAt,
	}, nil
}

// ListByProject returns all API keys for a project. The user must be a member
// of the project's parent organization.
func (s *APIKeyService) ListByProject(ctx context.Context, projectID, userID string) ([]repository.APIKey, error) {
	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("list api keys: project %s not found", projectID)
	}

	if _, err := s.members.GetRole(ctx, project.OrganizationID, userID); err != nil {
		return nil, fmt.Errorf("list api keys: membership check: %w", err)
	}

	keys, err := s.apiKeys.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []repository.APIKey{}
	}
	return keys, nil
}

// Revoke marks an API key as revoked. The user must be a member of the key's
// parent organization.
func (s *APIKeyService) Revoke(ctx context.Context, keyID, userID string) error {
	key, err := s.apiKeys.FindByID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if key == nil {
		return fmt.Errorf("revoke api key: key %s not found", keyID)
	}

	if _, err := s.members.GetRole(ctx, key.OrganizationID, userID); err != nil {
		return fmt.Errorf("revoke api key: membership check: %w", err)
	}

	if err := s.apiKeys.Revoke(ctx, keyID); err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: key.OrganizationID,
		ActorID:        userID,
		Action:         "apikey.revoked",
		ResourceType:   "api_key",
		ResourceID:     keyID,
	})
	return nil
}

// Validate checks whether a raw key is valid, not revoked, and not expired.
// Returns the associated API key record on success.
func (s *APIKeyService) Validate(ctx context.Context, rawKey string) (*repository.APIKey, error) {
	if rawKey == "" {
		return nil, fmt.Errorf("validate api key: key is empty")
	}

	keyHash := hashAPIKey(rawKey, s.pepper)
	key, err := s.apiKeys.FindByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("validate api key: %w", err)
	}
	if key == nil {
		return nil, fmt.Errorf("validate api key: invalid key")
	}

	if key.RevokedAt != nil {
		return nil, fmt.Errorf("validate api key: key has been revoked")
	}

	if key.ExpiresAt != nil && time.Now().UTC().After(*key.ExpiresAt) {
		return nil, fmt.Errorf("validate api key: key has expired")
	}

	// Update last-used timestamp (best-effort, non-blocking).
	_ = s.apiKeys.UpdateLastUsed(ctx, key.ID)

	return key, nil
}
