// 职责: 用合成服务账号验证 Vertex 认证、缓存、校验和凭证选择。
// 关联文件:
//
//	internal/server/biz/channel_vertex_credentials.go
//	internal/server/biz/channel_apikey_strategies.go
package biz

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func syntheticVertexAccount(t *testing.T, id string) objects.GCPCredential {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	encoded, err := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "identity-project", "client_email": id + "@identity-project.iam.gserviceaccount.com",
		"private_key_id": id, "private_key": string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		"token_uri": "http://untrusted.invalid/token",
	})
	require.NoError(t, err)
	return objects.GCPCredential{JSONData: string(encoded), ProjectID: "target-" + id, Region: "global"}
}

type vertexTokenTransport func(*http.Request) (*http.Response, error)

func (f vertexTokenTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVertexMixedAuthenticationAndRenewal(t *testing.T) {
	account := syntheticVertexAccount(t, "one")
	calls := 0
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: vertexTokenTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, vertexOAuthEndpoint, r.URL.String())
		require.NoError(t, r.Context().Err())
		deadline, ok := r.Context().Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 30*time.Second)
		require.NoError(t, r.ParseForm())
		parts := strings.Split(r.Form.Get("assertion"), ".")
		require.Len(t, parts, 3)
		claims, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)
		var payload map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(claims, &payload))
		require.JSONEq(t, `"https://oauth2.googleapis.com/token"`, string(payload["aud"]))
		require.JSONEq(t, `"https://www.googleapis.com/auth/cloud-platform"`, string(payload["scope"]))
		expiry := 3600
		if calls == 1 {
			expiry = 1
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"access_token":"synthetic-%d","token_type":"Bearer","expires_in":%d}`, calls, expiry)))}, nil
	})})
	entity := &ent.Channel{Type: channel.TypeGeminiVertex, BaseURL: "https://vertex.invalid/v1", Credentials: objects.ChannelCredentials{
		VertexAPIKeys: []objects.VertexAPIKey{{APIKey: "api-one", ProjectID: "key-project", Region: "us-central1"}}, VertexServiceAccounts: []objects.GCPCredential{account},
	}}
	ch := withStrategy(buildChannel(entity, client), objects.APIKeyStrategyRoundRobin)
	out, err := newVertexOutbound(ch, ch.BaseURL, "")
	require.NoError(t, err)
	ch.Outbound = out
	ctx, cancel := context.WithCancel(contexts.EnsureContainer(t.Context()))
	cancel()
	first, err := out.TransformRequest(ctx, vertexChatRequest("gemini-test"))
	require.NoError(t, err)
	require.Equal(t, "api-one", first.Auth.APIKey)
	require.Contains(t, first.URL, "/projects/key-project/locations/us-central1/")
	for i := range 3 {
		peer, err := newVertexOutbound(ch, "https://peer.invalid/prefix/v1", "/custom")
		require.NoError(t, err)
		// Advance the shared request counter to the service-account slot.
		if i > 0 {
			_, err = out.TransformRequest(ctx, vertexChatRequest("gemini-test"))
			require.NoError(t, err)
		}
		request, err := peer.TransformRequest(ctx, vertexChatRequest("gemini-test"))
		require.NoError(t, err)
		require.Equal(t, "bearer", request.Auth.Type)
		require.Equal(t, fmt.Sprintf("synthetic-%d", min(i+1, 2)), request.Auth.APIKey)
		require.Equal(t, "https://peer.invalid/prefix/v1/projects/target-one/locations/global/custom", request.URL)
		ref, ok := contexts.GetChannelAPIKey(ctx)
		require.True(t, ok)
		require.Equal(t, account.CredentialRef(), ref)
	}
	require.Equal(t, 2, calls)
}

func TestVertexServiceAccountValidation(t *testing.T) {
	valid := syntheticVertexAccount(t, "validation")
	for _, tc := range []struct {
		name string
		edit func(*objects.GCPCredential)
	}{
		{"malformed", func(c *objects.GCPCredential) { c.JSONData = "{secret" }},
		{"type", func(c *objects.GCPCredential) {
			c.JSONData = strings.Replace(c.JSONData, "service_account", "authorized_user", 1)
		}},
		{"identity", func(c *objects.GCPCredential) {
			c.JSONData = strings.Replace(c.JSONData, `"private_key_id":"validation"`, `"private_key_id":""`, 1)
		}},
		{"key", func(c *objects.GCPCredential) {
			c.JSONData = strings.ReplaceAll(c.JSONData, "RSA PRIVATE KEY", "INVALID")
		}},
		{"project", func(c *objects.GCPCredential) { c.ProjectID = "bad/project" }},
		{"region", func(c *objects.GCPCredential) { c.Region = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			tc.edit(&c)
			err := ValidateVertexAPIKeys(channel.TypeGeminiVertex, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{c}})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
	require.NoError(t, ValidateVertexAPIKeys(channel.TypeGeminiVertex, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{valid}}))
	require.Error(t, ValidateVertexAPIKeys(channel.TypeGeminiVertex, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{valid, valid}}))
	require.Error(t, ValidateVertexAPIKeys(channel.TypeOpenai, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{valid}}))
	require.Error(t, ValidateVertexAPIKeys(channel.TypeGeminiVertex, objects.ChannelCredentials{APIKey: "vertex-sa:reserved"}))
}

func TestVertexTokenExchangeUsesProxyAndRedactsFailure(t *testing.T) {
	var target string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target = r.Host
		http.Error(w, "synthetic-secret", http.StatusBadGateway)
	}))
	defer proxy.Close()
	client := httpclient.NewHttpClientWithProxy(&httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: proxy.URL})
	defer client.CloseIdleConnections()
	account := syntheticVertexAccount(t, "proxy")
	ch := buildChannel(&ent.Channel{Type: channel.TypeGeminiVertex, BaseURL: "https://vertex.invalid", Credentials: objects.ChannelCredentials{APIKey: "must-not-fallback", VertexServiceAccounts: []objects.GCPCredential{account}}, DisabledAPIKeys: []objects.DisabledAPIKey{{Key: account.CredentialRef()}}}, client)
	ch.apiKeyOverride = account.CredentialRef()
	out, err := newVertexOutbound(ch, ch.BaseURL, "")
	require.NoError(t, err)
	ctx := contexts.EnsureContainer(t.Context())
	_, err = out.TransformRequest(ctx, vertexChatRequest("gemini-test"))
	require.EqualError(t, err, "Vertex service-account token exchange failed")
	require.Equal(t, "oauth2.googleapis.com:443", target)
	ref, ok := contexts.GetChannelAPIKey(ctx)
	require.True(t, ok)
	require.Equal(t, account.CredentialRef(), ref)
	_, err = (&ChannelService{}).buildNonDefaultEndpointOutbound(ch.Channel, ch, objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIChatCompletion.String()})
	require.ErrorContains(t, err, "require Gemini endpoints")
}
