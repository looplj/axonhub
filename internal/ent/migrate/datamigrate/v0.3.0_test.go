package datamigrate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/role"
)

func TestV0_3_0_NoOwnerUser(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Run migration when no owner user exists
	err := NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify no project was created
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestV0_3_0_MultipleProjectsExist(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create multiple existing projects
	_, err := client.Project.Create().
		SetName("Existing Project 1").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Project.Create().
		SetName("Existing Project 2").
		Save(ctx)
	require.NoError(t, err)

	// Run migration - should skip without error
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify no additional projects were created
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestV0_3_0_WithOwnerUser(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create an owner user
	owner, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("hashedpassword").
		SetFirstName("System").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify default project was created
	proj, err := client.Project.Query().Where(project.NameEQ("Default")).Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Default", proj.Name)
	assert.Equal(t, "Default project", proj.Description)
	assert.Equal(t, project.StatusActive, proj.Status)

	// Verify owner is assigned to the project
	userProjects, err := client.UserProject.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, userProjects, 1)
	userProject := userProjects[0]
	assert.Equal(t, owner.ID, userProject.UserID)
	assert.Equal(t, proj.ID, userProject.ProjectID)
	assert.True(t, userProject.IsOwner)

	// Verify default roles were created (admin, developer, viewer)
	roles, err := client.Role.Query().Where(role.ProjectIDEQ(proj.ID)).All(ctx)
	require.NoError(t, err)
	assert.Len(t, roles, 3)

	// Check role names and codes
	roleNames := make(map[string]bool)

	for _, r := range roles {
		roleNames[r.Name] = true
		assert.Equal(t, role.LevelProject, r.Level)
		require.NotNil(t, r.ProjectID)
		assert.Equal(t, proj.ID, *r.ProjectID)
	}

	assert.True(t, roleNames["Admin"])
	assert.True(t, roleNames["Developer"])
	assert.True(t, roleNames["Viewer"])
}

func TestV0_3_0_ProjectAlreadyExists(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create an owner user
	owner, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("hashedpassword").
		SetFirstName("System").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Create an existing project
	existingProj, err := client.Project.Create().
		SetName("Existing Project").
		Save(ctx)
	require.NoError(t, err)

	// Assign owner to existing project
	_, err = client.UserProject.Create().
		SetUserID(owner.ID).
		SetProjectID(existingProj.ID).
		SetIsOwner(true).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify no new project was created (should still have only 1 project)
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the existing project is still there
	proj, err := client.Project.Query().Where(project.NameEQ("Existing Project")).Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Existing Project", proj.Name)
}

func TestV0_3_0_MultipleOwnerUsers(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create first owner user
	owner1, err := client.User.Create().
		SetEmail("owner1@example.com").
		SetPassword("hashedpassword").
		SetFirstName("First").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Create second owner user (edge case, but should handle gracefully)
	_, err = client.User.Create().
		SetEmail("owner2@example.com").
		SetPassword("hashedpassword").
		SetFirstName("Second").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration - should use the first owner found
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify default project was created
	proj, err := client.Project.Query().Where(project.NameEQ("Default")).Only(ctx)
	require.NoError(t, err)

	// Verify both owners are assigned to the project
	userProjects, err := client.UserProject.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, userProjects, 2) // Both owner1 and owner2 should be assigned

	// Find owner1's project assignment
	var owner1Project *ent.UserProject

	for _, up := range userProjects {
		if up.UserID == owner1.ID {
			owner1Project = up
			break
		}
	}

	require.NotNil(t, owner1Project)
	assert.Equal(t, owner1.ID, owner1Project.UserID)
	assert.Equal(t, proj.ID, owner1Project.ProjectID)
	assert.True(t, owner1Project.IsOwner) // owner1 is the project owner
}

func TestV0_3_0_Idempotency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create an owner user
	_, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("hashedpassword").
		SetFirstName("System").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration first time
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify project was created
	count1, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count1)

	// Run migration second time - should be idempotent
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify still only one project exists
	count2, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count2)

	// Verify still only 3 roles exist
	roleCount, err := client.Role.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, roleCount)
}

func TestV0_3_0_OwnerWithoutIsOwnerFlag(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a regular user (not marked as owner)
	_, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashedpassword").
		SetFirstName("Regular").
		SetLastName("User").
		SetIsOwner(false).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration - should not create project since no owner exists
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify no project was created
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestV0_3_0_VerifyRoleScopes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create an owner user
	_, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("hashedpassword").
		SetFirstName("System").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify admin role has correct scopes
	adminRole, err := client.Role.Query().Where(role.NameEQ("Admin")).Only(ctx)
	require.NoError(t, err)
	assert.Contains(t, adminRole.Scopes, "read_users")
	assert.Contains(t, adminRole.Scopes, "write_users")
	assert.Contains(t, adminRole.Scopes, "read_roles")
	assert.Contains(t, adminRole.Scopes, "write_roles")
	assert.Contains(t, adminRole.Scopes, "read_api_keys")
	assert.Contains(t, adminRole.Scopes, "write_api_keys")
	assert.Contains(t, adminRole.Scopes, "read_requests")
	assert.Contains(t, adminRole.Scopes, "write_requests")

	// Verify developer role has correct scopes
	developerRole, err := client.Role.Query().Where(role.NameEQ("Developer")).Only(ctx)
	require.NoError(t, err)
	assert.Contains(t, developerRole.Scopes, "read_users")
	assert.Contains(t, developerRole.Scopes, "read_api_keys")
	assert.Contains(t, developerRole.Scopes, "write_api_keys")
	assert.Contains(t, developerRole.Scopes, "read_requests")
	assert.NotContains(t, developerRole.Scopes, "write_users")
	assert.NotContains(t, developerRole.Scopes, "write_roles")

	// Verify viewer role has correct scopes
	viewerRole, err := client.Role.Query().Where(role.NameEQ("Viewer")).Only(ctx)
	require.NoError(t, err)
	assert.Contains(t, viewerRole.Scopes, "read_users")
	assert.Contains(t, viewerRole.Scopes, "read_requests")
	assert.NotContains(t, viewerRole.Scopes, "write_users")
	assert.NotContains(t, viewerRole.Scopes, "write_api_keys")
}

func TestV0_3_0_AssignUsersToDefaultProject(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create an owner user
	owner, err := client.User.Create().
		SetEmail("owner@example.com").
		SetPassword("hashedpassword").
		SetFirstName("System").
		SetLastName("Owner").
		SetIsOwner(true).
		SetScopes([]string{"*"}).
		Save(ctx)
	require.NoError(t, err)

	// Create regular users
	user1, err := client.User.Create().
		SetEmail("user1@example.com").
		SetPassword("hashedpassword").
		SetFirstName("User").
		SetLastName("One").
		SetIsOwner(false).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	user2, err := client.User.Create().
		SetEmail("user2@example.com").
		SetPassword("hashedpassword").
		SetFirstName("User").
		SetLastName("Two").
		SetIsOwner(false).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify default project was created
	proj, err := client.Project.Query().Where(project.NameEQ("Default")).Only(ctx)
	require.NoError(t, err)

	// Verify all users are assigned to the default project
	userProjects, err := client.UserProject.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, userProjects, 3) // owner + 2 regular users

	// Check that owner is assigned
	var (
		ownerProject *ent.UserProject
		user1Project *ent.UserProject
		user2Project *ent.UserProject
	)

	for _, up := range userProjects {
		if up.UserID == owner.ID {
			ownerProject = up
		} else if up.UserID == user1.ID {
			user1Project = up
		} else if up.UserID == user2.ID {
			user2Project = up
		}
	}

	require.NotNil(t, ownerProject)
	assert.Equal(t, owner.ID, ownerProject.UserID)
	assert.Equal(t, proj.ID, ownerProject.ProjectID)
	assert.True(t, ownerProject.IsOwner)

	require.NotNil(t, user1Project)
	assert.Equal(t, user1.ID, user1Project.UserID)
	assert.Equal(t, proj.ID, user1Project.ProjectID)
	assert.False(t, user1Project.IsOwner)

	require.NotNil(t, user2Project)
	assert.Equal(t, user2.ID, user2Project.UserID)
	assert.Equal(t, proj.ID, user2Project.ProjectID)
	assert.False(t, user2Project.IsOwner)
}

func TestV0_3_0_AssignUsersToDefaultProject_ConstraintError(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a regular user
	user1, err := client.User.Create().
		SetEmail("user1@example.com").
		SetPassword("hashedpassword").
		SetFirstName("User").
		SetLastName("One").
		SetIsOwner(false).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Create default project manually
	proj, err := client.Project.Create().
		SetName("Default").
		SetDescription("Default project").
		Save(ctx)
	require.NoError(t, err)

	// Assign user to project manually before migration
	_, err = client.UserProject.Create().
		SetUserID(user1.ID).
		SetProjectID(proj.ID).
		SetIsOwner(false).
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	// Run migration - should handle constraint error gracefully
	err = NewV0_3_0().Migrate(ctx, client)
	require.NoError(t, err)

	// Verify only one assignment exists for user1
	userProjects, err := client.UserProject.Query().All(ctx)
	require.NoError(t, err)

	count := 0

	for _, up := range userProjects {
		if up.UserID == user1.ID {
			count++
		}
	}

	assert.Equal(t, 1, count)
}
