package biz

import (
	"testing"

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
		RateLimit:               &objects.ChannelRateLimit{RPM: int64Ptr(60)},
		RetryableStatusCodes:    []int{429},
	}

	t.Run("nil patch keeps base", func(t *testing.T) {
		var patch *UpdateChannelSettingsInput
		merged := patch.Apply(base)
		require.Equal(t, *base, merged)
	})

	t.Run("nil base starts from zero settings", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{ExtraModelPrefix: strPtr("anthropic")}
		merged := patch.Apply(nil)
		require.Equal(t, "anthropic", merged.ExtraModelPrefix)
		require.Nil(t, merged.Proxy)
		require.Nil(t, merged.RateLimit)
	})

	t.Run("absent fields keep stored values", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{
			ExtraModelPrefix: strPtr("anthropic"),
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

	t.Run("present fields replace stored values", func(t *testing.T) {
		patch := &UpdateChannelSettingsInput{
			ExtraModelPrefix:   strPtr(""),
			HideOriginalModels: boolPtr(false),
			Proxy:              &httpclient.ProxyConfig{Type: "socks5", URL: "socks5://other.example.com:1080"},
			RateLimit:          &objects.ChannelRateLimit{TPM: int64Ptr(1000)},
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
		BaseURL:         strPtr("https://api.openai.com/v1"),
		Credentials:     objects.ChannelCredentials{APIKey: "key"},
		SupportedModels: []string{"gpt-test"},
		Settings: &objects.ChannelSettings{
			ExtraModelPrefix: "deepseek",
			RateLimit:        &objects.ChannelRateLimit{RPM: int64Ptr(60)},
		},
	})
	require.NoError(t, err)

	// Patch a single field; every other stored setting must survive.
	updated, err := svc.UpdateChannelSettings(ctx, created.ID, &UpdateChannelSettingsInput{
		ExtraModelPrefix: strPtr("anthropic"),
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Settings)
	require.Equal(t, "anthropic", updated.Settings.ExtraModelPrefix)
	require.Equal(t, int64Ptr(60), updated.Settings.RateLimit.RPM)

	// The channel itself is untouched by a settings-only patch.
	require.Equal(t, created.Name, updated.Name)
	require.Equal(t, created.Type, updated.Type)

	// Patching again keeps the previously patched value.
	updated, err = svc.UpdateChannelSettings(ctx, created.ID, &UpdateChannelSettingsInput{
		HideOriginalModels: boolPtr(true),
	})
	require.NoError(t, err)
	require.Equal(t, "anthropic", updated.Settings.ExtraModelPrefix)
	require.True(t, updated.Settings.HideOriginalModels)
	require.Equal(t, int64Ptr(60), updated.Settings.RateLimit.RPM)

	// A missing channel surfaces as an error.
	_, err = svc.UpdateChannelSettings(ctx, created.ID+1000, &UpdateChannelSettingsInput{})
	require.Error(t, err)
}

func int64Ptr(v int64) *int64 { return &v }
func strPtr(v string) *string { return &v }
func boolPtr(v bool) *bool    { return &v }
