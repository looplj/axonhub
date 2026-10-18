package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func newGroupedChannel(groups []objects.KeyGroup, disabled []objects.DisabledAPIKey) *Channel {
	credentials := objects.ChannelCredentials{KeyGroups: groups}

	return &Channel{
		Channel: &ent.Channel{
			Credentials:     credentials,
			DisabledAPIKeys: disabled,
		},
		cachedEnabledAPIKeys: credentials.GetEnabledAPIKeys(disabled),
	}
}

func TestTraceStickyKeyProvider_KeyGroups_SelectsByModel(t *testing.T) {
	ch := newGroupedChannel([]objects.KeyGroup{
		{Name: "codex", APIKey: "key-codex", Models: []string{"gpt-5-codex"}, Enabled: true},
		{Name: "chat", APIKey: "key-chat", Models: []string{"gpt-5"}, Enabled: true},
	}, nil)

	provider := NewTraceStickyKeyProvider(ch)

	t.Run("codex model routes to the codex group key", func(t *testing.T) {
		ctx := contexts.WithRequestModel(context.Background(), "gpt-5-codex")
		require.Equal(t, "key-codex", provider.Get(ctx))
	})

	t.Run("chat model routes to the chat group key", func(t *testing.T) {
		ctx := contexts.WithRequestModel(context.Background(), "gpt-5")
		require.Equal(t, "key-chat", provider.Get(ctx))
	})
}

func TestTraceStickyKeyProvider_KeyGroups_UnknownModelFallsBackToEnabledGroup(t *testing.T) {
	ch := newGroupedChannel([]objects.KeyGroup{
		{Name: "codex", APIKey: "key-codex", Models: []string{"gpt-5-codex"}, Enabled: true},
	}, nil)

	provider := NewTraceStickyKeyProvider(ch)
	ctx := contexts.WithRequestModel(context.Background(), "some-unknown-model")

	// The only group is scoped to a different model, so the request falls back to
	// every enabled group key rather than having no key at all.
	require.Equal(t, "key-codex", provider.Get(ctx))
}

func TestTraceStickyKeyProvider_KeyGroups_DisabledGroupExcluded(t *testing.T) {
	ch := newGroupedChannel([]objects.KeyGroup{
		{Name: "codex", APIKey: "key-codex", Models: []string{"gpt-5"}, Enabled: true},
		{Name: "old", APIKey: "key-old", Models: []string{"gpt-5"}, Enabled: false},
	}, nil)

	provider := NewTraceStickyKeyProvider(ch)
	ctx := contexts.WithRequestModel(context.Background(), "gpt-5")

	require.Equal(t, "key-codex", provider.Get(ctx))
}

func TestTraceStickyKeyProvider_KeyGroups_NoModelInContextFallsBackToEnabledGroup(t *testing.T) {
	ch := newGroupedChannel([]objects.KeyGroup{
		{Name: "chat", APIKey: "key-chat", Models: []string{"gpt-5"}, Enabled: true},
	}, nil)

	provider := NewTraceStickyKeyProvider(ch)
	// No request model recorded: routing must still hand out a key.
	require.Equal(t, "key-chat", provider.Get(context.Background()))
}

func TestTraceStickyKeyProvider_KeyGroups_AllGroupsDisabledFallsBack(t *testing.T) {
	ch := newGroupedChannel([]objects.KeyGroup{
		{Name: "old", APIKey: "key-old", Models: nil, Enabled: false},
	}, nil)

	provider := NewTraceStickyKeyProvider(ch)
	// No enabled key: the provider still yields a configured key so the request
	// is not dispatched with an empty credential.
	require.Equal(t, "key-old", provider.Get(context.Background()))
}

func TestTraceStickyKeyProvider_KeyGroups_DisableRecordExcludesKey(t *testing.T) {
	ch := newGroupedChannel(
		[]objects.KeyGroup{
			{Name: "codex", APIKey: "key-codex", Models: []string{"gpt-5"}, Enabled: true},
			{Name: "chat", APIKey: "key-chat", Models: []string{"gpt-5"}, Enabled: true},
		},
		[]objects.DisabledAPIKey{{Key: "key-codex"}},
	)

	provider := NewTraceStickyKeyProvider(ch)
	ctx := contexts.WithRequestModel(context.Background(), "gpt-5")

	for range 20 {
		require.Equal(t, "key-chat", provider.Get(ctx))
	}
}

func TestTraceStickyKeyProvider_NoKeyGroups_UsesFlatKeys(t *testing.T) {
	keys := []string{"key-1", "key-2"}
	ch := &Channel{
		Channel: &ent.Channel{
			Credentials: objects.ChannelCredentials{APIKeys: keys},
		},
		cachedEnabledAPIKeys: keys,
	}

	provider := NewTraceStickyKeyProvider(ch)
	ctx := contexts.WithRequestModel(context.Background(), "gpt-5")

	// Channels without groups ignore the model filter entirely.
	require.Contains(t, keys, provider.Get(ctx))
}
