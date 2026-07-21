package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/userrole"
)

func TestV1_0_0_Beta6_NormalizesLegacySystemRoles(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:legacy-system-role?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	legacyRole := client.Role.Create().SetName("admin").SetScopes([]string{}).SaveX(ctx)
	legacyRole = client.Role.UpdateOne(legacyRole).ClearProjectID().SaveX(ctx)
	require.Nil(t, legacyRole.ProjectID)

	err := datamigrate.NewV1_0_0_Beta6().Migrate(ctx, client)
	require.NoError(t, err)
	err = datamigrate.NewV1_0_0_Beta6().Migrate(ctx, client)
	require.NoError(t, err)

	legacyRole = client.Role.GetX(ctx, legacyRole.ID)
	require.NotNil(t, legacyRole.ProjectID)
	require.Zero(t, *legacyRole.ProjectID)

	_, err = client.Role.Create().SetName("admin").SetScopes([]string{}).Save(ctx)
	require.Error(t, err)
	require.True(t, ent.IsConstraintError(err))
}

func TestV1_0_0_Beta6_MergesDuplicateSystemRoles(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:duplicate-system-role?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	canonical := client.Role.Create().SetName("admin").SetScopes([]string{"read"}).SaveX(ctx)
	canonical = client.Role.UpdateOne(canonical).ClearProjectID().SaveX(ctx)
	duplicate := client.Role.Create().SetName("admin").SetScopes([]string{"read"}).SaveX(ctx)
	duplicate = client.Role.UpdateOne(duplicate).ClearProjectID().SaveX(ctx)
	firstUser := client.User.Create().SetEmail("first@example.com").SetPassword("password").SaveX(ctx)
	secondUser := client.User.Create().SetEmail("second@example.com").SetPassword("password").SaveX(ctx)
	client.UserRole.Create().SetUserID(firstUser.ID).SetRoleID(canonical.ID).ExecX(ctx)
	client.UserRole.Create().SetUserID(secondUser.ID).SetRoleID(duplicate.ID).ExecX(ctx)

	require.NoError(t, datamigrate.NewV1_0_0_Beta6().Migrate(ctx, client))

	roles := client.Role.Query().Where(role.LevelEQ(role.LevelSystem), role.NameEQ("admin")).AllX(ctx)
	require.Len(t, roles, 1)
	require.Equal(t, canonical.ID, roles[0].ID)
	require.NotNil(t, roles[0].ProjectID)
	require.Zero(t, *roles[0].ProjectID)
	require.Equal(t, 2, client.UserRole.Query().Where(userrole.RoleID(canonical.ID)).CountX(ctx))
	_, err := client.Role.Get(ctx, duplicate.ID)
	require.True(t, ent.IsNotFound(err))
}

func TestV1_0_0_Beta6_RejectsDuplicateRolesWithDifferentScopes(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:conflicting-system-role?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	first := client.Role.Create().SetName("admin").SetScopes([]string{"read"}).SaveX(ctx)
	client.Role.UpdateOne(first).ClearProjectID().ExecX(ctx)
	second := client.Role.Create().SetName("admin").SetScopes([]string{"write"}).SaveX(ctx)
	client.Role.UpdateOne(second).ClearProjectID().ExecX(ctx)

	err := datamigrate.NewV1_0_0_Beta6().Migrate(ctx, client)
	require.ErrorContains(t, err, "different scopes")

	roles := client.Role.Query().Where(role.LevelEQ(role.LevelSystem), role.NameEQ("admin")).AllX(ctx)
	require.Len(t, roles, 2)
	for _, systemRole := range roles {
		require.Nil(t, systemRole.ProjectID)
	}
}
