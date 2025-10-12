package biz

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/project"
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
func (s *ProjectService) CreateProject(ctx context.Context, input ent.CreateProjectInput, userID int) (*ent.Project, error) {
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

	project, err := createProject.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Assign the creator as project owner
	_, err = client.UserProject.Create().
		SetUserID(userID).
		SetProjectID(project.ID).
		SetIsOwner(true).
		SetScopes([]string{
			"*",
		}).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to assign user to project: %w", err)
	}

	return project, nil
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
