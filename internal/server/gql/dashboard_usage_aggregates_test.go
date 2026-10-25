package gql

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func newDashboardUsageTestResolver(t *testing.T) (*queryResolver, context.Context) {
	t.Helper()

	resetDashboardQueryCache()
	client := enttest.NewEntClient(t, "sqlite3", "file:dashboard-usage-aggregates?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	resolver := &queryResolver{&Resolver{
		client:        client,
		systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: client}),
	}}

	return resolver, ctx
}

func createDashboardUsageChannel(t *testing.T, ctx context.Context, client *ent.Client, name string) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName(name).
		SetStatus(channel.StatusEnabled).
		SetSupportedModels([]string{"model-a", "model-b"}).
		SetDefaultTestModel("model-a").
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func createDashboardUsageLog(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	channelID int,
	modelID string,
	promptTokens int64,
	completionTokens int64,
	cost *float64,
	apiKeyID *int,
) {
	t.Helper()

	create := client.UsageLog.Create().
		SetRequestID(1).
		SetChannelID(channelID).
		SetModelID(modelID).
		SetPromptTokens(promptTokens).
		SetCompletionTokens(completionTokens).
		SetTotalTokens(promptTokens + completionTokens)
	if cost != nil {
		create = create.SetTotalCost(*cost)
	}
	if apiKeyID != nil {
		create = create.SetAPIKeyID(*apiKeyID)
	}

	_, err := create.Save(ctx)
	require.NoError(t, err)
}

// TestDashboardUsageAggregatesShareOneGroupedScan verifies that the request,
// cost and token widgets for the same dimension all derive their data from the
// same grouped rows and preserve their own ranking/limit behavior.
func TestDashboardUsageAggregatesShareOneGroupedScan(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	channelA := createDashboardUsageChannel(t, ctx, client, "channel-a")
	channelB := createDashboardUsageChannel(t, ctx, client, "channel-b")

	costA := 0.5
	costB := 2.0
	createDashboardUsageLog(t, ctx, client, channelA.ID, "model-a", 100, 10, &costA, nil)
	createDashboardUsageLog(t, ctx, client, channelA.ID, "model-a", 200, 20, &costA, nil)
	createDashboardUsageLog(t, ctx, client, channelB.ID, "model-b", 300, 40, &costB, nil)

	requestStats, err := resolver.RequestStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, requestStats, 2)
	require.Equal(t, "channel-a", requestStats[0].ChannelName)
	require.Equal(t, 2, requestStats[0].Count)
	require.Equal(t, "channel-b", requestStats[1].ChannelName)
	require.Equal(t, 1, requestStats[1].Count)

	costStats, err := resolver.CostStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, costStats, 2)
	// channel-a has the most requests but channel-b has the highest total cost.
	require.Equal(t, "channel-b", costStats[0].ChannelName)
	require.InDelta(t, 2.0, costStats[0].Cost, 1e-9)
	require.Equal(t, "channel-a", costStats[1].ChannelName)
	require.InDelta(t, 1.0, costStats[1].Cost, 1e-9)

	tokenStats, err := resolver.TokenStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tokenStats, 2)
	require.Equal(t, "channel-b", tokenStats[0].ChannelName)
	require.Equal(t, 300, tokenStats[0].InputTokens)
	require.Equal(t, 40, tokenStats[0].OutputTokens)
	require.Equal(t, 340, tokenStats[0].TotalTokens)
	require.Equal(t, "channel-a", tokenStats[1].ChannelName)
	require.Equal(t, 300, tokenStats[1].InputTokens)
	require.Equal(t, 30, tokenStats[1].OutputTokens)
	require.Equal(t, 330, tokenStats[1].TotalTokens)

	modelStats, err := resolver.RequestStatsByModel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, modelStats, 2)
	require.Equal(t, "model-a", modelStats[0].ModelID)
	require.Equal(t, 2, modelStats[0].Count)
	require.Equal(t, "model-b", modelStats[1].ModelID)
	require.Equal(t, 1, modelStats[1].Count)
}

// countUsageLogGroupedScans counts the ent queries issued through the
// resolver's client against usage_logs. After the counter is installed, the
// dashboard aggregate GroupBy scans are the only code path that SELECTs from
// usage_logs in this test (fixture setup uses mutations, which do not go
// through ent query interceptors), so each intercepted query is one grouped
// scan of usage_logs.
func countUsageLogGroupedScans(client *ent.Client) *int {
	var scans int
	var mu sync.Mutex

	client.UsageLog.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, q ent.Query) (ent.Value, error) {
			mu.Lock()
			scans++
			mu.Unlock()
			return next.Query(ctx, q)
		})
	}))

	return &scans
}

// TestDashboardUsageWidgetsShareOneAggregateScan verifies that the request,
// cost and token widgets for one dimension are mapped from a single grouped
// scan of usage_logs. The existing TestDashboardUsageAggregatesShareOneGroupedScan
// covers the mapped results; this test counts the underlying scans through
// ent's query interceptor, which is the existing extension point the cache's
// single shared scan must go through.
func TestDashboardUsageWidgetsShareOneAggregateScan(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client
	scans := countUsageLogGroupedScans(client)

	channelA := createDashboardUsageChannel(t, ctx, client, "channel-a")
	channelB := createDashboardUsageChannel(t, ctx, client, "channel-b")

	costA := 0.5
	costB := 2.0
	createDashboardUsageLog(t, ctx, client, channelA.ID, "model-a", 100, 10, &costA, nil)
	createDashboardUsageLog(t, ctx, client, channelA.ID, "model-a", 200, 20, &costA, nil)
	createDashboardUsageLog(t, ctx, client, channelB.ID, "model-b", 300, 40, &costB, nil)

	requestStats, err := resolver.RequestStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 1, *scans, "the first widget run must be exactly one grouped scan")

	costStats, err := resolver.CostStatsByChannel(ctx, nil)
	require.NoError(t, err)
	tokenStats, err := resolver.TokenStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 1, *scans, "the three widgets for one dimension must share the one grouped scan")

	// The shared scan's rows are mapped independently per widget, preserving
	// each widget's ranking: channel-a leads in requests (2), channel-b
	// leads in both cost (2.0) and tokens (340).
	require.Equal(t, "channel-a", requestStats[0].ChannelName)
	require.Equal(t, 2, requestStats[0].Count)
	require.Equal(t, "channel-b", costStats[0].ChannelName)
	require.InDelta(t, 2.0, costStats[0].Cost, 1e-9)
	require.Equal(t, "channel-b", tokenStats[0].ChannelName)
	require.Equal(t, 340, tokenStats[0].TotalTokens)

	// The model dimension runs one group scan of its own.
	modelStats, err := resolver.RequestStatsByModel(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 2, *scans, "the model dimension must get its own single grouped scan")
	require.Equal(t, "model-a", modelStats[0].ModelID)
	require.Equal(t, 2, modelStats[0].Count)
}

func TestDashboardUsageAggregatesLimitEachDimension(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	const channels = dashboardUsageAggregateLimit + 1
	for i := range channels {
		ch := createDashboardUsageChannel(t, ctx, client, fmt.Sprintf("channel-%02d", i))
		// Later channels have more requests so the top ten is deterministic.
		for range i + 1 {
			createDashboardUsageLog(t, ctx, client, ch.ID, "model-a", 1, 1, nil, nil)
		}
	}

	stats, err := resolver.RequestStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, stats, dashboardUsageAggregateLimit)

	// The highest-volume channel (created last) is first, and the two lowest
	// volume channels are excluded.
	require.Equal(t, "channel-10", stats[0].ChannelName)
	require.Equal(t, 11, stats[0].Count)
	require.Equal(t, "channel-01", stats[len(stats)-1].ChannelName)
	require.Equal(t, 2, stats[len(stats)-1].Count)
}

func TestDashboardUsageAggregatesByAPIKeyJoinsNames(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	ch := createDashboardUsageChannel(t, ctx, client, "channel-a")
	apiKey, err := client.APIKey.Create().
		SetKey("test-key-1").
		SetName("Key One").
		Save(ctx)
	require.NoError(t, err)

	createDashboardUsageLog(t, ctx, client, ch.ID, "model-a", 100, 20, nil, &apiKey.ID)

	stats, err := resolver.TokenStatsByAPIKey(ctx, nil)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "Key One", stats[0].APIKeyName)
	require.Equal(t, 120, stats[0].TotalTokens)

	// Soft-deleted keys are excluded from the widget, matching the previous
	// behavior of resolving names through the ent API key query.
	_, err = client.APIKey.UpdateOneID(apiKey.ID).SetDeletedAt(1).Save(ctx)
	require.NoError(t, err)
	resetDashboardQueryCache()

	stats, err = resolver.TokenStatsByAPIKey(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, stats)
}

func TestCanonicalDashboardTimeWindow(t *testing.T) {
	tests := []struct {
		input *string
		want  string
	}{
		{input: nil, want: "allTime"},
		{input: new(""), want: "allTime"},
		{input: new("allTime"), want: "allTime"},
		{input: new("unknown"), want: "allTime"},
		{input: new(" day "), want: "day"},
		{input: new("week"), want: "week"},
		{input: new("month"), want: "month"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, canonicalDashboardTimeWindow(tt.input))
	}
}

func TestTopDashboardUsageAggregatesRankingAndTieBreak(t *testing.T) {
	rows := []dashboardUsageAggregate{
		{ID: 2, Name: "channel-b", RequestCount: 5, InputTokens: 100, OutputTokens: 0, TotalCost: 1},
		{ID: 1, Name: "channel-a", RequestCount: 5, InputTokens: 10, OutputTokens: 20, TotalCost: 5},
		{ID: 3, Name: "channel-c", RequestCount: 10, InputTokens: 150, OutputTokens: 0, TotalCost: 2},
	}

	byRequests := topDashboardUsageAggregates(rows, func(item dashboardUsageAggregate) float64 {
		return float64(item.RequestCount)
	})
	require.Equal(t, []string{"channel-c", "channel-a", "channel-b"}, []string{
		byRequests[0].Name, byRequests[1].Name, byRequests[2].Name,
	})

	byCost := topDashboardUsageAggregates(rows, func(item dashboardUsageAggregate) float64 {
		return item.TotalCost
	})
	require.Equal(t, "channel-a", byCost[0].Name)

	byTokens := topDashboardUsageAggregates(rows, func(item dashboardUsageAggregate) float64 {
		return float64(item.totalTokens())
	})
	require.Equal(t, "channel-c", byTokens[0].Name)
	require.Equal(t, "channel-b", byTokens[1].Name)

	// The returned slice is a clone; callers may rank it independently.
	require.Equal(t, "channel-c", rows[2].Name)
}

func TestDashboardUsageAggregatesNullableCostAndDeletedChannel(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	channelA := createDashboardUsageChannel(t, ctx, client, "channel-a")
	channelB := createDashboardUsageChannel(t, ctx, client, "channel-b")

	costB := 5.0
	createDashboardUsageLog(t, ctx, client, channelA.ID, "model-a", 10, 10, nil, nil)
	createDashboardUsageLog(t, ctx, client, channelB.ID, "model-b", 20, 20, &costB, nil)

	costStats, err := resolver.CostStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, costStats, 2)
	require.Equal(t, "channel-b", costStats[0].ChannelName)
	require.InDelta(t, 5.0, costStats[0].Cost, 1e-9)
	require.Equal(t, "channel-a", costStats[1].ChannelName)
	require.InDelta(t, 0.0, costStats[1].Cost, 1e-9)

	_, err = client.Channel.UpdateOneID(channelB.ID).SetDeletedAt(1).Save(ctx)
	require.NoError(t, err)
	resetDashboardQueryCache()

	requestStats, err := resolver.RequestStatsByChannel(ctx, nil)
	require.NoError(t, err)
	require.Len(t, requestStats, 1)
	require.Equal(t, "channel-a", requestStats[0].ChannelName)
}
