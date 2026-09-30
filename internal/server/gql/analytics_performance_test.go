package gql

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// TestAnalyticsDimensionStatsIncludesPerformanceMetrics verifies that model rows
// expose throughput and average TTFT calculated from their completed executions.
func TestAnalyticsDimensionStatsIncludesPerformanceMetrics(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:analytics-performance?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	resolver := &queryResolver{&Resolver{
		client:        client,
		systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: client}),
	}}

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenaiFake).
		SetName("Performance channel").
		SetStatus(channel.StatusEnabled).
		SetSupportedModels([]string{"performance-model"}).
		SetDefaultTestModel("performance-model").
		SetCredentials(objects.ChannelCredentials{}).
		Save(ctx)
	require.NoError(t, err)

	for _, sample := range []struct {
		completionTokens int64
		reasoningTokens  int64
		audioTokens      int64
		latencyMs        int64
		firstTokenMs     int64
	}{
		// Reasoning and audio tokens are a breakdown of completion tokens and must not be re-added.
		{completionTokens: 100, reasoningTokens: 20, audioTokens: 30, latencyMs: 2500, firstTokenMs: 500},
		{completionTokens: 100, latencyMs: 2500, firstTokenMs: 1500},
		// Cache hit: TTFT equals latency, so the generation time is clamped to biz.MinLatencyMs.
		{completionTokens: 50, latencyMs: 2500, firstTokenMs: 2500},
	} {
		req, createErr := client.Request.Create().
			SetModelID("performance-model").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStatus(request.StatusCompleted).
			SetChannelID(ch.ID).
			SetStream(true).
			Save(ctx)
		require.NoError(t, createErr)

		_, createErr = client.RequestExecution.Create().
			SetRequestID(req.ID).
			SetChannelID(ch.ID).
			SetModelID("performance-model").
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetStatus(requestexecution.StatusCompleted).
			SetStream(true).
			SetMetricsLatencyMs(sample.latencyMs).
			SetMetricsFirstTokenLatencyMs(sample.firstTokenMs).
			Save(ctx)
		require.NoError(t, createErr)

		_, createErr = client.UsageLog.Create().
			SetRequestID(req.ID).
			SetChannelID(ch.ID).
			SetModelID("performance-model").
			SetPromptTokens(10).
			SetCompletionTokens(sample.completionTokens).
			SetCompletionReasoningTokens(sample.reasoningTokens).
			SetCompletionAudioTokens(sample.audioTokens).
			SetTotalTokens(10 + sample.completionTokens).
			Save(ctx)
		require.NoError(t, createErr)
	}

	// 250 completion tokens over 2000 + 1000 + MinLatencyMs milliseconds of generation time.
	expectedTokensPerSecond := 250.0 * 1000 / float64(3000+biz.MinLatencyMs)
	expectedTTFTMs := 1500.0

	stats, err := resolver.AnalyticsDimensionStats(ctx, nil, "model")
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.NotNil(t, stats[0].TokensPerSecond)
	require.InDelta(t, expectedTokensPerSecond, *stats[0].TokensPerSecond, 0.001)
	require.NotNil(t, stats[0].TtftMs)
	require.InDelta(t, expectedTTFTMs, *stats[0].TtftMs, 0.001)

	channelStats, err := resolver.AnalyticsDimensionStats(ctx, nil, "channel")
	require.NoError(t, err)
	require.Len(t, channelStats, 1)
	require.NotNil(t, channelStats[0].TokensPerSecond)
	require.InDelta(t, expectedTokensPerSecond, *channelStats[0].TokensPerSecond, 0.001)
	require.NotNil(t, channelStats[0].TtftMs)
	require.InDelta(t, expectedTTFTMs, *channelStats[0].TtftMs, 0.001)
}
