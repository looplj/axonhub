package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestResourceNameFailureDoesNotBlockCounters(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	meter := provider.Meter("test")
	reg, err := RegisterResourceInfo(meter, func(context.Context) (ResourceNames, error) {
		return ResourceNames{}, errors.New("database unavailable")
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = reg.Unregister() })
	counter, err := meter.Int64Counter("request_test_total")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &collected))
	var count int64
	for _, scope := range collected.ScopeMetrics {
		for _, mt := range scope.Metrics {
			if mt.Name == "request_test_total" {
				for _, point := range mt.Data.(metricdata.Sum[int64]).DataPoints {
					count += point.Value
				}
			}
		}
	}
	require.Equal(t, int64(1), count)
}
