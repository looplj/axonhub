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
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	// Run migration when no owner user exists
	err := V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify no project was created
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestV0_3_0_WithOwnerUser(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
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
	err = V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify default project was created
	proj, err := client.Project.Query().Where(project.SlugEQ("default")).Only(ctx)
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
	roleCodes := make(map[string]bool)

	for _, r := range roles {
		roleNames[r.Name] = true
		roleCodes[r.Code] = true
		assert.Equal(t, role.LevelProject, r.Level)
		require.NotNil(t, r.ProjectID)
		assert.Equal(t, proj.ID, *r.ProjectID)
	}

	assert.True(t, roleNames["Admin"])
	assert.True(t, roleNames["Developer"])
	assert.True(t, roleNames["Viewer"])
	assert.True(t, roleCodes["default-admin"])
	assert.True(t, roleCodes["default-developer"])
	assert.True(t, roleCodes["default-viewer"])
}

func TestV0_3_0_ProjectAlreadyExists(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
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
		SetSlug("existing").
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
	err = V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify no new project was created (should still have only 1 project)
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the existing project is still there
	proj, err := client.Project.Query().Where(project.SlugEQ("existing")).Only(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Existing Project", proj.Name)
}

func TestV0_3_0_MultipleOwnerUsers(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
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
	err = V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify default project was created
	proj, err := client.Project.Query().Where(project.SlugEQ("default")).Only(ctx)
	require.NoError(t, err)

	// Verify first owner is assigned to the project
	userProjects, err := client.UserProject.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, userProjects, 1)
	userProject := userProjects[0]
	assert.Equal(t, owner1.ID, userProject.UserID)
	assert.Equal(t, proj.ID, userProject.ProjectID)
	assert.True(t, userProject.IsOwner)
}

func TestV0_3_0_Idempotency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
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
	err = V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify project was created
	count1, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count1)

	// Run migration second time - should be idempotent
	err = V0_3_0(ctx, client)
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
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
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
	err = V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify no project was created
	count, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestV0_3_0_VerifyRoleScopes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
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
	err = V0_3_0(ctx, client)
	require.NoError(t, err)

	// Verify admin role has correct scopes
	adminRole, err := client.Role.Query().Where(role.CodeEQ("default-admin")).Only(ctx)
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
	developerRole, err := client.Role.Query().Where(role.CodeEQ("default-developer")).Only(ctx)
	require.NoError(t, err)
	assert.Contains(t, developerRole.Scopes, "read_users")
	assert.Contains(t, developerRole.Scopes, "read_api_keys")
	assert.Contains(t, developerRole.Scopes, "write_api_keys")
	assert.Contains(t, developerRole.Scopes, "read_requests")
	assert.NotContains(t, developerRole.Scopes, "write_users")
	assert.NotContains(t, developerRole.Scopes, "write_roles")

	// Verify viewer role has correct scopes
	viewerRole, err := client.Role.Query().Where(role.CodeEQ("default-viewer")).Only(ctx)
	require.NoError(t, err)
	assert.Contains(t, viewerRole.Scopes, "read_users")
	assert.Contains(t, viewerRole.Scopes, "read_requests")
	assert.NotContains(t, viewerRole.Scopes, "write_users")
	assert.NotContains(t, viewerRole.Scopes, "write_api_keys")
}
