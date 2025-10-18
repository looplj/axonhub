package biz

import (
	"context"
	"fmt"

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

	role, err := client.Role.Create().
		SetCode(input.Code).
		SetName(input.Name).
		SetScopes(input.Scopes).
		SetNillableProjectID(input.ProjectID).
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

// invalidateUserCache clears all user cache when a role is modified.
// Since role changes affect user scopes, we clear the entire cache for simplicity.
func (s *RoleService) invalidateUserCache(ctx context.Context) {
	s.userService.clearUserCache(ctx)
}
