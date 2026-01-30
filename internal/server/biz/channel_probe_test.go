package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelProbeService_ComputeChannelProbeStats_UsageLogCreatedAfterWindow(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := ent.NewContext(t.Context(), client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("c1").
		SetStatus(channel.StatusEnabled).
		SetSupportedModels([]string{"gpt-4o-mini"}).
		SetDefaultTestModel("gpt-4o-mini").
		Save(ctx)
	require.NoError(t, err)

	endTime := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	startTime := endTime.Add(-time.Minute)

	// Create request (needed for usage_log relation)
	req1, err := client.Request.Create().
		SetModelID("gpt-4o-mini").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusCompleted).
		SetChannelID(ch.ID).
		SetStream(true).
		SetMetricsLatencyMs(2000).
		SetMetricsFirstTokenLatencyMs(500).
		SetCreatedAt(startTime.Add(10 * time.Second)).
		SetUpdatedAt(endTime.Add(10 * time.Second)).
		Save(ctx)
	require.NoError(t, err)

	// Create request_execution (used by probe stats)
	_, err = client.RequestExecution.Create().
		SetRequestID(req1.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4o-mini").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetStream(true).
		SetMetricsLatencyMs(2000).
		SetMetricsFirstTokenLatencyMs(500).
		SetCreatedAt(startTime.Add(10 * time.Second)).
		SetUpdatedAt(endTime.Add(10 * time.Second)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(req1.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4o-mini").
		SetTotalTokens(300).
		SetCreatedAt(startTime.Add(15 * time.Second)).
		SetUpdatedAt(startTime.Add(15 * time.Second)).
		Save(ctx)
	require.NoError(t, err)

	req2, err := client.Request.Create().
		SetModelID("gpt-4o-mini").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusCompleted).
		SetChannelID(ch.ID).
		SetStream(false).
		SetMetricsLatencyMs(1000).
		SetCreatedAt(startTime.Add(20 * time.Second)).
		SetUpdatedAt(endTime.Add(20 * time.Second)).
		Save(ctx)
	require.NoError(t, err)

	// Create request_execution for req2
	_, err = client.RequestExecution.Create().
		SetRequestID(req2.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4o-mini").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetStream(false).
		SetMetricsLatencyMs(1000).
		SetCreatedAt(startTime.Add(20 * time.Second)).
		SetUpdatedAt(endTime.Add(20 * time.Second)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(req2.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4o-mini").
		SetTotalTokens(100).
		SetCreatedAt(startTime.Add(25 * time.Second)).
		SetUpdatedAt(startTime.Add(25 * time.Second)).
		Save(ctx)
	require.NoError(t, err)

	svc := &ChannelProbeService{
		AbstractService: &AbstractService{db: client},
	}

	allStats, err := svc.computeAllChannelProbeStats(ctx, []int{ch.ID}, startTime, endTime)
	require.NoError(t, err)

	stats, ok := allStats[ch.ID]
	require.True(t, ok)
	require.Equal(t, 2, stats.total)
	require.Equal(t, 2, stats.success)
	require.NotNil(t, stats.avgTokensPerSecond)
	require.InDelta(t, 133.333333, *stats.avgTokensPerSecond, 0.0001)
	require.NotNil(t, stats.avgTimeToFirstTokenMs)
	require.InDelta(t, 500.0, *stats.avgTimeToFirstTokenMs, 0.0001)
}
