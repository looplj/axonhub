package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestCodexCompatibilitySettings_DefaultDisabled(t *testing.T) {
	// Given an old database without a Codex setting.
	svc, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	// When the setting is read.
	settings, err := svc.CodexCompatibilitySettings(ctx)
	// Then no migration or selected channel is required.
	require.NoError(t, err)
	require.Equal(t, &CodexCompatibilitySettings{}, settings)
}

func TestCodexCompatibilitySettings_AllChannelStates(t *testing.T) {
	for _, status := range []channel.Status{channel.StatusEnabled, channel.StatusDisabled, channel.StatusArchived} {
		t.Run(string(status), func(t *testing.T) {
			svc, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
			defer db.Close()
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
			source := db.Channel.Create().SetDefaultTestModel("test").SetName("catalog").SetType(channel.TypeCodex).SetStatus(status).SetBaseURL("https://example.invalid").SetSupportedModels([]string{"gpt-test"}).SetCredentials(objects.ChannelCredentials{APIKey: "secret"}).SaveX(ctx)
			// When an offline source is saved, then no preflight or status change occurs.
			require.NoError(t, svc.SetCodexCompatibilitySettings(ctx, CodexCompatibilitySettings{Enabled: true, ChannelID: &source.ID}))
			got, err := svc.CodexCompatibilitySettings(ctx)
			require.NoError(t, err)
			require.Equal(t, source.ID, *got.ChannelID)
			require.Equal(t, status, db.Channel.GetX(ctx, source.ID).Status)
		})
	}
}

func TestCodexCompatibilitySettings_InvalidReference(t *testing.T) {
	svc, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	missing := 999
	require.Error(t, svc.SetCodexCompatibilitySettings(ctx, CodexCompatibilitySettings{Enabled: true}))
	require.Error(t, svc.SetCodexCompatibilitySettings(ctx, CodexCompatibilitySettings{ChannelID: &missing}))
	source := db.Channel.Create().SetDefaultTestModel("test").SetName("other").SetType(channel.TypeOpenai).SetBaseURL("https://example.invalid").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "secret"}).SaveX(ctx)
	require.Error(t, svc.SetCodexCompatibilitySettings(ctx, CodexCompatibilitySettings{ChannelID: &source.ID}))
}

func TestCodexCompatibilitySettings_CacheInvalidation(t *testing.T) {
	svc, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	source := db.Channel.Create().SetDefaultTestModel("test").SetName("catalog").SetType(channel.TypeCodex).SetBaseURL("https://example.invalid").SetSupportedModels([]string{"test"}).SetCredentials(objects.ChannelCredentials{APIKey: "secret"}).SaveX(ctx)
	require.NoError(t, svc.SetCodexCompatibilitySettings(ctx, CodexCompatibilitySettings{Enabled: true, ChannelID: &source.ID}))
	_, err := svc.CodexCompatibilitySettings(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.SetCodexCompatibilitySettings(ctx, CodexCompatibilitySettings{}))
	got, err := svc.CodexCompatibilitySettings(ctx)
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.Nil(t, got.ChannelID)
}
