// 职责: 校验 Responses 续接从归档冷启动与从热会话准备时得到同一份 upstream 输入。
// 关联文件:
//
//	internal/server/biz/request.go
//	internal/server/biz/request_body_chunks.go
//	internal/server/orchestrator/responses_attempt.go
//	internal/server/orchestrator/responses_session.go
//
// 遵循的 Rules:
//
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	CDC 分块归档只沿用 NUL 清理，surrogate 必须保留，否则冷启动续接会改写 upstream 输入。
//	该用例只使用 SQLite 与进程内状态，不连接外部服务。

package orchestrator

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesContinuationArchiveKeepsSurrogate(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ResponsesContinuationStorage?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	project := client.Project.Create().SetName("storage-boundary").SaveX(ctx)
	key := client.APIKey.Create().SetName("storage-boundary").SetKey("synthetic-continuation-key").SetProjectID(project.ID).SaveX(ctx)
	ctx = contexts.WithProjectID(contexts.WithAPIKey(ctx, key), project.ID)
	ctx = shared.WithSessionScope(ctx, "continuation-scope")

	client.DataStorage.Create().
		SetName("primary").
		SetDescription("storage-boundary").
		SetPrimary(true).
		SetType("database").
		SetSettings(&objects.DataStorageSettings{}).
		SaveX(ctx)

	system := biz.NewSystemService(biz.SystemServiceParams{Ent: client})
	storage := biz.NewDataStorageService(biz.DataStorageServiceParams{Client: client, SystemService: system})
	svc := biz.NewRequestService(client, system.CacheConfig, system, nil, storage, biz.NewLiveStreamRegistry())

	// 超过分块阈值，走 CDC bytea 归档路径。
	root := []byte(`{"model":"gpt-5","input":[{"type":"function_call_output","call_id":"call_1","output":"` +
		strings.Repeat("x", 40000) + `a\ud800"}]}`)
	rootOriginal := bytes.Clone(root)
	response := []byte(`{"id":"resp_continuation","status":"completed","output":[]}`)

	row, err := svc.CreateRequest(ctx, &llm.Request{Model: "gpt-5"}, &httpclient.Request{JSONBody: root}, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)
	require.True(t, row.RequestBodyChunked)
	require.NoError(t, svc.UpdateRequestFinalized(ctx, row, request.StatusCompleted, "resp_continuation", response, nil, nil))

	savedRequest, _, found, err := svc.LoadCompletedResponsesSession(ctx, "resp_continuation")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, rootOriginal, root, "persisting must not mutate caller bytes")
	require.Contains(t, string(savedRequest), `a\ud800`, "archive must keep the surrogate escape")

	current := []byte(`{"model":"gpt-5","previous_response_id":"resp_continuation","input":[{"type":"message","role":"user","content":"next"}]}`)
	prepare := func(store *responsesSessionStore) []byte {
		p := &PersistentOutboundTransformer{state: &PersistenceState{ResponsesContinuation: newResponsesContinuation(store, current)}}
		body, err := prepareResponsesAttemptBody(ctx, p, responsesAttemptTarget{
			origin:     shared.ResponsesOrigin{ChannelID: 1, Format: "openai/responses", Model: "gpt-5"},
			forwardsID: true,
		})
		require.NoError(t, err)

		return body
	}

	hot := newResponsesSessionStore()
	hot.record(ctx, root, response)
	hotBody := prepare(hot)

	cold := newResponsesSessionStore(func(c context.Context, id string) ([]byte, []byte, bool, error) {
		return svc.LoadCompletedResponsesSession(c, id)
	})
	coldBody := prepare(cold)

	require.Equal(t, hotBody, coldBody, "restoring a session from its archive must not change upstream input")
	require.Contains(t, string(coldBody), `a\ud800`)
	require.NotContains(t, string(coldBody), `\ufffd`, "archive path must not rewrite surrogates")
}
