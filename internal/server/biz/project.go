package biz

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/scopes"
)

// GenerateSlug generates a URL-friendly slug from a given string
// It converts to lowercase, replaces spaces and special characters with hyphens.
func GenerateSlug(s string) string {
	// Convert to lowercase
	slug := strings.ToLower(s)

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove all characters except alphanumeric, hyphens, and underscores
	reg := regexp.MustCompile(`[^a-z0-9\-_]+`)
	slug = reg.ReplaceAllString(slug, "")

	// Replace multiple consecutive hyphens with a single hyphen
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

type ProjectServiceParams struct {
	fx.In
}

type ProjectService struct{}

func NewProjectService(params ProjectServiceParams) *ProjectService {
	return &ProjectService{}
}

// CreateProject creates a new project with owner role and assigns the creator as owner.
// It also creates three default project-level roles: admin, developer, and viewer.
func (s *ProjectService) CreateProject(ctx context.Context, input ent.CreateProjectInput) (*ent.Project, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	client := ent.FromContext(ctx)

	// Generate slug from name if not provided
	slug := input.Slug
	if slug == "" {
		slug = GenerateSlug(input.Name)
	}

	// Create the project
	createProject := client.Project.Create().
		SetSlug(slug).
		SetName(input.Name)

	if input.Description != nil {
		createProject.SetDescription(*input.Description)
	}

	proj, err := createProject.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Create three default project-level roles
	// Admin role - full permissions
	adminScopes := []string{
		string(scopes.ScopeReadUsers),
		string(scopes.ScopeWriteUsers),
		string(scopes.ScopeReadRoles),
		string(scopes.ScopeWriteRoles),
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadRequests),
		string(scopes.ScopeWriteRequests),
	}

	_, err = client.Role.Create().
		SetCode(fmt.Sprintf("%s-admin", slug)).
		SetName("Admin").
		SetLevel(role.LevelProject).
		SetProjectID(proj.ID).
		SetScopes(adminScopes).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin role: %w", err)
	}

	// Developer role - read/write channels, read users, read requests
	developerScopes := []string{
		string(scopes.ScopeReadUsers),
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadRequests),
	}

	_, err = client.Role.Create().
		SetCode(fmt.Sprintf("%s-developer", slug)).
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(proj.ID).
		SetScopes(developerScopes).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create developer role: %w", err)
	}

	// Viewer role - read-only permissions
	viewerScopes := []string{
		string(scopes.ScopeReadUsers),
		string(scopes.ScopeReadRequests),
	}

	_, err = client.Role.Create().
		SetCode(fmt.Sprintf("%s-viewer", slug)).
		SetName("Viewer").
		SetLevel(role.LevelProject).
		SetProjectID(proj.ID).
		SetScopes(viewerScopes).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create viewer role: %w", err)
	}

	// Assign the creator as project owner
	_, err = client.UserProject.Create().
		SetUserID(currentUser.ID).
		SetProjectID(proj.ID).
		SetIsOwner(true).
		SetScopes([]string{}).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to assign user to project: %w", err)
	}

	return proj, nil
}

// UpdateProject updates an existing project.
func (s *ProjectService) UpdateProject(ctx context.Context, id int, input ent.UpdateProjectInput) (*ent.Project, error) {
	client := ent.FromContext(ctx)

	mut := client.Project.UpdateOneID(id)
	mut.SetNillableName(input.Name)
	mut.SetNillableDescription(input.Description)

	if input.ClearUsers {
		mut.ClearUsers()
	}

	if input.AddUserIDs != nil {
		mut.AddUserIDs(input.AddUserIDs...)
	}

	if input.RemoveUserIDs != nil {
		mut.RemoveUserIDs(input.RemoveUserIDs...)
	}

	project, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	return project, nil
}

// UpdateProjectStatus updates the status of a project.
func (s *ProjectService) UpdateProjectStatus(ctx context.Context, id int, status project.Status) (*ent.Project, error) {
	client := ent.FromContext(ctx)

	proj, err := client.Project.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update project status: %w", err)
	}

	return proj, nil
}
