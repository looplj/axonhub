// 职责: 验证 Vertex 测试枚举、强制选择、重试刷新和事件归因。
// 关联文件:
//
//	internal/server/orchestrator/tester.go
//	internal/server/orchestrator/candidates.go
//	internal/server/orchestrator/retry_channel_refresh.go
//	internal/server/orchestrator/events.go
package orchestrator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func vertexConsumerAccount(t *testing.T) objects.GCPCredential {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	data, err := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "synthetic-project", "client_email": "synthetic@example.invalid", "private_key_id": "synthetic-key-id",
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"token_uri":   "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)
	return objects.GCPCredential{ProjectID: "synthetic-target", Region: "global", JSONData: string(data)}
}

func TestVertexTesterEnumeratesAndValidatesRefsWhenDisabled(t *testing.T) {
	ctx, client := setupTest(t)
	ctx = contexts.WithProjectID(ctx, createTestProject(t, ctx, client).ID)
	account := vertexConsumerAccount(t)
	ref := account.CredentialRef()
	credentials := objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{{APIKey: "synthetic-api-key", ProjectID: "synthetic-project", Region: "global"}}, VertexServiceAccounts: []objects.GCPCredential{account}}
	ch, err := client.Channel.Create().SetType(channel.TypeGeminiVertex).SetName("disabled mixed tests").
		SetBaseURL("https://example.invalid").SetCredentials(credentials).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetStatus(channel.StatusDisabled).SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "synthetic-api-key"}, {Key: ref}}).
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIEmbedding.String(), Path: "/embeddings"}}).
		SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "test-model", APIFormats: []string{llm.APIFormatOpenAIEmbedding.String()}}}}).
		SetPolicies(objects.ChannelPolicies{Stream: objects.CapabilityPolicyRequire}).Save(ctx)
	require.NoError(t, err)
	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	protection := biz.NewPromptProtectionRuleService(biz.PromptProtectionRuleServiceParams{Ent: client})
	t.Cleanup(protection.Stop)
	processor := NewTestChannelOrchestrator(channelService, requestService, systemService, usageLogService, protection, httpclient.NewHttpClient())
	id := objects.GUID{Type: "Channel", ID: ch.ID}

	result, err := processor.TestSingleAPIKey(ctx, id, ref, nil, nil)
	require.NoError(t, err)
	require.True(t, result.Disabled)
	require.Equal(t, "vertex-sa:****"+ref[len(ref)-4:], result.KeyPrefix)
	require.NotNil(t, result.Error)
	require.Contains(t, *result.Error, "embeddings do not support streaming")
	_, err = processor.TestSingleAPIKey(ctx, id, "vertex-sa:unknown", nil, nil)
	require.ErrorContains(t, err, "not configured")
	batch, err := processor.TestChannelAPIKeys(ctx, id, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Total)
	require.Equal(t, 2, batch.FailedCount)
	require.Equal(t, testCredentialPrefix("synthetic-api-key"), batch.Results[0].KeyPrefix)
	require.Equal(t, result.KeyPrefix, batch.Results[1].KeyPrefix)
	require.True(t, batch.Results[1].Disabled)
}

func TestVertexSpecifiedSelectorAllowsDisabledRef(t *testing.T) {
	ctx, client := setupTest(t)
	account := vertexConsumerAccount(t)
	ref := account.CredentialRef()
	ch, err := client.Channel.Create().SetType(channel.TypeGeminiVertex).SetName("forced account").SetBaseURL("https://example.invalid/v1").
		SetCredentials(objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetStatus(channel.StatusDisabled).SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: ref}}).Save(ctx)
	require.NoError(t, err)
	svc := biz.NewChannelServiceForTest(client)
	selector := NewSpecifiedChannelSelector(svc, objects.GUID{Type: "Channel", ID: ch.ID})
	selector.SelectedAPIKey = ref

	candidates, err := selector.Select(ctx, &llm.Request{Model: "test-model"})

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].Channel.Outbound)
	_, err = svc.GetChannel(ctx, ch.ID)
	require.Error(t, err, "ordinary construction must not select the disabled account")
}

func TestVertexRetryRefreshUsesCredentialRefs(t *testing.T) {
	ctx, client := setupTest(t)
	account := vertexConsumerAccount(t)
	ref := account.CredentialRef()
	ch, err := client.Channel.Create().SetType(channel.TypeGeminiVertex).SetName("retry account").SetBaseURL("https://example.invalid/v1").
		SetCredentials(objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{{APIKey: "synthetic-api-key", ProjectID: "key-project", Region: "global"}}, VertexServiceAccounts: []objects.GCPCredential{account}}).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	svc := biz.NewChannelServiceForTest(client)
	snapshot := retryGuardSnapshot(t, ctx, svc, ch.ID)
	_, err = client.Channel.UpdateOneID(ch.ID).SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: ref}}).Save(ctx)
	require.NoError(t, err)
	transformer := newRetryGuardTransformer(svc, snapshot, ref)

	require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))

	fresh := transformer.state.CurrentCandidate.Channel
	require.NotSame(t, snapshot, fresh)
	require.Equal(t, []string{"synthetic-api-key"}, fresh.Credentials.GetEnabledCredentialRefs(fresh.DisabledAPIKeys))
}

func TestVertexRequestEventAttribution(t *testing.T) {
	account := vertexConsumerAccount(t)
	ref := account.CredentialRef()
	ctx := contexts.EnsureContainer(t.Context())
	contexts.WithChannelAPIKey(ctx, ref)
	oauth, selected := upstreamCredentialKey(ctx, nil, nil)
	require.False(t, oauth)
	require.Equal(t, ref, selected)
	got := credentialEventRef(oauth, selected)
	require.Equal(t, events.Fingerprint(ref), got.Fingerprint)
	require.Equal(t, "vertex-sa:****"+ref[len(ref)-4:], got.Masked)
}
