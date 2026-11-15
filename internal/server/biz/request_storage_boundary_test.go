// 职责: 校验请求与响应正文在 CDC 归档、内联 jsonb 和外置存储三条路径上的字节边界。
// 关联文件:
//
//	internal/server/biz/request.go
//	internal/server/biz/request_body_chunks.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	CDC 分块归档必须与热会话字节一致，surrogate 不能因 jsonb 修复被改写。
//	外置 chunks 保持 marshal 原始字节；只有 DB 的 jsonb 列需要净化。
//	只使用 SQLite 与内存 fs，不连接外部服务。

package biz

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestExternalResponseChunksPreservePayload 验证外置 chunks 等于
// marshalStreamEventForStorage 的原始结果，不被 jsonb 净化改写。
func TestExternalResponseChunksPreservePayload(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	policy := defaultStoragePolicy
	policy.StoreChunks = true
	require.NoError(t, svc.SystemService.SetStoragePolicy(ctx, &policy))

	ds := client.DataStorage.Create().
		SetName("external-memory").
		SetDescription("storage-boundary").
		SetPrimary(false).
		SetType(datastorage.TypeFs).
		SetSettings(&objects.DataStorageSettings{}).
		SetStatus(datastorage.StatusActive).
		SaveX(ctx)

	// 注入内存文件系统，避免触碰真实目录。
	svc.DataStorageService.fsCache[ds.ID] = afero.NewMemMapFs()

	for _, payload := range []string{`{"delta":"a\u0000b"}`, `{"delta":"\ud800"}`} {
		chunk := &httpclient.StreamEvent{Type: "message", Data: []byte(payload)}
		before := bytes.Clone(chunk.Data)

		raw, err := marshalStreamEventForStorage(chunk)
		require.NoError(t, err)

		expected, err := json.Marshal([]objects.JSONRawMessage{raw})
		require.NoError(t, err)

		markers, key, err := svc.prepareResponseChunks(ctx, ds, "/storage-boundary/chunks.json", []*httpclient.StreamEvent{chunk})
		require.NoError(t, err)
		require.Equal(t, ExternalResponseChunksMarker, markers)

		got, err := svc.DataStorageService.LoadData(ctx, ds, key)
		require.NoError(t, err)
		require.Equal(t, before, chunk.Data, "source chunk must stay immutable")
		require.Equal(t, expected, got, "external chunk payload must keep the marshaled bytes")
	}
}

// TestChunkedRequestKeepsSurrogateInlineReplaces 验证 CDC 分块路径保留
// surrogate（只去 NUL），内联 jsonb 路径才替换为 U+FFFD。
func TestChunkedRequestKeepsSurrogateInlineReplaces(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	project := client.Project.Create().SetName("storage-boundary").SaveX(ctx)
	ctx = contexts.WithProjectID(ctx, project.ID)

	client.DataStorage.Create().
		SetName("primary").
		SetDescription("storage-boundary").
		SetPrimary(true).
		SetType("database").
		SetSettings(&objects.DataStorageSettings{}).
		SaveX(ctx)

	row := client.Channel.Create().
		SetName("storage-boundary").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://example.test").
		SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).
		SetSupportedModels([]string{"test"}).
		SetDefaultTestModel("test").
		SaveX(ctx)

	large := []byte(`{"model":"test","input":[{"role":"user","content":"` +
		strings.Repeat("x", 40000) + `\ud800\u0000\ud83d\ude00"}]}`)
	largeOriginal := bytes.Clone(large)
	wantChunked := strings.ReplaceAll(string(large), `\u0000`, "")

	req, err := svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: large}, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.True(t, req.RequestBodyChunked)
	require.Equal(t, largeOriginal, large, "caller bytes must not be mutated")

	stored, err := svc.LoadRequestBody(ctx, req)
	require.NoError(t, err)
	require.Equal(t, wantChunked, string(stored), "CDC archive keeps the surrogate and strips NUL")
	require.Contains(t, string(stored), `\ud800`)
	require.NotContains(t, string(stored), `\ufffd`)

	exec, err := svc.CreateRequestExecution(ctx, &Channel{Channel: row}, "test", req,
		httpclient.Request{JSONBody: large}, llm.APIFormatOpenAIResponse, false)
	require.NoError(t, err)
	require.True(t, exec.RequestBodyChunked)

	execStored, err := svc.LoadRequestExecutionRequestBody(ctx, exec)
	require.NoError(t, err)
	require.Equal(t, wantChunked, string(execStored), "execution CDC archive keeps the surrogate")
	require.Contains(t, string(execStored), `\ud800`)
	require.NotContains(t, string(execStored), `\ufffd`)

	small := []byte(`{"model":"test","input":"\ud800"}`)
	inline, err := svc.CreateRequest(ctx, &llm.Request{Model: "test"}, &httpclient.Request{JSONBody: small}, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.False(t, inline.RequestBodyChunked)
	require.Equal(t, `{"model":"test","input":"\ufffd"}`, string(inline.RequestBody))

	inlineExec, err := svc.CreateRequestExecution(ctx, &Channel{Channel: row}, "test", inline,
		httpclient.Request{JSONBody: small}, llm.APIFormatOpenAIResponse, false)
	require.NoError(t, err)
	require.False(t, inlineExec.RequestBodyChunked)
	require.Equal(t, `{"model":"test","input":"\ufffd"}`, string(inlineExec.RequestBody))
}
