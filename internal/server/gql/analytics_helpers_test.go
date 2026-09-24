package gql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

func TestQueryAPIKeyStats_ExcludesZeroID(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	p, err := client.Project.Create().SetName("p").Save(ctx)
	require.NoError(t, err)
	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)
	client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(0).
		SetProjectID(p.ID).
		SetModelID("gpt-4").
		SetPromptTokens(100).
		SetTotalTokens(100).
		SaveX(ctx)

	stats, err := resolver.queryAPIKeyStats(ctx, &AnalyticsFilter{}, nil, false, time.UTC)
	require.NoError(t, err)
	require.Empty(t, stats, "zero is the legacy sentinel for a missing API key, not a key ID")
}

// An aggregate over an empty request_executions table still returns one row,
// so the counts must come back as zero rather than an error.
func TestQueryExecutionSuccessCounts_EmptyTable(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	success, failed, err := resolver.queryExecutionSuccessCounts(ctx, nil, nil, false, time.UTC)
	require.NoError(t, err)
	require.Zero(t, success)
	require.Zero(t, failed)
}

// A transport failure never writes a usage log, so scoping executions through usage_logs
// would drop exactly those failures from the denominator and overstate the success rate.
// The API-key filter has to reach them through the request they belong to.
func TestQueryExecutionSuccessCounts_CountsFailuresWithoutAUsageLog(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	p, err := client.Project.Create().SetName("p").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusFailed).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	// No usage log is written for this one: the request never produced a billable result.
	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(req.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusFailed).
		SaveX(ctx)

	// A non-nil filter is required: buildAnalyticsExecutionWhere returns early on nil, so
	// a nil filter would skip the API-key subquery this test exists to exercise.
	success, failed, err := resolver.queryExecutionSuccessCounts(ctx, &AnalyticsFilter{}, []int{1}, false, time.UTC)
	require.NoError(t, err)
	require.Zero(t, success)
	require.Equal(t, 1, failed, "a failure with no usage log still belongs to the API key's window")
}

// usage_logs.created_at is a native timestamp column, so the bucket labels come from the
// native-timestamp expression builder. Routing them through the Unix-epoch builder instead
// yields to_timestamp(timestamptz) on Postgres (no such function) and NULL on SQLite, which
// surfaces as a scan error rather than as wrong-looking data.
func TestAnalyticsDailyStats_BucketLabels(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	p, err := client.Project.Create().SetName("p").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	// 10:30 UTC on a fixed date, so both bucket layouts are unambiguous.
	at := time.Date(2026, 9, 23, 10, 30, 0, 0, time.UTC)
	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetCreatedAt(at).
		Save(ctx)
	require.NoError(t, err)

	client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(1).
		SetProjectID(p.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetPromptTokens(100).
		SetCompletionTokens(200).
		SetTotalTokens(300).
		SetTotalCost(0.5).
		SetCreatedAt(at).
		SetUpdatedAt(at).
		SaveX(ctx)

	t.Run("a long range buckets by day", func(t *testing.T) {
		stats, err := resolver.AnalyticsDailyStats(ctx, &AnalyticsFilter{
			StartTime: strptr("2026-09-01"),
			EndTime:   strptr("2026-09-23"),
		})
		require.NoError(t, err)

		byDate := make(map[string]int, len(stats))
		for _, s := range stats {
			byDate[s.Date] = s.TotalTokens
		}
		require.Contains(t, byDate, "2026-09-23")
		assert.Equal(t, 300, byDate["2026-09-23"])
	})

	t.Run("the relative window buckets by hour", func(t *testing.T) {
		// last24Hours is resolved against the query instant, so the row has to be written
		// recently enough to fall inside it rather than on the fixed date above.
		recent := time.Now().UTC().Add(-2 * time.Hour)
		client.UsageLog.Create().
			SetRequestID(req.ID).
			SetAPIKeyID(1).
			SetProjectID(p.ID).
			SetChannelID(1).
			SetModelID("gpt-4").
			SetPromptTokens(1).
			SetCompletionTokens(1).
			SetTotalTokens(2).
			SetTotalCost(0).
			SetCreatedAt(recent).
			SetUpdatedAt(recent).
			SaveX(ctx)

		stats, err := resolver.AnalyticsDailyStats(ctx, &AnalyticsFilter{TimeWindow: strptr("last24Hours")})
		require.NoError(t, err)

		wantLabel := recent.In(time.UTC).Format("2006-01-02 15:00")
		byDate := make(map[string]int, len(stats))
		for _, s := range stats {
			byDate[s.Date] = s.TotalTokens
		}
		require.Contains(t, byDate, wantLabel, "hourly buckets must carry the hour, not just the day")
	})
}

func strptr(v string) *string { return &v }

// The MAX_ID pattern ranks by "this row is the latest execution of its request", and on
// SQLite that comparison is a correlated subquery. If the subquery is not bounded to the
// same window as the outer query, a request whose newest attempt falls after the window
// leaves its in-window row failing the comparison, so the request drops out entirely.
// SQLite is the dialect that exercises MAX_ID, so this runs in CI.
func TestChannelPerformanceStats_CountsExecutionWhenANewerOneIsOutsideTheWindow(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	p, err := client.Project.Create().SetName("p").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	inside := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetCreatedAt(inside).
		Save(ctx)
	require.NoError(t, err)

	client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(1).
		SetProjectID(p.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetCompletionTokens(1000).
		SetCreatedAt(inside).
		SetUpdatedAt(inside).
		SaveX(ctx)

	// The qualifying in-window execution.
	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(req.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetMetricsLatencyMs(1000).
		SetCreatedAt(inside).
		SetUpdatedAt(inside).
		SaveX(ctx)

	// A newer attempt for the same request, two days past the window's end.
	outside := inside.AddDate(0, 0, 2)
	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(req.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetMetricsLatencyMs(1000).
		SetCreatedAt(outside).
		SetUpdatedAt(outside).
		SaveX(ctx)

	stats, err := resolver.ChannelPerformanceStats(ctx, nil, strptr("2026-09-10"), strptr("2026-09-10"))
	require.NoError(t, err)

	total := 0
	for _, s := range stats {
		total += s.RequestCount
	}
	assert.Equal(t, 1, total, "the in-window execution must count even though a newer one exists outside the window")
}
