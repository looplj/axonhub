package gql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func setupTestSystemMutationResolver(t *testing.T) (*mutationResolver, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := &biz.SystemService{
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	resolver := &mutationResolver{&Resolver{systemService: systemService}}
	return resolver, ctx, client
}

func TestMutationResolver_UpdateSystemChannelSettings_MergesAutoSyncWithoutOverwritingProbe(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	err := resolver.systemService.SetChannelSetting(ctx, biz.SystemChannelSettings{
		Probe: biz.ChannelProbeSetting{
			Enabled:   true,
			Frequency: biz.ProbeFrequency5Min,
		},
		AutoSync: biz.ChannelModelAutoSyncSetting{
			Frequency: biz.AutoSyncFrequencyOneHour,
		},
	})
	require.NoError(t, err)

	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.SystemChannelSettings{
		AutoSync: biz.ChannelModelAutoSyncSetting{
			Frequency: biz.AutoSyncFrequencySixHours,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)

	setting, err := resolver.systemService.ChannelSetting(ctx)
	require.NoError(t, err)
	require.True(t, setting.Probe.Enabled)
	require.Equal(t, biz.ProbeFrequency5Min, setting.Probe.Frequency)
	require.Equal(t, biz.AutoSyncFrequencySixHours, setting.AutoSync.Frequency)
}

func TestMutationResolver_UpdateSystemChannelSettings_MergesProbeWithoutOverwritingAutoSync(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	err := resolver.systemService.SetChannelSetting(ctx, biz.SystemChannelSettings{
		Probe: biz.ChannelProbeSetting{
			Enabled:   true,
			Frequency: biz.ProbeFrequency5Min,
		},
		AutoSync: biz.ChannelModelAutoSyncSetting{
			Frequency: biz.AutoSyncFrequencySixHours,
		},
	})
	require.NoError(t, err)

	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.SystemChannelSettings{
		Probe: biz.ChannelProbeSetting{
			Enabled:   false,
			Frequency: biz.ProbeFrequency1Hour,
		},
	})
	require.NoError(t, err)
	require.True(t, ok)

	setting, err := resolver.systemService.ChannelSetting(ctx)
	require.NoError(t, err)
	require.False(t, setting.Probe.Enabled)
	require.Equal(t, biz.ProbeFrequency1Hour, setting.Probe.Frequency)
	require.Equal(t, biz.AutoSyncFrequencySixHours, setting.AutoSync.Frequency)
}
