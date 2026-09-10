package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/ent"
	appmetrics "github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestInboundFirstOutputDoesNotRecordRequestDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := appmetrics.Metrics
	t.Cleanup(func() {
		appmetrics.Metrics = previous
		_ = provider.Shutdown(context.Background())
	})
	require.NoError(t, appmetrics.SetupMetrics(provider, "inbound-test"))
	stream := &InboundPersistentStream{
		ctx:     t.Context(),
		request: &ent.Request{CreatedAt: time.Now().Add(-time.Second), ModelID: "requested", Stream: true},
		state:   &PersistenceState{},
		stream: &mockStream{events: []*httpclient.StreamEvent{
			{Data: []byte(`{"choices":[{"delta":{"content":"hello"}}]}`)},
			{Data: []byte(`{"choices":[{"delta":{"content":" world"}}]}`)},
		}},
	}
	for stream.Next() {
		stream.Current()
	}

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &collected))
	var ttftCount uint64
	for _, scope := range collected.ScopeMetrics {
		for _, mt := range scope.Metrics {
			require.NotEqual(t, "axonhub_downstream_request_duration_seconds", mt.Name,
				"first output must not emit an end-to-end duration sample")
			if mt.Name == "axonhub_downstream_ttft_seconds" {
				for _, point := range mt.Data.(metricdata.Histogram[float64]).DataPoints {
					ttftCount += point.Count
				}
			}
		}
	}
	require.Equal(t, uint64(1), ttftCount)
}
