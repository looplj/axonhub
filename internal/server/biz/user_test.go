package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestUserService(t *testing.T) (*UserService, *ent.Client) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&_fk=1")

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
	require.NotNil(t, userInfo)

	// Verify basic fields
	require.Equal(t, "test@example.com", userInfo.Email)
	require.Equal(t, "John", userInfo.FirstName)
	require.Equal(t, "Doe", userInfo.LastName)
	require.Equal(t, "en", userInfo.PreferLanguage)
	require.Equal(t, false, userInfo.IsOwner)
	require.NotNil(t, userInfo.Avatar)
	require.Equal(t, "https://example.com/avatar.jpg", *userInfo.Avatar)

	// Verify scopes
	require.ElementsMatch(t, []string{"read_channels", "write_channels"}, userInfo.Scopes)

	// Verify empty roles and projects
	require.Empty(t, userInfo.Roles)
	require.Empty(t, userInfo.Projects)
}

func TestConvertUserToUserInfo_WithGlobalRoles(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create global roles
	adminRole, err := client.Role.Create().
		SetName("Administrator").
		SetLevel(role.LevelSystem).
		SetScopes([]string{"manage_users", "manage_projects", "manage_channels"}).
		Save(ctx)
	require.NoError(t, err)

	viewerRole, err := client.Role.Create().
		SetName("Viewer").
		SetLevel(role.LevelSystem).
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
	require.NotNil(t, userInfo)

	// Verify roles
	require.Len(t, userInfo.Roles, 2)
	roleNames := []string{userInfo.Roles[0].Name, userInfo.Roles[1].Name}
	require.ElementsMatch(t, []string{"Administrator", "Viewer"}, roleNames)

	// Verify scopes include user scopes + role scopes
	expectedScopes := []string{"custom_scope", "manage_users", "manage_projects", "manage_channels", "read_channels"}
	require.ElementsMatch(t, expectedScopes, userInfo.Scopes)
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
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project-specific roles
	projectAdminRole, err := client.Role.Create().
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project_channels", "manage_project_users"}).
		Save(ctx)
	require.NoError(t, err)

	projectMemberRole, err := client.Role.Create().
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
	require.NotNil(t, userProject)

	// Load user with edges
	testUser, err = client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		WithProjectUsers().
		Only(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	require.NotNil(t, userInfo)

	// Verify global roles (should be empty since all roles are project-specific)
	require.Empty(t, userInfo.Roles)

	// Verify global scopes (should be empty since user has no global scopes or roles)
	require.Empty(t, userInfo.Scopes)

	// Verify projects
	require.Len(t, userInfo.Projects, 1)
	projectInfo := userInfo.Projects[0]
	require.Equal(t, testProject.ID, projectInfo.ProjectID.ID)
	require.Equal(t, ent.TypeProject, projectInfo.ProjectID.Type)
	require.Equal(t, false, projectInfo.IsOwner)
	require.ElementsMatch(t, []string{"project_scope_1", "project_scope_2"}, projectInfo.Scopes)

	// Verify project roles
	require.Len(t, projectInfo.Roles, 2)
	projectRoleNames := []string{projectInfo.Roles[0].Name, projectInfo.Roles[1].Name}
	require.ElementsMatch(t, []string{"Project Admin", "Project Member"}, projectRoleNames)
}

func TestConvertUserToUserInfo_MixedRoles(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create global role
	globalRole, err := client.Role.Create().
		SetName("Global Admin").
		SetLevel(role.LevelSystem).
		SetScopes([]string{"global_scope_1", "global_scope_2"}).
		Save(ctx)
	require.NoError(t, err)

	// Create project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project role
	projectRole, err := client.Role.Create().
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
	require.NotNil(t, userInfo)

	// Verify global roles (only global_admin)
	require.Len(t, userInfo.Roles, 1)
	require.Equal(t, "Global Admin", userInfo.Roles[0].Name)

	// Verify global scopes (user scopes + global role scopes)
	expectedGlobalScopes := []string{"user_scope_1", "global_scope_1", "global_scope_2"}
	require.ElementsMatch(t, expectedGlobalScopes, userInfo.Scopes)

	// Verify projects
	require.Len(t, userInfo.Projects, 1)
	projectInfo := userInfo.Projects[0]
	require.Equal(t, true, projectInfo.IsOwner)
	require.ElementsMatch(t, []string{"up_scope_1"}, projectInfo.Scopes)

	// Verify project roles
	require.Len(t, projectInfo.Roles, 1)
	require.Equal(t, "Project Admin", projectInfo.Roles[0].Name)
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
		SetDescription("First project").
		Save(ctx)
	require.NoError(t, err)

	project2, err := client.Project.Create().
		SetName("Project 2").
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
	require.NotNil(t, userInfo)

	// Verify projects
	require.Len(t, userInfo.Projects, 2)

	// Check that both projects are present
	projectIDs := []int{userInfo.Projects[0].ProjectID.ID, userInfo.Projects[1].ProjectID.ID}
	require.ElementsMatch(t, []int{project1.ID, project2.ID}, projectIDs)
}

func TestAddUserToProject_Success(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project without roles
	isOwner := false
	scopes := []string{"read_project", "write_project"}
	userProject, err := userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, scopes, nil)

	require.NoError(t, err)
	require.NotNil(t, userProject)
	require.Equal(t, testUser.ID, userProject.UserID)
	require.Equal(t, testProject.ID, userProject.ProjectID)
	require.Equal(t, false, userProject.IsOwner)
	require.ElementsMatch(t, scopes, userProject.Scopes)
}

func TestAddUserToProject_WithRoles(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project roles
	projectRole1, err := client.Role.Create().
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project"}).
		Save(ctx)
	require.NoError(t, err)

	projectRole2, err := client.Role.Create().
		SetName("Project Member").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"read_project"}).
		Save(ctx)
	require.NoError(t, err)

	// Add user to project with roles
	isOwner := true
	scopes := []string{"custom_scope"}
	roleIDs := []int{projectRole1.ID, projectRole2.ID}
	userProject, err := userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, scopes, roleIDs)

	require.NoError(t, err)
	require.NotNil(t, userProject)
	require.Equal(t, testUser.ID, userProject.UserID)
	require.Equal(t, testProject.ID, userProject.ProjectID)
	require.Equal(t, true, userProject.IsOwner)
	require.ElementsMatch(t, scopes, userProject.Scopes)

	// Verify roles were added to user
	updatedUser, err := client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		Only(ctx)
	require.NoError(t, err)
	require.Len(t, updatedUser.Edges.Roles, 2)

	userRoleIDs := []int{updatedUser.Edges.Roles[0].ID, updatedUser.Edges.Roles[1].ID}
	require.ElementsMatch(t, roleIDs, userRoleIDs)
}

func TestAddUserToProject_WithNilOwner(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project with nil isOwner (should use default)
	userProject, err := userService.AddUserToProject(ctx, testUser.ID, testProject.ID, nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, userProject)
	require.Equal(t, testUser.ID, userProject.UserID)
	require.Equal(t, testProject.ID, userProject.ProjectID)
	// Default value for isOwner should be false
	require.Equal(t, false, userProject.IsOwner)
}

func TestAddUserToProject_DuplicateRelationship(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project first time
	isOwner := false
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, nil)
	require.NoError(t, err)

	// Try to add the same user to the same project again (should fail)
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, nil)
	require.Error(t, err)
}

func TestRemoveUserFromProject_Success(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project
	isOwner := false
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, nil)
	require.NoError(t, err)

	// Remove user from project
	err = userService.RemoveUserFromProject(ctx, testUser.ID, testProject.ID)
	require.NoError(t, err)

	// Verify the relationship no longer exists
	exists, err := client.UserProject.Query().
		Where(
			userproject.UserID(testUser.ID),
			userproject.ProjectID(testProject.ID),
		).
		Exist(ctx)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestRemoveUserFromProject_NotFound(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Try to remove a relationship that doesn't exist
	err = userService.RemoveUserFromProject(ctx, testUser.ID, testProject.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to find user project relationship")
}

func TestUpdateProjectUser_UpdateScopes(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project with initial scopes
	isOwner := false
	initialScopes := []string{"read_project"}
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, initialScopes, nil)
	require.NoError(t, err)

	// Update scopes
	newScopes := []string{"read_project", "write_project", "delete_project"}
	userProject, err := userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, newScopes, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, userProject)
	require.ElementsMatch(t, newScopes, userProject.Scopes)
}

func TestUpdateProjectUser_AddRoles(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project roles
	projectRole1, err := client.Role.Create().
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project"}).
		Save(ctx)
	require.NoError(t, err)

	projectRole2, err := client.Role.Create().
		SetName("Project Member").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"read_project"}).
		Save(ctx)
	require.NoError(t, err)

	// Add user to project without roles
	isOwner := false
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, nil)
	require.NoError(t, err)

	// Add roles to the project user
	addRoleIDs := []int{projectRole1.ID, projectRole2.ID}
	_, err = userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, nil, addRoleIDs, nil)
	require.NoError(t, err)

	// Verify roles were added
	updatedUser, err := client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		Only(ctx)
	require.NoError(t, err)
	require.Len(t, updatedUser.Edges.Roles, 2)

	userRoleIDs := []int{updatedUser.Edges.Roles[0].ID, updatedUser.Edges.Roles[1].ID}
	require.ElementsMatch(t, addRoleIDs, userRoleIDs)
}

func TestUpdateProjectUser_RemoveRoles(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project roles
	projectRole1, err := client.Role.Create().
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project"}).
		Save(ctx)
	require.NoError(t, err)

	projectRole2, err := client.Role.Create().
		SetName("Project Member").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"read_project"}).
		Save(ctx)
	require.NoError(t, err)

	// Add user to project with roles
	isOwner := false
	roleIDs := []int{projectRole1.ID, projectRole2.ID}
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, roleIDs)
	require.NoError(t, err)

	// Remove one role
	removeRoleIDs := []int{projectRole1.ID}
	_, err = userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, nil, nil, removeRoleIDs)
	require.NoError(t, err)

	// Verify only one role remains
	updatedUser, err := client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		Only(ctx)
	require.NoError(t, err)
	require.Len(t, updatedUser.Edges.Roles, 1)
	require.Equal(t, projectRole2.ID, updatedUser.Edges.Roles[0].ID)
}

func TestUpdateProjectUser_AddAndRemoveRoles(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project roles
	projectRole1, err := client.Role.Create().
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project"}).
		Save(ctx)
	require.NoError(t, err)

	projectRole2, err := client.Role.Create().
		SetName("Project Member").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"read_project"}).
		Save(ctx)
	require.NoError(t, err)

	projectRole3, err := client.Role.Create().
		SetName("Project Viewer").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"view_project"}).
		Save(ctx)
	require.NoError(t, err)

	// Add user to project with initial role
	isOwner := false
	roleIDs := []int{projectRole1.ID}
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, roleIDs)
	require.NoError(t, err)

	// Add new roles and remove the old one
	addRoleIDs := []int{projectRole2.ID, projectRole3.ID}
	removeRoleIDs := []int{projectRole1.ID}
	_, err = userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, nil, addRoleIDs, removeRoleIDs)
	require.NoError(t, err)

	// Verify roles were updated correctly
	updatedUser, err := client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		Only(ctx)
	require.NoError(t, err)
	require.Len(t, updatedUser.Edges.Roles, 2)

	userRoleIDs := []int{updatedUser.Edges.Roles[0].ID, updatedUser.Edges.Roles[1].ID}
	require.ElementsMatch(t, []int{projectRole2.ID, projectRole3.ID}, userRoleIDs)
}

func TestUpdateProjectUser_NotFound(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Try to update a relationship that doesn't exist
	newScopes := []string{"read_project"}
	_, err = userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, newScopes, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to find user project relationship")
}

func TestUpdateProjectUser_UpdateScopesAndRoles(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashed-password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		SetDescription("A test project").
		Save(ctx)
	require.NoError(t, err)

	// Create project role
	projectRole, err := client.Role.Create().
		SetName("Project Admin").
		SetLevel(role.LevelProject).
		SetProjectID(testProject.ID).
		SetScopes([]string{"manage_project"}).
		Save(ctx)
	require.NoError(t, err)

	// Add user to project with initial scopes
	isOwner := false
	initialScopes := []string{"read_project"}
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, initialScopes, nil)
	require.NoError(t, err)

	// Update both scopes and roles
	newScopes := []string{"read_project", "write_project"}
	addRoleIDs := []int{projectRole.ID}
	userProject, err := userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, newScopes, addRoleIDs, nil)

	require.NoError(t, err)
	require.NotNil(t, userProject)
	require.ElementsMatch(t, newScopes, userProject.Scopes)

	// Verify role was added
	updatedUser, err := client.User.Query().
		Where(user.IDEQ(testUser.ID)).
		WithRoles().
		Only(ctx)
	require.NoError(t, err)
	require.Len(t, updatedUser.Edges.Roles, 1)
	require.Equal(t, projectRole.ID, updatedUser.Edges.Roles[0].ID)
}

func TestUpdateUser_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
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

	// Update user
	newEmail := "newemail@example.com"
	input := ent.UpdateUserInput{
		Email: &newEmail,
	}
	_, err = userService.UpdateUser(ctx, testUser.ID, input)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after update")
}

func TestUpdateUserStatus_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Load user into cache
	_, err = userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Update user status
	_, err = userService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after status update")
}

func TestAddUserToProject_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		Save(ctx)
	require.NoError(t, err)

	// Load user into cache
	_, err = userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Add user to project
	isOwner := false
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, nil)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after adding to project")
}

func TestRemoveUserFromProject_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project
	isOwner := false
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, nil, nil)
	require.NoError(t, err)

	// Load user into cache
	_, err = userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Remove user from project
	err = userService.RemoveUserFromProject(ctx, testUser.ID, testProject.ID)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after removing from project")
}

func TestUpdateProjectUser_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create a project
	testProject, err := client.Project.Create().
		SetName("Test Project").
		Save(ctx)
	require.NoError(t, err)

	// Add user to project
	isOwner := false
	initialScopes := []string{"read"}
	_, err = userService.AddUserToProject(ctx, testUser.ID, testProject.ID, &isOwner, initialScopes, nil)
	require.NoError(t, err)

	// Load user into cache
	_, err = userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Update project user scopes
	newScopes := []string{"read", "write"}
	_, err = userService.UpdateProjectUser(ctx, testUser.ID, testProject.ID, newScopes, nil, nil)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after updating project user")
}
