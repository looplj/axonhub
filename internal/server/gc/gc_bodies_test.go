package gc

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCleanupRequestBodies_StripsOldPayloadsAndKeepsRows(t *testing.T) {
	worker, ctx, client, proj := setupBodyCleanupWorker(t)

	oldAt := time.Now().AddDate(0, 0, -10)
	recentAt := time.Now().Add(-time.Hour)

	oldReq := createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{"prompt":"old"}`), objects.JSONRawMessage(`{"authorization":"sk"}`))
	recentReq := createBodyTestRequest(t, ctx, client, proj.ID, recentAt, objects.JSONRawMessage(`{"prompt":"new"}`), objects.JSONRawMessage(`{"authorization":"sk"}`))
	placeholderReq := createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{}`), objects.JSONRawMessage(`{}`))

	oldExec := createBodyTestExecution(t, ctx, client, oldReq, oldAt, objects.JSONRawMessage(`{"upstream":true}`), objects.JSONRawMessage(`{"x-api-key":"k"}`))

	require.NoError(t, worker.cleanupRequestBodies(ctx, 7))

	oldReq = client.Request.GetX(ctx, oldReq.ID)
	require.JSONEq(t, `{}`, string(oldReq.RequestBody))
	require.Empty(t, oldReq.RequestHeaders)
	require.Equal(t, request.StatusCompleted, oldReq.Status)

	recentReq = client.Request.GetX(ctx, recentReq.ID)
	require.JSONEq(t, `{"prompt":"new"}`, string(recentReq.RequestBody))
	require.JSONEq(t, `{"authorization":"sk"}`, string(recentReq.RequestHeaders))

	placeholderReq = client.Request.GetX(ctx, placeholderReq.ID)
	require.JSONEq(t, `{}`, string(placeholderReq.RequestBody))

	oldExec = client.RequestExecution.GetX(ctx, oldExec.ID)
	require.JSONEq(t, `{}`, string(oldExec.RequestBody))
	require.Empty(t, oldExec.RequestHeaders)

	count, err := client.Request.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func TestCleanupRequestBodies_SkipsRetainedTraces(t *testing.T) {
	worker, ctx, client, proj := setupBodyCleanupWorker(t)

	oldAt := time.Now().AddDate(0, 0, -10)
	tr, err := client.Trace.Create().
		SetProjectID(proj.ID).
		SetTraceID("retained-trace").
		SetStatus(trace.StatusRetained).
		Save(ctx)
	require.NoError(t, err)

	retained, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{"keep":true}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{"h":"1"}`)).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, worker.cleanupRequestBodies(ctx, 7))

	retained = client.Request.GetX(ctx, retained.ID)
	require.JSONEq(t, `{"keep":true}`, string(retained.RequestBody))
	require.JSONEq(t, `{"h":"1"}`, string(retained.RequestHeaders))
}

func TestCleanupResponseBodies_DoesNotTouchRequestBodies(t *testing.T) {
	worker, ctx, client, proj := setupBodyCleanupWorker(t)

	oldAt := time.Now().AddDate(0, 0, -10)
	req := createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{"prompt":"keep"}`), objects.JSONRawMessage(`{"h":"1"}`))
	_, err := client.Request.UpdateOneID(req.ID).
		SetResponseBody(objects.JSONRawMessage(`{"text":"drop"}`)).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, worker.cleanupResponseBodies(ctx, 7))

	req = client.Request.GetX(ctx, req.ID)
	require.JSONEq(t, `{"prompt":"keep"}`, string(req.RequestBody))
	require.JSONEq(t, `{"h":"1"}`, string(req.RequestHeaders))
	require.JSONEq(t, `{}`, string(req.ResponseBody))
}

func TestCleanupResponseChunks_StripsOldChunks(t *testing.T) {
	worker, ctx, client, proj := setupBodyCleanupWorker(t)

	oldAt := time.Now().AddDate(0, 0, -10)
	req := createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{}`), objects.JSONRawMessage(`{}`))
	_, err := client.Request.UpdateOneID(req.ID).
		SetResponseChunks([]objects.JSONRawMessage{objects.JSONRawMessage(`{"delta":"x"}`)}).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, worker.cleanupResponseChunks(ctx, 3))

	req = client.Request.GetX(ctx, req.ID)
	require.Empty(t, req.ResponseChunks)
}

func TestCleanupRequestBodies_DeletesExternalFilesOnly(t *testing.T) {
	worker, ctx, fsStorage, baseDir := setupWorkerWithFSStorage(t)
	client := worker.Ent
	reqSvc := newBodyTestRequestService(worker)

	proj, err := client.Project.Create().
		SetName("body-cleanup-fs").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	oldAt := time.Now().AddDate(0, 0, -10)
	// 生产外存形态：DB 占位 {} + headers 仍在 + 正文只在对象存储。
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{"h":"1"}`)).
		Save(ctx)
	require.NoError(t, err)

	bodyKey := biz.GenerateRequestBodyKey(req.ProjectID, req.ID)
	audioKey := biz.GenerateAudioKey(req.ProjectID, req.ID, "audio.mp3")
	require.NoError(t, os.MkdirAll(filepath.Dir(pathForKey(baseDir, bodyKey)), 0o755))
	require.NoError(t, os.WriteFile(pathForKey(baseDir, bodyKey), []byte(`{"prompt":"s3"}`), 0o644))
	createFileForKey(t, baseDir, audioKey)

	loaded, err := reqSvc.LoadRequestBody(ctx, req)
	require.NoError(t, err)
	require.JSONEq(t, `{"prompt":"s3"}`, string(loaded))

	require.NoError(t, worker.cleanupRequestBodies(ctx, 7))

	assertRemoved(t, baseDir, bodyKey)
	_, err = os.Stat(pathForKey(baseDir, audioKey))
	require.NoError(t, err, "generated audio must stay")

	req = client.Request.GetX(ctx, req.ID)
	require.JSONEq(t, `{}`, string(req.RequestBody))
	require.Empty(t, req.RequestHeaders)

	loaded, err = reqSvc.LoadRequestBody(ctx, req)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(loaded))

	item, err := worker.previewBodyCleanup(ctx, biz.CleanupResourceRequestBodies, 7, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, item.EstimatedCount)
}

func TestPreviewBodyCleanup_CountsWindowRowsWithoutReadingPayloads(t *testing.T) {
	worker, ctx, client, proj := setupBodyCleanupWorker(t)

	oldAt := time.Now().AddDate(0, 0, -10)
	createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{"prompt":"old"}`), objects.JSONRawMessage(`{}`))
	createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{}`), objects.JSONRawMessage(`{}`))

	item, err := worker.previewBodyCleanup(ctx, biz.CleanupResourceRequestBodies, 7, time.Now())
	require.NoError(t, err)
	require.Equal(t, 2, item.EstimatedCount)
	require.Equal(t, biz.CleanupResourceRequestBodies, item.ResourceType)
}

func setupBodyCleanupWorker(t *testing.T) (*Worker, context.Context, *ent.Client, *ent.Project) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() {
		client.Close()
	})

	ctx := authz.WithTestBypass(context.Background())
	ctx = ent.NewContext(ctx, client)
	ctx = schematype.SkipSoftDelete(ctx)

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheConfig})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Client:        client,
	})

	proj, err := client.Project.Create().
		SetName("body-cleanup").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.DataStorage.Create().
		SetName("primary").
		SetDescription("primary database").
		SetPrimary(true).
		SetType(datastorage.TypeDatabase).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	worker := &Worker{
		SystemService:      systemService,
		DataStorageService: dataStorageService,
		Ent:                client,
	}

	return worker, ctx, client, proj
}

func createBodyTestRequest(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	projectID int,
	createdAt time.Time,
	body, headers objects.JSONRawMessage,
) *ent.Request {
	t.Helper()

	req, err := client.Request.Create().
		SetProjectID(projectID).
		SetCreatedAt(createdAt).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(body).
		SetRequestHeaders(headers).
		Save(ctx)
	require.NoError(t, err)

	return req
}

func createBodyTestExecution(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	req *ent.Request,
	createdAt time.Time,
	body, headers objects.JSONRawMessage,
) *ent.RequestExecution {
	t.Helper()

	exec, err := client.RequestExecution.Create().
		SetRequestID(req.ID).
		SetProjectID(req.ProjectID).
		SetCreatedAt(createdAt).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetStatus(requestexecution.StatusCompleted).
		SetStream(false).
		SetRequestBody(body).
		SetRequestHeaders(headers).
		Save(ctx)
	require.NoError(t, err)

	return exec
}

func TestCleanupRequestBodies_ZeroDaysIsNoop(t *testing.T) {
	worker := &Worker{Ent: nil}
	require.NoError(t, worker.cleanupRequestBodies(context.Background(), 0))
	require.NoError(t, worker.cleanupResponseBodies(context.Background(), -1))
	require.NoError(t, worker.cleanupResponseChunks(context.Background(), 0))
}

func TestImmutableJSONPlaceholder_PostgresUsesJSONBLiteral(t *testing.T) {
	pg := entsql.Update("requests")
	pg.SetDialect(dialect.Postgres)
	setImmutableJSONObject(pg, "request_body")
	query, args := pg.Query()
	require.Contains(t, query, "'{}'::jsonb")
	require.Empty(t, args)

	lite := entsql.Update("requests")
	lite.SetDialect(dialect.SQLite)
	setImmutableJSONObject(lite, "request_body")
	query, args = lite.Query()
	require.Contains(t, query, "'{}'")
	require.NotContains(t, query, "::jsonb")
	require.Empty(t, args)
}

func TestCleanupResponseBodies_DoesNotTreatExternalNullAsPayload(t *testing.T) {
	worker, ctx, fsStorage, _ := setupWorkerWithFSStorage(t)
	client := worker.Ent

	proj, err := client.Project.Create().
		SetName("null-not-payload").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	oldAt := time.Now().AddDate(0, 0, -10)
	_, err = client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	item, err := worker.previewBodyCleanup(ctx, biz.CleanupResourceResponseBodies, 7, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, item.EstimatedCount)

	item, err = worker.previewBodyCleanup(ctx, biz.CleanupResourceResponseChunks, 7, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, item.EstimatedCount)

	require.NoError(t, worker.cleanupResponseBodies(ctx, 7))
	req := client.Request.Query().OnlyX(ctx)
	require.JSONEq(t, `{}`, string(req.ResponseBody))
}

func TestCleanupResponseBodies_StripsExternalMarkerAndHistoricalNull(t *testing.T) {
	worker, ctx, fsStorage, baseDir := setupWorkerWithFSStorage(t)
	client := worker.Ent

	proj, err := client.Project.Create().
		SetName("ext-response").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	oldAt := time.Now().AddDate(0, 0, -10)
	marked, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetResponseBody(biz.ExternalResponseBodyMarker).
		Save(ctx)
	require.NoError(t, err)

	historical, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	markedKey := biz.GenerateResponseBodyKey(proj.ID, marked.ID)
	histKey := biz.GenerateResponseBodyKey(proj.ID, historical.ID)
	createFileForKey(t, baseDir, markedKey)
	createFileForKey(t, baseDir, histKey)

	require.NoError(t, worker.cleanupResponseBodies(ctx, 7))

	assertRemoved(t, baseDir, markedKey)
	assertRemoved(t, baseDir, histKey)

	marked = client.Request.GetX(ctx, marked.ID)
	historical = client.Request.GetX(ctx, historical.ID)
	require.JSONEq(t, `{}`, string(marked.ResponseBody))
	require.JSONEq(t, `{}`, string(historical.ResponseBody))
}

func TestPreviewCleanup_SameDaysShareOneWindowCount(t *testing.T) {
	worker, ctx, client, proj := setupBodyCleanupWorker(t)

	oldAt := time.Now().AddDate(0, 0, -10)
	createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{"prompt":"old"}`), objects.JSONRawMessage(`{}`))

	items, err := worker.PreviewCleanup(ctx, TriggerGcCleanupInput{
		RequestBodiesCleanupDays:  7,
		ResponseBodiesCleanupDays: 7,
		ResponseChunksCleanupDays: 7,
	})
	require.NoError(t, err)
	require.Len(t, items, 3)

	require.Equal(t, items[0].EstimatedCount, items[1].EstimatedCount)
	require.Equal(t, items[0].EstimatedCount, items[2].EstimatedCount)
	require.Equal(t, items[0].CutoffTime, items[1].CutoffTime)
	require.Equal(t, items[0].CutoffTime, items[2].CutoffTime)
	require.Equal(t, biz.CleanupResourceRequestBodies, items[0].ResourceType)
	require.Equal(t, biz.CleanupResourceResponseBodies, items[1].ResourceType)
	require.Equal(t, biz.CleanupResourceResponseChunks, items[2].ResourceType)
}

func TestCleanupRequestBodies_ExternalDeleteFailureLeavesRowRetryable(t *testing.T) {
	worker, ctx, fsStorage, baseDir := setupWorkerWithFSStorage(t)
	client := worker.Ent

	proj, err := client.Project.Create().
		SetName("delete-fail").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	oldAt := time.Now().AddDate(0, 0, -10)
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{"h":"1"}`)).
		Save(ctx)
	require.NoError(t, err)

	bodyKey := biz.GenerateRequestBodyKey(req.ProjectID, req.ID)
	createDirForKey(t, baseDir, bodyKey)
	require.NoError(t, os.WriteFile(filepath.Join(pathForKey(baseDir, bodyKey), "child"), []byte("x"), 0o644))

	err = worker.cleanupRequestBodies(ctx, 7)
	require.Error(t, err)

	req = client.Request.GetX(ctx, req.ID)
	require.JSONEq(t, `{}`, string(req.RequestBody))
	require.JSONEq(t, `{"h":"1"}`, string(req.RequestHeaders))

	item, err := worker.previewBodyCleanup(ctx, biz.CleanupResourceRequestBodies, 7, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, item.EstimatedCount)
}

func TestCleanupRequestBodies_SkipsFailedDeleteAndContinues(t *testing.T) {
	worker, ctx, fsStorage, baseDir := setupWorkerWithFSStorage(t)
	client := worker.Ent

	proj, err := client.Project.Create().
		SetName("skip-and-continue").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	oldAt := time.Now().AddDate(0, 0, -10)
	blocked, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{"h":"1"}`)).
		Save(ctx)
	require.NoError(t, err)

	okReq, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetCreatedAt(oldAt).
		SetDataStorageID(fsStorage.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetClientIP("127.0.0.1").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{"h":"2"}`)).
		Save(ctx)
	require.NoError(t, err)

	blockedKey := biz.GenerateRequestBodyKey(proj.ID, blocked.ID)
	okKey := biz.GenerateRequestBodyKey(proj.ID, okReq.ID)
	createDirForKey(t, baseDir, blockedKey)
	require.NoError(t, os.WriteFile(filepath.Join(pathForKey(baseDir, blockedKey), "child"), []byte("x"), 0o644))
	createFileForKey(t, baseDir, okKey)

	err = worker.cleanupRequestBodies(ctx, 7)
	require.Error(t, err)

	blocked = client.Request.GetX(ctx, blocked.ID)
	okReq = client.Request.GetX(ctx, okReq.ID)
	require.JSONEq(t, `{"h":"1"}`, string(blocked.RequestHeaders))
	require.Nil(t, okReq.RequestHeaders)
	assertRemoved(t, baseDir, okKey)
}

func TestCleanupRequestBodies_PostgresImmutableJSON(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PG_DSN not set; skipping real-Postgres request_body strip")
	}

	client := newPostgresBodyCleanupClient(t, dsn)
	t.Cleanup(func() { _ = client.Close() })

	ctx := authz.WithTestBypass(context.Background())
	ctx = ent.NewContext(ctx, client)
	ctx = schematype.SkipSoftDelete(ctx)

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheConfig})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Client:        client,
	})

	proj, err := client.Project.Create().
		SetName("pg-body-cleanup").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.DataStorage.Create().
		SetName("primary").
		SetDescription("primary database").
		SetPrimary(true).
		SetType(datastorage.TypeDatabase).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	worker := &Worker{
		SystemService:      systemService,
		DataStorageService: dataStorageService,
		Ent:                client,
	}

	oldAt := time.Now().AddDate(0, 0, -10)
	req := createBodyTestRequest(t, ctx, client, proj.ID, oldAt, objects.JSONRawMessage(`{"prompt":"pg"}`), objects.JSONRawMessage(`{"h":"1"}`))

	require.NoError(t, worker.cleanupRequestBodies(ctx, 7))

	req = client.Request.GetX(ctx, req.ID)
	require.JSONEq(t, `{}`, string(req.RequestBody))
	require.Empty(t, req.RequestHeaders)

	item, err := worker.previewBodyCleanup(ctx, biz.CleanupResourceRequestBodies, 7, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, item.EstimatedCount)
}

func newBodyTestRequestService(worker *Worker) *biz.RequestService {
	return biz.NewRequestService(
		worker.Ent,
		xcache.Config{Mode: xcache.ModeMemory},
		worker.SystemService,
		nil,
		worker.DataStorageService,
		nil,
	)
}

func newPostgresBodyCleanupClient(t *testing.T, dsn string) *ent.Client {
	t.Helper()

	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	require.NoError(t, sqlDB.PingContext(context.Background()))
	t.Cleanup(func() { _ = sqlDB.Close() })

	return enttest.NewClient(t, enttest.WithOptions(ent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB))))
}
