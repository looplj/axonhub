package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type testBuiltin struct {
	id string
}

func (b testBuiltin) PluginID() string { return b.id }

func (b testBuiltin) Apply(context.Context) error { return nil }

func (b testBuiltin) OnDispose(context.Context) error { return nil }

func TestCountersArePerInstance(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	require.Equal(t, xcache.ModeMemory, manager.cacheMode)
	require.True(t, manager.CountersArePerInstance())

	empty, err := newManager(ManagerParams{
		Client: client,
		Cache:  xcache.Config{},
	}, managerConfig{
		refreshInterval: time.Hour,
		debounce:        time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, empty.Stop(context.Background()))
	})
	require.Equal(t, xcache.ModeMemory, empty.cacheMode)
	require.True(t, empty.CountersArePerInstance())

	manager.cacheMode = xcache.ModeTwoLevel
	require.False(t, manager.CountersArePerInstance())
}

func TestDeleteBuiltinDoesNotRemoveRow(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	ctx := pluginCtx()

	installed := installUI(t, manager, uiZip(t, nil, nil))
	manager.RegisterBuiltin(testBuiltin{id: installed.PluginID})
	err := manager.Delete(ctx, installed.ID)
	require.ErrorContains(t, err, "built in")
	require.ErrorContains(t, err, installed.PluginID)
	require.NotNil(t, manager.Current().ByID(installed.ID))

	require.NoError(t, manager.Enable(ctx, installed.ID, []string{"read_channels"}))
	require.NoError(t, manager.Disable(ctx, installed.ID))
	require.False(t, manager.Current().ByID(installed.ID).Enabled)

	reserved, err := client.Plugin.Create().
		SetPluginID("axonhub.webhook").
		SetManifest(objects.PluginManifest{
			SchemaVersion:       1,
			SchemaVersionSet:    true,
			ID:                  "axonhub.webhook",
			Name:                "Webhook",
			Version:             "1.0.0",
			AxonhubVersion:      "^1.0.0",
			PluginAPIVersion:    1,
			PluginAPIVersionSet: true,
		}).
		Save(ctx)
	require.NoError(t, err)
	err = manager.Delete(ctx, reserved.ID)
	require.ErrorContains(t, err, "built in")
	count, err := client.Plugin.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
