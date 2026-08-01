package biz

import (
	"context"
	"errors"
	"fmt"

	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/invitation"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

type RoleServiceParams struct {
	fx.In

	UserService *UserService
	Ent         *ent.Client
}

type RoleService struct {
	*AbstractService

	userService         *UserService
	permissionValidator *PermissionValidator
}

func NewRoleService(params RoleServiceParams) *RoleService {
	return &RoleService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		userService:         params.UserService,
		permissionValidator: NewPermissionValidator(),
	}
}

// CreateRole creates a new role.
func (s *RoleService) CreateRole(ctx context.Context, input ent.CreateRoleInput) (*ent.Role, error) {
	// Validate that current user can grant these scopes
	var projectID *int
	if input.ProjectID != nil {
		projectID = input.ProjectID
	}

	if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, projectID); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	client := s.entFromContext(ctx)

	var (
		level           role.Level
		projectIDValue  int
		projectIDForAPI *int
	)

	if input.Level == nil {
		if input.ProjectID != nil {
			return nil, fmt.Errorf("project ID is not allowed for system roles")
		}

		level = role.LevelSystem
		projectIDValue = 0
		projectIDForAPI = lo.ToPtr(0)
	} else {
		switch *input.Level {
		case role.LevelSystem:
			if input.ProjectID != nil {
				return nil, fmt.Errorf("project ID is not allowed for system roles")
			}

			level = role.LevelSystem
			projectIDValue = 0
			projectIDForAPI = lo.ToPtr(0)
		case role.LevelProject:
			if input.ProjectID == nil {
				return nil, fmt.Errorf("project ID is required for project roles")
			}

			level = role.LevelProject
			projectIDValue = *input.ProjectID
			projectIDForAPI = input.ProjectID
		default:
			return nil, fmt.Errorf("invalid role level")
		}
	}

	exists, err := s.RoleNameExists(ctx, level, input.Name, projectIDForAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to check role name uniqueness: %w", err)
	}

	if exists {
		return nil, xerrors.DuplicateNameError("role", input.Name)
	}

	role, err := client.Role.Create().
		SetName(input.Name).
		SetScopes(input.Scopes).
		SetLevel(level).
		SetProjectID(projectIDValue).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return role, nil
}

// UpdateRole updates an existing role.
func (s *RoleService) UpdateRole(ctx context.Context, id int, input ent.UpdateRoleInput) (*ent.Role, error) {
	// First check if user can edit this role
	if err := s.permissionValidator.CanEditRole(ctx, id, nil); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate new scopes if being updated
	if input.Scopes != nil {
		role, err := s.entFromContext(ctx).Role.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get role: %w", err)
		}

		var projectID *int
		if !role.IsSystemRole() {
			projectID = role.ProjectID
		}

		if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, projectID); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	client := s.entFromContext(ctx)

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
				return nil, xerrors.DuplicateNameError("role", *input.Name)
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
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		roleEntity, err := client.Role.Query().
			Where(role.IDEQ(id)).
			Only(ctx)
		if err != nil {
			return fmt.Errorf("failed to check role existence: %w", err)
		}
		if roleEntity.Name == "Developer" {
			return fmt.Errorf("cannot delete the default Developer role")
		}

		if err := revokeRoleInvitations(ctx, id); err != nil {
			return err
		}
		if _, err := client.UserRole.Delete().Where(userrole.RoleID(id)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user role relationships: %w", err)
		}
		if err := client.Role.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete role: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.invalidateUserCache(ctx)

	return nil
}

// BulkDeleteRoles deletes multiple roles and all associated user-role relationships.
func (s *RoleService) BulkDeleteRoles(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		roles, err := client.Role.Query().Where(role.IDIn(ids...)).All(ctx)
		if err != nil {
			return fmt.Errorf("failed to query roles: %w", err)
		}
		if len(roles) != len(ids) {
			return fmt.Errorf("expected to find %d roles, but found %d", len(ids), len(roles))
		}
		for _, r := range roles {
			if r.Name == "Developer" {
				return fmt.Errorf("cannot delete the default Developer role")
			}
		}

		if err := revokeRoleInvitations(ctx, ids...); err != nil {
			return err
		}
		if _, err := client.UserRole.Delete().Where(userrole.RoleIDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user role relationships: %w", err)
		}
		if _, err := client.Role.Delete().Where(role.IDIn(ids...)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete roles: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.invalidateUserCache(ctx)

	return nil
}

func revokeRoleInvitations(ctx context.Context, roleIDs ...int) error {
	_, err := authz.RunWithSystemBypass(ctx, "revoke-role-invitations", func(ctx context.Context) (int, error) {
		return ent.FromContext(ctx).Invitation.Delete().Where(invitation.RoleIDIn(roleIDs...)).Exec(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to revoke role invitations: %w", err)
	}

	return nil
}

// RoleNameExists checks if a role name already exists within a specific project.
func (s *RoleService) RoleNameExists(ctx context.Context, level role.Level, name string, projectID *int) (bool, error) {
	client := s.entFromContext(ctx)

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
