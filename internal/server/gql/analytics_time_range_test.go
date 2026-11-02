package gql

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// TestAnalyticsFilterUsesTimestampPrecision verifies that the analytics filter
// honours second (and millisecond) precision instead of rounding the range to
// whole calendar days.
func TestAnalyticsFilterUsesTimestampPrecision(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:analytics-time-range?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	resolver := &queryResolver{&Resolver{
		client:        client,
		systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: client}),
	}}

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("Time range channel").
		SetStatus(channel.StatusEnabled).
		SetSupportedModels([]string{"time-range-model"}).
		SetDefaultTestModel("time-range-model").
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	// Three requests on the same calendar day, one hour apart.
	day := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)

	for _, hour := range []int{10, 12, 14} {
		_, createErr := client.UsageLog.Create().
			SetRequestID(0).
			SetChannelID(ch.ID).
			SetModelID("time-range-model").
			SetPromptTokens(10).
			SetCompletionTokens(10).
			SetTotalTokens(20).
			SetTotalCost(1).
			SetCreatedAt(day.Add(time.Duration(hour) * time.Hour)).
			Save(ctx)
		require.NoError(t, createErr)
	}

	// The metadata exposes the earliest usage instant, not a calendar date, so
	// clients in another timezone cannot shift the window.
	metadata, err := resolver.AnalyticsMetadata(ctx)
	require.NoError(t, err)
	require.NotNil(t, metadata.EarliestTime)
	require.True(t, metadata.EarliestTime.Equal(day.Add(10*time.Hour)), "earliest time = %s", metadata.EarliestTime)

	requestCount := func(filter *AnalyticsFilter) int {
		overview, err := resolver.AnalyticsOverview(ctx, filter)
		require.NoError(t, err)

		return overview.TotalRequests
	}

	// A range that only contains the 12:00 request; a day-granularity filter
	// would have returned all three.
	require.Equal(t, 1, requestCount(&AnalyticsFilter{
		StartTime: lo.ToPtr(time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)),
		EndTime:   lo.ToPtr(time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC)),
	}))

	// Both bounds are inclusive.
	require.Equal(t, 2, requestCount(&AnalyticsFilter{
		StartTime: lo.ToPtr(time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)),
		EndTime:   lo.ToPtr(time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)),
	}))

	// Millisecond precision is honoured at the lower bound: the 12:00:00.000
	// request is excluded by a start of 12:00:00.500.
	require.Equal(t, 1, requestCount(&AnalyticsFilter{
		StartTime: lo.ToPtr(time.Date(2026, 3, 10, 12, 0, 0, 500*int(time.Millisecond), time.UTC)),
	}))

	// Dimension statistics use the same window.
	channelStats, err := resolver.AnalyticsDimensionStats(ctx, &AnalyticsFilter{
		StartTime: lo.ToPtr(time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC)),
		EndTime:   lo.ToPtr(time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC)),
	}, "channel")
	require.NoError(t, err)
	require.Len(t, channelStats, 1)
	require.Equal(t, 1, channelStats[0].RequestCount)

	// The daily trend keeps one bucket per local calendar day, covering the
	// partial days at both ends of the range.
	dailyStats, err := resolver.AnalyticsDailyStats(ctx, &AnalyticsFilter{
		StartTime: lo.ToPtr(time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)),
		EndTime:   lo.ToPtr(time.Date(2026, 3, 10, 13, 0, 0, 0, time.UTC)),
	})
	require.NoError(t, err)
	require.Len(t, dailyStats, 1)
	require.Equal(t, "2026-03-10", dailyStats[0].Date)
	require.Equal(t, 1, dailyStats[0].RequestCount)
}
