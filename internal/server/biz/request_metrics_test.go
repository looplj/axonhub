package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
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
