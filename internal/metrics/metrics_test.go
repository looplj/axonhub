package metrics

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdk "go.opentelemetry.io/otel/sdk/metric"
)

func TestRecordChatRequestAndTokens(t *testing.T) {
	previousMetrics := Metrics
	reader := sdk.NewManualReader()
	provider := sdk.NewMeterProvider(sdk.WithReader(reader))

	t.Cleanup(func() {
		Metrics = previousMetrics
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	require.NoError(t, SetupMetrics(provider, "test"))

	ctx := context.Background()

	Metrics.RecordChatRequest(ctx, "1", "channel-a", 0.5, true)
	Metrics.RecordChatRequest(ctx, "1", "channel-a", 1.5, false)
	Metrics.RecordChatRequest(ctx, "", "", 2, false)
	Metrics.RecordChatTokens(ctx, "1", "channel-a", 100)
	Metrics.RecordChatTokens(ctx, "1", "channel-a", 0)

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))

	metricsByName := collectMetricsByName(t, data)

	requestCount := sumInt64(t, metricsByName, "chat_request_count")
	require.Equal(t, int64(3), requestCount.total)
	channelA, ok := requestCount.byLabel["channel_id=1|channel_name=channel-a"]
	require.True(t, ok, "expected a data point attributed to channel-a")
	require.Equal(t, int64(2), channelA)

	require.Equal(t, int64(2), sumInt64(t, metricsByName, "chat_failure_count").total)
	require.Equal(t, int64(1), sumInt64(t, metricsByName, "chat_success_count").total)

	tokens := sumInt64(t, metricsByName, "chat_token_count")
	require.Equal(t, int64(100), tokens.total)
	require.Equal(t, int64(100), tokens.byLabel["channel_id=1|channel_name=channel-a"])

	duration := histogram(t, metricsByName, "chat_request_duration_seconds")
	require.Equal(t, uint64(3), duration.count)
	require.InDelta(t, 4.0, duration.sum, 1e-9)
}

type int64Sum struct {
	total   int64
	byLabel map[string]int64
}

func sumInt64(t *testing.T, metricsByName map[string]metricdata.Metrics, name string) int64Sum {
	t.Helper()

	m, ok := metricsByName[name]
	require.True(t, ok, "missing metric %s", name)

	sum, ok := m.Data.(metricdata.Sum[int64])
	require.True(t, ok, "metric %s is not an int64 sum", name)

	result := int64Sum{byLabel: map[string]int64{}}
	for _, dp := range sum.DataPoints {
		result.total += dp.Value
		result.byLabel[labelKey(dp.Attributes)] += dp.Value
	}

	return result
}

type histogramSummary struct {
	count uint64
	sum   float64
}

func histogram(t *testing.T, metricsByName map[string]metricdata.Metrics, name string) histogramSummary {
	t.Helper()

	m, ok := metricsByName[name]
	require.True(t, ok, "missing metric %s", name)

	hist, ok := m.Data.(metricdata.Histogram[float64])
	require.True(t, ok, "metric %s is not a float64 histogram", name)

	var result histogramSummary
	for _, dp := range hist.DataPoints {
		result.count += dp.Count
		result.sum += dp.Sum
	}

	return result
}

func collectMetricsByName(t *testing.T, data metricdata.ResourceMetrics) map[string]metricdata.Metrics {
	t.Helper()

	result := map[string]metricdata.Metrics{}
	for _, sm := range data.ScopeMetrics {
		for _, m := range sm.Metrics {
			result[m.Name] = m
		}
	}

	return result
}

func labelKey(attrs attribute.Set) string {
	parts := make([]string, 0, attrs.Len())
	for _, kv := range attrs.ToSlice() {
		parts = append(parts, string(kv.Key)+"="+kv.Value.Emit())
	}

	return strings.Join(parts, "|")
}

func TestEnabled(t *testing.T) {
	previousEnabled := enabled
	t.Cleanup(func() {
		enabled = previousEnabled
	})

	provider, err := NewProvider(Config{Enabled: false})
	require.NoError(t, err)
	require.False(t, Enabled())
	require.NoError(t, provider.Shutdown(context.Background()))

	provider, err = NewProvider(Config{
		Enabled:  true,
		Exporter: ExporterConfig{Type: "stdout"},
	})
	require.NoError(t, err)
	require.True(t, Enabled())
	require.NoError(t, provider.Shutdown(context.Background()))
}

func TestRecordChatRequestWithoutSetup(t *testing.T) {
	// The package-level default must be safe to use without SetupMetrics, as
	// happens in tests of packages that only record metrics.
	require.NotNil(t, Metrics)

	Metrics.RecordChatRequest(context.Background(), "1", "channel-a", 1, true)
	Metrics.RecordChatTokens(context.Background(), "1", "channel-a", 10)
}
