package gql

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestSortChannelModelEntries_SortsByRequestModelAscending(t *testing.T) {
	entries := []biz.ChannelModelEntry{
		{RequestModel: "gpt-4o", ActualModel: "gpt-4o", Source: "direct"},
		{RequestModel: "claude-sonnet-4", ActualModel: "claude-sonnet-4", Source: "direct"},
		{RequestModel: "deepseek-chat", ActualModel: "deepseek-v3", Source: "mapping"},
		{RequestModel: "kimi-k3", ActualModel: "moonshot-v1", Source: "auto_trim"},
	}

	sorted := sortChannelModelEntries(entries)

	got := make([]string, 0, len(sorted))
	for _, entry := range sorted {
		got = append(got, entry.RequestModel)
	}

	require.Equal(t, []string{"claude-sonnet-4", "deepseek-chat", "gpt-4o", "kimi-k3"}, got)
}

func TestSortChannelModelEntries_EmptyInput(t *testing.T) {
	require.Empty(t, sortChannelModelEntries(nil))
	require.Empty(t, sortChannelModelEntries([]biz.ChannelModelEntry{}))
}

func TestSortChannelModelEntries_DuplicateRequestModelStable(t *testing.T) {
	entries := []biz.ChannelModelEntry{
		{RequestModel: "gpt-4o", ActualModel: "gpt-4o", Source: "direct"},
		{RequestModel: "gpt-4o", ActualModel: "ft:gpt-4o", Source: "mapping"},
		{RequestModel: "claude-sonnet-4", ActualModel: "claude-sonnet-4", Source: "direct"},
	}

	first := sortChannelModelEntries(entries)

	// Same input always produces the same output.
	for range 20 {
		require.Equal(t, first, sortChannelModelEntries(entries))
	}

	// Entries sharing a RequestModel keep their relative input order.
	require.Equal(t, []biz.ChannelModelEntry{
		{RequestModel: "claude-sonnet-4", ActualModel: "claude-sonnet-4", Source: "direct"},
		{RequestModel: "gpt-4o", ActualModel: "gpt-4o", Source: "direct"},
		{RequestModel: "gpt-4o", ActualModel: "ft:gpt-4o", Source: "mapping"},
	}, first)
}

func TestChannelResolver_AllModelEntries_ReturnsSortedEntries(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))

	channelEntity, err := client.Channel.Create().
		SetName("Unordered models").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4o", "claude-sonnet-4", "deepseek-chat", "kimi-k3"}).
		SetDefaultTestModel("gpt-4o").
		Save(ctx)
	require.NoError(t, err)

	resolver := &channelResolver{&Resolver{}}
	entries, err := resolver.AllModelEntries(ctx, channelEntity)
	require.NoError(t, err)

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.RequestModel)
	}

	require.Equal(t, []string{"claude-sonnet-4", "deepseek-chat", "gpt-4o", "kimi-k3"}, got)
}
