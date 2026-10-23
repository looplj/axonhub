package biz

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestUpdateChannelSettingsInput_Apply(t *testing.T) {
	base := &objects.ChannelSettings{
		ExtraModelPrefix:        "deepseek",
		AutoTrimedModelPrefixes: []string{"openai"},
		HideOriginalModels:      true,
		LowercaseModelID:        true,
		Proxy:                   &httpclient.ProxyConfig{Type: "http", URL: "http://proxy.example.com:8080"},
		RateLimit:               &objects.ChannelRateLimit{RPM: lo.ToPtr(int64(60))},
		RetryableStatusCodes:    []int{429},
	}

	t.Run("nil patch keeps base", func(t *testing.T) {
		var patch *UpdateChannelSettingsInput
		merged := patch.Apply(base)
		require.Equal(t, *base, merged)
	})

	t.Run("nil base starts from zero settings", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{ExtraModelPrefix: lo.ToPtr("anthropic")}
		merged := patch.Apply(nil)
		require.Equal(t, "anthropic", merged.ExtraModelPrefix)
		require.Nil(t, merged.Proxy)
		require.Nil(t, merged.RateLimit)
	})

	t.Run("absent fields keep stored values", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{
			ExtraModelPrefix: lo.ToPtr("anthropic"),
		}
		merged := patch.Apply(base)
		require.Equal(t, "anthropic", merged.ExtraModelPrefix)
		require.True(t, merged.HideOriginalModels)
		require.True(t, merged.LowercaseModelID)
		require.Equal(t, []string{"openai"}, merged.AutoTrimedModelPrefixes)
		require.Equal(t, base.Proxy, merged.Proxy)
		require.Equal(t, base.RateLimit, merged.RateLimit)
		require.Equal(t, []int{429}, merged.RetryableStatusCodes)
	})

	t.Run("absent api key policy keeps stored values", func(t *testing.T) {
		stored := &objects.ChannelSettings{
			APIKeyStrategy:              lo.ToPtr("round_robin"),
			APIKeyRoundRobinSwitchAfter: lo.ToPtr(3),
		}
		patch := &UpdateChannelSettingsInput{ExtraModelPrefix: lo.ToPtr("anthropic")}
		merged := patch.Apply(stored)
		require.Equal(t, "round_robin", *merged.APIKeyStrategy)
		require.Equal(t, 3, *merged.APIKeyRoundRobinSwitchAfter)
	})

	t.Run("present api key policy replaces stored values", func(t *testing.T) {
		stored := &objects.ChannelSettings{
			APIKeyStrategy:              lo.ToPtr("round_robin"),
			APIKeyRoundRobinSwitchAfter: lo.ToPtr(3),
		}
		patch := &UpdateChannelSettingsInput{
			APIKeyStrategy:              lo.ToPtr("priority"),
			APIKeyRoundRobinSwitchAfter: lo.ToPtr(1),
		}
		merged := patch.Apply(stored)
		require.Equal(t, "priority", *merged.APIKeyStrategy)
		require.Equal(t, 1, *merged.APIKeyRoundRobinSwitchAfter)
	})

	t.Run("present fields replace stored values", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{
			ExtraModelPrefix:   lo.ToPtr(""),
			HideOriginalModels: lo.ToPtr(false),
			Proxy:              &httpclient.ProxyConfig{Type: "socks5", URL: "socks5://other.example.com:1080"},
			RateLimit:          &objects.ChannelRateLimit{TPM: lo.ToPtr(int64(1000))},
		}
		merged := patch.Apply(base)
		require.Equal(t, "", merged.ExtraModelPrefix)
		require.False(t, merged.HideOriginalModels)
		require.Equal(t, patch.Proxy, merged.Proxy)
		require.Equal(t, patch.RateLimit, merged.RateLimit)
		// Untouched fields survive.
		require.True(t, merged.LowercaseModelID)
	})

	t.Run("explicitly empty list clears, absent list keeps", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{
			AutoTrimedModelPrefixes: []string{},
			RetryableStatusCodes:    nil,
		}
		merged := patch.Apply(base)
		require.Empty(t, merged.AutoTrimedModelPrefixes)
		require.NotNil(t, merged.AutoTrimedModelPrefixes)
		require.Equal(t, []int{429}, merged.RetryableStatusCodes)
	})
}

func TestChannelService_UpdateChannelSettings_PreservesUntouchedFields(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)

	created, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:            channel.TypeOpenai,
		Name:            "Settings Patch",
		BaseURL:         lo.ToPtr("https://api.openai.com/v1"),
		Credentials:     objects.ChannelCredentials{APIKey: "key"},
		SupportedModels: []string{"gpt-test"},
		Settings: &objects.ChannelSettings{
			ExtraModelPrefix: "deepseek",
			RateLimit:        &objects.ChannelRateLimit{RPM: lo.ToPtr(int64(60))},
		},
	})
	require.NoError(t, err)

	// Patch a single field; every other stored setting must survive.
	updated, err := svc.UpdateChannelSettings(ctx, created.ID, &UpdateChannelSettingsInput{
		ExtraModelPrefix: lo.ToPtr("anthropic"),
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Settings)
	require.Equal(t, "anthropic", updated.Settings.ExtraModelPrefix)
	require.Equal(t, lo.ToPtr(int64(60)), updated.Settings.RateLimit.RPM)

	// The channel itself is untouched by a settings-only patch.
	require.Equal(t, created.Name, updated.Name)
	require.Equal(t, created.Type, updated.Type)

	// Patching again keeps the previously patched value.
	updated, err = svc.UpdateChannelSettings(ctx, created.ID, &UpdateChannelSettingsInput{
		HideOriginalModels: lo.ToPtr(true),
	})
	require.NoError(t, err)
	require.Equal(t, "anthropic", updated.Settings.ExtraModelPrefix)
	require.True(t, updated.Settings.HideOriginalModels)
	require.Equal(t, lo.ToPtr(int64(60)), updated.Settings.RateLimit.RPM)

	// A missing channel surfaces as an error.
	_, err = svc.UpdateChannelSettings(ctx, created.ID+1000, &UpdateChannelSettingsInput{})
	require.Error(t, err)
}

func TestChannelService_UpdateChannelSettings_ConcurrentPatchesBothSurvive(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)

	created, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:            channel.TypeOpenai,
		Name:            "Settings Patch Race",
		BaseURL:         lo.ToPtr("https://api.openai.com/v1"),
		Credentials:     objects.ChannelCredentials{APIKey: "key"},
		SupportedModels: []string{"gpt-test"},
	})
	require.NoError(t, err)

	// Concurrent patches to different fields must all survive: the merge is
	// optimistic-locked against the version it read, and a conflicting commit
	// triggers a re-read and re-merge instead of overwriting the other patch.
	const patches = 4

	var wg sync.WaitGroup
	errs := make([]error, patches)
	for i := range patches {
		patch := &UpdateChannelSettingsInput{}
		switch i % 4 {
		case 0:
			patch.ExtraModelPrefix = lo.ToPtr(fmt.Sprintf("prefix-%d", i))
		case 1:
			patch.HideOriginalModels = lo.ToPtr(true)
		case 2:
			patch.LowercaseModelID = lo.ToPtr(true)
		case 3:
			patch.HideMappedModels = lo.ToPtr(true)
		}
		wg.Go(func() {
			_, errs[i] = svc.UpdateChannelSettings(ctx, created.ID, patch)
		})
	}
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "patch %d", i)
	}

	final, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, final.Settings)
	require.Regexp(t, `^prefix-\d+$`, final.Settings.ExtraModelPrefix)
	require.True(t, final.Settings.HideOriginalModels)
	require.True(t, final.Settings.LowercaseModelID)
	require.True(t, final.Settings.HideMappedModels)
}

func TestChannelService_UpdateChannelSettingsVersionGuard(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)

	created, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:            channel.TypeOpenai,
		Name:            "Settings Version Guard",
		BaseURL:         lo.ToPtr("https://api.openai.com/v1"),
		Credentials:     objects.ChannelCredentials{APIKey: "key"},
		SupportedModels: []string{"gpt-test"},
	})
	require.NoError(t, err)

	// An update guarded by a stale version fails instead of committing.
	stale := created.UpdatedAt.Add(-time.Nanosecond)
	_, err = svc.UpdateChannel(withChannelExpectedVersion(ctx, stale), created.ID, &ent.UpdateChannelInput{Name: lo.ToPtr("Renamed")})
	require.ErrorIs(t, err, ErrChannelUpdatedConcurrently)
}
