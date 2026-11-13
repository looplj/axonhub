// 职责: 验证请求正文共享块的清理、保留保护和事务边界。
// 关联文件:
//
//	internal/server/gc/gc.go
//	internal/server/gc/gc_bodies.go
//	internal/server/biz/request_body_chunks.go
//
// 注意事项: 与生产一样不创建外键，回收不能依赖数据库级联删除。
package gc

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
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestbodychunkref"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCleanupRequestBodies_ChunkReferencesAreIndependent(t *testing.T) {
	worker, ctx, client, proj := setupChunkBodyCleanupWorker(t)
	body := chunkGCBody("shared-history")
	oldAt := time.Now().AddDate(0, 0, -10)
	oldReq := createChunkGCRequest(t, worker, ctx, proj.ID, 0, oldAt, body)
	oldExec := createChunkGCExecution(t, worker, ctx, oldReq, oldAt, body)
	recentReq := createChunkGCRequest(t, worker, ctx, proj.ID, 0, time.Now(), body)
	before := client.RequestBodyChunk.Query().CountX(ctx)
	require.Positive(t, before)

	require.NoError(t, worker.cleanupRequestBodies(ctx, 7))
	require.NoError(t, worker.cleanupRequestBodyChunks(ctx))
	oldReq = client.Request.GetX(ctx, oldReq.ID)
	oldExec = client.RequestExecution.GetX(ctx, oldExec.ID)
	require.False(t, oldReq.RequestBodyChunked)
	require.False(t, oldExec.RequestBodyChunked)
	require.JSONEq(t, `{}`, string(oldReq.RequestBody))
	require.JSONEq(t, `{}`, string(oldExec.RequestBody))
	require.Equal(t, before, client.RequestBodyChunk.Query().CountX(ctx))
	loaded, err := newBodyTestRequestService(worker).LoadRequestBody(ctx, client.Request.GetX(ctx, recentReq.ID))
	require.NoError(t, err)
	require.Equal(t, []byte(body), []byte(loaded))

	require.NoError(t, worker.clearRequestPayloadColumns(ctx, bodyPayloadRequest, []int{recentReq.ID}))
	require.NoError(t, worker.cleanupRequestBodyChunks(ctx))
	require.Zero(t, client.RequestBodyChunkRef.Query().CountX(ctx))
	require.Zero(t, client.RequestBodyChunk.Query().CountX(ctx))
}

func TestCleanupRequests_CollectsOrphanChunkRefsWithoutForeignKeys(t *testing.T) {
	worker, ctx, client, proj := setupChunkBodyCleanupWorker(t)
	oldAt := time.Now().AddDate(0, 0, -10)
	oldBody := chunkGCBody("shared-history")
	oldReq := createChunkGCRequest(t, worker, ctx, proj.ID, 0, oldAt, oldBody)
	oldExec := createChunkGCExecution(t, worker, ctx, oldReq, oldAt, oldBody)
	recentBody := oldBody
	recentReq := createChunkGCRequest(t, worker, ctx, proj.ID, 0, time.Now(), recentBody)
	before := client.RequestBodyChunk.Query().CountX(ctx)

	require.NoError(t, worker.cleanupRequests(ctx, 7, false))
	_, err := client.Request.Get(ctx, oldReq.ID)
	require.True(t, ent.IsNotFound(err))
	_, err = client.RequestExecution.Get(ctx, oldExec.ID)
	require.True(t, ent.IsNotFound(err))
	orphans := requestbodychunkref.Or(requestbodychunkref.RequestIDEQ(oldReq.ID), requestbodychunkref.ExecutionIDEQ(oldExec.ID))
	require.Positive(t, client.RequestBodyChunkRef.Query().Where(orphans).CountX(ctx), "owner deletion has no FK cascade")
	require.NoError(t, worker.cleanupRequestBodyChunks(ctx))
	require.Zero(t, client.RequestBodyChunkRef.Query().Where(orphans).CountX(ctx))
	require.Equal(t, before, client.RequestBodyChunk.Query().CountX(ctx), "the remaining request still references every block")
	loaded, err := newBodyTestRequestService(worker).LoadRequestBody(ctx, client.Request.GetX(ctx, recentReq.ID))
	require.NoError(t, err)
	require.Equal(t, []byte(recentBody), []byte(loaded))

	require.NoError(t, client.Request.DeleteOneID(recentReq.ID).Exec(ctx))
	require.Positive(t, client.RequestBodyChunkRef.Query().CountX(ctx))
	require.NoError(t, worker.cleanupRequestBodyChunks(ctx))
	require.Zero(t, client.RequestBodyChunkRef.Query().CountX(ctx))
	require.Zero(t, client.RequestBodyChunk.Query().CountX(ctx))
}

func TestCleanupBodyChunks_RetainedTraceProtectsReferences(t *testing.T) {
	worker, ctx, client, proj := setupChunkBodyCleanupWorker(t)
	tr := client.Trace.Create().SetProjectID(proj.ID).SetTraceID("chunk-retention").SetStatus(trace.StatusRetained).SaveX(ctx)
	body := chunkGCBody("retained")
	oldAt := time.Now().AddDate(0, 0, -10)
	req := createChunkGCRequest(t, worker, ctx, proj.ID, tr.ID, oldAt, body)
	exec := createChunkGCExecution(t, worker, ctx, req, oldAt, body)
	refs := client.RequestBodyChunkRef.Query().CountX(ctx)

	require.NoError(t, worker.cleanupRequestBodies(ctx, 7))
	require.NoError(t, worker.cleanupRequests(ctx, 7, false))
	// The final clear operation must also honor current retention.
	require.NoError(t, worker.clearRequestPayloadColumns(ctx, bodyPayloadRequest, []int{req.ID}))
	require.NoError(t, worker.clearExecutionPayloadColumns(ctx, bodyPayloadRequest, []int{exec.ID}))
	require.NoError(t, worker.cleanupRequestBodyChunks(ctx))
	require.Equal(t, refs, client.RequestBodyChunkRef.Query().CountX(ctx))
	require.True(t, client.Request.GetX(ctx, req.ID).RequestBodyChunked)
	require.True(t, client.RequestExecution.GetX(ctx, exec.ID).RequestBodyChunked)
	loaded, err := newBodyTestRequestService(worker).LoadRequestBody(ctx, client.Request.GetX(ctx, req.ID))
	require.NoError(t, err)
	require.Equal(t, []byte(body), []byte(loaded))
}

func TestCleanupRequestBodies_ReferenceFailureRollsBackClear(t *testing.T) {
	for _, owner := range []string{"request", "execution"} {
		t.Run(owner, func(t *testing.T) {
			worker, ctx, client, proj := setupChunkBodyCleanupWorker(t)
			body := chunkGCBody("atomic-clear")
			req := createChunkGCRequest(t, worker, ctx, proj.ID, 0, time.Now().AddDate(0, 0, -10), body)
			exec := createChunkGCExecution(t, worker, ctx, req, req.CreatedAt, body)
			refs := client.RequestBodyChunkRef.Query().CountX(ctx)
			client.RequestBodyChunkRef.Use(func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					if m.Op().Is(ent.OpDelete | ent.OpDeleteOne) {
						return nil, errors.New("reference delete failed")
					}
					return next.Mutate(ctx, m)
				})
			})

			var err error
			if owner == "request" {
				err = worker.clearRequestPayloadColumns(ctx, bodyPayloadRequest, []int{req.ID})
			} else {
				err = worker.clearExecutionPayloadColumns(ctx, bodyPayloadRequest, []int{exec.ID})
			}
			require.ErrorContains(t, err, "reference delete failed")
			req = client.Request.GetX(ctx, req.ID)
			exec = client.RequestExecution.GetX(ctx, exec.ID)
			require.True(t, req.RequestBodyChunked)
			require.True(t, exec.RequestBodyChunked)
			require.Equal(t, refs, client.RequestBodyChunkRef.Query().CountX(ctx))
			loaded, err := newBodyTestRequestService(worker).LoadRequestBody(ctx, req)
			require.NoError(t, err)
			require.Equal(t, []byte(body), []byte(loaded))
			loaded, err = newBodyTestRequestService(worker).LoadRequestExecutionRequestBody(ctx, exec)
			require.NoError(t, err)
			require.Equal(t, []byte(body), []byte(loaded))
		})
	}
}

func chunkGCBody(text string) objects.JSONRawMessage {
	return objects.JSONRawMessage(`{"input":[{"type":"function_call_output","call_id":"call-1","output":"` + strings.Repeat(text, 6000) + `"}]}`)
}

func setupChunkBodyCleanupWorker(t *testing.T) (*Worker, context.Context, *ent.Client, *ent.Project) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared&_fk=0",
		enttest.WithMigrateOptions(migrate.WithGlobalUniqueID(false), migrate.WithForeignKeys(false)))
	t.Cleanup(func() { _ = client.Close() })
	ctx := schematype.SkipSoftDelete(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	cache := xcache.Config{Mode: xcache.ModeMemory}
	systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cache})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		SystemService: systemService, CacheConfig: cache, Client: client,
	})
	proj := client.Project.Create().SetName("chunk-body-cleanup").SaveX(ctx)
	client.DataStorage.Create().SetName("primary").SetDescription("primary database").
		SetPrimary(true).SetType(datastorage.TypeDatabase).SetSettings(&objects.DataStorageSettings{}).SaveX(ctx)
	worker := &Worker{SystemService: systemService, DataStorageService: dataStorageService, Ent: client}
	return worker, ctx, client, proj
}

func createChunkGCRequest(t *testing.T, worker *Worker, ctx context.Context, projectID, traceID int, createdAt time.Time, body objects.JSONRawMessage) *ent.Request {
	t.Helper()
	plan := biz.PrepareChunkedRequestBody(body)
	require.NotNil(t, plan)
	var req *ent.Request
	require.NoError(t, worker.SystemService.RunInTransaction(ctx, func(txCtx context.Context) error {
		client := ent.FromContext(txCtx)
		builder := client.Request.Create().SetProjectID(projectID).SetCreatedAt(createdAt).
			SetModelID("model").SetStatus(request.StatusCompleted).
			SetRequestBody(plan.Manifest).SetRequestBodyChunked(true)
		if traceID != 0 {
			builder.SetTraceID(traceID)
		}
		var err error
		req, err = builder.Save(txCtx)
		if err != nil {
			return err
		}
		return plan.Save(txCtx, client, projectID, req.ID, 0)
	}))
	return worker.Ent.Request.GetX(ctx, req.ID)
}

func createChunkGCExecution(t *testing.T, worker *Worker, ctx context.Context, req *ent.Request, createdAt time.Time, body objects.JSONRawMessage) *ent.RequestExecution {
	t.Helper()
	plan := biz.PrepareChunkedRequestBody(body)
	require.NotNil(t, plan)
	var execution *ent.RequestExecution
	require.NoError(t, worker.SystemService.RunInTransaction(ctx, func(txCtx context.Context) error {
		client := ent.FromContext(txCtx)
		var err error
		execution, err = client.RequestExecution.Create().SetProjectID(req.ProjectID).SetRequestID(req.ID).
			SetCreatedAt(createdAt).SetModelID("model").SetStatus(requestexecution.StatusCompleted).
			SetRequestBody(plan.Manifest).SetRequestBodyChunked(true).Save(txCtx)
		if err != nil {
			return err
		}
		return plan.Save(txCtx, client, req.ProjectID, 0, execution.ID)
	}))
	return worker.Ent.RequestExecution.GetX(ctx, execution.ID)
}
