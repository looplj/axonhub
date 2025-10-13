package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestUserService(t *testing.T) (*UserService, *ent.Client) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	userService := &UserService{
		UserCache: xcache.NewFromConfig[ent.User](cacheConfig),
	}

	return userService, client
}

func TestConvertUserToUserInfo_BasicUser(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a basic user without roles or projects
	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("hashed-password").
		SetFirstName("John").
		SetLastName("Doe").
		SetPreferLanguage("en").
		SetAvatar("https://example.com/avatar.jpg").
		SetIsOwner(false).
		SetScopes([]string{"read_channels", "write_channels"}).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Load user with edges
	testUser, err = client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	assert.NotNil(t, userInfo)

	// Verify basic fields
	assert.Equal(t, "test@example.com", userInfo.Email)
	assert.Equal(t, "John", userInfo.FirstName)
	assert.Equal(t, "Doe", userInfo.LastName)
	assert.Equal(t, "en", userInfo.PreferLanguage)
	assert.Equal(t, false, userInfo.IsOwner)
	assert.NotNil(t, userInfo.Avatar)
	assert.Equal(t, "https://example.com/avatar.jpg", *userInfo.Avatar)

	// Verify scopes
	assert.ElementsMatch(t, []string{"read_channels", "write_channels"}, userInfo.Scopes)

	// Verify empty roles and projects
	assert.Empty(t, userInfo.Roles)
	assert.Empty(t, userInfo.Projects)
}

func TestConvertUserToUserInfo_WithGlobalRoles(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create global roles
	adminRole, err := client.Role.Create().
		SetCode("admin").
		SetName("Administrator").
		SetLevel(role.LevelGlobal).
		SetScopes([]string{"manage_users", "manage_projects", "manage_channels"}).
		Save(ctx)
	require.NoError(t, err)

	viewerRole, err := client.Role.Create().
		SetCode("viewer").
		SetName("Viewer").
		SetLevel(role.LevelGlobal).
		SetScopes([]string{"read_channels"}).
		Save(ctx)
	require.NoError(t, err)

	// Create user with global roles
	testUser, err := client.User.Create().
		SetEmail("admin@example.com").
		SetPassword("hashed-password").
		SetFirstName("Admin").
		SetLastName("User").
		SetPreferLanguage("en").
		SetIsOwner(false).
		SetScopes([]string{"custom_scope"}).
		SetStatus(user.StatusActivated).
		AddRoles(adminRole, viewerRole).
		Save(ctx)
	require.NoError(t, err)

	// Load user with edges
	testUser, err = client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	assert.NotNil(t, userInfo)

	// Verify roles
	assert.Len(t, userInfo.Roles, 2)
	roleCodes := []string{userInfo.Roles[0].Code, userInfo.Roles[1].Code}
	assert.ElementsMatch(t, []string{"admin", "viewer"}, roleCodes)

	// Verify scopes include user scopes + role scopes
	expectedScopes := []string{"custom_scope", "manage_users", "manage_projects", "manage_channels", "read_channels"}
	assert.ElementsMatch(t, expectedScopes, userInfo.Scopes)
}

func TestConvertUserToUserInfo_WithProjectRoles(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetSlug("test-project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project-specific roles
	projectAdminRole, err := client.Role.Create().
		SetCode("project_admin").
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project_channels", "manage_project_users"}).
		Save(ctx)
	require.NoError(t, err)

	projectMemberRole, err := client.Role.Create().
		SetCode("project_member").
		SetName("Project Member").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"read_project_channels"}).
		Save(ctx)
	require.NoError(t, err)

	// Create user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Project").
		SetLastName("User").
		SetPreferLanguage("en").
		SetIsOwner(false).
		SetScopes([]string{}).
		SetStatus(user.StatusActivated).
		AddRoles(projectAdminRole, projectMemberRole).
		Save(ctx)
	require.NoError(t, err)

	// Create UserProject relationship
	userProject, err := client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(false).
		SetScopes([]string{"project_scope_1", "project_scope_2"}).
		Save(ctx)
	require.NoError(t, err)
	assert.NotNil(t, userProject)

	// Load user with edges
	testUser, err = client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	assert.NotNil(t, userInfo)

	// Verify global roles (should be empty since all roles are project-specific)
	assert.Empty(t, userInfo.Roles)

	// Verify global scopes (should be empty since user has no global scopes or roles)
	assert.Empty(t, userInfo.Scopes)

	// Verify projects
	assert.Len(t, userInfo.Projects, 1)
	projectInfo := userInfo.Projects[0]
	assert.Equal(t, testProject.ID, projectInfo.ProjectID.ID)
	assert.Equal(t, ent.TypeProject, projectInfo.ProjectID.Type)
	assert.Equal(t, false, projectInfo.IsOwner)
	assert.ElementsMatch(t, []string{"project_scope_1", "project_scope_2"}, projectInfo.Scopes)

	// Verify project roles
	assert.Len(t, projectInfo.Roles, 2)
	projectRoleCodes := []string{projectInfo.Roles[0].Code, projectInfo.Roles[1].Code}
	assert.ElementsMatch(t, []string{"project_admin", "project_member"}, projectRoleCodes)
}

func TestConvertUserToUserInfo_MixedRoles(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create global role
	globalRole, err := client.Role.Create().
		SetCode("global_admin").
		SetName("Global Admin").
		SetLevel(role.LevelGlobal).
		SetScopes([]string{"global_scope_1", "global_scope_2"}).
		Save(ctx)
	require.NoError(t, err)

	// Create project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetSlug("test-project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project role
	projectRole, err := client.Role.Create().
		SetCode("project_admin").
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"project_scope_1"}).
		Save(ctx)
	require.NoError(t, err)

	// Create user with both global and project roles
	testUser, err := client.User.Create().
		SetEmail("mixed@example.com").
		SetPassword("hashed-password").
		SetFirstName("Mixed").
		SetLastName("User").
		SetPreferLanguage("en").
		SetIsOwner(true).
		SetScopes([]string{"user_scope_1"}).
		SetStatus(user.StatusActivated).
		AddRoles(globalRole, projectRole).
		Save(ctx)
	require.NoError(t, err)

	// Create UserProject relationship
	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(testProject.ID).
		SetIsOwner(true).
		SetScopes([]string{"up_scope_1"}).
		Save(ctx)
	require.NoError(t, err)

	// Load user with edges
	testUser, err = client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	assert.NotNil(t, userInfo)

	// Verify global roles (only global_admin)
	assert.Len(t, userInfo.Roles, 1)
	assert.Equal(t, "global_admin", userInfo.Roles[0].Code)

	// Verify global scopes (user scopes + global role scopes)
	expectedGlobalScopes := []string{"user_scope_1", "global_scope_1", "global_scope_2"}
	assert.ElementsMatch(t, expectedGlobalScopes, userInfo.Scopes)

	// Verify projects
	assert.Len(t, userInfo.Projects, 1)
	projectInfo := userInfo.Projects[0]
	assert.Equal(t, true, projectInfo.IsOwner)
	assert.ElementsMatch(t, []string{"up_scope_1"}, projectInfo.Scopes)

	// Verify project roles
	assert.Len(t, projectInfo.Roles, 1)
	assert.Equal(t, "project_admin", projectInfo.Roles[0].Code)
}

func TestConvertUserToUserInfo_NilUser(t *testing.T) {
	// Test with nil user
	require.Panics(t, func() {
		ConvertUserToUserInfo(context.Background(), nil)
	})
}

func TestConvertUserToUserInfo_MultipleProjects(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create multiple projects
	project1, err := client.Project.Create().
		SetName("Project 1").
		SetSlug("project-1").
		SetDescription("First project").
		Save(ctx)
	require.NoError(t, err)

	project2, err := client.Project.Create().
		SetName("Project 2").
		SetSlug("project-2").
		SetDescription("Second project").
		Save(ctx)
	require.NoError(t, err)

	// Create user
	testUser, err := client.User.Create().
		SetEmail("multi@example.com").
		SetPassword("hashed-password").
		SetFirstName("Multi").
		SetLastName("Project").
		SetPreferLanguage("en").
		SetIsOwner(false).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create UserProject relationships
	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(project1.ID).
		SetIsOwner(true).
		SetScopes([]string{"p1_scope"}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserProject.Create().
		SetUserID(testUser.ID).
		SetProjectID(project2.ID).
		SetIsOwner(false).
		SetScopes([]string{"p2_scope"}).
		Save(ctx)
	require.NoError(t, err)

	// Load user with edges
	testUser, err = client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	assert.NotNil(t, userInfo)

	// Verify projects
	assert.Len(t, userInfo.Projects, 2)

	// Check that both projects are present
	projectIDs := []int{userInfo.Projects[0].ProjectID.ID, userInfo.Projects[1].ProjectID.ID}
	assert.ElementsMatch(t, []int{project1.ID, project2.ID}, projectIDs)
}
