// 职责: 验证六种选择策略统一使用混合 Vertex 凭证引用。
// 关联文件:
//
//	internal/server/biz/channel_apikey_strategies.go
//	internal/server/biz/channel_apikey_state.go
//	internal/server/biz/channel_vertex_credentials.go
package biz

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestVertexMixedSelectionStrategies(t *testing.T) {
	accounts := []objects.GCPCredential{syntheticVertexAccount(t, "a"), syntheticVertexAccount(t, "b")}
	credentials := objects.ChannelCredentials{
		VertexAPIKeys:         []objects.VertexAPIKey{{APIKey: "key-a", ProjectID: "p-a", Region: "global"}, {APIKey: "key-b", ProjectID: "p-b", Region: "global"}},
		VertexServiceAccounts: accounts,
	}
	refs := credentials.GetAllCredentialRefs()
	for _, strategy := range []string{objects.APIKeyStrategySticky, objects.APIKeyStrategyRandom, objects.APIKeyStrategyRoundRobin, objects.APIKeyStrategyRoundRobinSuccess, objects.APIKeyStrategyPriority, objects.APIKeyStrategyFixed} {
		t.Run(strategy, func(t *testing.T) {
			svc := &ChannelService{}
			entity := &ent.Channel{ID: 1, Type: channel.TypeGeminiVertex, Credentials: credentials}
			ch := withStrategy(buildChannel(entity, nil), strategy)
			ch.apiKeyState = svc.apiKeySelectionStateFor(ch)
			svc.publishAPIKeySelectionSnapshot(ch)
			provider, err := getAPIKeyProvider(ch)
			require.NoError(t, err)
			ctx := contexts.EnsureContainer(t.Context())
			if strategy == objects.APIKeyStrategyRoundRobin || strategy == objects.APIKeyStrategyRoundRobinSuccess {
				for _, ref := range append(refs, refs[0]) {
					selected := provider.Get(ctx)
					require.Equal(t, ref, selected)
					if strategy == objects.APIKeyStrategyRoundRobinSuccess {
						svc.onAPIKeySuccess(1, selected)
					}
				}
			}
			// Disable each competing credential so every strategy must reach each ref.
			for _, ref := range refs {
				copyEntity := *entity
				for _, other := range refs {
					if other != ref {
						copyEntity.DisabledAPIKeys = append(copyEntity.DisabledAPIKeys, objects.DisabledAPIKey{Key: other})
					}
				}
				snapshot := withStrategy(buildChannel(&copyEntity, nil), strategy)
				snapshot.apiKeyState = svc.apiKeySelectionStateFor(snapshot)
				svc.publishAPIKeySelectionSnapshot(snapshot)
				provider, err = getAPIKeyProvider(snapshot)
				require.NoError(t, err)
				require.Equal(t, ref, provider.Get(ctx))
			}
			for _, ref := range refs {
				entity.DisabledAPIKeys = append(entity.DisabledAPIKeys, objects.DisabledAPIKey{Key: ref})
			}
			empty := withStrategy(buildChannel(entity, nil), strategy)
			empty.apiKeyState = svc.apiKeySelectionStateFor(empty)
			_, err = getAPIKeyProvider(empty)
			require.Error(t, err)
			require.Empty(t, newMultiKeyProvider(empty).Get(ctx))
			empty.apiKeyOverride = accounts[1].CredentialRef()
			provider, err = getAPIKeyProvider(empty)
			require.NoError(t, err)
			require.Equal(t, accounts[1].CredentialRef(), provider.Get(ctx))
		})
	}
}

func TestVertexSnapshotReplacementDoesNotReuseToken(t *testing.T) {
	oldAccount := syntheticVertexAccount(t, "old")
	newAccount := syntheticVertexAccount(t, "new")
	entity := &ent.Channel{Type: channel.TypeGeminiVertex, BaseURL: "https://vertex.invalid", Credentials: objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{oldAccount}}}
	calls := 0
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: vertexTokenTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"access_token":"snapshot-%d","token_type":"Bearer","expires_in":3600}`, calls)))}, nil
	})})
	old := buildChannel(entity, client)
	oldOutbound, err := newVertexOutbound(old, entity.BaseURL, "")
	require.NoError(t, err)
	ctx := contexts.EnsureContainer(t.Context())
	before, err := oldOutbound.TransformRequest(ctx, vertexChatRequest("gemini-test"))
	require.NoError(t, err)
	copyEntity := *entity
	copyEntity.Credentials = objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{newAccount}}
	current := buildChannel(&copyEntity, client)
	newOutbound, err := newVertexOutbound(current, entity.BaseURL, "")
	require.NoError(t, err)
	after, err := newOutbound.TransformRequest(ctx, vertexChatRequest("gemini-test"))
	require.NoError(t, err)
	require.Equal(t, "snapshot-1", before.Auth.APIKey)
	require.Equal(t, "snapshot-2", after.Auth.APIKey)
	require.Contains(t, after.URL, "/projects/target-new/")
	inflight, err := oldOutbound.TransformRequest(ctx, vertexChatRequest("gemini-test"))
	require.NoError(t, err)
	require.Equal(t, "snapshot-1", inflight.Auth.APIKey)
	require.Contains(t, inflight.URL, "/projects/target-old/")
	require.Equal(t, 2, calls)
}
