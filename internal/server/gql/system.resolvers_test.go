package gql

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
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

func TestQuotaEnforcementSettingsResolver_AllowedChannelIDs(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	qrs := resolver.QuotaEnforcementSettings().(*quotaEnforcementSettingsResolver)

	t.Run("non-empty slice maps to []*objects.GUID correctly", func(t *testing.T) {
		settings := &biz.QuotaEnforcementSettings{
			AllowedChannelIDs: []int{42, 7, 99},
		}
		guids, err := qrs.AllowedChannelIDs(ctx, settings)
		require.NoError(t, err)
		require.Len(t, guids, 3)
		require.Equal(t, "Channel", guids[0].Type)
		require.Equal(t, 42, guids[0].ID)
		require.Equal(t, "Channel", guids[1].Type)
		require.Equal(t, 7, guids[1].ID)
		require.Equal(t, "Channel", guids[2].Type)
		require.Equal(t, 99, guids[2].ID)
	})

	t.Run("nil slice returns empty slice", func(t *testing.T) {
		settings := &biz.QuotaEnforcementSettings{
			AllowedChannelIDs: nil,
		}
		guids, err := qrs.AllowedChannelIDs(ctx, settings)
		require.NoError(t, err)
		require.NotNil(t, guids)
		require.Empty(t, guids)
	})

	t.Run("empty slice returns empty slice", func(t *testing.T) {
		settings := &biz.QuotaEnforcementSettings{
			AllowedChannelIDs: []int{},
		}
		guids, err := qrs.AllowedChannelIDs(ctx, settings)
		require.NoError(t, err)
		require.NotNil(t, guids)
		require.Empty(t, guids)
	})

	t.Run("round-trips with mutation resolver IntGuids conversion", func(t *testing.T) {
		originalIDs := []int{10, 20, 30}
		settings := &biz.QuotaEnforcementSettings{
			AllowedChannelIDs: originalIDs,
		}
		// Query resolver: int -> []*GUID
		guids, err := qrs.AllowedChannelIDs(ctx, settings)
		require.NoError(t, err)
		// Mutation resolver: []*GUID -> int
		back := objects.IntGuids(guids)
		require.Equal(t, originalIDs, back)
	})
}

func TestMutationResolver_UpdateQuotaEnforcementSettings(t *testing.T) {
	resolver, ctx, client := setupTestSystemMutationResolver(t)
	defer client.Close()

	t.Run("AllowedChannelIDs round-trips through mutation", func(t *testing.T) {
		channelIDs := []*objects.GUID{
			{Type: "Channel", ID: 5},
			{Type: "Channel", ID: 13},
		}
		ok, err := resolver.UpdateQuotaEnforcementSettings(ctx, UpdateQuotaEnforcementSettingsInput{
			Enabled:           lo.ToPtr(true),
			Mode:              lo.ToPtr(biz.QuotaEnforcementModeDePrioritize),
			AllowedChannelIDs: channelIDs,
		})
		require.NoError(t, err)
		require.True(t, ok)

		// Verify via service
		settings, err := resolver.systemService.QuotaEnforcementSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, []int{5, 13}, settings.AllowedChannelIDs)
		require.True(t, settings.Enabled)
		require.Equal(t, biz.QuotaEnforcementModeDePrioritize, settings.Mode)
	})

	t.Run("partial update preserves other fields", func(t *testing.T) {
		// Seed existing settings
		require.NoError(t, resolver.systemService.SetQuotaEnforcementSettings(ctx, biz.QuotaEnforcementSettings{
			Enabled:           true,
			Mode:              biz.QuotaEnforcementModeDePrioritize,
			AllowedChannelIDs: []int{1, 2, 3},
		}))

		// Only change AllowedChannelIDs
		newIDs := []*objects.GUID{{Type: "Channel", ID: 99}}
		ok, err := resolver.UpdateQuotaEnforcementSettings(ctx, UpdateQuotaEnforcementSettingsInput{
			AllowedChannelIDs: newIDs,
		})
		require.NoError(t, err)
		require.True(t, ok)

		settings, err := resolver.systemService.QuotaEnforcementSettings(ctx)
		require.NoError(t, err)
		// AllowedChannelIDs updated
		require.Equal(t, []int{99}, settings.AllowedChannelIDs)
		// Other fields preserved
		require.True(t, settings.Enabled)
		require.Equal(t, biz.QuotaEnforcementModeDePrioritize, settings.Mode)
	})

	t.Run("nil AllowedChannelIDs input preserves existing", func(t *testing.T) {
		// Seed existing settings
		require.NoError(t, resolver.systemService.SetQuotaEnforcementSettings(ctx, biz.QuotaEnforcementSettings{
			Enabled:           true,
			Mode:              biz.QuotaEnforcementModeExhaustedOnly,
			AllowedChannelIDs: []int{7, 8},
		}))

		// Send an update that does NOT include AllowedChannelIDs (nil)
		ok, err := resolver.UpdateQuotaEnforcementSettings(ctx, UpdateQuotaEnforcementSettingsInput{})
		require.NoError(t, err)
		require.True(t, ok)

		settings, err := resolver.systemService.QuotaEnforcementSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, []int{7, 8}, settings.AllowedChannelIDs)
	})
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
