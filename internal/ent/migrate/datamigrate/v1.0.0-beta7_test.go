package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/invitation"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
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
	require.ElementsMatch(t, []string{"read_api_keys", "write_api_keys", "read_prompts", "write_prompts", "write_requests"}, developerRole.Scopes)
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

func TestV1_0_0_Beta7_RestoresSoftDeletedDeveloperRole(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:soft-deleted-developer-role?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("soft-deleted-developer-project").SaveX(ctx)

	// Create and soft-delete a Developer role.
	softDeletedRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"old_scope"}).
		SaveX(ctx)
	require.NoError(t, client.Role.DeleteOneID(softDeletedRole.ID).Exec(ctx))

	// Create a legacy invitation.
	legacyInvitation := client.Invitation.Create().
		SetTokenHash("soft-deleted-role-token").
		SetProjectID(project.ID).
		SaveX(ctx)

	// Run migration - should restore the soft-deleted role, not create a new one.
	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	// Verify the role was restored.
	restoredRole := client.Role.Query().Where(
		role.LevelEQ(role.LevelProject),
		role.ProjectIDEQ(project.ID),
		role.NameEQ("Developer"),
	).OnlyX(schematype.SkipSoftDelete(ctx))
	require.Equal(t, softDeletedRole.ID, restoredRole.ID)
	require.Equal(t, 0, restoredRole.DeletedAt)
	require.Equal(t, []string{
		"read_api_keys", "write_api_keys",
		"read_prompts", "write_prompts", "write_requests",
	}, restoredRole.Scopes)

	// Verify the invitation was assigned to the restored role.
	legacyInvitation = client.Invitation.GetX(ctx, legacyInvitation.ID)
	require.NotNil(t, legacyInvitation.RoleID)
	require.Equal(t, softDeletedRole.ID, *legacyInvitation.RoleID)
}
