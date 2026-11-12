package biz

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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

func TestTerminalUpdatesIncludeChunks(t *testing.T) {
	for _, mode := range []string{"database", "disabled", "external", "storage_failure", "update_failure"} {
		for _, writer := range []string{"request_final", "execution_final", "request_error", "execution_error"} {
			t.Run(mode+"/"+writer, func(t *testing.T) {
				client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
				ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
				system := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
				storage := NewDataStorageService(DataStorageServiceParams{Client: client, SystemService: system, CacheConfig: system.CacheConfig})
				svc := NewRequestService(client, system.CacheConfig, system, nil, storage, NewLiveStreamRegistry())
				policy := defaultStoragePolicy
				policy.StoreChunks = mode != "disabled"
				// Chunk storage must work independently of response body storage.
				policy.StoreResponseBody = false
				require.NoError(t, system.SetStoragePolicy(ctx, &policy))
				var ds *ent.DataStorage
				if mode != "database" && mode != "disabled" {
					dir := t.TempDir()
					ds = client.DataStorage.Create().SetName("chunks").SetDescription("test chunks").SetPrimary(false).SetType(datastorage.TypeFs).
						SetSettings(&objects.DataStorageSettings{Directory: &dir}).SetStatus(datastorage.StatusActive).SaveX(ctx)
				}
				reqCreate := client.Request.Create().SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(request.StatusProcessing)
				if ds != nil {
					reqCreate.SetDataStorageID(ds.ID)
				}
				req := reqCreate.SaveX(ctx)
				execCreate := client.RequestExecution.Create().SetRequestID(req.ID).SetModelID("model").SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing)
				if ds != nil {
					execCreate.SetDataStorageID(ds.ID)
				}
				execution := execCreate.SaveX(ctx)

				key := GenerateResponseChunksKey(req.ProjectID, req.ID)
				isExecution := writer == "execution_final" || writer == "execution_error"
				if isExecution {
					key = GenerateExecutionResponseChunksKey(execution.ProjectID, execution.RequestID, execution.ID)
				}
				if mode == "storage_failure" {
					// A directory at the payload filename makes the write fail deterministically.
					require.NoError(t, os.MkdirAll(filepath.Join(*ds.Settings.Directory, key), 0o700))
				}
				updates := 0
				injected := errors.New("injected terminal update failure")
				client.Use(func(next ent.Mutator) ent.Mutator {
					return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
						if m.Op().Is(ent.OpUpdate|ent.OpUpdateOne) && (m.Type() == "Request" || m.Type() == "RequestExecution") {
							updates++
							_, hasChunks := m.Field("response_chunks")
							require.Equal(t, mode != "disabled" && mode != "storage_failure", hasChunks, "chunks must be part of the terminal mutation")
							if mode == "update_failure" {
								return nil, injected
							}
						}
						return next.Mutate(ctx, m)
					})
				})
				chunks := []*httpclient.StreamEvent{{Type: "message", Data: []byte(`{"delta":"hello"}`)}, {Data: []byte(`[DONE]`)}}
				var err error
				switch writer {
				case "request_final":
					err = svc.UpdateRequestFinalized(ctx, req, request.StatusCompleted, "response", nil, nil, nil, chunks...)
				case "execution_final":
					err = svc.UpdateRequestExecutionFinalized(ctx, execution, requestexecution.StatusCompleted, "", "response", nil, nil, "", nil, chunks...)
				case "request_error":
					err = svc.UpdateRequestStatusFromError(ctx, req, context.Canceled, nil, chunks...)
				case "execution_error":
					err = svc.UpdateRequestExecutionStatusWithMetrics(ctx, execution, requestexecution.StatusCanceled, "canceled", nil, nil, "", nil, chunks...)
				}
				require.Equal(t, 1, updates)
				if mode == "update_failure" {
					require.ErrorIs(t, err, injected)
					_, loadErr := storage.LoadData(ctx, ds, key)
					require.Error(t, loadErr, "external chunks must be rolled back")
					return
				}
				if mode == "storage_failure" {
					require.ErrorContains(t, err, "failed to save chunks")
				} else {
					require.NoError(t, err)
				}
				var stored []objects.JSONRawMessage
				var status string
				if isExecution {
					row := client.RequestExecution.GetX(ctx, execution.ID)
					stored = row.ResponseChunks
					status = string(row.Status)
				} else {
					row := client.Request.GetX(ctx, req.ID)
					stored = row.ResponseChunks
					status = string(row.Status)
				}
				expectedStatus := "completed"
				if writer == "request_error" || writer == "execution_error" {
					expectedStatus = "canceled"
				}
				require.Equal(t, expectedStatus, status, "a chunk write failure must not prevent finalization")
				switch mode {
				case "disabled", "storage_failure":
					require.Empty(t, stored)
				case "database":
					require.Len(t, stored, 1)
					require.JSONEq(t, `{"event":"message","data":{"delta":"hello"}}`, string(stored[0]))
				case "external":
					require.Equal(t, ExternalResponseChunksMarker, stored)
					payload, err := storage.LoadData(ctx, ds, key)
					require.NoError(t, err)
					require.JSONEq(t, `[{"event":"message","data":{"delta":"hello"}}]`, string(payload))
				}
			})
		}
	}
}
