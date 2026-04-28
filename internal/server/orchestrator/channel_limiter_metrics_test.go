package orchestrator

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestChannelLimiterMetrics_NilMeterIsNoop(t *testing.T) {
	t.Parallel()

	mgr := NewChannelLimiterManager()
	m, err := NewChannelLimiterMetrics(nil, mgr)
	require.NoError(t, err)
	require.NotNil(t, m)

	// All emission methods are safe to call.
	ch := makeBizChannel(1, "x")
	m.IncQueueFull(t.Context(), ch)
	m.IncQueueTimeout(t.Context(), ch)
	m.ObserveQueueWait(t.Context(), ch, 0)
}

func TestChannelLimiterMetrics_GaugeCallbackEmitsLimiterStats(t *testing.T) {
	t.Parallel()

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	meter := provider.Meter("axonhub-test")
	mgr := NewChannelLimiterManager()

	_, err := NewChannelLimiterMetrics(meter, mgr)
	require.NoError(t, err)

	// Provision a limiter with two slots in flight.
	ch := &biz.Channel{
		Channel: &ent.Channel{
			ID:   42,
			Name: "kimi",
			Settings: &objects.ChannelSettings{
				RateLimit: &objects.ChannelRateLimit{
					MaxConcurrent: lo.ToPtr(int64(5)),
					QueueSize:     lo.ToPtr(int64(3)),
				},
			},
		},
	}
	lim := mgr.GetOrCreate(ch)
	require.NoError(t, lim.Acquire(t.Context()))
	require.NoError(t, lim.Acquire(t.Context()))

	rm := metricdata.ResourceMetrics{}
	require.NoError(t, reader.Collect(context.Background(), &rm))

	inflight := findGauge(t, rm, "axonhub_channel_inflight")
	require.Len(t, inflight.DataPoints, 1)
	assert.Equal(t, int64(2), inflight.DataPoints[0].Value)

	waiting := findGauge(t, rm, "axonhub_channel_queue_waiting")
	require.Len(t, waiting.DataPoints, 1)
	assert.Equal(t, int64(0), waiting.DataPoints[0].Value)

	lim.Release()
	lim.Release()
}

func TestChannelLimiterMetrics_CountersAndHistogram(t *testing.T) {
	t.Parallel()

	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	meter := provider.Meter("axonhub-test")
	m, err := NewChannelLimiterMetrics(meter, nil)
	require.NoError(t, err)

	ch := makeBizChannel(7, "openai")
	m.IncQueueFull(t.Context(), ch)
	m.IncQueueFull(t.Context(), ch)
	m.IncQueueTimeout(t.Context(), ch)
	m.ObserveQueueWait(t.Context(), ch, 0) // a no-wait acquisition

	rm := metricdata.ResourceMetrics{}
	require.NoError(t, reader.Collect(context.Background(), &rm))

	full := findCounter(t, rm, "axonhub_channel_queue_full_total")
	require.Len(t, full.DataPoints, 1)
	assert.Equal(t, int64(2), full.DataPoints[0].Value)

	timeout := findCounter(t, rm, "axonhub_channel_queue_timeout_total")
	require.Len(t, timeout.DataPoints, 1)
	assert.Equal(t, int64(1), timeout.DataPoints[0].Value)

	wait := findHistogram(t, rm, "axonhub_channel_queue_wait_seconds")
	require.Len(t, wait.DataPoints, 1)
	assert.Equal(t, uint64(1), wait.DataPoints[0].Count)
}

// findGauge returns the Int64 gauge data for a metric name, failing the test if absent.
func findGauge(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Gauge[int64] {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, mt := range sm.Metrics {
			if mt.Name != name {
				continue
			}

			g, ok := mt.Data.(metricdata.Gauge[int64])
			require.True(t, ok, "metric %q is not an int64 gauge", name)

			return g
		}
	}

	t.Fatalf("gauge %q not present in collected metrics", name)
	return metricdata.Gauge[int64]{}
}

func findCounter(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Sum[int64] {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, mt := range sm.Metrics {
			if mt.Name != name {
				continue
			}

			s, ok := mt.Data.(metricdata.Sum[int64])
			require.True(t, ok, "metric %q is not an int64 sum", name)

			return s
		}
	}

	t.Fatalf("counter %q not present in collected metrics", name)
	return metricdata.Sum[int64]{}
}

func findHistogram(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Histogram[float64] {
	t.Helper()

	for _, sm := range rm.ScopeMetrics {
		for _, mt := range sm.Metrics {
			if mt.Name != name {
				continue
			}

			h, ok := mt.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "metric %q is not a float64 histogram", name)

			return h
		}
	}

	t.Fatalf("histogram %q not present in collected metrics", name)
	return metricdata.Histogram[float64]{}
}
