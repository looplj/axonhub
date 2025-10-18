package biz

import (
	"context"
	"errors"
	"fmt"

	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/userrole"
)

type RoleServiceParams struct {
	fx.In

	UserService *UserService
}

type RoleService struct {
	userService *UserService
}

func NewRoleService(params RoleServiceParams) *RoleService {
	return &RoleService{
		userService: params.UserService,
	}
}

// CreateRole creates a new role.
func (s *RoleService) CreateRole(ctx context.Context, input ent.CreateRoleInput) (*ent.Role, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	var (
		level     role.Level
		projectID int
	)

	if input.Level == nil {
		if input.ProjectID != nil {
			return nil, fmt.Errorf("project ID is not allowed for system roles")
		}

		level = role.LevelSystem
		projectID = 0
	} else {
		switch *input.Level {
		case role.LevelSystem:
			if input.ProjectID != nil {
				return nil, fmt.Errorf("project ID is not allowed for system roles")
			}

			level = role.LevelSystem
			projectID = 0
		case role.LevelProject:
			if input.ProjectID == nil {
				return nil, fmt.Errorf("project ID is required for project roles")
			}

			level = role.LevelProject
			projectID = *input.ProjectID
		default:
			return nil, fmt.Errorf("invalid role level")
		}
	}

	exists, err := s.RoleNameExists(ctx, level, input.Name, lo.ToPtr(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to check role name uniqueness: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("role name '%s' already exists", input.Name)
	}

	role, err := client.Role.Create().
		SetName(input.Name).
		SetScopes(input.Scopes).
		SetLevel(level).
		SetProjectID(projectID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return role, nil
}

// UpdateRole updates an existing role.
func (s *RoleService) UpdateRole(ctx context.Context, id int, input ent.UpdateRoleInput) (*ent.Role, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	// If name is being updated, check for duplicates
	if input.Name != nil {
		// Get the current role to find its project_id
		currentRole, err := client.Role.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get current role: %w", err)
		}

		if *input.Name != currentRole.Name {
			exists, err := s.RoleNameExists(ctx, currentRole.Level, *input.Name, currentRole.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("failed to check role name uniqueness: %w", err)
			}

			if exists {
				return nil, fmt.Errorf("role name '%s' already exists", *input.Name)
			}
		}
	}

	mut := client.Role.UpdateOneID(id).
		SetNillableName(input.Name)

	if input.Scopes != nil {
		mut.SetScopes(input.Scopes)
	}

	role, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	s.invalidateUserCache(ctx)

	return role, nil
}

// DeleteRole deletes a role and all associated user-role relationships.
// It uses the UserRole entity to delete all relationships through the role_id.
func (s *RoleService) DeleteRole(ctx context.Context, id int) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	// First, check if the role exists
	exists, err := client.Role.Query().
		Where(role.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to check role existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("role not found")
	}

	// Delete all UserRole relationships for this role
	_, err = client.UserRole.Delete().
		Where(userrole.RoleID(id)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user role relationships: %w", err)
	}

	// Now delete the role itself
	err = client.Role.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	// Invalidate cache for all users with this role BEFORE deleting relationships
	s.invalidateUserCache(ctx)

	return nil
}

// BulkDeleteRoles deletes multiple roles and all associated user-role relationships.
func (s *RoleService) BulkDeleteRoles(ctx context.Context, ids []int) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	if len(ids) == 0 {
		return nil
	}

	// Verify all roles exist
	count, err := client.Role.Query().
		Where(role.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to query roles: %w", err)
	}

	if count != len(ids) {
		return fmt.Errorf("expected to find %d roles, but found %d", len(ids), count)
	}

	// Delete all UserRole relationships for these roles
	_, err = client.UserRole.Delete().
		Where(userrole.RoleIDIn(ids...)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user role relationships: %w", err)
	}

	// Now delete all roles
	_, err = client.Role.Delete().
		Where(role.IDIn(ids...)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete roles: %w", err)
	}

	s.invalidateUserCache(ctx)

	return nil
}

// RoleNameExists checks if a role name already exists within a specific project.
func (s *RoleService) RoleNameExists(ctx context.Context, level role.Level, name string, projectID *int) (bool, error) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	client := ent.FromContext(ctx)

	if level == role.LevelSystem {
		return client.Role.Query().
			Where(
				role.ProjectIDEQ(0),
				role.NameEQ(name),
			).
			Exist(ctx)
	}

	if projectID == nil {
		return false, errors.New("project ID is required for project roles")
	}

	return client.Role.Query().
		Where(
			role.ProjectIDEQ(*projectID),
			role.NameEQ(name),
		).
		Exist(ctx)
}

// invalidateUserCache clears all user cache when a role is modified.
// Since role changes affect user scopes, we clear the entire cache for simplicity.
func (s *RoleService) invalidateUserCache(ctx context.Context) {
	s.userService.clearUserCache(ctx)
}
