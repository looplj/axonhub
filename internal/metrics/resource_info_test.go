package metrics

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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

func TestResourceNameCollectionDoesNotWaitForRefresh(t *testing.T) {
	oldInterval := resourceInfoRefreshInterval
	resourceInfoRefreshInterval = time.Millisecond
	t.Cleanup(func() { resourceInfoRefreshInterval = oldInterval })
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	started := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	reg, err := RegisterResourceInfo(provider.Meter("test"), func(context.Context) (ResourceNames, error) {
		calls++
		if calls == 1 {
			return ResourceNames{Projects: []ResourceName{{ID: 1, Name: "initial"}}}, nil
		}
		if calls == 2 {
			close(started)
			<-release
		}
		return ResourceNames{Projects: []ResourceName{{ID: 1, Name: "refreshed"}}}, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		close(release)
		_ = reg.Unregister()
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("refresh did not start")
	}
	collectionDone := make(chan error, 1)
	go func() {
		defer func() {
			if cause := recover(); cause != nil {
				collectionDone <- fmt.Errorf("collection panic: %v", cause)
			}
		}()
		var collected metricdata.ResourceMetrics
		collectionDone <- reader.Collect(context.Background(), &collected)
	}()
	select {
	case err := <-collectionDone:
		require.NoError(t, err)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("collection waited for a blocked refresh")
	}
}
