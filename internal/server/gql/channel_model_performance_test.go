package gql

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

// channelModelRow is a compact projection for assertions.
type channelModelRow struct {
	channelName string
	modelID     string
	request     int
	success     int
	failed      int
	successRate float64
	throughput  *float64
	ttftMs      *float64
	totalTokens int
}

func toChannelModelRows(t *testing.T, stats []*ChannelModelPerformanceStat) []channelModelRow {
	t.Helper()
	rows := make([]channelModelRow, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, channelModelRow{
			channelName: s.ChannelName,
			modelID:     s.ModelID,
			request:     s.RequestCount,
			success:     s.SuccessCount,
			failed:      s.FailedCount,
			successRate: s.SuccessRate,
			throughput:  s.Throughput,
			ttftMs:      s.TtftMs,
			totalTokens: s.TotalTokens,
		})
	}
	return rows
}

func findChannelModelRow(t *testing.T, rows []channelModelRow, channelName, modelID string) channelModelRow {
	t.Helper()
	for _, r := range rows {
		if r.channelName == channelName && r.modelID == modelID {
			return r
		}
	}
	t.Fatalf("row not found: channel=%s model=%s in %+v", channelName, modelID, rows)
	return channelModelRow{}
}

// TestChannelModelPerformanceStatsSplitsByChannelAndModel verifies the 2D
// (channel x model) breakdown: the same model on different channels and
// different models on the same channel each get their own row.
func TestChannelModelPerformanceStatsSplitsByChannelAndModel(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	chA := createDashboardUsageChannel(t, ctx, client, "channel-a")
	chB := createDashboardUsageChannel(t, ctx, client, "channel-b")

	// channel-a / gpt-4o: 1 completed
	reqA1, err := client.Request.Create().
		SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusCompleted).
		SetChannelID(chA.ID).
		SetStream(true).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RequestExecution.Create().
		SetRequestID(reqA1.ID).SetChannelID(chA.ID).SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).SetStatus(requestexecution.StatusCompleted).SetStream(true).
		SetMetricsLatencyMs(1000).SetMetricsFirstTokenLatencyMs(200).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetRequestID(reqA1.ID).SetChannelID(chA.ID).SetModelID("gpt-4o").
		SetPromptTokens(10).SetCompletionTokens(100).SetTotalTokens(110).
		Save(ctx)
	require.NoError(t, err)

	// channel-b / gpt-4o: 1 completed (same model, different channel)
	reqB1, err := client.Request.Create().
		SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusCompleted).
		SetChannelID(chB.ID).
		SetStream(true).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RequestExecution.Create().
		SetRequestID(reqB1.ID).SetChannelID(chB.ID).SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).SetStatus(requestexecution.StatusCompleted).SetStream(true).
		SetMetricsLatencyMs(2000).SetMetricsFirstTokenLatencyMs(400).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetRequestID(reqB1.ID).SetChannelID(chB.ID).SetModelID("gpt-4o").
		SetPromptTokens(20).SetCompletionTokens(200).SetTotalTokens(220).
		Save(ctx)
	require.NoError(t, err)

	// channel-a / o1: 1 completed (same channel, different model)
	reqA2, err := client.Request.Create().
		SetModelID("o1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusCompleted).
		SetChannelID(chA.ID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RequestExecution.Create().
		SetRequestID(reqA2.ID).SetChannelID(chA.ID).SetModelID("o1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).SetStatus(requestexecution.StatusCompleted).SetStream(false).
		SetMetricsLatencyMs(3000).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetRequestID(reqA2.ID).SetChannelID(chA.ID).SetModelID("o1").
		SetPromptTokens(5).SetCompletionTokens(50).SetTotalTokens(55).
		Save(ctx)
	require.NoError(t, err)

	allTime := "allTime"
	stats, err := resolver.ChannelModelPerformanceStats(ctx, &allTime, nil)
	require.NoError(t, err)
	rows := toChannelModelRows(t, stats)
	require.Len(t, rows, 3, "one row per channel-model pair")

	// channel-a/gpt-4o: streaming, generation time = 1000-200 = 800ms,
	// throughput = 100 tokens * 1000 / 800 = 125 tok/s, TTFT = 200ms.
	rowAGpt := findChannelModelRow(t, rows, "channel-a", "gpt-4o")
	require.Equal(t, 1, rowAGpt.request)
	require.Equal(t, 1, rowAGpt.success)
	require.Equal(t, 0, rowAGpt.failed)
	require.InDelta(t, 125.0, *rowAGpt.throughput, 0.01)
	require.InDelta(t, 200.0, *rowAGpt.ttftMs, 0.01)
	require.Equal(t, 110, rowAGpt.totalTokens)

	// channel-b/gpt-4o: streaming, generation time = 2000-400 = 1600ms,
	// throughput = 200 tokens * 1000 / 1600 = 125 tok/s.
	rowBGpt := findChannelModelRow(t, rows, "channel-b", "gpt-4o")
	require.Equal(t, 1, rowBGpt.request)
	require.InDelta(t, 125.0, *rowBGpt.throughput, 0.01)
	require.Equal(t, 220, rowBGpt.totalTokens)

	// channel-a/o1: non-streaming, no TTFT
	rowAO1 := findChannelModelRow(t, rows, "channel-a", "o1")
	require.Equal(t, 1, rowAO1.request)
	require.Nil(t, rowAO1.ttftMs, "non-streaming requests have no TTFT")
}

// TestChannelModelPerformanceStatsCountsFailures verifies that failed
// executions (which never write a usage_log) still contribute to request_count
// and the success-rate denominator.
func TestChannelModelPerformanceStatsCountsFailures(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	ch := createDashboardUsageChannel(t, ctx, client, "channel-a")

	// 2 completed + 1 failed, no usage_log for the failed one.
	for range 2 {
		req, err := client.Request.Create().
			SetModelID("gpt-4o").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStatus(request.StatusCompleted).
			SetChannelID(ch.ID).SetStream(true).
			Save(ctx)
		require.NoError(t, err)
		_, err = client.RequestExecution.Create().
			SetRequestID(req.ID).SetChannelID(ch.ID).SetModelID("gpt-4o").
			SetRequestBody(objects.JSONRawMessage(`{}`)).SetStatus(requestexecution.StatusCompleted).SetStream(true).
			SetMetricsLatencyMs(1000).SetMetricsFirstTokenLatencyMs(200).
			Save(ctx)
		require.NoError(t, err)
		_, err = client.UsageLog.Create().
			SetRequestID(req.ID).SetChannelID(ch.ID).SetModelID("gpt-4o").
			SetPromptTokens(10).SetCompletionTokens(100).SetTotalTokens(110).
			Save(ctx)
		require.NoError(t, err)
	}
	// failed execution, no usage_log
	reqFail, err := client.Request.Create().
		SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusFailed).
		SetChannelID(ch.ID).SetStream(true).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RequestExecution.Create().
		SetRequestID(reqFail.ID).SetChannelID(ch.ID).SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).SetStatus(requestexecution.StatusFailed).SetStream(true).
		Save(ctx)
	require.NoError(t, err)

	allTime := "allTime"
	stats, err := resolver.ChannelModelPerformanceStats(ctx, &allTime, nil)
	require.NoError(t, err)
	rows := toChannelModelRows(t, stats)
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, "channel-a", row.channelName)
	require.Equal(t, "gpt-4o", row.modelID)
	require.Equal(t, 3, row.request, "request_count includes the failed attempt")
	require.Equal(t, 2, row.success)
	require.Equal(t, 1, row.failed)
	require.InDelta(t, 2.0/3.0*100, row.successRate, 0.01, "success rate = 2/3")
}

// TestChannelModelPerformanceStatsUsageOnlyByStatus verifies that a usage_log
// written for a failed stream (no in-window execution) is counted by the
// joined execution's status, not as an automatic success.
func TestChannelModelPerformanceStatsUsageOnlyByStatus(t *testing.T) {
	resolver, ctx := newDashboardUsageTestResolver(t)
	client := resolver.client

	ch := createDashboardUsageChannel(t, ctx, client, "channel-a")

	// A usage_log whose joined execution is failed. This simulates a stream that
	// reported usage but ended in failure.
	req, err := client.Request.Create().
		SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusFailed).
		SetChannelID(ch.ID).SetStream(true).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RequestExecution.Create().
		SetRequestID(req.ID).SetChannelID(ch.ID).SetModelID("gpt-4o").
		SetRequestBody(objects.JSONRawMessage(`{}`)).SetStatus(requestexecution.StatusFailed).SetStream(true).
		SetMetricsLatencyMs(1000).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetRequestID(req.ID).SetChannelID(ch.ID).SetModelID("gpt-4o").
		SetPromptTokens(10).SetCompletionTokens(50).SetTotalTokens(60).
		Save(ctx)
	require.NoError(t, err)

	allTime := "allTime"
	stats, err := resolver.ChannelModelPerformanceStats(ctx, &allTime, nil)
	require.NoError(t, err)
	rows := toChannelModelRows(t, stats)
	require.Len(t, rows, 1)
	row := rows[0]
	// The usage_log exists but the execution is failed, so it must NOT be
	// counted as a success.
	require.Equal(t, 0, row.success, "failed stream with usage must not count as success")
	require.Equal(t, 1, row.failed)
	require.Equal(t, 60, row.totalTokens, "tokens still reported from the usage_log")
}
