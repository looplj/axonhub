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
)

func TestV1_0_0_Beta7_BackfillsLegacyInvitationRoles(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:legacy-invitation-role?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("legacy-invitation-project").SaveX(ctx)
	developerRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"read_api_keys", "write_api_keys", "read_prompts", "write_prompts", "write_requests"}).
		SaveX(ctx)
	legacyInvitation := client.Invitation.Create().SetTokenHash("legacy-token").SetProjectID(project.ID).SaveX(ctx)

	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))
	require.NoError(t, datamigrate.NewV1_0_0_Beta7().Migrate(ctx, client))

	legacyInvitation = client.Invitation.GetX(ctx, legacyInvitation.ID)
	require.NotNil(t, legacyInvitation.RoleID)
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
