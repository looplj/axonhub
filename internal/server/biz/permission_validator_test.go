package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestPermissionValidator(t *testing.T) (*PermissionValidator, *ent.Client, *UserService) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")

	validator := NewPermissionValidator()
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	userService := &UserService{
		UserCache:           xcache.NewFromConfig[ent.User](cacheConfig),
		permissionValidator: validator,
	}

	return validator, client, userService
}

func TestCanGrantScopes(t *testing.T) {
	validator, client, userService := setupTestPermissionValidator(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	t.Run("owner can grant any scopes", func(t *testing.T) {
		// Create owner user
		owner, err := client.User.Create().
			SetEmail("owner@example.com").
			SetFirstName("Owner").
			SetLastName("User").
			SetPassword("password").
			SetIsOwner(true).
			Save(ctx)
		require.NoError(t, err)

		// Load owner with edges (simulating middleware behavior)
		owner, err = userService.GetUserByID(ctx, owner.ID)
		require.NoError(t, err)

		// Set owner in context
		ctxWithUser := contexts.WithUser(ctx, owner)

		// Owner should be able to grant any scopes
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_users", "write_users", "read_projects"}, nil)
		require.NoError(t, err)
	})

	t.Run("user can grant scopes they possess", func(t *testing.T) {
		// Create user with specific scopes
		user, err := client.User.Create().
			SetEmail("user@example.com").
			SetFirstName("Regular").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users", "write_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should be able to grant scopes they have
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_users"}, nil)
		require.NoError(t, err)

		err = validator.CanGrantScopes(ctxWithUser, []string{"read_users", "write_users"}, nil)
		require.NoError(t, err)
	})

	t.Run("user cannot grant scopes they don't possess", func(t *testing.T) {
		// Create user with limited scopes
		user, err := client.User.Create().
			SetEmail("limited@example.com").
			SetFirstName("Limited").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should NOT be able to grant scopes they don't have
		err = validator.CanGrantScopes(ctxWithUser, []string{"write_users"}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "insufficient permissions")

		err = validator.CanGrantScopes(ctxWithUser, []string{"read_users", "write_projects"}, nil)
		require.Error(t, err)
	})

	t.Run("user with role can grant role scopes", func(t *testing.T) {
		// Create a role with scopes
		testRole, err := client.Role.Create().
			SetName("Test Role").
			SetScopes([]string{"read_channels", "write_channels"}).
			SetLevel(role.LevelSystem).
			SetProjectID(0).
			Save(ctx)
		require.NoError(t, err)

		// Create user with this role
		user, err := client.User.Create().
			SetEmail("roleuser@example.com").
			SetFirstName("Role").
			SetLastName("User").
			SetPassword("password").
			AddRoles(testRole).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should be able to grant scopes from their role
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_channels"}, nil)
		require.NoError(t, err)
	})
}

func TestCanEditUserPermissions(t *testing.T) {
	validator, client, userService := setupTestPermissionValidator(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	t.Run("owner can edit any user", func(t *testing.T) {
		// Create owner
		owner, err := client.User.Create().
			SetEmail("owner@example.com").
			SetFirstName("Owner").
			SetLastName("User").
			SetPassword("password").
			SetIsOwner(true).
			Save(ctx)
		require.NoError(t, err)

		// Create target user with high permissions
		targetUser, err := client.User.Create().
			SetEmail("target@example.com").
			SetFirstName("Target").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users", "write_users", "read_projects"}).
			Save(ctx)
		require.NoError(t, err)

		// Load owner with edges
		owner, err = userService.GetUserByID(ctx, owner.ID)
		require.NoError(t, err)

		ctxWithOwner := contexts.WithUser(ctx, owner)

		// Owner should be able to edit any user
		err = validator.CanEditUserPermissions(ctxWithOwner, targetUser.ID, nil)
		require.NoError(t, err)
	})

	t.Run("user can edit user with lower permissions", func(t *testing.T) {
		// Create current user with high permissions
		currentUser, err := client.User.Create().
			SetEmail("current@example.com").
			SetFirstName("Current").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users", "write_users", "read_channels"}).
			Save(ctx)
		require.NoError(t, err)

		// Create target user with subset of permissions
		targetUser, err := client.User.Create().
			SetEmail("target2@example.com").
			SetFirstName("Target2").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Load current user with edges
		currentUser, err = userService.GetUserByID(ctx, currentUser.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, currentUser)

		// User should be able to edit user with lower permissions
		err = validator.CanEditUserPermissions(ctxWithUser, targetUser.ID, nil)
		require.NoError(t, err)
	})

	t.Run("user cannot edit user with higher permissions", func(t *testing.T) {
		// Create current user with limited permissions
		currentUser, err := client.User.Create().
			SetEmail("limited2@example.com").
			SetFirstName("Limited2").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Create target user with more permissions
		targetUser, err := client.User.Create().
			SetEmail("target3@example.com").
			SetFirstName("Target3").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users", "write_users", "read_projects"}).
			Save(ctx)
		require.NoError(t, err)

		// Load current user with edges
		currentUser, err = userService.GetUserByID(ctx, currentUser.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, currentUser)

		// User should NOT be able to edit user with higher permissions
		err = validator.CanEditUserPermissions(ctxWithUser, targetUser.ID, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "insufficient permissions")
	})

	t.Run("non-owner cannot edit owner", func(t *testing.T) {
		// Create regular user
		regularUser, err := client.User.Create().
			SetEmail("regular@example.com").
			SetFirstName("Regular").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users", "write_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Create owner user
		ownerUser, err := client.User.Create().
			SetEmail("owner2@example.com").
			SetFirstName("Owner2").
			SetLastName("User").
			SetPassword("password").
			SetIsOwner(true).
			Save(ctx)
		require.NoError(t, err)

		// Load regular user with edges
		regularUser, err = userService.GetUserByID(ctx, regularUser.ID)
		require.NoError(t, err)

		ctxWithRegular := contexts.WithUser(ctx, regularUser)

		// Regular user should NOT be able to edit owner
		err = validator.CanEditUserPermissions(ctxWithRegular, ownerUser.ID, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot edit owner")
	})
}

func TestCanEditRole(t *testing.T) {
	validator, client, userService := setupTestPermissionValidator(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	t.Run("user can edit role with scopes they possess", func(t *testing.T) {
		// Create user with specific scopes
		user, err := client.User.Create().
			SetEmail("user@example.com").
			SetFirstName("User").
			SetLastName("Test").
			SetPassword("password").
			SetScopes([]string{"read_users", "write_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Create role with subset of user's scopes
		testRole, err := client.Role.Create().
			SetName("Test Role").
			SetScopes([]string{"read_users"}).
			SetLevel(role.LevelSystem).
			SetProjectID(0).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should be able to edit this role
		err = validator.CanEditRole(ctxWithUser, testRole.ID, nil)
		require.NoError(t, err)
	})

	t.Run("user cannot edit role with scopes they don't possess", func(t *testing.T) {
		// Create user with limited scopes
		user, err := client.User.Create().
			SetEmail("limited3@example.com").
			SetFirstName("Limited3").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Create role with scopes user doesn't have
		testRole, err := client.Role.Create().
			SetName("High Permission Role").
			SetScopes([]string{"read_users", "write_users", "read_projects"}).
			SetLevel(role.LevelSystem).
			SetProjectID(0).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should NOT be able to edit this role
		err = validator.CanEditRole(ctxWithUser, testRole.ID, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "insufficient permissions")
	})
}

func TestProjectLevelPermissions(t *testing.T) {
	validator, client, userService := setupTestPermissionValidator(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	t.Run("project owner can grant any project scopes", func(t *testing.T) {
		// Create project
		project, err := client.Project.Create().
			SetName("Test Project").
			Save(ctx)
		require.NoError(t, err)

		// Create user
		user, err := client.User.Create().
			SetEmail("projectowner@example.com").
			SetFirstName("Project").
			SetLastName("Owner").
			SetPassword("password").
			Save(ctx)
		require.NoError(t, err)

		// Add user to project as owner
		_, err = client.UserProject.Create().
			SetUserID(user.ID).
			SetProjectID(project.ID).
			SetIsOwner(true).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// Project owner should be able to grant any project scopes
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_api_keys", "write_api_keys"}, &project.ID)
		require.NoError(t, err)
	})

	t.Run("user with project scopes can grant those scopes", func(t *testing.T) {
		// Create project
		project, err := client.Project.Create().
			SetName("Test Project 2").
			Save(ctx)
		require.NoError(t, err)

		// Create user
		user, err := client.User.Create().
			SetEmail("projectuser@example.com").
			SetFirstName("Project").
			SetLastName("User").
			SetPassword("password").
			Save(ctx)
		require.NoError(t, err)

		// Add user to project with specific scopes
		_, err = client.UserProject.Create().
			SetUserID(user.ID).
			SetProjectID(project.ID).
			SetScopes([]string{"read_api_keys"}).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should be able to grant project scopes they have
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_api_keys"}, &project.ID)
		require.NoError(t, err)

		// User should NOT be able to grant scopes they don't have
		err = validator.CanGrantScopes(ctxWithUser, []string{"write_api_keys"}, &project.ID)
		require.Error(t, err)
	})

	t.Run("user with project role can grant role scopes", func(t *testing.T) {
		// Create project
		project, err := client.Project.Create().
			SetName("Test Project 3").
			Save(ctx)
		require.NoError(t, err)

		// Create project role
		projectRole, err := client.Role.Create().
			SetName("Project Role").
			SetScopes([]string{"read_requests", "write_requests"}).
			SetLevel(role.LevelProject).
			SetProjectID(project.ID).
			Save(ctx)
		require.NoError(t, err)

		// Create user with project role
		user, err := client.User.Create().
			SetEmail("projectroleuser@example.com").
			SetFirstName("ProjectRole").
			SetLastName("User").
			SetPassword("password").
			AddRoles(projectRole).
			Save(ctx)
		require.NoError(t, err)

		// Add user to project
		_, err = client.UserProject.Create().
			SetUserID(user.ID).
			SetProjectID(project.ID).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should be able to grant scopes from their project role
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_requests"}, &project.ID)
		require.NoError(t, err)
	})
}

func TestIntegrationWithRoleService(t *testing.T) {
	validator, client, userService := setupTestPermissionValidator(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	t.Run("creating role validates permissions", func(t *testing.T) {
		// Create user with limited scopes
		user, err := client.User.Create().
			SetEmail("creator@example.com").
			SetFirstName("Creator").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User should NOT be able to create role with scopes they don't have
		err = validator.CanGrantScopes(ctxWithUser, []string{"write_users", "read_projects"}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "insufficient permissions")
	})

	t.Run("updating role validates permissions", func(t *testing.T) {
		// Create role
		testRole, err := client.Role.Create().
			SetName("Editable Role").
			SetScopes([]string{"read_users"}).
			SetLevel(role.LevelSystem).
			SetProjectID(0).
			Save(ctx)
		require.NoError(t, err)

		// Create user with limited scopes
		user, err := client.User.Create().
			SetEmail("editor@example.com").
			SetFirstName("Editor").
			SetLastName("User").
			SetPassword("password").
			SetScopes([]string{"read_users"}).
			Save(ctx)
		require.NoError(t, err)

		// Load user with edges
		user, err = userService.GetUserByID(ctx, user.ID)
		require.NoError(t, err)

		ctxWithUser := contexts.WithUser(ctx, user)

		// User can edit role with scopes they have
		err = validator.CanEditRole(ctxWithUser, testRole.ID, nil)
		require.NoError(t, err)

		// But cannot grant new scopes they don't have
		err = validator.CanGrantScopes(ctxWithUser, []string{"read_users", "write_projects"}, nil)
		require.Error(t, err)
	})
}
