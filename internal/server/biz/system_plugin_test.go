package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// Settings getters are host reads: who triggers a read or a cache miss must not change the result.
func TestSystemSettingsReadsAreHostReads(t *testing.T) {
	service, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	t.Cleanup(func() { db.Close() })
	setup := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	require.NoError(t, service.setSystemValue(setup, SystemKeyBrandName, "host-brand"))
	require.NoError(t, service.setSystemValue(setup, SystemKeyGeneralSettings, `{"timezone":"Asia/Shanghai"}`))

	user := &ent.User{ID: 1, IsOwner: true}
	parent := contexts.WithUser(authz.NewUserContext(ent.NewContext(context.Background(), db), user.ID), user)
	plugin, err := authz.DelegatePlugin(parent, 10, nil)
	require.NoError(t, err)

	// A cold cache filled by a plugin without any grant.
	settings, err := service.GeneralSettings(plugin)
	require.NoError(t, err)
	require.Equal(t, "Asia/Shanghai", settings.Timezone)
	brand, err := service.BrandName(plugin)
	require.NoError(t, err)
	require.Equal(t, "host-brand", brand)
	_, err = service.ModelSettings(plugin)
	require.NoError(t, err)
	_, err = service.OnboardingInfo(plugin)
	require.NoError(t, err)
	system, userPrompt, err := service.ChannelTestPrompts(plugin)
	require.NoError(t, err)
	require.NotEmpty(t, system)
	require.NotEmpty(t, userPrompt)

	// Reads without any principal, as background jobs do.
	brand, err = service.BrandName(ent.NewContext(context.Background(), db))
	require.NoError(t, err)
	require.Equal(t, "host-brand", brand)

	// Inside a transaction the host read still sees the transaction's own writes.
	require.NoError(t, service.RunInTransaction(setup, func(txCtx context.Context) error {
		require.NoError(t, service.setSystemValue(txCtx, SystemKeyBrandName, "tx-brand"))
		brand, err := service.BrandName(txCtx)
		require.NoError(t, err)
		require.Equal(t, "tx-brand", brand)
		return nil
	}))
}
