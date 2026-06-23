package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/seaveywong/lanyu-token-gateway/apps/control-plane/internal/repository"
)

// ProjectService handles project lifecycle within organizations.
type ProjectService struct {
	projects *repository.ProjectRepo
	orgs     *repository.OrgRepo
	members  *repository.MemberRepo
	audit    *repository.AuditRepo
}

// NewProjectService returns a ProjectService with the given repositories.
func NewProjectService(
	projects *repository.ProjectRepo,
	orgs *repository.OrgRepo,
	members *repository.MemberRepo,
	audit *repository.AuditRepo,
) *ProjectService {
	return &ProjectService{
		projects: projects,
		orgs:     orgs,
		members:  members,
		audit:    audit,
	}
}

// Create creates a new project in the given organization. The user must be a
// member of the organization.
func (s *ProjectService) Create(ctx context.Context, orgID, userID, name, description string) (*repository.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("create project: name is required")
	}

	if err := s.requireMember(ctx, orgID, userID); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	project, err := s.projects.Create(ctx, orgID, name, description)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: orgID,
		ActorID:        userID,
		Action:         "project.created",
		ResourceType:   "project",
		ResourceID:     project.ID,
	})
	return project, nil
}

// GetByID returns a project by its UUID.
func (s *ProjectService) GetByID(ctx context.Context, id string) (*repository.Project, error) {
	project, err := s.projects.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("get project: project %s not found", id)
	}
	return project, nil
}

// ListByOrg returns all projects in an organization. The user must be a member
// of the organization.
func (s *ProjectService) ListByOrg(ctx context.Context, orgID, userID string) ([]repository.Project, error) {
	if err := s.requireMember(ctx, orgID, userID); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects, err := s.projects.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []repository.Project{}
	}
	return projects, nil
}

// Update changes the name and description of a project. The user must be a
// member of the parent organization.
func (s *ProjectService) Update(ctx context.Context, id, userID, name, description string) error {
	project, err := s.projects.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if project == nil {
		return fmt.Errorf("update project: project %s not found", id)
	}

	if err := s.requireMember(ctx, project.OrganizationID, userID); err != nil {
		return fmt.Errorf("update project: %w", err)
	}

	if err := s.projects.Update(ctx, id, name, description); err != nil {
		return fmt.Errorf("update project: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: project.OrganizationID,
		ActorID:        userID,
		Action:         "project.updated",
		ResourceType:   "project",
		ResourceID:     id,
	})
	return nil
}

// UpdateBudget changes the daily and monthly budget limits. The user must be a
// member of the parent organization.
func (s *ProjectService) UpdateBudget(ctx context.Context, id, userID string, daily, monthly int64) error {
	if daily < 0 || monthly < 0 {
		return fmt.Errorf("update budget: budgets cannot be negative")
	}

	project, err := s.projects.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("update budget: %w", err)
	}
	if project == nil {
		return fmt.Errorf("update budget: project %s not found", id)
	}

	if err := s.requireMember(ctx, project.OrganizationID, userID); err != nil {
		return fmt.Errorf("update budget: %w", err)
	}

	if err := s.projects.UpdateBudget(ctx, id, daily, monthly); err != nil {
		return fmt.Errorf("update budget: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: project.OrganizationID,
		ActorID:        userID,
		Action:         "project.budget_updated",
		ResourceType:   "project",
		ResourceID:     id,
	})
	return nil
}

// Delete removes a project. The user must be a member of the parent
// organization.
func (s *ProjectService) Delete(ctx context.Context, id, userID string) error {
	project, err := s.projects.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if project == nil {
		return fmt.Errorf("delete project: project %s not found", id)
	}

	if err := s.requireMember(ctx, project.OrganizationID, userID); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	if err := s.projects.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	_ = s.audit.Create(ctx, repository.CreateAuditParams{
		OrganizationID: project.OrganizationID,
		ActorID:        userID,
		Action:         "project.deleted",
		ResourceType:   "project",
		ResourceID:     id,
	})
	return nil
}

// requireMember verifies that the actor is a member of the given organization.
func (s *ProjectService) requireMember(ctx context.Context, orgID, userID string) error {
	_, err := s.members.GetRole(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("check membership: %w", err)
	}
	return nil
}
