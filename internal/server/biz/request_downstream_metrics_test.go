package biz

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/sync/errgroup"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	appmetrics "github.com/looplj/axonhub/internal/metrics"
)

func TestConcurrentDownstreamFinalizationEmitsOnce(t *testing.T) {
	for _, outcome := range []request.Status{request.StatusCompleted, request.StatusFailed, request.StatusCanceled} {
		t.Run(string(outcome), func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
			defer client.Close()
			ctx, cancel := context.WithTimeout(authz.WithTestBypass(ent.NewContext(t.Context(), client)), 15*time.Second)
			defer cancel()
			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			previous := appmetrics.Metrics
			t.Cleanup(func() {
				appmetrics.Metrics = previous
				_ = provider.Shutdown(context.Background())
			})
			require.NoError(t, appmetrics.SetupMetrics(provider, "downstream-test"))
			svc := &RequestService{AbstractService: &AbstractService{db: client}, SystemService: NewSystemService(SystemServiceParams{Ent: client})}
			req, err := client.Request.Create().SetModelID("requested").SetRequestBody([]byte(`{}`)).
				SetStatus(request.StatusProcessing).SetStream(true).Save(ctx)
			require.NoError(t, err)
			appmetrics.Metrics.RecordDownstreamRequestCreated(ctx, requestMetricAttributes(ctx, req, 0, ""))
			ready := make(chan struct{})
			var arrivals atomic.Int32
			client.Request.Use(func(next ent.Mutator) ent.Mutator {
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
			var group errgroup.Group
			for _, finalized := range []bool{false, true} {
				group.Go(func() (err error) {
					defer func() {
						if recovered := recover(); recovered != nil {
							err = fmt.Errorf("downstream finalizer panic: %v", recovered)
							log.Error(ctx, "downstream finalizer test panic", log.Cause(err))
						}
					}()
					if finalized {
						return svc.UpdateRequestFinalized(ctx, req.ID, outcome, "response", []byte(`{"result":"done"}`), nil)
					}
					return svc.UpdateRequestStatus(ctx, req.ID, request.StatusCanceled)
				})
			}
			require.NoError(t, group.Wait())
			stored, err := client.Request.Get(ctx, req.ID)
			require.NoError(t, err)
			// Late callbacks through either path must not change the winning result.
			require.NoError(t, svc.UpdateRequestStatus(ctx, req.ID, request.StatusFailed))
			require.NoError(t, svc.UpdateRequestFinalized(ctx, req.ID, request.StatusCompleted, "late", []byte(`{}`), nil))
			again, err := client.Request.Get(ctx, req.ID)
			require.NoError(t, err)
			require.Equal(t, stored.Status, again.Status)
			require.Equal(t, stored.ResponseBody, again.ResponseBody)
			var collected metricdata.ResourceMetrics
			require.NoError(t, reader.Collect(ctx, &collected))
			seen := 0
			for _, scope := range collected.ScopeMetrics {
				for _, mt := range scope.Metrics {
					switch mt.Name {
					case "axonhub_downstream_requests_total", "axonhub_downstream_active_requests":
						points := mt.Data.(metricdata.Sum[int64]).DataPoints
						require.Len(t, points, 1)
						want := int64(0)
						if mt.Name == "axonhub_downstream_requests_total" {
							want = 1
							status, _ := points[0].Attributes.Value("status")
							require.Equal(t, string(stored.Status), status.AsString())
						}
						require.Equal(t, want, points[0].Value)
						seen++
					case "axonhub_downstream_request_duration_seconds":
						points := mt.Data.(metricdata.Histogram[float64]).DataPoints
						require.Len(t, points, 1)
						require.Equal(t, uint64(1), points[0].Count)
						seen++
					}
				}
			}
			require.Equal(t, 3, seen)
		})
	}
}
