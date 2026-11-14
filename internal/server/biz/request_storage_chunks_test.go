// 职责: 验证真实请求持久化入口使用共享正文且保持旧数据和开关语义。
// 关联文件:
//
//	internal/server/biz/request.go
//	internal/server/biz/request_body_chunks.go
//	internal/server/biz/system.go
//
// 注意事项: 读取必须完整还原原始 JSON，不向调用者泄露内部清单。

package biz

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestServiceChunkedBodies(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	project, err := client.Project.Create().SetName("chunked-bodies").Save(ctx)
	require.NoError(t, err)
	ctx = contexts.WithProjectID(ctx, project.ID)
	key := client.APIKey.Create().SetKey("chunked-session-owner").SetName("owner").SetProjectID(project.ID).SaveX(ctx)
	ctx = contexts.WithAPIKey(ctx, key)
	client.DataStorage.Create().SetName("primary").SetDescription("test storage").SetPrimary(true).SetType("database").
		SetSettings(&objects.DataStorageSettings{}).SaveX(ctx)
	channelRow, err := client.Channel.Create().SetName("chunked-bodies").SetType(channel.TypeOpenai).
		SetBaseURL("https://example.test").SetCredentials(objects.ChannelCredentials{APIKey: "test"}).
		SetSupportedModels([]string{"test"}).SetDefaultTestModel("test").Save(ctx)
	require.NoError(t, err)

	body := []byte(`{ "model": "test", "input": [{"type":"function_call_output","namespace":"agent","call_id":"","output":"` + strings.Repeat("long shared history ", 7000) + `"}], "big":9007199254740993 }`)
	req, err := svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: body}, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.True(t, req.RequestBodyChunked)
	require.Less(t, len(req.RequestBody), 256)
	stored, err := svc.LoadRequestBody(ctx, req)
	require.NoError(t, err)
	require.Equal(t, body, []byte(stored))
	chunkCount, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Positive(t, chunkCount)

	execution, err := svc.CreateRequestExecution(ctx, &Channel{Channel: channelRow}, "test", req,
		httpclient.Request{Body: body}, llm.APIFormatOpenAIResponse, false)
	require.NoError(t, err)
	require.True(t, execution.RequestBodyChunked)
	stored, err = svc.LoadRequestExecutionRequestBody(ctx, execution)
	require.NoError(t, err)
	require.Equal(t, body, []byte(stored))
	after, err := client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, chunkCount, after, "an identical execution reuses all stored content")

	fresh, err := client.Request.Get(ctx, req.ID)
	require.NoError(t, err)
	stored, err = svc.LoadRequestBody(ctx, fresh)
	require.NoError(t, err)
	require.Equal(t, body, []byte(stored))

	response := []byte(`{"id":"resp_chunked","status":"completed","output":[]}`)
	_, err = client.Request.UpdateOneID(req.ID).SetExternalID("resp_chunked").
		SetStatus(request.StatusCompleted).SetResponseBody(response).Save(ctx)
	require.NoError(t, err)
	lookup := contexts.WithAPIKey(ctx, key)
	sessionBody, sessionResponse, found, err := svc.LoadCompletedResponsesSession(lookup, "resp_chunked")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, body, sessionBody)
	require.Equal(t, response, sessionResponse)

	policy := defaultStoragePolicy
	policy.StoreRequestBody = false
	require.NoError(t, svc.SystemService.SetStoragePolicy(ctx, &policy))
	withoutBody, err := svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: body}, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.False(t, withoutBody.RequestBodyChunked)
	require.JSONEq(t, `{}`, string(withoutBody.RequestBody))
	after, err = client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, chunkCount, after)
}

func TestRequestServiceChunkWriteFailureRollsBackOwner(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	project, err := client.Project.Create().SetName("chunked-rollback").Save(ctx)
	require.NoError(t, err)
	ctx = contexts.WithProjectID(ctx, project.ID)
	failure := errors.New("reference write rejected")
	client.RequestBodyChunkRef.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) { return nil, failure })
	})
	body := []byte(`{"input":"` + strings.Repeat("rollback history", 4000) + `"}`)
	_, err = svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: body}, llm.APIFormatOpenAIResponse)
	require.ErrorIs(t, err, failure)
	count, err := client.Request.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = client.RequestBodyChunk.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	owner := client.Request.Create().SetProjectID(project.ID).SetModelID("test").SetRequestBody([]byte(`{}`)).
		SetStatus(request.StatusProcessing).SaveX(ctx)
	channelRow := client.Channel.Create().SetName("execution-rollback").SetType(channel.TypeOpenai).
		SetBaseURL("https://example.test").SetCredentials(objects.ChannelCredentials{APIKey: "test"}).
		SetSupportedModels([]string{"test"}).SetDefaultTestModel("test").SaveX(ctx)
	_, err = svc.CreateRequestExecution(ctx, &Channel{Channel: channelRow}, "test", owner,
		httpclient.Request{JSONBody: body}, llm.APIFormatOpenAIResponse, false)
	require.ErrorIs(t, err, failure)
	require.Zero(t, client.RequestExecution.Query().CountX(ctx))
	require.Zero(t, client.RequestBodyChunk.Query().CountX(ctx))
}

func TestRequestServiceChunkedBodiesWithoutProjectStayInline(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	system := NewSystemService(SystemServiceParams{Ent: client})
	channels := NewChannelServiceForTest(client)
	usage := NewUsageLogService(client, system, channels)
	storage := NewDataStorageService(DataStorageServiceParams{SystemService: system, Client: client})
	svc := NewRequestService(client, system.CacheConfig, system, usage, storage, NewLiveStreamRegistry())
	channelRow := client.Channel.Create().SetName("projectless-test").SetType(channel.TypeOpenai).
		SetBaseURL("https://example.test").SetCredentials(objects.ChannelCredentials{APIKey: "test"}).
		SetSupportedModels([]string{"test"}).SetDefaultTestModel("test").SaveX(ctx)
	body := []byte(`{"model":"test","input":"` + strings.Repeat("channel test prompt ", 3000) + `"}`)

	req, err := svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: body}, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.Zero(t, req.ProjectID)
	require.False(t, req.RequestBodyChunked)
	require.Equal(t, body, []byte(req.RequestBody))

	execution, err := svc.CreateRequestExecution(ctx, &Channel{Channel: channelRow}, "test", req,
		httpclient.Request{Body: body}, llm.APIFormatOpenAIResponse, false)
	require.NoError(t, err)
	require.Zero(t, execution.ProjectID)
	require.False(t, execution.RequestBodyChunked)
	require.Equal(t, body, []byte(execution.RequestBody))
	require.Zero(t, client.RequestBodyChunk.Query().CountX(ctx))
}
