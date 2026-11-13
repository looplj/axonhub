// 职责: 验证请求正文分块保真、跨记录去重、项目隔离和安全回收。
// 关联文件:
//
//	internal/server/biz/request_body_chunks.go
//	internal/ent/schema/request_body_chunk.go
//	internal/ent/schema/request_body_chunk_ref.go
//
// 注意事项: 默认不建外键，回收逻辑不能依赖级联；另外验证启用外键的附加保护。

package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestbodychunk"
	"github.com/looplj/axonhub/internal/ent/requestbodychunkref"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
)

func chunkTestBody() []byte {
	text := make([]byte, 256*1024)
	random := rand.New(rand.NewSource(12))
	for i := range text {
		text[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[random.Intn(36)]
	}
	return append(append([]byte("{\n  \"model\": \"gpt-6\", \"input\": [{\"type\":\"function_call_output\",\"namespace\":\"agent\",\"output\":\""), text...), []byte("\"}], \"integer\":9007199254740993, \"decimal\":1.000000000000000000001\n}\n")...)
}

func newChunkTestClient(t *testing.T) (*ent.Client, context.Context, int) {
	return newChunkTestClientWithForeignKeys(t, false)
}

func newChunkTestClientWithForeignKeys(t *testing.T, foreignKeys bool) (*ent.Client, context.Context, int) {
	t.Helper()
	client := enttest.Open(t, dialect.SQLite, "file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared&_fk=1", enttest.WithMigrateOptions(migrate.WithForeignKeys(foreignKeys)))
	client.Driver().(*entsql.Driver).DB().SetMaxOpenConns(1)
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(t.Context())
	project, err := client.Project.Create().SetName("chunks").Save(ctx)
	require.NoError(t, err)
	return client, ctx, project.ID
}

func saveChunkTestRequest(t *testing.T, client *ent.Client, ctx context.Context, projectID int, body []byte) *ent.Request {
	t.Helper()
	plan := PrepareChunkedRequestBody(body)
	require.NotNil(t, plan)
	var owner *ent.Request
	service := AbstractService{db: client}
	err := service.RunInTransaction(ctx, func(ctx context.Context) error {
		tx := ent.FromContext(ctx)
		var err error
		owner, err = tx.Request.Create().SetProjectID(projectID).SetModelID("model").SetRequestBody(plan.Manifest).
			SetRequestBodyChunked(true).SetStatus(request.StatusProcessing).Save(ctx)
		if err != nil {
			return err
		}
		return plan.Save(ctx, tx, projectID, owner.ID, 0)
	})
	require.NoError(t, err)
	return owner
}

func TestPrepareChunkedRequestBodyPreservesBytesAndLocalChanges(t *testing.T) {
	require.Nil(t, PrepareChunkedRequestBody([]byte(`{"message":"small"}`)))
	require.Nil(t, PrepareChunkedRequestBody(bytes.Repeat([]byte{' '}, requestBodyChunkThreshold-1)))
	boundary := bytes.Repeat([]byte{' '}, requestBodyChunkThreshold)
	require.Equal(t, boundary, bytes.Join(PrepareChunkedRequestBody(boundary).chunks, nil))
	body := chunkTestBody()
	require.True(t, json.Valid(body))
	plan := PrepareChunkedRequestBody(body)
	require.Equal(t, body, bytes.Join(plan.chunks, nil))
	for _, chunk := range plan.chunks {
		require.LessOrEqual(t, len(chunk), requestBodyChunkMax)
	}
	require.Equal(t, plan.Manifest, PrepareChunkedRequestBody(body).Manifest)

	edited := bytes.Clone(body)
	edited = append(append(append([]byte(nil), edited[:80000]...), []byte("one-new-tool-output")...), edited[80000:]...)
	changed := PrepareChunkedRequestBody(edited)
	require.Equal(t, edited, bytes.Join(changed.chunks, nil))
	originalHashes := make(map[string]bool)
	for _, chunk := range plan.chunks {
		originalHashes[requestBodyHash(chunk)] = true
	}
	sharedBytes := 0
	for _, chunk := range changed.chunks {
		if originalHashes[requestBodyHash(chunk)] {
			sharedBytes += len(chunk)
		}
	}
	require.Greater(t, sharedBytes, len(edited)*7/10, "a local insertion must not invalidate the whole history")
}

func TestChunkedRequestBodyRoundTripSharesAndIsolatesProjects(t *testing.T) {
	client, ctx, projectID := newChunkTestClient(t)
	body := chunkTestBody()
	first := saveChunkTestRequest(t, client, ctx, projectID, body)
	count, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	second := saveChunkTestRequest(t, client, ctx, projectID, body)
	var execution *ent.RequestExecution
	service := AbstractService{db: client}
	err = service.RunInTransaction(ctx, func(txCtx context.Context) error {
		plan := PrepareChunkedRequestBody(body)
		tx := ent.FromContext(txCtx)
		var err error
		execution, err = tx.RequestExecution.Create().SetRequestID(first.ID).SetProjectID(projectID).SetModelID("model").
			SetRequestBody(plan.Manifest).SetRequestBodyChunked(true).SetStatus(requestexecution.StatusProcessing).Save(txCtx)
		if err != nil {
			return err
		}
		return plan.Save(txCtx, tx, projectID, 0, execution.ID)
	})
	require.NoError(t, err)
	newCount, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, count, newCount)
	loaded, err := LoadChunkedRequestBody(context.Background(), client, projectID, second.ID, 0, second.RequestBody)
	require.NoError(t, err)
	require.Equal(t, body, []byte(loaded))
	loaded, err = LoadChunkedRequestBody(ctx, client, projectID, 0, execution.ID, execution.RequestBody)
	require.NoError(t, err)
	require.Equal(t, body, []byte(loaded))
	_, err = client.RequestBodyChunk.Query().Count(context.Background())
	require.Error(t, err, "content storage is private, independent of direct client use")

	other, err := client.Project.Create().SetName("other-project").Save(ctx)
	require.NoError(t, err)
	third := saveChunkTestRequest(t, client, ctx, other.ID, body)
	newCount, err = client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, count*2, newCount)
	_, err = LoadChunkedRequestBody(ctx, client, other.ID, first.ID, 0, first.RequestBody)
	require.Error(t, err)
	_, err = LoadChunkedRequestBody(ctx, client, projectID, third.ID, 0, third.RequestBody)
	require.Error(t, err)
	require.Error(t, PrepareChunkedRequestBody(body).Save(ctx, client, projectID, first.ID, 0), "nontransactional writes must be rejected")
}

func TestChunkedRequestBodyDetectsMissingAndCorruptBlocks(t *testing.T) {
	t.Run("missing reference", func(t *testing.T) {
		client, ctx, projectID := newChunkTestClient(t)
		owner := saveChunkTestRequest(t, client, ctx, projectID, chunkTestBody())
		ref, err := client.RequestBodyChunkRef.Query().Where(requestbodychunkref.RequestIDEQ(owner.ID)).First(ctx)
		require.NoError(t, err)
		require.NoError(t, client.RequestBodyChunkRef.DeleteOne(ref).Exec(ctx))
		_, err = LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, owner.RequestBody)
		require.ErrorContains(t, err, "count mismatch")
	})
	t.Run("manifest mismatch", func(t *testing.T) {
		client, ctx, projectID := newChunkTestClient(t)
		owner := saveChunkTestRequest(t, client, ctx, projectID, chunkTestBody())
		var manifest requestBodyManifest
		require.NoError(t, json.Unmarshal(owner.RequestBody, &manifest))
		manifest.SHA256 = strings.Repeat("0", 64)
		wrong, err := json.Marshal(manifest)
		require.NoError(t, err)
		_, err = LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, wrong)
		require.ErrorContains(t, err, "checksum mismatch")
	})
	t.Run("wrong block order", func(t *testing.T) {
		client, ctx, projectID := newChunkTestClient(t)
		owner := saveChunkTestRequest(t, client, ctx, projectID, chunkTestBody())
		refs, err := client.RequestBodyChunkRef.Query().Where(requestbodychunkref.RequestIDEQ(owner.ID)).Order(ent.Asc(requestbodychunkref.FieldPosition)).All(ctx)
		require.NoError(t, err)
		last := refs[len(refs)-1]
		require.NoError(t, client.RequestBodyChunkRef.DeleteOne(last).Exec(ctx))
		_, err = client.RequestBodyChunkRef.Create().SetRequestID(owner.ID).SetChunkID(last.ChunkID).SetPosition(last.Position + 1).Save(ctx)
		require.NoError(t, err)
		_, err = LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, owner.RequestBody)
		require.ErrorContains(t, err, "missing or corrupt")
	})
}

func TestChunkedRequestBodyRollbackAndSharedBlockCleanup(t *testing.T) {
	client, ctx, projectID := newChunkTestClient(t)
	body := chunkTestBody()
	plan := PrepareChunkedRequestBody(body)
	service := AbstractService{db: client}
	err := service.RunInTransaction(ctx, func(ctx context.Context) error {
		tx := ent.FromContext(ctx)
		owner, err := tx.Request.Create().SetProjectID(projectID).SetModelID("model").SetRequestBody(plan.Manifest).
			SetRequestBodyChunked(true).SetStatus(request.StatusProcessing).Save(ctx)
		if err != nil {
			return err
		}
		if err := plan.Save(ctx, tx, projectID, owner.ID, 0); err != nil {
			return err
		}
		return context.Canceled
	})
	require.ErrorIs(t, err, context.Canceled)
	count, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	first := saveChunkTestRequest(t, client, ctx, projectID, body)
	second := saveChunkTestRequest(t, client, ctx, projectID, body)
	require.NoError(t, client.Request.DeleteOne(first).Exec(ctx))
	orphans, err := client.RequestBodyChunkRef.Query().Where(requestbodychunkref.RequestIDEQ(first.ID)).Count(ctx)
	require.NoError(t, err)
	require.Positive(t, orphans, "production does not create cascade foreign keys")
	deleted, err := DeleteUnreferencedRequestBodyChunks(ctx, client, 100)
	require.NoError(t, err)
	require.Zero(t, deleted)
	loaded, err := LoadChunkedRequestBody(ctx, client, projectID, second.ID, 0, second.RequestBody)
	require.NoError(t, err)
	require.Equal(t, body, []byte(loaded))
	require.NoError(t, client.Request.DeleteOne(second).Exec(ctx))
	deleted, err = DeleteUnreferencedRequestBodyChunks(ctx, client, 100)
	require.NoError(t, err)
	require.Positive(t, deleted)
	count, err = client.RequestBodyChunkRef.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestChunkedRequestBodyForeignKeysAddDeletionProtection(t *testing.T) {
	client, ctx, projectID := newChunkTestClientWithForeignKeys(t, true)
	owner := saveChunkTestRequest(t, client, ctx, projectID, chunkTestBody())
	chunk, err := client.RequestBodyChunk.Query().First(ctx)
	require.NoError(t, err)
	require.Error(t, client.RequestBodyChunk.DeleteOne(chunk).Exec(ctx))
	require.NoError(t, client.Request.DeleteOne(owner).Exec(ctx))
	count, err := client.RequestBodyChunkRef.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

type chunkCleanupZeroAffectedDriver struct {
	dialect.Driver

	lockUpdates int
}

func (d *chunkCleanupZeroAffectedDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := d.Driver.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return &chunkCleanupZeroAffectedTx{Tx: tx, driver: d}, nil
}

type chunkCleanupZeroAffectedTx struct {
	dialect.Tx

	driver *chunkCleanupZeroAffectedDriver
}

func (tx *chunkCleanupZeroAffectedTx) Exec(ctx context.Context, query string, args, result any) error {
	err := tx.Tx.Exec(ctx, query, args, result)
	if err == nil && strings.HasPrefix(query, "UPDATE ") && strings.Contains(query, requestbodychunk.Table) {
		tx.driver.lockUpdates++
		r := result.(*entsql.Result)
		*r = zeroAffectedResult{*r}
	}
	return err
}

func TestChunkedRequestBodyCleanupContinuesAfterUnchangedLockUpdate(t *testing.T) {
	client, ctx, projectID := newChunkTestClient(t)
	owner := saveChunkTestRequest(t, client, ctx, projectID, chunkTestBody())
	chunkCount, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Positive(t, chunkCount)
	require.NoError(t, client.Request.DeleteOne(owner).Exec(ctx))

	// MySQL can return zero affected rows for hash=hash while retaining its
	// exclusive row lock. That result must not skip orphan block collection.
	driver := &chunkCleanupZeroAffectedDriver{Driver: client.Driver()}
	cleanupClient := ent.NewClient(ent.Driver(driver))
	deleted, err := DeleteUnreferencedRequestBodyChunks(ctx, cleanupClient, 100)
	require.NoError(t, err)
	require.Equal(t, chunkCount, driver.lockUpdates)
	require.Equal(t, chunkCount, deleted)
	remaining, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, remaining)
}

func TestChunkedRequestBodyConcurrentSaveAndCleanupPreservesLiveOwner(t *testing.T) {
	client, ctx, projectID := newChunkTestClient(t)
	body := chunkTestBody()
	owner := saveChunkTestRequest(t, client, ctx, projectID, body)
	plan := PrepareChunkedRequestBody(body)
	// SQLite serializes writers; all operations still use the real transaction and FK path.
	var wg sync.WaitGroup
	errors := make(chan error, 18)
	reportPanic := func() {
		if recovered := recover(); recovered != nil {
			errors <- fmt.Errorf("chunk operation panic: %v", recovered)
		}
	}
	for range 6 {
		wg.Go(func() {
			defer reportPanic()
			_, err := DeleteUnreferencedRequestBodyChunks(ctx, client, 100)
			errors <- err
		})
		wg.Go(func() {
			defer reportPanic()
			loaded, err := LoadChunkedRequestBody(ctx, client, projectID, owner.ID, 0, owner.RequestBody)
			if err == nil && !bytes.Equal(loaded, body) {
				err = context.Canceled
			}
			errors <- err
		})
		wg.Go(func() {
			defer reportPanic()
			service := AbstractService{db: client}
			errors <- service.RunInTransaction(ctx, func(txCtx context.Context) error {
				tx := ent.FromContext(txCtx)
				owner, err := tx.Request.Create().SetProjectID(projectID).SetModelID("model").SetRequestBody(plan.Manifest).
					SetRequestBodyChunked(true).SetStatus(request.StatusProcessing).Save(txCtx)
				if err != nil {
					return err
				}
				return plan.Save(txCtx, tx, projectID, owner.ID, 0)
			})
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	count, err := client.RequestBodyChunk.Query().Where(requestbodychunk.ProjectIDEQ(projectID)).Count(ctx)
	require.NoError(t, err)
	require.Positive(t, count)
}
