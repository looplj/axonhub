// 职责: 验证分块请求正文的可移植备份、恢复与重复恢复。
// 关联文件:
//
//	internal/server/backup/backup_ops.go
//	internal/server/backup/restore.go
//	internal/server/biz/request_body_chunks.go
//
// 注意事项: 备份展开正文，不导出数据库内部清单或共享块引用。
package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/biz"
)

func chunkedBackupRequest(t *testing.T) (*ent.Client, *BackupService, context.Context, *ent.Request, []byte) {
	t.Helper()
	client, service, ctx := setupBackupTest(t)
	t.Cleanup(func() { _ = client.Close() })
	project := createBackupTestProject(t, client, ctx, "Chunked Project", "backup")
	ch := createBackupTestChannel(t, client, ctx, "Chunked Channel", channel.TypeOpenai)
	user := client.User.Query().OnlyX(ctx)
	key := createBackupTestAPIKey(t, client, ctx, user, project, "chunked key", "sk-chunked-backup-test")
	body := []byte(fmt.Sprintf(`{"input":%q,"counter":9007199254740993,"unknown":{"preserve":true}}`, strings.Repeat("conversation history ", 6000)))
	plan := biz.PrepareChunkedRequestBody(body)
	require.NotNil(t, plan)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	req, err := tx.Client().Request.Create().SetProjectID(project.ID).SetAPIKeyID(key.ID).SetChannelID(ch.ID).
		SetModelID("gpt-4").SetRequestBody(plan.Manifest).SetRequestBodyChunked(true).SetStatus(request.StatusCompleted).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, plan.Save(ctx, tx.Client(), project.ID, req.ID, 0))
	require.NoError(t, tx.Commit())
	req = client.Request.GetX(ctx, req.ID)
	_, err = client.UsageLog.Create().SetRequestID(req.ID).SetProjectID(project.ID).SetAPIKeyID(key.ID).SetChannelID(ch.ID).
		SetModelID("gpt-4").SetPromptTokens(100).SetCompletionTokens(20).SetTotalTokens(120).
		SetSource("api").SetFormat("openai/chat_completions").Save(ctx)
	require.NoError(t, err)
	return client, service, ctx, req, body
}

func TestBackupChunkedRequestBodiesRoundTrip(t *testing.T) {
	client, service, ctx, req, body := chunkedBackupRequest(t)
	data, err := service.Backup(ctx, BackupOptions{IncludeAPIKeys: true, IncludeRequestLogs: true, IncludeUsageStats: true})
	require.NoError(t, err)
	require.NotContains(t, string(data), "request_body_chunked")
	var backup BackupData
	require.NoError(t, json.Unmarshal(data, &backup))
	require.Equal(t, BackupVersion, backup.Version)
	require.Len(t, backup.UsageRequests, 1)
	var compact bytes.Buffer
	require.NoError(t, json.Compact(&compact, body))
	require.Equal(t, compact.String(), string(backup.UsageRequests[0].RequestBody))
	chunkCount := client.RequestBodyChunk.Query().CountX(ctx)
	referenceCount := client.RequestBodyChunkRef.Query().CountX(ctx)
	require.Positive(t, chunkCount)
	require.Positive(t, referenceCount)

	options := RestoreOptions{IncludeRequestLogs: true, IncludeUsageStats: true}
	require.NoError(t, service.Restore(ctx, bytes.NewReader(data), options))
	require.Equal(t, 1, client.Request.Query().CountX(ctx))
	require.Equal(t, referenceCount, client.RequestBodyChunkRef.Query().CountX(ctx))

	_, err = client.UsageLog.Delete().Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Request.DeleteOneID(req.ID).Exec(ctx))
	require.NoError(t, service.Restore(ctx, bytes.NewReader(data), options))
	restored := client.Request.Query().OnlyX(ctx)
	require.True(t, restored.RequestBodyChunked)
	loaded, err := biz.LoadChunkedRequestBody(ctx, client, restored.ProjectID, restored.ID, 0, restored.RequestBody)
	require.NoError(t, err)
	require.Equal(t, compact.Bytes(), []byte(loaded))
	require.Equal(t, chunkCount, client.RequestBodyChunk.Query().CountX(ctx))
	restoredReferences := client.RequestBodyChunkRef.Query().CountX(ctx)
	require.NoError(t, service.Restore(ctx, bytes.NewReader(data), options))
	require.Equal(t, 1, client.Request.Query().CountX(ctx))
	require.Equal(t, 1, client.UsageLog.Query().CountX(ctx))
	require.Equal(t, restoredReferences, client.RequestBodyChunkRef.Query().CountX(ctx))
}

func TestBackupChunkedRequestBodiesRejectsMissingChunk(t *testing.T) {
	client, service, ctx, req, _ := chunkedBackupRequest(t)
	ref := client.RequestBodyChunkRef.Query().FirstX(ctx)
	require.NoError(t, client.RequestBodyChunkRef.DeleteOneID(ref.ID).Exec(ctx))
	_, err := service.Backup(ctx, BackupOptions{IncludeRequestLogs: true})
	require.ErrorContains(t, err, fmt.Sprintf("hydrate request body %d", req.ID))
	_, err = service.Backup(ctx, BackupOptions{IncludeUsageStats: true})
	require.NoError(t, err, "statistics-only backups must not hydrate request bodies")
}
