package datamigrate_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/invitation"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userrole"
)

func TestV1_0_0_Beta7_BackfillsLegacyInvitationRoles(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:legacy-invitation-role?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("legacy-invitation-project").SaveX(ctx)
	customDeveloperRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"*"}).
		SaveX(ctx)
	legacyInvitation := client.Invitation.Create().SetTokenHash("legacy-token").SetProjectID(project.ID).SaveX(ctx)

	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))
	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	legacyInvitation = client.Invitation.GetX(ctx, legacyInvitation.ID)
	require.NotNil(t, legacyInvitation.RoleID)
	developerRole := client.Role.Query().Where(
		role.LevelEQ(role.LevelProject),
		role.ProjectIDEQ(project.ID),
		role.NameEQ("Developer"),
	).OnlyX(ctx)
	require.Equal(t, customDeveloperRole.ID, developerRole.ID)
	require.Equal(t, []string{"*"}, developerRole.Scopes)
	require.Equal(t, developerRole.ID, *legacyInvitation.RoleID)
	backfilled, err := client.Invitation.Query().Where(invitation.IDEQ(legacyInvitation.ID), invitation.RoleIDEQ(developerRole.ID)).Exist(ctx)
	require.NoError(t, err)
	require.True(t, backfilled)
}

func TestV1_0_0_Beta7_CreatesMissingDefaultDeveloperRole(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:missing-default-developer-role?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("missing-default-developer-project").SaveX(ctx)
	legacyInvitation := client.Invitation.Create().SetTokenHash("missing-default-developer-token").SetProjectID(project.ID).SaveX(ctx)

	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	developerRole := client.Role.Query().Where(role.LevelEQ(role.LevelProject), role.ProjectIDEQ(project.ID), role.NameEQ("Developer")).OnlyX(ctx)
	require.ElementsMatch(t, []string{"read_api_keys", "write_api_keys", "read_requests", "write_requests"}, developerRole.Scopes)
	legacyInvitation = client.Invitation.GetX(ctx, legacyInvitation.ID)
	require.NotNil(t, legacyInvitation.RoleID)
	require.Equal(t, developerRole.ID, *legacyInvitation.RoleID)
}

func TestV1_0_0_Beta7_SkipsRevokedLegacyInvitations(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:revoked-legacy-invitation?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("revoked-legacy-invitation-project").SaveX(ctx)
	revoked := client.Invitation.Create().SetTokenHash("revoked-legacy-token").SetProjectID(project.ID).SaveX(ctx)
	require.NoError(t, client.Invitation.DeleteOneID(revoked.ID).Exec(ctx))

	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	revokedInvitation := client.Invitation.Query().Where(invitation.IDEQ(revoked.ID)).OnlyX(schematype.SkipSoftDelete(ctx))
	require.Nil(t, revokedInvitation.RoleID)
}

func TestV1_0_0_Beta7_PreservesUnrelatedSoftDeletedDeveloper(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:unrelated-soft-deleted-developer?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("unrelated-soft-deleted-developer-project").SaveX(ctx)
	developerRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"old_scope"}).
		SaveX(ctx)
	testUser := client.User.Create().
		SetEmail("unrelated-role-user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SaveX(ctx)
	client.UserRole.Create().SetUserID(testUser.ID).SetRoleID(developerRole.ID).SaveX(ctx)
	client.Role.DeleteOneID(developerRole.ID).ExecX(ctx)

	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	preservedRole := client.Role.Query().Where(role.IDEQ(developerRole.ID)).OnlyX(schematype.SkipSoftDelete(ctx))
	require.NotEqual(t, 0, preservedRole.DeletedAt)
	require.Equal(t, 1, client.UserRole.Query().Where(userrole.RoleID(developerRole.ID)).CountX(schematype.SkipSoftDelete(ctx)))
}

func TestV1_0_0_Beta7_CleansSoftDeletedDeveloperAndRecreates(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:soft-deleted-developer-cleanup?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("soft-deleted-developer-project").SaveX(ctx)

	// Create a Developer role with users.
	oldDeveloperRole, err := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"old_scope"}).
		Save(ctx)
	require.NoError(t, err)

	// Create a user and assign the role.
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserRole.Create().
		SetUserID(testUser.ID).
		SetRoleID(oldDeveloperRole.ID).
		Save(ctx)
	require.NoError(t, err)

	// Soft-delete the Developer role.
	require.NoError(t, client.Role.DeleteOneID(oldDeveloperRole.ID).Exec(ctx))

	// Verify UserRole relationship still exists (soft delete doesn't cascade).
	count, err := client.UserRole.Query().Where(userrole.RoleID(oldDeveloperRole.ID)).Count(schematype.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Create a legacy invitation.
	legacyInvitation, err := client.Invitation.Create().
		SetTokenHash("soft-deleted-cleanup-token").
		SetProjectID(project.ID).
		Save(ctx)
	require.NoError(t, err)

	// Run migration.
	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	// Verify old soft-deleted role is permanently deleted.
	exists, err := client.Role.Query().Where(role.IDEQ(oldDeveloperRole.ID)).Exist(schematype.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.False(t, exists)

	// Verify stale UserRole relationship is cleaned up.
	count, err = client.UserRole.Query().Where(userrole.RoleID(oldDeveloperRole.ID)).Count(schematype.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Verify new Developer role is created with default scopes.
	newDeveloperRole, err := client.Role.Query().Where(
		role.LevelEQ(role.LevelProject),
		role.ProjectIDEQ(project.ID),
		role.NameEQ("Developer"),
	).Only(ctx)
	require.NoError(t, err)
	require.NotEqual(t, oldDeveloperRole.ID, newDeveloperRole.ID)
	require.ElementsMatch(t, []string{
		"read_api_keys", "write_api_keys",
		"read_requests", "write_requests",
	}, newDeveloperRole.Scopes)

	// Verify the user is NOT assigned to the new role.
	count, err = client.UserRole.Query().Where(userrole.RoleID(newDeveloperRole.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Verify legacy invitation is assigned to the new role.
	legacyInvitation = client.Invitation.GetX(ctx, legacyInvitation.ID)
	require.NotNil(t, legacyInvitation.RoleID)
	require.Equal(t, newDeveloperRole.ID, *legacyInvitation.RoleID)
}

func TestV1_0_0_Beta7_SkipsExpiredAndExhaustedInvitations(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:invalid-legacy-invitations?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("invalid-legacy-invitations-project").SaveX(ctx)
	developerRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"old_scope"}).
		SaveX(ctx)
	testUser := client.User.Create().
		SetEmail("invalid-invitation-user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SaveX(ctx)
	client.UserRole.Create().SetUserID(testUser.ID).SetRoleID(developerRole.ID).SaveX(ctx)
	client.Role.DeleteOneID(developerRole.ID).ExecX(ctx)
	expired := client.Invitation.Create().
		SetTokenHash("expired-legacy-token").
		SetProjectID(project.ID).
		SetExpiresAt(time.Now().Add(-time.Hour)).
		SaveX(ctx)
	exhausted := client.Invitation.Create().
		SetTokenHash("exhausted-legacy-token").
		SetProjectID(project.ID).
		SetMaxUses(1).
		SetUsedCount(1).
		SaveX(ctx)

	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	expired = client.Invitation.GetX(ctx, expired.ID)
	exhausted = client.Invitation.GetX(ctx, exhausted.ID)
	require.Nil(t, expired.RoleID)
	require.Nil(t, exhausted.RoleID)
	preservedRole := client.Role.Query().Where(role.IDEQ(developerRole.ID)).OnlyX(schematype.SkipSoftDelete(ctx))
	require.NotEqual(t, 0, preservedRole.DeletedAt)
	require.Equal(t, 1, client.UserRole.Query().Where(userrole.RoleID(developerRole.ID)).CountX(schematype.SkipSoftDelete(ctx)))
}
