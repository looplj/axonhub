package objects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnabledGroupKeysForModel(t *testing.T) {
	credentials := &ChannelCredentials{
		KeyGroups: []KeyGroup{
			{Name: "codex", APIKey: "key-codex", Models: []string{"gpt-5-codex"}, Enabled: true},
			{Name: "chat", APIKey: "key-chat", Models: []string{"gpt-5", "gpt-5-mini"}, Enabled: true},
			{Name: "disabled", APIKey: "key-disabled", Models: []string{"gpt-5-codex"}, Enabled: false},
			{Name: "unscoped", APIKey: "key-unscoped", Models: nil, Enabled: true},
		},
	}

	t.Run("matches group with the requested model and unscoped groups", func(t *testing.T) {
		keys, ok := credentials.EnabledGroupKeysForModel("gpt-5")
		require.True(t, ok)
		// chat matches directly; unscoped matches everything. The disabled group
		// never contributes.
		require.ElementsMatch(t, []string{"key-chat", "key-unscoped"}, keys)
	})

	t.Run("disabled group is never selected", func(t *testing.T) {
		keys, ok := credentials.EnabledGroupKeysForModel("gpt-5-codex")
		require.True(t, ok)
		require.ElementsMatch(t, []string{"key-codex", "key-unscoped"}, keys)
		require.NotContains(t, keys, "key-disabled")
	})

	t.Run("unknown model falls back", func(t *testing.T) {
		// Only the unscoped group applies, but the model is not in any scoped
		// list. The unscoped group still matches, so a key is returned.
		keys, ok := credentials.EnabledGroupKeysForModel("totally-unknown")
		require.True(t, ok)
		require.Equal(t, []string{"key-unscoped"}, keys)
	})

	t.Run("no matching group reports not-ok", func(t *testing.T) {
		scopedOnly := &ChannelCredentials{
			KeyGroups: []KeyGroup{
				{Name: "codex", APIKey: "key-codex", Models: []string{"gpt-5-codex"}, Enabled: true},
			},
		}

		keys, ok := scopedOnly.EnabledGroupKeysForModel("gpt-5")
		require.False(t, ok)
		require.Empty(t, keys)
	})

	t.Run("empty model reports not-ok", func(t *testing.T) {
		keys, ok := credentials.EnabledGroupKeysForModel("")
		require.False(t, ok)
		require.Empty(t, keys)
	})

	t.Run("no groups reports not-ok", func(t *testing.T) {
		keys, ok := (&ChannelCredentials{}).EnabledGroupKeysForModel("gpt-5")
		require.False(t, ok)
		require.Empty(t, keys)
	})
}

func TestGetAllAPIKeysWithKeyGroups(t *testing.T) {
	t.Run("grouped channel ignores legacy flat fields", func(t *testing.T) {
		credentials := &ChannelCredentials{
			APIKey:  "legacy-single",
			APIKeys: []string{"legacy-flat"},
			KeyGroups: []KeyGroup{
				{Name: "a", APIKey: "key-a", Enabled: true},
				{Name: "b", APIKey: "key-b", Enabled: false},
			},
		}

		// Disabled groups still surface their key here: this list feeds the
		// disable/recovery bookkeeping, which must know the key exists.
		require.Equal(t, []string{"key-a", "key-b"}, credentials.GetAllAPIKeys())
	})

	t.Run("legacy channel unchanged", func(t *testing.T) {
		credentials := &ChannelCredentials{
			APIKey:  "legacy-single",
			APIKeys: []string{"flat-1", "flat-2"},
		}

		require.Equal(t, []string{"legacy-single", "flat-1", "flat-2"}, credentials.GetAllAPIKeys())
	})
}

func TestGetEnabledAPIKeysWithKeyGroups(t *testing.T) {
	credentials := &ChannelCredentials{
		APIKeys: []string{"legacy-flat"},
		KeyGroups: []KeyGroup{
			{Name: "a", APIKey: "key-a", Enabled: true},
			{Name: "b", APIKey: "key-b", Enabled: false},
		},
	}

	t.Run("disabled group excluded from enabled set", func(t *testing.T) {
		require.Equal(t, []string{"key-a"}, credentials.GetEnabledAPIKeys(nil))
	})

	t.Run("disable record excludes the group key too", func(t *testing.T) {
		disabled := []DisabledAPIKey{{Key: "key-a"}}
		require.Empty(t, credentials.GetEnabledAPIKeys(disabled))
	})

	t.Run("legacy channel uses flat list", func(t *testing.T) {
		legacy := &ChannelCredentials{APIKeys: []string{"flat-1", "flat-2"}}
		require.Equal(t, []string{"flat-1", "flat-2"}, legacy.GetEnabledAPIKeys(nil))
	})
}

func TestEnabledGroupKeys(t *testing.T) {
	credentials := &ChannelCredentials{
		KeyGroups: []KeyGroup{
			{Name: "a", APIKey: "key-a", Enabled: true},
			{Name: "b", APIKey: "key-b", Enabled: false},
			{Name: "blank", APIKey: "", Enabled: true},
			{Name: "c", APIKey: "key-c", Enabled: true},
		},
	}

	require.Equal(t, []string{"key-a", "key-c"}, credentials.EnabledGroupKeys())
}
