package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// OrgService handles organization lifecycle and member management.
type OrgService struct {
	orgs    *repository.OrgRepo
	members *repository.MemberRepo
	audit   *repository.AuditRepo
}

// NewOrgService returns an OrgService with the given repositories.
func NewOrgService(orgs *repository.OrgRepo, members *repository.MemberRepo, audit *repository.AuditRepo) *OrgService {
	return &OrgService{orgs: orgs, members: members, audit: audit}
}

// slugRe matches characters allowed in an organization slug.
var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Create creates an organization with the given name, generates a slug, and
// adds the creating user as an org_owner.
func (s *OrgService) Create(ctx context.Context, userID, name string) (*repository.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("create org: name is required")
	}

	slug := generateSlug(name)

	org, err := s.orgs.Create(ctx, name, slug)
	if err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	if err := s.members.Add(ctx, org.ID, userID, "org_owner"); err != nil {
		return nil, fmt.Errorf("create org: add owner: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: org.ID,
		ActorID:        userID,
		Action:         "org.created",
		ResourceType:   "organization",
		ResourceID:     org.ID,
	})
	return org, nil
}

// GetByID returns an organization by its UUID.
func (s *OrgService) GetByID(ctx context.Context, id string) (*repository.Organization, error) {
	org, err := s.orgs.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("get org: organization %s not found", id)
	}
	return org, nil
}

// ListByUser returns all organizations the user is a member of.
func (s *OrgService) ListByUser(ctx context.Context, userID string) ([]repository.Organization, error) {
	orgs, err := s.orgs.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if orgs == nil {
		orgs = []repository.Organization{}
	}
	return orgs, nil
}

// AddMember adds a user to an organization with the given role. The actor must
// be an org_owner.
func (s *OrgService) AddMember(ctx context.Context, orgID, actorID, userID, role string) error {
	if err := s.requireOwner(ctx, orgID, actorID); err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	if role == "" {
		role = "developer"
	}

	if err := s.members.Add(ctx, orgID, userID, role); err != nil {
		return fmt.Errorf("add member: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		Action:         "org.member_added",
		ResourceType:   "organization_member",
		ResourceID:     userID,
	})
	return nil
}

// RemoveMember removes a user from an organization. The actor must be an
// org_owner. An owner cannot remove themselves.
func (s *OrgService) RemoveMember(ctx context.Context, orgID, actorID, userID string) error {
	if err := s.requireOwner(ctx, orgID, actorID); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if actorID == userID {
		return fmt.Errorf("remove member: cannot remove yourself as owner")
	}

	if err := s.members.Remove(ctx, orgID, userID); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		Action:         "org.member_removed",
		ResourceType:   "organization_member",
		ResourceID:     userID,
	})
	return nil
}

// UpdateMemberRole changes a member's role. The actor must be an org_owner.
func (s *OrgService) UpdateMemberRole(ctx context.Context, orgID, actorID, userID, role string) error {
	if err := s.requireOwner(ctx, orgID, actorID); err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	if role == "" {
		return fmt.Errorf("update member role: role is required")
	}

	if err := s.members.UpdateRole(ctx, orgID, userID, role); err != nil {
		return fmt.Errorf("update member role: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		Action:         "org.member_role_updated",
		ResourceType:   "organization_member",
		ResourceID:     userID,
	})
	return nil
}

// ListMembers returns all members of an organization.
func (s *OrgService) ListMembers(ctx context.Context, orgID string) ([]repository.Member, error) {
	members, err := s.members.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []repository.Member{}
	}
	return members, nil
}

// requireOwner verifies that the actor has the org_owner role in the given org.
func (s *OrgService) requireOwner(ctx context.Context, orgID, actorID string) error {
	role, err := s.members.GetRole(ctx, orgID, actorID)
	if err != nil {
		return fmt.Errorf("check ownership: %w", err)
	}
	if role != "org_owner" {
		return fmt.Errorf("check ownership: actor is not an org_owner")
	}
	return nil
}

// generateSlug creates a URL-safe slug from an organization name.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove any characters not allowed in the slug
	slug = slugRe.ReplaceAllString(slug, "")
	if slug == "" {
		slug = "org"
	}
	return slug
}
