// 职责: 验证请求执行使用 Vertex 凭证引用记录安全的位置和后缀。
// 关联文件:
//
//	internal/server/biz/request.go
//	internal/objects/channel.go
package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entrequest "github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestVertexRequestCredentialAttribution(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:vertex_attribution?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newRequestServiceForAPIKeyIndex(t, client)
	account := objects.GCPCredential{JSONData: `{"client_email":"synthetic@example.invalid","private_key_id":"key","private_key":"synthetic-secret"}`}
	ch, err := client.Channel.Create().SetName("mixed attribution").SetType(channel.TypeGeminiVertex).
		SetSupportedModels([]string{"synthetic-model"}).SetDefaultTestModel("synthetic-model").
		SetCredentials(objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{{APIKey: "synthetic-key"}}, VertexServiceAccounts: []objects.GCPCredential{account}}).Save(ctx)
	require.NoError(t, err)
	req, err := client.Request.Create().SetModelID("synthetic-model").SetFormat(string(llm.APIFormatOpenAIChatCompletion)).
		SetRequestBody([]byte(`{}`)).SetStatus(entrequest.StatusProcessing).SetStream(false).Save(ctx)
	require.NoError(t, err)
	ctx = contexts.EnsureContainer(ctx)
	ref := account.CredentialRef()
	contexts.WithChannelAPIKey(ctx, ref)

	execution, err := svc.CreateRequestExecution(ctx, &Channel{Channel: ch}, "synthetic-model", req,
		httpclient.Request{Body: []byte(`{}`)}, llm.APIFormatOpenAIChatCompletion, false)

	require.NoError(t, err)
	require.NotNil(t, execution.ChannelAPIKeyIndex)
	require.Equal(t, 2, *execution.ChannelAPIKeyIndex)
	require.NotNil(t, execution.ChannelAPIKeySuffix)
	require.Equal(t, ref[len(ref)-4:], *execution.ChannelAPIKeySuffix)
}
