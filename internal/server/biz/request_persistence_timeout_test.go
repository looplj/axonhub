package biz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm/httpclient"
)

type deadlineChunkStore struct {
	ObjectStore

	t                 *testing.T
	lateSuccess       bool
	uploaded, deleted []string
}

func (s *deadlineChunkStore) PutObject(ctx context.Context, key string, _ []byte) error {
	require.True(s.t, strings.HasSuffix(key, "response_chunks.json"))
	_, ok := ctx.Deadline()
	require.True(s.t, ok)
	<-ctx.Done()
	if s.lateSuccess {
		s.uploaded = append(s.uploaded, key)
		return nil
	}
	return ctx.Err()
}

func (s *deadlineChunkStore) DeleteObject(ctx context.Context, key string) error {
	require.NoError(s.t, ctx.Err(), "cleanup must not reuse an expired upload or database context")
	deadline, ok := ctx.Deadline()
	require.True(s.t, ok, "cleanup must remain bounded")
	require.Positive(s.t, time.Until(deadline))
	s.deleted = append(s.deleted, key)
	return nil
}

func TestTerminalWriteSurvivesChunkUploadDeadline(t *testing.T) {
	for _, mode := range []string{"upload_timeout", "late_upload_database_failure"} {
		for _, writer := range []string{"request_final", "execution_final", "request_failed", "execution_failed", "request_canceled", "execution_canceled"} {
			t.Run(mode+"/"+writer, func(t *testing.T) {
				client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
				defer client.Close()
				ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
				system := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
				storage := NewDataStorageService(DataStorageServiceParams{Client: client, SystemService: system, CacheConfig: system.CacheConfig})
				svc := NewRequestService(client, system.CacheConfig, system, nil, storage, NewLiveStreamRegistry())
				policy := defaultStoragePolicy
				policy.StoreChunks = true
				policy.StoreResponseBody = false
				require.NoError(t, system.SetStoragePolicy(ctx, &policy))
				ds := client.DataStorage.Create().SetName("timeout").SetDescription("timeout test").SetPrimary(false).
					SetType(datastorage.TypeS3).SetSettings(&objects.DataStorageSettings{S3: &objects.S3{}}).
					SetStatus(datastorage.StatusActive).SaveX(ctx)
				store := &deadlineChunkStore{t: t, lateSuccess: mode == "late_upload_database_failure"}
				storage.fsCacheMu.Lock()
				storage.objectStoreCache[ds.ID] = store
				storage.fsCacheMu.Unlock()
				req := client.Request.Create().SetModelID("model").SetRequestBody([]byte(`{}`)).SetDataStorageID(ds.ID).
					SetStatus(request.StatusProcessing).SaveX(ctx)
				execution := client.RequestExecution.Create().SetRequestID(req.ID).SetModelID("model").SetRequestBody([]byte(`{}`)).
					SetDataStorageID(ds.ID).SetStatus(requestexecution.StatusProcessing).SaveX(ctx)
				injected := errors.New("database update failed")
				updates := 0
				client.Use(func(next ent.Mutator) ent.Mutator {
					return ent.MutateFunc(func(saveCtx context.Context, m ent.Mutation) (ent.Value, error) {
						if m.Type() == "Request" || m.Type() == "RequestExecution" {
							updates++
							require.NoError(t, saveCtx.Err(), "terminal update must have a fresh context")
							require.Same(t, client, ent.FromContext(saveCtx), "request context values must be preserved")
							deadline, ok := saveCtx.Deadline()
							require.True(t, ok)
							require.Greater(t, time.Until(deadline), 8*time.Second)
							if store.lateSuccess {
								return nil, injected
							}
						}
						return next.Mutate(saveCtx, m)
					})
				})
				_, err := system.StoragePolicy(ctx)
				require.NoError(t, err)
				_, err = storage.GetDataStorageByID(ctx, ds.ID)
				require.NoError(t, err)

				// Only the upload consumes this short budget; terminal persistence must
				// receive a new budget after it returns.
				uploadCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
				defer cancel()
				chunks := []*httpclient.StreamEvent{{Type: "message", Data: []byte(`{"delta":"hello"}`)}}
				switch writer {
				case "request_final":
					err = svc.UpdateRequestFinalized(uploadCtx, req, request.StatusCompleted, "response", nil, nil, nil, chunks...)
				case "execution_final":
					err = svc.UpdateRequestExecutionFinalized(uploadCtx, execution, requestexecution.StatusCompleted, "", "response", nil, nil, "", nil, chunks...)
				case "request_failed":
					err = svc.UpdateRequestStatusFromError(uploadCtx, req, errors.New("upstream failed"), nil, chunks...)
				case "request_canceled":
					err = svc.UpdateRequestStatusFromError(uploadCtx, req, context.Canceled, nil, chunks...)
				case "execution_failed":
					err = svc.UpdateRequestExecutionStatusWithMetrics(uploadCtx, execution, requestexecution.StatusFailed, "failed", nil, nil, "", nil, chunks...)
				case "execution_canceled":
					err = svc.UpdateRequestExecutionStatusWithMetrics(uploadCtx, execution, requestexecution.StatusCanceled, "canceled", nil, nil, "", nil, chunks...)
				}
				require.ErrorIs(t, uploadCtx.Err(), context.DeadlineExceeded)
				require.Equal(t, 1, updates)
				expectedStatus := "completed"
				if strings.HasSuffix(writer, "failed") {
					expectedStatus = "failed"
				}
				if strings.HasSuffix(writer, "canceled") {
					expectedStatus = "canceled"
				}
				if store.lateSuccess {
					require.ErrorIs(t, err, injected)
					require.Len(t, store.uploaded, 1)
					require.Equal(t, store.uploaded, store.deleted, "uploaded chunks must be rolled back using a live context")
					expectedStatus = "processing"
				} else {
					require.ErrorIs(t, err, context.DeadlineExceeded, "report the upload failure even though status was saved")
				}
				if strings.HasPrefix(writer, "execution") {
					row := client.RequestExecution.GetX(ctx, execution.ID)
					require.Equal(t, expectedStatus, string(row.Status))
					require.Empty(t, row.ResponseChunks)
				} else {
					row := client.Request.GetX(ctx, req.ID)
					require.Equal(t, expectedStatus, string(row.Status))
					require.Empty(t, row.ResponseChunks)
				}
			})
		}
	}
}
