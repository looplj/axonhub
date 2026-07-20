package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
)

func TestV1_0_0_Beta6_NormalizesLegacySystemRoles(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:legacy-system-role?mode=memory&_fk=1")
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
