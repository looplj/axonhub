package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
)

type ProjectServiceParams struct {
	fx.In

	CacheConfig xcache.Config
	Ent         *ent.Client
}

type ProjectService struct {
	*AbstractService

	ProjectCache xcache.Cache[ent.Project]
}

func NewProjectService(params ProjectServiceParams) *ProjectService {
	return &ProjectService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		ProjectCache: xcache.NewFromConfig[ent.Project](params.CacheConfig),
	}
}

// CreateProject creates a new project with owner role and assigns the creator as owner.
// It also creates three default project-level roles: admin, developer, and viewer.
func (s *ProjectService) CreateProject(ctx context.Context, input ent.CreateProjectInput) (*ent.Project, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	client := s.entFromContext(ctx)

	// Check for duplicate project name
	exists, err := client.Project.Query().
		Where(project.NameEQ(input.Name)).
		Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check project name uniqueness: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("project name '%s' already exists", input.Name)
	}

	// Create the project
	createProject := client.Project.Create().
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
	client := s.entFromContext(ctx)

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

	// Invalidate cache
	s.invalidateProjectCache(ctx, id)

	return project, nil
}

// GetProjectByID retrieves a project by its ID with caching.
func (s *ProjectService) GetProjectByID(ctx context.Context, id int) (*ent.Project, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Try cache first
	cacheKey := buildProjectCacheKey(id)
	if proj, err := s.ProjectCache.Get(ctx, cacheKey); err == nil {
		return &proj, nil
	}

	// Query database
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	proj, err := client.Project.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Cache the project
	err = s.ProjectCache.Set(ctx, cacheKey, *proj)
	if err != nil {
		log.Warn(ctx, "failed to cache project", zap.Error(err))
	}

	return proj, nil
}

// UpdateProjectStatus updates the status of a project.
func (s *ProjectService) UpdateProjectStatus(ctx context.Context, id int, status project.Status) (*ent.Project, error) {
	client := s.entFromContext(ctx)

	proj, err := client.Project.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update project status: %w", err)
	}

	// Invalidate cache
	s.invalidateProjectCache(ctx, id)

	return proj, nil
}

func buildProjectCacheKey(id int) string {
	return fmt.Sprintf("project:%d", id)
}

// invalidateProjectCache removes a project from cache.
func (s *ProjectService) invalidateProjectCache(ctx context.Context, id int) {
	cacheKey := buildProjectCacheKey(id)
	_ = s.ProjectCache.Delete(ctx, cacheKey)
}
