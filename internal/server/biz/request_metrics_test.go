package biz

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/sync/errgroup"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
	appmetrics "github.com/looplj/axonhub/internal/metrics"
)

func TestFailedExecutionEmitsLatency(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:failed_execution_metrics?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := appmetrics.Metrics
	t.Cleanup(func() {
		appmetrics.Metrics = previous
		_ = provider.Shutdown(context.Background())
	})
	require.NoError(t, appmetrics.SetupMetrics(provider, "request-test"))
	req, err := client.Request.Create().SetModelID("requested").SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusProcessing).SetStream(true).Save(ctx)
	require.NoError(t, err)
	execution, err := client.RequestExecution.Create().SetRequestID(req.ID).SetModelID("actual").
		SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing).SetStream(true).Save(ctx)
	require.NoError(t, err)
	svc := &RequestService{AbstractService: &AbstractService{db: client}}
	latency := &LatencyMetrics{LatencyMs: lo.ToPtr(int64(2500)), FirstTokenLatencyMs: lo.ToPtr(int64(500))}
	require.NoError(t, svc.UpdateRequestExecutionStatusWithMetrics(ctx, execution.ID, requestexecution.StatusFailed, "stream failed", nil, latency))
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &collected))
	want := map[string]float64{"axonhub_upstream_request_duration_seconds": 2.5, "axonhub_upstream_ttft_seconds": 0.5}
	for _, scope := range collected.ScopeMetrics {
		for _, mt := range scope.Metrics {
			if sum, ok := want[mt.Name]; ok {
				hist := mt.Data.(metricdata.Histogram[float64])
				require.Len(t, hist.DataPoints, 1)
				require.Equal(t, uint64(1), hist.DataPoints[0].Count)
				require.Equal(t, sum, hist.DataPoints[0].Sum)
				status, _ := hist.DataPoints[0].Attributes.Value("status")
				require.Equal(t, "failed", status.AsString())
				delete(want, mt.Name)
			}
		}
	}
	require.Empty(t, want)
}

func TestConcurrentExecutionFinalizationEmitsOnce(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:concurrent_execution_metrics?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := appmetrics.Metrics
	t.Cleanup(func() {
		appmetrics.Metrics = previous
		_ = provider.Shutdown(context.Background())
	})
	require.NoError(t, appmetrics.SetupMetrics(provider, "concurrent-request-test"))
	svc := &RequestService{
		AbstractService: &AbstractService{db: client},
		SystemService:   NewSystemService(SystemServiceParams{Ent: client}),
	}
	req, err := client.Request.Create().SetModelID("requested").SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusProcessing).SetStream(true).Save(ctx)
	require.NoError(t, err)
	execution, err := client.RequestExecution.Create().SetRequestID(req.ID).SetModelID("actual").
		SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing).SetStream(true).Save(ctx)
	require.NoError(t, err)
	appmetrics.Metrics.RecordUpstreamRequestCreated(ctx, executionMetricAttributes(execution, req))

	// Both calls must read processing before either conditional UPDATE executes.
	ready := make(chan struct{})
	var arrivals atomic.Int32
	client.RequestExecution.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if mutation.Op() == ent.OpUpdateOne {
				if arrivals.Add(1) == 2 {
					close(ready)
				}
				select {
				case <-ready:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return next.Mutate(ctx, mutation)
		})
	})
	latency := &LatencyMetrics{LatencyMs: lo.ToPtr(int64(2500)), FirstTokenLatencyMs: lo.ToPtr(int64(500))}
	var group errgroup.Group
	for _, finalizeResponse := range []bool{false, true} {
		group.Go(func() (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("finalization panic: %v", recovered)
					log.Error(ctx, "concurrent finalization test panic", log.Cause(err))
				}
			}()
			if finalizeResponse {
				return svc.UpdateRequestExecutionFinalized(ctx, execution.ID, requestexecution.StatusCompleted, "", "response-id", []byte(`{}`), latency)
			}
			return svc.UpdateRequestExecutionStatusWithMetrics(ctx, execution.ID, requestexecution.StatusFailed, "stream failed", nil, latency)
		})
	}
	require.NoError(t, group.Wait())
	stored, err := client.RequestExecution.Get(ctx, execution.ID)
	require.NoError(t, err)
	require.True(t, isTerminalExecutionStatus(stored.Status))
	// A repeated terminal callback must neither change the winning outcome nor emit.
	require.NoError(t, svc.UpdateRequestExecutionStatusWithMetrics(ctx, execution.ID, requestexecution.StatusCanceled, "late cancellation", nil, latency))
	again, err := client.RequestExecution.Get(ctx, execution.ID)
	require.NoError(t, err)
	require.Equal(t, stored.Status, again.Status)
	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &collected))
	seen := map[string]bool{}
	for _, scope := range collected.ScopeMetrics {
		for _, mt := range scope.Metrics {
			switch mt.Name {
			case "axonhub_upstream_requests_total", "axonhub_upstream_active_requests":
				points := mt.Data.(metricdata.Sum[int64]).DataPoints
				require.Len(t, points, 1)
				want := int64(0)
				if mt.Name == "axonhub_upstream_requests_total" {
					want = 1
					status, _ := points[0].Attributes.Value("status")
					require.Equal(t, string(stored.Status), status.AsString())
				}
				require.Equal(t, want, points[0].Value)
				seen[mt.Name] = true
			case "axonhub_upstream_request_duration_seconds", "axonhub_upstream_ttft_seconds":
				points := mt.Data.(metricdata.Histogram[float64]).DataPoints
				require.Len(t, points, 1)
				require.Equal(t, uint64(1), points[0].Count)
				seen[mt.Name] = true
			}
		}
	}
	require.Len(t, seen, 4)
}
