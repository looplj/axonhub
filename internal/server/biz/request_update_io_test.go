package biz

import (
	"context"
	"net/http"
	"testing"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type requestIODriver struct {
	dialect.Driver

	queries, writes, transactions int
	zeroAffected                  bool
}

func (d *requestIODriver) Exec(ctx context.Context, query string, args, result any) error {
	d.writes++
	err := d.Driver.Exec(ctx, query, args, result)
	if err == nil && d.zeroAffected {
		r := result.(*sql.Result)
		*r = zeroAffectedResult{*r}
	}
	return err
}

func (d *requestIODriver) Query(ctx context.Context, query string, args, result any) error {
	d.queries++
	return d.Driver.Query(ctx, query, args, result)
}

func (d *requestIODriver) Tx(ctx context.Context) (dialect.Tx, error) {
	d.transactions++
	return d.Driver.Tx(ctx)
}

type zeroAffectedResult struct{ sql.Result }

func (zeroAffectedResult) RowsAffected() (int64, error) { return 0, nil }

func TestRequestWritersAvoidReadback(t *testing.T) {
	dsn := "file:request-update-io?mode=memory&cache=shared&_fk=0"
	schemaClient := enttest.NewEntClient(t, "sqlite3", dsn)
	defer schemaClient.Close()
	raw, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	raw.DB().SetMaxOpenConns(1)
	driver := &requestIODriver{Driver: raw}
	client := ent.NewClient(ent.Driver(driver))
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	system := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	storage := NewDataStorageService(DataStorageServiceParams{Client: client, SystemService: system, CacheConfig: system.CacheConfig})
	svc := NewRequestService(client, system.CacheConfig, system, nil, storage, NewLiveStreamRegistry())
	require.NoError(t, system.SetStoragePolicy(ctx, &defaultStoragePolicy))
	_, err = system.StoragePolicy(ctx) // Warm the settings cache before counting SQL.
	require.NoError(t, err)
	req := client.Request.Create().SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(request.StatusProcessing).SaveX(ctx)
	execution := client.RequestExecution.Create().SetRequestID(req.ID).SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing).SaveX(ctx)
	other := client.Request.Create().SetModelID("other").SetRequestBody([]byte(`{}`)).SetStatus(request.StatusProcessing).SaveX(ctx)
	otherExec := client.RequestExecution.Create().SetRequestID(other.ID).SetModelID("other").SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing).SaveX(ctx)
	writers := []struct {
		name  string
		write func(*ent.Request, *ent.RequestExecution) error
	}{
		{"headers", func(r *ent.Request, _ *ent.RequestExecution) error {
			return svc.UpdateRequestResponseHeaders(ctx, r.ID, http.Header{"Content-Type": {"application/json"}})
		}},
		{"request_final", func(r *ent.Request, _ *ent.RequestExecution) error {
			return svc.UpdateRequestFinalized(ctx, r, request.StatusCompleted, "response", map[string]any{"ok": true}, nil)
		}},
		{"audio", func(r *ent.Request, _ *ent.RequestExecution) error {
			return svc.UpdateRequestCompletedWithAudio(ctx, r, "audio", map[string]any{"ok": true}, nil, "audio.mp3", nil)
		}},
		{"async", func(r *ent.Request, _ *ent.RequestExecution) error {
			return svc.UpdateRequestStatusExternalIDAndResponseBody(ctx, r, request.StatusCompleted, "task", map[string]any{"ok": true}, nil)
		}},
		{"request_status", func(r *ent.Request, _ *ent.RequestExecution) error {
			return svc.UpdateRequestStatus(ctx, r.ID, request.StatusFailed)
		}},
		{"request_error", func(r *ent.Request, _ *ent.RequestExecution) error {
			return svc.UpdateRequestStatusFromError(ctx, r, context.Canceled)
		}},
		{"execution_final", func(_ *ent.Request, e *ent.RequestExecution) error {
			return svc.UpdateRequestExecutionFinalized(ctx, e, requestexecution.StatusCompleted, "", "response", map[string]any{"ok": true}, nil, "model", nil)
		}},
		{"execution_status", func(_ *ent.Request, e *ent.RequestExecution) error {
			return svc.UpdateRequestExecutionStatusWithMetrics(ctx, e, requestexecution.StatusFailed, "error", nil, nil, "", nil)
		}},
	}
	for _, writer := range writers {
		t.Run(writer.name, func(t *testing.T) {
			for _, mode := range []string{"normal", "unchanged", "missing"} {
				t.Run(mode, func(t *testing.T) {
					r, e := req, execution
					driver.zeroAffected = mode == "unchanged"
					if mode == "missing" {
						r = &ent.Request{ID: 99999}
						e = &ent.RequestExecution{ID: 99999}
					}
					driver.queries, driver.writes, driver.transactions = 0, 0, 0
					err := writer.write(r, e)
					if mode == "missing" {
						require.True(t, ent.IsNotFound(err), "expected not-found, got %v", err)
					} else {
						require.NoError(t, err)
					}
					require.Equal(t, 1, driver.writes)
					require.Zero(t, driver.transactions, "a single update must not open an explicit transaction")
					if mode == "normal" {
						require.Zero(t, driver.queries, "successful update must not read back")
					} else {
						require.Equal(t, 1, driver.queries)
					}
				})
			}
		})
	}
	driver.zeroAffected = false
	require.Equal(t, request.StatusProcessing, client.Request.GetX(ctx, other.ID).Status)
	require.Equal(t, requestexecution.StatusProcessing, client.RequestExecution.GetX(ctx, otherExec.ID).Status)
	// A zero-row update must retain the caller's extra predicates during the
	// fallback lookup instead of accepting a row that the update did not match.
	err = svc.saveRequestUpdate(ctx, req.ID, client.Request.Update().Where(request.ModelID("different-model")).SetStatus(request.StatusFailed))
	require.True(t, ent.IsNotFound(err))
	err = svc.saveRequestExecutionUpdate(ctx, execution.ID, client.RequestExecution.Update().Where(requestexecution.ModelID("different-model")).SetStatus(requestexecution.StatusFailed))
	require.True(t, ent.IsNotFound(err))
}
