// 职责: 验证真实渠道构造入口支持 Vertex JSON 凭证和指定禁用凭证。
// 关联文件:
//
//	internal/server/biz/channel_llm.go
//	internal/server/biz/channel_vertex_credentials.go
package biz

import (
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

func TestVertexChannelConstructionSelectsConfiguredCredentials(t *testing.T) {
	account := syntheticVertexAccount(t, "construction")
	ref := account.CredentialRef()
	keys := []objects.VertexAPIKey{{APIKey: "synthetic-key", ProjectID: "key-project", Region: "global"}}
	for _, tc := range []struct {
		name        string
		credentials objects.ChannelCredentials
		disabled    []objects.DisabledAPIKey
		override    string
		wantErr     string
		wantRef     string
		wantAuth    string
		wantProject string
	}{
		{name: "JSON only", credentials: objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, wantRef: ref, wantAuth: "bearer", wantProject: "target-construction"},
		{name: "disabled JSON forced", credentials: objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, disabled: []objects.DisabledAPIKey{{Key: ref}}, override: ref, wantRef: ref, wantAuth: "bearer", wantProject: "target-construction"},
		{name: "disabled JSON ordinary", credentials: objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, disabled: []objects.DisabledAPIKey{{Key: ref}}, wantErr: "no enabled Vertex credential"},
		{name: "disabled key forced", credentials: objects.ChannelCredentials{VertexAPIKeys: keys}, disabled: []objects.DisabledAPIKey{{Key: "synthetic-key"}}, override: "synthetic-key", wantRef: "synthetic-key", wantAuth: "api_key", wantProject: "key-project"},
		{name: "mixed JSON forced", credentials: objects.ChannelCredentials{VertexAPIKeys: keys, VertexServiceAccounts: []objects.GCPCredential{account}}, override: ref, wantRef: ref, wantAuth: "bearer", wantProject: "target-construction"},
		{name: "unknown ref", credentials: objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, override: "vertex-sa:unknown", wantErr: "not configured"},
		{name: "unknown key", credentials: objects.ChannelCredentials{VertexAPIKeys: keys}, override: "unconfigured-key", wantErr: "not configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenCalls := 0
			client := httpclient.NewHttpClientWithClient(&http.Client{Transport: vertexTokenTransport(func(r *http.Request) (*http.Response, error) {
				tokenCalls++
				require.Equal(t, vertexOAuthEndpoint, r.URL.String())
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"access_token":"synthetic-token","token_type":"Bearer","expires_in":3600}`))}, nil
			})})
			svc := &ChannelService{httpClient: client}
			entity := &ent.Channel{Type: channel.TypeGeminiVertex, BaseURL: "https://vertex.invalid/v1", Credentials: tc.credentials, DisabledAPIKeys: tc.disabled, SupportedModels: []string{"gemini-test"}}

			built, err := svc.buildChannelWithTransformer(entity, tc.override)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Zero(t, tokenCalls)
				return
			}
			require.NoError(t, err)
			require.Zero(t, tokenCalls, "construction must not exchange tokens")
			ctx := contexts.EnsureContainer(t.Context())
			request, err := built.Outbound.TransformRequest(ctx, vertexChatRequest("gemini-test"))
			require.NoError(t, err)
			require.Equal(t, tc.wantAuth, request.Auth.Type)
			require.Contains(t, request.URL, "/projects/"+tc.wantProject+"/locations/global/")
			selected, ok := contexts.GetChannelAPIKey(ctx)
			require.True(t, ok)
			require.Equal(t, tc.wantRef, selected)
			if tc.wantAuth == "bearer" {
				require.Equal(t, "synthetic-token", request.Auth.APIKey)
				require.Equal(t, 1, tokenCalls)
			} else {
				require.Equal(t, "synthetic-key", request.Auth.APIKey)
				require.Zero(t, tokenCalls)
			}
		})
	}
}
