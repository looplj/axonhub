package gql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

// The trailing-24h helpers call TimeLocation, which setupTestQueryResolver leaves nil,
// so they need a resolver wired with a system service.
func setupRecentPerformanceResolver(t *testing.T) (*queryResolver, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})

	resolver := &queryResolver{&Resolver{client: client, systemService: systemService}}

	return resolver, ctx, client
}

func ptrTimeWindow(value string) *string { return &value }

// An empty request_executions table still aggregates to one row, so both counts must
// come back zero and the rate must stay zero rather than dividing by an empty window.
func TestLast24HoursExecutionStats_EmptyTable(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	stats := resolver.last24HoursExecutionStats(ctx)

	assert.Equal(t, 0, stats.Succeeded)
	assert.Equal(t, 0, stats.Failed)
	assert.Equal(t, 0.0, stats.SuccessRate)
}

// The window is a rolling 24 hours, not a calendar day: an execution from yesterday
// evening belongs to it and one from 25 hours ago does not.
func TestLast24HoursExecutionStats_Boundary(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	now := time.Now().UTC()

	for _, tc := range []struct {
		name    string
		offset  time.Duration
		status  requestexecution.Status
		counted bool
	}{
		{name: "inside the window", offset: -24*time.Hour + time.Minute, status: requestexecution.StatusCompleted, counted: true},
		{name: "outside the window", offset: -24*time.Hour - time.Minute, status: requestexecution.StatusCompleted, counted: false},
		{name: "failed inside the window", offset: -2 * time.Hour, status: requestexecution.StatusFailed, counted: true},
		{name: "canceled is not terminal for this rate", offset: -2 * time.Hour, status: requestexecution.StatusCanceled, counted: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client.RequestExecution.Create().
				SetProjectID(1).
				SetRequestID(1000).
				SetChannelID(1).
				SetModelID("gpt-4").
				SetRequestBody(objects.JSONRawMessage(`{}`)).
				SetStatus(tc.status).
				SetCreatedAt(now.Add(tc.offset)).
				SetUpdatedAt(now).
				SaveX(ctx)
		})
	}

	stats := resolver.last24HoursExecutionStats(ctx)

	assert.Equal(t, 1, stats.Succeeded)
	assert.Equal(t, 1, stats.Failed)
	assert.InDelta(t, 50.0, stats.SuccessRate, 0.001)
}

// Every field is null when the window holds no completed generation: an empty install
// and an idle one look the same, and neither is a measured zero.
func TestLast24HoursPerformance_EmptyTable(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	stats := resolver.last24HoursPerformance(ctx)

	assert.Nil(t, stats.Throughput)
	assert.Nil(t, stats.FirstTokenP50Ms)
	assert.Nil(t, stats.FirstTokenP90Ms)
}

// One completed streaming execution inside the window, one 25 hours old. The stale one
// must not reach either the throughput ratio or the latency sample.
func TestLast24HoursPerformance_SingleStreamingExecution(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	p, err := client.Project.Create().SetName("p").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetCreatedAt(now.Add(-time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(1).
		SetProjectID(p.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetCompletionTokens(2000).
		SetCreatedAt(now.Add(-time.Hour)).
		SaveX(ctx)

	// 2000 tokens over 1s of generation after a 400ms first-token wait.
	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(req.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetStream(true).
		SetMetricsLatencyMs(1400).
		SetMetricsFirstTokenLatencyMs(400).
		SetCreatedAt(now.Add(-time.Hour)).
		SetUpdatedAt(now).
		SaveX(ctx)

	stale, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetCreatedAt(now.Add(-30 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	client.UsageLog.Create().
		SetRequestID(stale.ID).
		SetAPIKeyID(1).
		SetProjectID(p.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetCompletionTokens(99999).
		SetCreatedAt(now.Add(-30 * time.Hour)).
		SaveX(ctx)

	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(stale.ID).
		SetChannelID(1).
		SetModelID("gpt-4").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetStream(true).
		SetMetricsLatencyMs(10).
		SetMetricsFirstTokenLatencyMs(1).
		SetCreatedAt(now.Add(-30 * time.Hour)).
		SetUpdatedAt(now).
		SaveX(ctx)

	stats := resolver.last24HoursPerformance(ctx)

	require.NotNil(t, stats.Throughput)
	assert.InDelta(t, 2000.0, *stats.Throughput, 0.001)

	require.NotNil(t, stats.FirstTokenP50Ms)
	assert.Equal(t, 400, *stats.FirstTokenP50Ms)

	require.NotNil(t, stats.FirstTokenP90Ms)
	assert.Equal(t, 400, *stats.FirstTokenP90Ms)
}

// Nearest rank is the value at index ceil(n*percent/100)-1, so a two-sample window reports
// the lower of the two at p50 and the higher one at p90.
func TestNearestRankOffset(t *testing.T) {
	tests := []struct {
		name      string
		n         int
		percent   int
		wantIndex int
	}{
		{name: "single sample is both percentiles", n: 1, percent: 50, wantIndex: 0},
		{name: "single sample at p90", n: 1, percent: 90, wantIndex: 0},
		{name: "two samples split exactly at p50", n: 2, percent: 50, wantIndex: 0},
		{name: "two samples at p90", n: 2, percent: 90, wantIndex: 1},
		{name: "a hundred samples at p50", n: 100, percent: 50, wantIndex: 49},
		{name: "a hundred samples at p90", n: 100, percent: 90, wantIndex: 89},
		{name: "empty window has no row", n: 0, percent: 50, wantIndex: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantIndex, nearestRankOffset(tt.n, tt.percent))
		})
	}
}

// "last24Hours" must reach the applyFilter branch. The switch's default silently resets
// applyFilter to false, which would turn the window into an unfiltered all-time query.
func TestParseTimeWindow_Last24Hours(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	before := time.Now().UTC().Add(-24 * time.Hour)
	since, applyFilter := resolver.parseTimeWindow(ctx, ptrTimeWindow(relativeWindowLast24Hours))

	assert.True(t, applyFilter)
	assert.False(t, since.IsZero())
	assert.False(t, since.Before(before.Add(-time.Minute)))
	assert.False(t, since.After(time.Now().UTC()))

	for _, value := range []string{"day", "week", "month"} {
		t.Run(value+" still filters", func(t *testing.T) {
			_, filtered := resolver.parseTimeWindow(ctx, ptrTimeWindow(value))
			assert.True(t, filtered)
		})
	}

	t.Run("allTime still skips filtering", func(t *testing.T) {
		_, filtered := resolver.parseTimeWindow(ctx, ptrTimeWindow("allTime"))
		assert.False(t, filtered)
	})
}

// A relative window ends at the current instant, so it is bucketed by hour and the
// upper bound is passed through rather than extended by a whole day.
func TestResolvePerformanceWindow_Last24Hours(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	window := resolver.resolvePerformanceWindow(ctx, ptrTimeWindow(relativeWindowLast24Hours), nil, nil)

	assert.Equal(t, 24*time.Hour, window.endLocal.Sub(window.startLocal).Round(time.Hour))
	assert.False(t, window.startLocal.After(window.endLocal))

	t.Run("an explicit range still overrides nothing and stays daily", func(t *testing.T) {
		dated := resolver.resolvePerformanceWindow(ctx, nil, ptrTimeWindow("2026-09-01"), ptrTimeWindow("2026-09-02"))
		assert.Equal(t, 48*time.Hour, dated.endLocal.Sub(dated.startLocal))
	})
}
