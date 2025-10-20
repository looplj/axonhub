package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestRoleService(t *testing.T) (*RoleService, *UserService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	userService := &UserService{
		UserCache:           xcache.NewFromConfig[ent.User](cacheConfig),
		permissionValidator: NewPermissionValidator(),
	}

	roleService := &RoleService{
		userService:         userService,
		permissionValidator: NewPermissionValidator(),
	}

	return roleService, userService, client
}

func TestCreateRole(t *testing.T) {
	roleService, _, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create owner user for permission checks
	owner, err := client.User.Create().
		SetEmail("owner@test.com").
		SetPassword("password").
		SetFirstName("Owner").
		SetLastName("User").
		SetIsOwner(true).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithUser(ctx, owner)

	t.Run("create global role successfully", func(t *testing.T) {
		input := ent.CreateRoleInput{
			Name:   "Administrator",
			Scopes: []string{"manage_users", "manage_projects"},
		}

		createdRole, err := roleService.CreateRole(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, createdRole)
		require.Equal(t, "Administrator", createdRole.Name)
		require.ElementsMatch(t, []string{"manage_users", "manage_projects"}, createdRole.Scopes)
		require.NotNil(t, createdRole.ProjectID)
		require.Zero(t, *createdRole.ProjectID)
	})

	t.Run("create project-specific role successfully", func(t *testing.T) {
		// Create a project first
		project, err := client.Project.Create().
			SetName("Test Project").
			Save(ctx)
		require.NoError(t, err)

		input := ent.CreateRoleInput{
			Name:      "Project Administrator",
			Level:     lo.ToPtr(role.LevelProject),
			Scopes:    []string{"manage_project"},
			ProjectID: &project.ID,
		}

		createdRole, err := roleService.CreateRole(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, createdRole)
		require.Equal(t, "Project Administrator", createdRole.Name)
		require.NotNil(t, createdRole.ProjectID)
		require.Equal(t, project.ID, *createdRole.ProjectID)
	})

	t.Run("fail to create role with duplicate code", func(t *testing.T) {
		input := ent.CreateRoleInput{
			Name:   "Duplicate Role",
			Scopes: []string{"read"},
		}

		_, err := roleService.CreateRole(ctx, input)
		require.NoError(t, err)

		// Try to create another role with the same code
		_, err = roleService.CreateRole(ctx, input)
		require.Error(t, err)
	})
}

func TestUpdateRole(t *testing.T) {
	roleService, _, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create owner user for permission checks
	owner, err := client.User.Create().
		SetEmail("owner@test.com").
		SetPassword("password").
		SetFirstName("Owner").
		SetLastName("User").
		SetIsOwner(true).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithUser(ctx, owner)

	// Create a role first
	createdRole, err := client.Role.Create().
		SetName("Editor").
		SetScopes([]string{"read", "write"}).
		Save(ctx)
	require.NoError(t, err)

	t.Run("update role name successfully", func(t *testing.T) {
		newName := "Senior Editor"
		input := ent.UpdateRoleInput{
			Name: &newName,
		}

		updatedRole, err := roleService.UpdateRole(ctx, createdRole.ID, input)
		require.NoError(t, err)
		require.NotNil(t, updatedRole)
		require.Equal(t, "Senior Editor", updatedRole.Name)
	})

	t.Run("update role scopes successfully", func(t *testing.T) {
		newScopes := []string{"read", "write", "delete"}
		input := ent.UpdateRoleInput{
			Scopes: newScopes,
		}

		updatedRole, err := roleService.UpdateRole(ctx, createdRole.ID, input)
		require.NoError(t, err)
		require.NotNil(t, updatedRole)
		require.ElementsMatch(t, newScopes, updatedRole.Scopes)
	})

	t.Run("fail to update non-existent role", func(t *testing.T) {
		newName := "Non-existent"
		input := ent.UpdateRoleInput{
			Name: &newName,
		}

		_, err := roleService.UpdateRole(ctx, 99999, input)
		require.Error(t, err)
	})
}

func TestDeleteRole(t *testing.T) {
	roleService, _, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	t.Run("delete role without users successfully", func(t *testing.T) {
		// Create a role
		testRole, err := client.Role.Create().
			SetName("Temporary Role").
			SetScopes([]string{"temp"}).
			Save(ctx)
		require.NoError(t, err)

		// Delete the role
		err = roleService.DeleteRole(ctx, testRole.ID)
		require.NoError(t, err)

		// Verify role is deleted
		exists, err := client.Role.Query().
			Where(role.IDEQ(testRole.ID)).
			Exist(ctx)
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("delete role with users successfully", func(t *testing.T) {
		// Create a role
		testRole, err := client.Role.Create().
			SetName("Role With Users").
			SetScopes([]string{"read"}).
			Save(ctx)
		require.NoError(t, err)

		// Create users
		user1, err := client.User.Create().
			SetEmail("user1@example.com").
			SetPassword("password").
			SetStatus(user.StatusActivated).
			Save(ctx)
		require.NoError(t, err)

		user2, err := client.User.Create().
			SetEmail("user2@example.com").
			SetPassword("password").
			SetStatus(user.StatusActivated).
			Save(ctx)
		require.NoError(t, err)

		// Create UserRole relationships
		_, err = client.UserRole.Create().
			SetUserID(user1.ID).
			SetRoleID(testRole.ID).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.UserRole.Create().
			SetUserID(user2.ID).
			SetRoleID(testRole.ID).
			Save(ctx)
		require.NoError(t, err)

		// Verify UserRole relationships exist
		count, err := client.UserRole.Query().
			Where(userrole.RoleID(testRole.ID)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		// Delete the role
		err = roleService.DeleteRole(ctx, testRole.ID)
		require.NoError(t, err)

		// Verify role is deleted
		exists, err := client.Role.Query().
			Where(role.IDEQ(testRole.ID)).
			Exist(ctx)
		require.NoError(t, err)
		require.False(t, exists)

		// Verify UserRole relationships are deleted
		count, err = client.UserRole.Query().
			Where(userrole.RoleID(testRole.ID)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Verify users still exist
		exists, err = client.User.Query().
			Where(user.IDEQ(user1.ID)).
			Exist(ctx)
		require.NoError(t, err)
		require.True(t, exists)

		exists, err = client.User.Query().
			Where(user.IDEQ(user2.ID)).
			Exist(ctx)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("fail to delete non-existent role", func(t *testing.T) {
		err := roleService.DeleteRole(ctx, 99999)
		require.Error(t, err)
		require.Contains(t, err.Error(), "role not found")
	})
}

func TestBulkDeleteRoles(t *testing.T) {
	roleService, _, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	t.Run("bulk delete roles without users successfully", func(t *testing.T) {
		// Create multiple roles
		role1, err := client.Role.Create().
			SetName("Bulk Role 1").
			SetScopes([]string{"read"}).
			Save(ctx)
		require.NoError(t, err)

		role2, err := client.Role.Create().
			SetName("Bulk Role 2").
			SetScopes([]string{"write"}).
			Save(ctx)
		require.NoError(t, err)

		role3, err := client.Role.Create().
			SetName("Bulk Role 3").
			SetScopes([]string{"delete"}).
			Save(ctx)
		require.NoError(t, err)

		// Bulk delete roles
		roleIDs := []int{role1.ID, role2.ID, role3.ID}
		err = roleService.BulkDeleteRoles(ctx, roleIDs)
		require.NoError(t, err)

		// Verify all roles are deleted
		count, err := client.Role.Query().
			Where(role.IDIn(roleIDs...)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("bulk delete roles with users successfully", func(t *testing.T) {
		// Create roles
		role1, err := client.Role.Create().
			SetName("Bulk Role Users 1").
			SetScopes([]string{"read"}).
			Save(ctx)
		require.NoError(t, err)

		role2, err := client.Role.Create().
			SetName("Bulk Role Users 2").
			SetScopes([]string{"write"}).
			Save(ctx)
		require.NoError(t, err)

		// Create users
		user1, err := client.User.Create().
			SetEmail("bulk_user1@example.com").
			SetPassword("password").
			SetStatus(user.StatusActivated).
			Save(ctx)
		require.NoError(t, err)

		user2, err := client.User.Create().
			SetEmail("bulk_user2@example.com").
			SetPassword("password").
			SetStatus(user.StatusActivated).
			Save(ctx)
		require.NoError(t, err)

		// Create UserRole relationships
		_, err = client.UserRole.Create().
			SetUserID(user1.ID).
			SetRoleID(role1.ID).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.UserRole.Create().
			SetUserID(user1.ID).
			SetRoleID(role2.ID).
			Save(ctx)
		require.NoError(t, err)

		_, err = client.UserRole.Create().
			SetUserID(user2.ID).
			SetRoleID(role1.ID).
			Save(ctx)
		require.NoError(t, err)

		// Verify UserRole relationships exist
		count, err := client.UserRole.Query().
			Where(userrole.RoleIDIn(role1.ID, role2.ID)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, count)

		// Bulk delete roles
		roleIDs := []int{role1.ID, role2.ID}
		err = roleService.BulkDeleteRoles(ctx, roleIDs)
		require.NoError(t, err)

		// Verify all roles are deleted
		count, err = client.Role.Query().
			Where(role.IDIn(roleIDs...)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Verify all UserRole relationships are deleted
		count, err = client.UserRole.Query().
			Where(userrole.RoleIDIn(roleIDs...)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Verify users still exist
		count, err = client.User.Query().
			Where(user.IDIn(user1.ID, user2.ID)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("fail to bulk delete with non-existent role", func(t *testing.T) {
		// Create one valid role
		validRole, err := client.Role.Create().
			SetName("Valid Role").
			SetScopes([]string{"read"}).
			Save(ctx)
		require.NoError(t, err)

		// Try to delete with one valid and one invalid ID
		roleIDs := []int{validRole.ID, 99999}
		err = roleService.BulkDeleteRoles(ctx, roleIDs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected to find")

		// Verify the valid role still exists (transaction should rollback)
		exists, err := client.Role.Query().
			Where(role.IDEQ(validRole.ID)).
			Exist(ctx)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("bulk delete with empty list", func(t *testing.T) {
		err := roleService.BulkDeleteRoles(ctx, []int{})
		require.NoError(t, err)
	})
}

func TestUpdateRole_CacheInvalidation(t *testing.T) {
	roleService, userService, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create owner user for permission checks
	owner, err := client.User.Create().
		SetEmail("owner@test.com").
		SetPassword("password").
		SetFirstName("Owner").
		SetLastName("User").
		SetIsOwner(true).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithUser(ctx, owner)

	// Create a role
	testRole, err := client.Role.Create().
		SetName("Test Role").
		SetScopes([]string{"read", "write"}).
		Save(ctx)
	require.NoError(t, err)

	// Create a user with this role
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		AddRoles(testRole).
		Save(ctx)
	require.NoError(t, err)

	// Load user into cache
	cachedUser, err := userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)
	require.NotNil(t, cachedUser)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Update role scopes
	newScopes := []string{"read", "write", "delete"}
	input := ent.UpdateRoleInput{
		Scopes: newScopes,
	}
	_, err = roleService.UpdateRole(ctx, testRole.ID, input)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after role update")
}

func TestDeleteRole_CacheInvalidation(t *testing.T) {
	roleService, userService, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a role
	testRole, err := client.Role.Create().
		SetName("Test Role").
		SetScopes([]string{"read"}).
		Save(ctx)
	require.NoError(t, err)

	// Create multiple users with this role
	user1, err := client.User.Create().
		SetEmail("user1@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		AddRoles(testRole).
		Save(ctx)
	require.NoError(t, err)

	user2, err := client.User.Create().
		SetEmail("user2@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		AddRoles(testRole).
		Save(ctx)
	require.NoError(t, err)

	// Load users into cache
	_, err = userService.GetUserByID(ctx, user1.ID)
	require.NoError(t, err)
	_, err = userService.GetUserByID(ctx, user2.ID)
	require.NoError(t, err)

	// Verify both users are in cache
	cacheKey1 := buildUserCacheKey(user1.ID)
	cacheKey2 := buildUserCacheKey(user2.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey1)
	require.NoError(t, err, "User1 should be in cache")
	_, err = userService.UserCache.Get(ctx, cacheKey2)
	require.NoError(t, err, "User2 should be in cache")

	// Delete the role
	err = roleService.DeleteRole(ctx, testRole.ID)
	require.NoError(t, err)

	// Verify both users' caches were invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey1)
	require.Error(t, err, "User1 cache should be invalidated after role deletion")
	_, err = userService.UserCache.Get(ctx, cacheKey2)
	require.Error(t, err, "User2 cache should be invalidated after role deletion")
}

func TestBulkDeleteRoles_CacheInvalidation(t *testing.T) {
	roleService, userService, client := setupTestRoleService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create multiple roles
	role1, err := client.Role.Create().
		SetName("Role 1").
		SetScopes([]string{"read"}).
		Save(ctx)
	require.NoError(t, err)

	role2, err := client.Role.Create().
		SetName("Role 2").
		SetScopes([]string{"write"}).
		Save(ctx)
	require.NoError(t, err)

	// Create users with different roles
	user1, err := client.User.Create().
		SetEmail("user1@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		AddRoles(role1).
		Save(ctx)
	require.NoError(t, err)

	user2, err := client.User.Create().
		SetEmail("user2@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		AddRoles(role2).
		Save(ctx)
	require.NoError(t, err)

	user3, err := client.User.Create().
		SetEmail("user3@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		AddRoles(role1, role2).
		Save(ctx)
	require.NoError(t, err)

	// Load all users into cache
	_, err = userService.GetUserByID(ctx, user1.ID)
	require.NoError(t, err)
	_, err = userService.GetUserByID(ctx, user2.ID)
	require.NoError(t, err)
	_, err = userService.GetUserByID(ctx, user3.ID)
	require.NoError(t, err)

	// Verify all users are in cache
	cacheKey1 := buildUserCacheKey(user1.ID)
	cacheKey2 := buildUserCacheKey(user2.ID)
	cacheKey3 := buildUserCacheKey(user3.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey1)
	require.NoError(t, err, "User1 should be in cache")
	_, err = userService.UserCache.Get(ctx, cacheKey2)
	require.NoError(t, err, "User2 should be in cache")
	_, err = userService.UserCache.Get(ctx, cacheKey3)
	require.NoError(t, err, "User3 should be in cache")

	// Bulk delete roles
	roleIDs := []int{role1.ID, role2.ID}
	err = roleService.BulkDeleteRoles(ctx, roleIDs)
	require.NoError(t, err)

	// Verify all users' caches were invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey1)
	require.Error(t, err, "User1 cache should be invalidated")
	_, err = userService.UserCache.Get(ctx, cacheKey2)
	require.Error(t, err, "User2 cache should be invalidated")
	_, err = userService.UserCache.Get(ctx, cacheKey3)
	require.Error(t, err, "User3 cache should be invalidated")
}
