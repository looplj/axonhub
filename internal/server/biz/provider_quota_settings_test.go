package biz

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestProviderQuotaChannelTypes_include_xAI_subscription(t *testing.T) {
	require.True(t, slices.Contains(providerQuotaChannelTypes, channel.TypeXaiSubscription))
	require.True(t, slices.Contains(providerQuotaChannelTypes, channel.TypeAntigravity))
	require.True(t, slices.Contains(providerQuotaChannelTypes, channel.TypeZai))
	require.True(t, slices.Contains(providerQuotaChannelTypes, channel.TypeZaiAnthropic))
	require.True(t, slices.Contains(supportedProviderQuotaTypes, "zai"))
}

func setupProviderQuotaSettingsTest(t *testing.T) (*SystemService, *ent.Client) {
	t.Helper()

	service, client := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	return service, client
}

func TestSystemService_ProviderQuotaCollectionSettings_DefaultsToEnabled(t *testing.T) {
	service, client := setupProviderQuotaSettingsTest(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	settings, err := service.ProviderQuotaCollectionSettings(ctx)

	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.True(t, settings.Providers["codex"])
	require.True(t, settings.Providers["antigravity"])
	require.True(t, settings.Providers["xai_subscription"])
	require.True(t, settings.Providers["minimax"])
	require.True(t, settings.Providers["zhipu"])
}

func TestSystemService_ProviderQuotaCollectionSettings_CachesDefaults(t *testing.T) {
	service, client := setupProviderQuotaSettingsTest(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	_, err := service.ProviderQuotaCollectionSettings(ctx)
	require.NoError(t, err)

	cached, err := service.Cache.Get(ctx, "system:"+SystemKeyProviderQuotaCollectionSettings)
	require.NoError(t, err)

	var settings ProviderQuotaCollectionSettings
	require.NoError(t, json.Unmarshal([]byte(cached.Value), &settings))
	require.Equal(t, defaultProviderQuotaCollectionSettings(), &settings)
}

func TestSystemService_ProviderQuotaCollectionSettings_DefaultsMissingProvidersToEnabled(t *testing.T) {
	service, client := setupProviderQuotaSettingsTest(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	stored, err := json.Marshal(ProviderQuotaCollectionSettings{
		Enabled: true,
		Providers: map[string]bool{
			"codex":   true,
			"minimax": false,
		},
	})
	require.NoError(t, err)
	require.NoError(t, service.setSystemValue(ctx, SystemKeyProviderQuotaCollectionSettings, string(stored)))

	settings, err := service.ProviderQuotaCollectionSettings(ctx)

	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.False(t, settings.Providers["minimax"])
	require.True(t, settings.Providers["zhipu"])
}

func TestSystemService_ProviderQuotaCollectionSettings_UpdateMergesProviders(t *testing.T) {
	service, client := setupProviderQuotaSettingsTest(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	require.NoError(t, service.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: false},
	}))

	settings, err := service.ProviderQuotaCollectionSettings(ctx)

	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.False(t, settings.Providers["minimax"])
	require.True(t, settings.Providers["codex"])
	require.True(t, settings.Providers["zhipu"])
}

func TestSystemService_IsProviderQuotaCollectionEnabled(t *testing.T) {
	service, client := setupProviderQuotaSettingsTest(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	// A supported provider type keeps normal settings behavior: enabled by
	// default, and IsProviderQuotaCollectionEnabled must not treat it as
	// retired data.
	enabled, err := service.IsProviderQuotaCollectionEnabled(ctx, "minimax")
	require.NoError(t, err)
	require.True(t, enabled)

	// A retired provider type from a surviving row must be distinguishable
	// from a supported one by the exported helper, so the read path can
	// surface "no status" instead of an error.
	require.False(t, IsSupportedProviderQuotaType("wafer"))
	require.False(t, IsSupportedProviderQuotaType("typo-wafer"))
	require.True(t, IsSupportedProviderQuotaType("minimax"))

	// An unsupported (e.g. typo) provider type must not be silently
	// accepted as enabled.
	enabled, err = service.IsProviderQuotaCollectionEnabled(ctx, "wafer")
	require.False(t, enabled)
	require.ErrorContains(t, err, "unsupported provider quota type")
}

func TestSystemService_ProviderQuotaCollectionSettings_RejectsInvalidProviderUpdates(t *testing.T) {
	service, client := setupProviderQuotaSettingsTest(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	err := service.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "unsupported", Enabled: false},
	})
	require.ErrorContains(t, err, "unsupported provider quota type")

	err = service.UpdateProviderQuotaCollectionSettings(ctx, nil, []ProviderQuotaCollectionProvider{
		{Provider: "minimax", Enabled: false},
		{Provider: "minimax", Enabled: true},
	})
	require.ErrorContains(t, err, "duplicate provider quota type")
}
