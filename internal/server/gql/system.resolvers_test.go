package gql

import (
	"context"
	"testing"

	"github.com/samber/lo"
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

	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.UpdateSystemChannelSettings{
		AutoSync: &biz.ChannelModelAutoSyncSetting{
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

func TestMutationResolver_UpdateProviderQuotaCollectionSettings_MergesProviders(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	require.NoError(t, resolver.systemService.UpdateProviderQuotaCollectionSettings(ctx, nil, []biz.ProviderQuotaCollectionProvider{
		{Provider: "codex", Enabled: false},
	}))

	ok, err := resolver.UpdateProviderQuotaCollectionSettings(ctx, UpdateProviderQuotaCollectionSettingsInput{
		Providers: []*ProviderQuotaCollectionProviderInput{
			{Provider: "minimax", Enabled: false},
		},
	})
	require.NoError(t, err)
	require.True(t, ok)

	settings, err := resolver.systemService.ProviderQuotaCollectionSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.Providers["codex"])
	require.False(t, settings.Providers["minimax"])
	require.True(t, settings.Providers["zhipu"])
}

func TestMutationResolver_UpdateProviderQuotaCollectionSettings_RejectsInvalidProviders(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	_, err := resolver.UpdateProviderQuotaCollectionSettings(ctx, UpdateProviderQuotaCollectionSettingsInput{
		Providers: []*ProviderQuotaCollectionProviderInput{
			{Provider: "unsupported", Enabled: false},
		},
	})
	require.ErrorContains(t, err, "unsupported provider quota type")

	_, err = resolver.UpdateProviderQuotaCollectionSettings(ctx, UpdateProviderQuotaCollectionSettingsInput{
		Providers: []*ProviderQuotaCollectionProviderInput{
			{Provider: "minimax", Enabled: false},
			{Provider: "minimax", Enabled: true},
		},
	})
	require.ErrorContains(t, err, "duplicate provider quota type")
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

	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.UpdateSystemChannelSettings{
		Probe: &biz.ChannelProbeSetting{
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

func TestMutationResolver_UpdateSystemChannelSettings_MergesPrompts(t *testing.T) {
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
		TestSystemPrompt: "initial system",
		TestUserPrompt:   "initial user",
	})
	require.NoError(t, err)

	userPrompt := "updated user"
	ok, err := resolver.UpdateSystemChannelSettings(ctx, biz.UpdateSystemChannelSettings{
		TestUserPrompt: &userPrompt,
	})
	require.NoError(t, err)
	require.True(t, ok)

	setting, err := resolver.systemService.ChannelSetting(ctx)
	require.NoError(t, err)
	require.Equal(t, "initial system", setting.TestSystemPrompt)
	require.Equal(t, "updated user", setting.TestUserPrompt)
	require.True(t, setting.Probe.Enabled)
	require.Equal(t, biz.AutoSyncFrequencyOneHour, setting.AutoSync.Frequency)

	emptyPrompt := ""
	ok, err = resolver.UpdateSystemChannelSettings(ctx, biz.UpdateSystemChannelSettings{
		TestSystemPrompt: &emptyPrompt,
	})
	require.NoError(t, err)
	require.True(t, ok)

	setting, err = resolver.systemService.ChannelSetting(ctx)
	require.NoError(t, err)
	require.Equal(t, "You are a helpful assistant.", setting.TestSystemPrompt)
	require.Equal(t, "updated user", setting.TestUserPrompt)
}

func TestUpdateSystemChannelSettingsInput_PromptPresence(t *testing.T) {
	ec := &executionContext{}

	tests := []struct {
		name           string
		input          map[string]any
		expectedSystem *string
		expectedUser   *string
	}{
		{name: "omitted", input: map[string]any{}},
		{name: "null", input: map[string]any{"testSystemPrompt": nil, "testUserPrompt": nil}},
		{
			name:           "empty",
			input:          map[string]any{"testSystemPrompt": "", "testUserPrompt": " "},
			expectedSystem: lo.ToPtr(""),
			expectedUser:   lo.ToPtr(" "),
		},
		{
			name:           "value",
			input:          map[string]any{"testSystemPrompt": "custom system", "testUserPrompt": "custom user"},
			expectedSystem: lo.ToPtr("custom system"),
			expectedUser:   lo.ToPtr("custom user"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := ec.unmarshalInputUpdateSystemChannelSettingsInput(t.Context(), tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expectedSystem, input.TestSystemPrompt)
			require.Equal(t, tt.expectedUser, input.TestUserPrompt)
		})
	}
}
