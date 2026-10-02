package biz

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func externalFinalizationFixture(t *testing.T) (context.Context, *ent.Client, *RequestService, *ent.DataStorage, *ent.RequestExecution) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(authz.WithTestBypass(ent.NewContext(t.Context(), client)), 15*time.Second)
	t.Cleanup(cancel)
	system := NewSystemService(SystemServiceParams{Ent: client})
	storage := NewDataStorageService(DataStorageServiceParams{Client: client, SystemService: system, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	svc := &RequestService{AbstractService: &AbstractService{db: client}, SystemService: system, DataStorageService: storage}
	dir := t.TempDir()
	ds, err := client.DataStorage.Create().SetName("finalization-fs").SetDescription("test storage").SetPrimary(false).
		SetType(datastorage.TypeFs).SetStatus(datastorage.StatusActive).
		SetSettings(&objects.DataStorageSettings{Directory: &dir}).Save(ctx)
	require.NoError(t, err)
	req, err := client.Request.Create().SetModelID("requested").SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusProcessing).SetDataStorageID(ds.ID).Save(ctx)
	require.NoError(t, err)
	execution, err := client.RequestExecution.Create().SetRequestID(req.ID).SetModelID("actual").
		SetRequestBody([]byte(`{}`)).SetStatus(requestexecution.StatusProcessing).SetDataStorageID(ds.ID).Save(ctx)
	require.NoError(t, err)
	return ctx, client, svc, ds, execution
}

func TestConcurrentFinalizationKeepsWinningExternalBody(t *testing.T) {
	ctx, client, svc, ds, execution := externalFinalizationFixture(t)
	key := GenerateExecutionResponseBodyKey(execution.ProjectID, execution.RequestID, execution.ID)
	fs, err := svc.DataStorageService.GetFileSystem(ctx, ds)
	require.NoError(t, err)
	var arrivals atomic.Int32
	ready := make(chan struct{})
	client.RequestExecution.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			m := mutation.(*ent.RequestExecutionMutation)
			if _, terminalUpdate := m.Status(); terminalUpdate {
				// Both finalizers pause after their reads, before the atomic UPDATE.
				// Neither may have touched the shared file at this point.
				_, statErr := fs.Stat(key)
				if statErr == nil {
					return nil, errors.New("external body written before claiming finalization")
				}
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
	for _, id := range []string{"first", "second"} {
		group.Go(func() (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("finalizer panic: %v", recovered)
					log.Error(ctx, "external finalization test panic", log.Cause(err))
				}
			}()
			return svc.UpdateRequestExecutionFinalized(ctx, execution.ID, requestexecution.StatusCompleted, "", id, map[string]string{"winner": id}, nil)
		})
	}
	require.NoError(t, group.Wait())
	stored, err := client.RequestExecution.Get(ctx, execution.ID)
	require.NoError(t, err)
	require.True(t, isExternalResponseBodyMarker(stored.ResponseBody))
	body, err := svc.DataStorageService.LoadData(ctx, ds, key)
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"winner":%q}`, stored.ExternalID), string(body))
	loaded, err := svc.LoadRequestExecutionResponseBody(ctx, stored)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(loaded))
}

func TestConcurrentDownstreamFinalizationKeepsWinningExternalBody(t *testing.T) {
	ctx, client, svc, ds, execution := externalFinalizationFixture(t)
	req, err := client.Request.Get(ctx, execution.RequestID)
	require.NoError(t, err)
	key := GenerateResponseBodyKey(req.ProjectID, req.ID)
	fs, err := svc.DataStorageService.GetFileSystem(ctx, ds)
	require.NoError(t, err)
	var arrivals atomic.Int32
	ready := make(chan struct{})
	client.Request.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			m := mutation.(*ent.RequestMutation)
			if _, terminalUpdate := m.Status(); terminalUpdate {
				// Both finalizers pause after their reads, before the atomic UPDATE.
				// Neither may have touched the shared file at this point.
				_, statErr := fs.Stat(key)
				if statErr == nil {
					return nil, errors.New("external body written before claiming finalization")
				}
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
	for _, id := range []string{"first", "second"} {
		group.Go(func() (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("finalizer panic: %v", recovered)
					log.Error(ctx, "external finalization test panic", log.Cause(err))
				}
			}()
			return svc.UpdateRequestFinalized(ctx, req.ID, request.StatusCompleted, id, map[string]string{"winner": id}, nil)
		})
	}
	require.NoError(t, group.Wait())
	stored, err := client.Request.Get(ctx, req.ID)
	require.NoError(t, err)
	require.True(t, isExternalResponseBodyMarker(stored.ResponseBody))
	body, err := svc.DataStorageService.LoadData(ctx, ds, key)
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"winner":%q}`, stored.ExternalID), string(body))
	loaded, err := svc.LoadResponseBody(ctx, stored)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(loaded))
}

func TestExternalFinalizationRetainsBodyOnOffloadFailure(t *testing.T) {
	for _, failure := range []string{"file-write", "marker-update"} {
		t.Run(failure, func(t *testing.T) {
			ctx, client, svc, ds, execution := externalFinalizationFixture(t)
			if failure == "file-write" {
				fs, err := svc.DataStorageService.GetFileSystem(ctx, ds)
				require.NoError(t, err)
				svc.DataStorageService.fsCache[ds.ID] = afero.NewReadOnlyFs(fs)
			} else {
				client.RequestExecution.Use(func(next ent.Mutator) ent.Mutator {
					return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
						body, _ := mutation.(*ent.RequestExecutionMutation).ResponseBody()
						if isExternalResponseBodyMarker(body) {
							return nil, errors.New("marker update failed")
						}
						return next.Mutate(ctx, mutation)
					})
				})
			}
			body := []byte(`{"result":"keep me"}`)
			require.NoError(t, svc.UpdateRequestExecutionFinalized(ctx, execution.ID, requestexecution.StatusCompleted, "", "response", body, nil))
			stored, err := client.RequestExecution.Get(ctx, execution.ID)
			require.NoError(t, err)
			require.Equal(t, requestexecution.StatusCompleted, stored.Status)
			require.JSONEq(t, string(body), string(stored.ResponseBody))
			loaded, err := svc.LoadRequestExecutionResponseBody(ctx, stored)
			require.NoError(t, err)
			require.JSONEq(t, string(body), string(loaded))
		})
	}
}

func TestExternalFinalizationRollbackDoesNotPublishFile(t *testing.T) {
	ctx, client, svc, ds, execution := externalFinalizationFixture(t)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := ent.NewContext(ctx, tx.Client())
	require.NoError(t, svc.UpdateRequestExecutionFinalized(txCtx, execution.ID, requestexecution.StatusCompleted, "", "response", []byte(`{"result":"rollback"}`), nil))
	key := GenerateExecutionResponseBodyKey(execution.ProjectID, execution.RequestID, execution.ID)
	_, err = svc.DataStorageService.LoadData(ctx, ds, key)
	require.Error(t, err)
	require.NoError(t, tx.Rollback())
	stored, err := client.RequestExecution.Get(ctx, execution.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusProcessing, stored.Status)
}
