package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestLLMMetricsRecordLifecycleUsageAndPerformance(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	previous := Metrics
	t.Cleanup(func() { Metrics = previous })

	require.NoError(t, SetupMetrics(provider, "axonhub-test"))
	attrs := RequestAttributes{
		ProjectID:      1,
		ChannelID:      2,
		RequestModelID: "requested-model",
		ModelID:        "actual-model",
		APIKeyID:       3,
		UserID:         4,
		Source:         "api",
		Format:         "openai/chat_completions",
		Stream:         true,
	}

	// Routing and context enrichment change between creation and completion.
	downstreamCreated := attrs
	downstreamCreated.ChannelID = 0
	downstreamCreated.ModelID = ""
	downstreamCreated.UserID = 0
	upstreamCreated := attrs
	upstreamCreated.RequestModelID = ""
	upstreamCreated.APIKeyID = 0
	upstreamCreated.UserID = 0
	upstreamCreated.Source = ""
	Metrics.RecordDownstreamRequestCreated(t.Context(), downstreamCreated)
	Metrics.RecordUpstreamRequestCreated(t.Context(), upstreamCreated)
	active := metricdata.ResourceMetrics{}
	require.NoError(t, reader.Collect(t.Context(), &active))
	assert.Equal(t, int64(1), metricCounterValue(t, active, "axonhub_downstream_active_requests"))
	assert.Equal(t, int64(1), metricCounterValue(t, active, "axonhub_upstream_active_requests"))
	Metrics.RecordDownstreamRequestCompleted(t.Context(), attrs, "completed")
	Metrics.RecordUpstreamRequestCompleted(t.Context(), attrs, "failed")
	cost := 0.125
	Metrics.RecordLLMUsage(t.Context(), attrs, 100, 40, 20, 5, &cost)
	ttft := 0.25
	Metrics.RecordUpstreamPerformance(t.Context(), attrs, "completed", 1.5, &ttft)

	rm := metricdata.ResourceMetrics{}
	require.NoError(t, reader.Collect(context.Background(), &rm))

	assert.Equal(t, int64(1), metricCounterValue(t, rm, "axonhub_downstream_requests_total"))
	assert.Equal(t, int64(0), metricCounterValue(t, rm, "axonhub_downstream_active_requests"))
	assert.Equal(t, int64(1), metricCounterValue(t, rm, "axonhub_upstream_requests_total"))
	assert.Equal(t, int64(0), metricCounterValue(t, rm, "axonhub_upstream_active_requests"))
	assert.Equal(t, int64(100), metricCounterValueByType(t, rm, "axonhub_llm_tokens_total", "input"))
	assert.Equal(t, int64(40), metricCounterValueByType(t, rm, "axonhub_llm_tokens_total", "output"))
	assert.Equal(t, int64(20), metricCounterValueByType(t, rm, "axonhub_llm_tokens_total", "cached"))
	assert.Equal(t, int64(5), metricCounterValueByType(t, rm, "axonhub_llm_tokens_total", "reasoning"))
	assert.InDelta(t, cost, metricFloatCounterValue(t, rm, "axonhub_llm_cost_total"), 1e-9)
	assert.Equal(t, uint64(1), metricHistogramCount(t, rm, "axonhub_upstream_request_duration_seconds"))
	assert.Equal(t, uint64(1), metricHistogramCount(t, rm, "axonhub_upstream_ttft_seconds"))
}

func metricCounterValue(t *testing.T, rm metricdata.ResourceMetrics, name string) int64 {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			data, ok := metric.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			require.Len(t, data.DataPoints, 1)
			return data.DataPoints[0].Value
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

func metricCounterValueByType(t *testing.T, rm metricdata.ResourceMetrics, name, tokenType string) int64 {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			data, ok := metric.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			for _, point := range data.DataPoints {
				value, ok := point.Attributes.Value(attribute.Key("token_type"))
				if ok && value.AsString() == tokenType {
					return point.Value
				}
			}
		}
	}
	t.Fatalf("metric %q with token_type=%q not found", name, tokenType)
	return 0
}

func metricFloatCounterValue(t *testing.T, rm metricdata.ResourceMetrics, name string) float64 {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			data, ok := metric.Data.(metricdata.Sum[float64])
			require.True(t, ok)
			require.Len(t, data.DataPoints, 1)
			return data.DataPoints[0].Value
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}

func metricHistogramCount(t *testing.T, rm metricdata.ResourceMetrics, name string) uint64 {
	t.Helper()
	for _, scope := range rm.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			data, ok := metric.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			require.Len(t, data.DataPoints, 1)
			return data.DataPoints[0].Count
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
}
