package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdk "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/metrics"
)

func TestRequestMetricsUseCachedChannel(t *testing.T) {
	ctx := context.Background()
	enabledProvider, err := metrics.NewProvider(metrics.Config{Enabled: true, Exporter: metrics.ExporterConfig{Type: "stdout"}})
	require.NoError(t, err)
	oldMetrics := metrics.Metrics
	reader := sdk.NewManualReader()
	provider := sdk.NewMeterProvider(sdk.WithReader(reader))
	require.NoError(t, metrics.SetupMetrics(provider, "request-cache-test"))
	t.Cleanup(func() {
		metrics.Metrics = oldMetrics
		require.NoError(t, provider.Shutdown(ctx))
		require.NoError(t, enabledProvider.Shutdown(ctx))
		disabled, err := metrics.NewProvider(metrics.Config{})
		require.NoError(t, err)
		require.NoError(t, disabled.Shutdown(ctx))
	})
	client := enttest.NewEntClient(t, "sqlite3", "file:request-metric-cache?mode=memory&_fk=0")
	channels := NewChannelServiceForTest(client)
	t.Cleanup(channels.Stop)
	channels.SetEnabledChannelsForTest([]*Channel{{Channel: &ent.Channel{ID: 7, Name: "cached-channel"}}})
	svc := &RequestService{UsageLogService: &UsageLogService{ChannelService: channels}, metricClaims: newRequestMetricClaims()}
	// These entities deliberately have no database client: labels must be resolved
	// entirely from the cache, including a cache miss for a disabled/deleted channel.
	svc.recordChatRequestMetrics(ctx, &ent.Request{ID: 1, ChannelID: 7, Status: request.StatusProcessing}, request.StatusCompleted, nil)
	svc.recordChatRequestMetrics(ctx, &ent.Request{ID: 2, ChannelID: 8, Status: request.StatusProcessing}, request.StatusFailed, nil)
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	labels := map[string]string{}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "chat_request_count" {
				continue
			}
			for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
				id, _ := point.Attributes.Value("channel_id")
				name, _ := point.Attributes.Value("channel_name")
				labels[id.AsString()] = name.AsString()
			}
		}
	}
	require.Equal(t, map[string]string{"7": "cached-channel", "8": ""}, labels)
}
