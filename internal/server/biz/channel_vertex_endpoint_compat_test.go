// 职责: 验证 Vertex 显式端点的保存兼容性和所选凭证的资源地址配对。
// 关联文件:
//
//	internal/server/biz/channel.go
//	internal/server/biz/channel_llm.go
//	internal/server/biz/channel_vertex_credentials.go
package biz

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

func TestVertexJSONCredentialsSaveGeminiEndpoints(t *testing.T) {
	account := syntheticVertexAccount(t, "save-endpoint")
	credentials := objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}
	for _, format := range []llm.APIFormat{llm.APIFormatGeminiContents, llm.APIFormatGeminiEmbedding} {
		for _, operation := range []string{"create", "add JSON", "add endpoint"} {
			t.Run(format.String()+"/"+operation, func(t *testing.T) {
				svc, client := setupTestChannelService(t)
				defer client.Close()
				ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
				baseURL := "https://vertex.invalid/v1"
				endpoints := []objects.ChannelEndpoint{{APIFormat: format.String(), BaseURL: "https://peer.invalid/proxy", Path: "/custom"}}
				var saved *ent.Channel
				var err error
				if operation == "create" {
					saved, err = svc.CreateChannel(ctx, ent.CreateChannelInput{
						Type: channel.TypeGeminiVertex, Name: "Vertex endpoint", BaseURL: &baseURL,
						Credentials: credentials, Endpoints: endpoints, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model",
					})
				} else {
					initialCredentials := objects.ChannelCredentials{APIKey: "synthetic-legacy-key"}
					initialEndpoints := endpoints
					patch := &ent.UpdateChannelInput{Credentials: &credentials}
					if operation == "add endpoint" {
						initialCredentials, initialEndpoints = credentials, nil
						patch = &ent.UpdateChannelInput{Endpoints: endpoints}
					}
					stored, saveErr := client.Channel.Create().SetType(channel.TypeGeminiVertex).SetName("Vertex endpoint").SetBaseURL(baseURL).
						SetCredentials(initialCredentials).SetEndpoints(initialEndpoints).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").Save(ctx)
					require.NoError(t, saveErr)
					saved, err = svc.UpdateChannel(ctx, stored.ID, patch)
				}
				require.NoError(t, err)
				persisted, err := client.Channel.Get(ctx, saved.ID)
				require.NoError(t, err)
				require.Equal(t, credentials, persisted.Credentials)
				require.Contains(t, persisted.Endpoints, endpoints[0])
			})
		}
	}
}

func TestVertexEndpointPreservesLegacyBaseAndBoundCredentialPairing(t *testing.T) {
	account := syntheticVertexAccount(t, "endpoint-pair")
	for _, tc := range []struct {
		name           string
		credentials    objects.ChannelCredentials
		wantURL        string
		wantAuth       string
		wantKey        string
		wantRef        string
		wantTokenCalls int
	}{
		{"legacy", objects.ChannelCredentials{APIKey: "legacy-key"}, "https://peer.invalid/proxy/custom", "api_key", "legacy-key", "legacy-key", 0},
		{"bound API key", objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{{APIKey: "bound-key", ProjectID: "bound-project", Region: "us-central1"}}}, "https://peer.invalid/proxy/v1/projects/bound-project/locations/us-central1/custom", "api_key", "bound-key", "bound-key", 0},
		{"JSON", objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, "https://peer.invalid/proxy/v1/projects/target-endpoint-pair/locations/global/custom", "bearer", "synthetic-token", account.CredentialRef(), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokenCalls := 0
			client := httpclient.NewHttpClientWithClient(&http.Client{Transport: vertexTokenTransport(func(r *http.Request) (*http.Response, error) {
				tokenCalls++
				require.Equal(t, vertexOAuthEndpoint, r.URL.String())
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"access_token":"synthetic-token","token_type":"Bearer","expires_in":3600}`))}, nil
			})})
			svc := &ChannelService{httpClient: client}
			entity := &ent.Channel{Type: channel.TypeGeminiVertex, BaseURL: "https://vertex.invalid/v1", Credentials: tc.credentials, SupportedModels: []string{"gemini-test"}}
			built, err := svc.buildChannelWithTransformer(entity)
			require.NoError(t, err)
			// Warm the primary token cache, then exercise both endpoint formats.
			_, err = built.Outbound.TransformRequest(contexts.EnsureContainer(t.Context()), vertexChatRequest("gemini-test"))
			require.NoError(t, err)
			for _, format := range []llm.APIFormat{llm.APIFormatGeminiContents, llm.APIFormatGeminiEmbedding} {
				peer, err := svc.buildNonDefaultEndpointOutbound(entity, built, objects.ChannelEndpoint{APIFormat: format.String(), BaseURL: "https://peer.invalid/proxy", Path: "/custom"})
				require.NoError(t, err)
				ctx := contexts.EnsureContainer(t.Context())
				input := vertexChatRequest("gemini-test")
				if format == llm.APIFormatGeminiEmbedding {
					input = &llm.Request{
						Model: "text-embedding-005", RequestType: llm.RequestTypeEmbedding, APIFormat: format,
						Embedding: &llm.EmbeddingRequest{Input: llm.EmbeddingInput{StringArray: []string{"hello"}}},
					}
				}
				request, err := peer.TransformRequest(ctx, input)
				require.NoError(t, err)
				require.Equal(t, tc.wantURL, request.URL)
				require.Equal(t, tc.wantAuth, request.Auth.Type)
				require.Equal(t, tc.wantKey, request.Auth.APIKey)
				ref, ok := contexts.GetChannelAPIKey(ctx)
				require.True(t, ok)
				require.Equal(t, tc.wantRef, ref)
			}
			require.Equal(t, tc.wantTokenCalls, tokenCalls)
		})
	}
}

func TestVertexLegacyResolvedURLsMatchConstructor(t *testing.T) {
	for _, version := range []string{"", "/v1", "/v1beta"} {
		for _, format := range []llm.APIFormat{llm.APIFormatGeminiContents, llm.APIFormatGeminiEmbedding} {
			t.Run(version+"/"+format.String(), func(t *testing.T) {
				svc := &ChannelService{}
				entity := &ent.Channel{
					Type: channel.TypeGeminiVertex, BaseURL: "https://vertex.invalid" + version,
					Credentials: objects.ChannelCredentials{APIKey: "legacy-key"}, SupportedModels: []string{"gemini-test"},
				}
				built, err := svc.buildChannelWithTransformer(entity)
				require.NoError(t, err)
				input := vertexChatRequest("gemini-test")
				if format == llm.APIFormatGeminiEmbedding {
					input = &llm.Request{
						Model: "text-embedding-005", RequestType: llm.RequestTypeEmbedding, APIFormat: format,
						Embedding: &llm.EmbeddingRequest{Input: llm.EmbeddingInput{StringArray: []string{"hello"}}},
					}
				}
				legacy, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{BaseURL: entity.BaseURL, PlatformType: gemini.PlatformVertex})
				require.NoError(t, err)
				want, err := legacy.TransformRequest(t.Context(), input)
				require.NoError(t, err)
				t.Run("primary", func(t *testing.T) {
					got, err := built.Outbound.TransformRequest(contexts.EnsureContainer(t.Context()), input)
					require.NoError(t, err)
					require.Equal(t, want.URL, got.URL, "primary must retain the legacy constructor URL")
				})
				for _, path := range []string{"", "/custom"} {
					t.Run("peer"+path, func(t *testing.T) {
						baseURL := "https://peer.invalid/proxy" + version
						peer, err := svc.buildNonDefaultEndpointOutbound(entity, built, objects.ChannelEndpoint{
							APIFormat: format.String(), BaseURL: baseURL, Path: path,
						})
						require.NoError(t, err)
						legacy, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
							BaseURL: baseURL, EndpointPath: path, PlatformType: gemini.PlatformVertex,
						})
						require.NoError(t, err)
						want, err := legacy.TransformRequest(t.Context(), input)
						require.NoError(t, err)
						got, err := peer.TransformRequest(contexts.EnsureContainer(t.Context()), input)
						require.NoError(t, err)
						require.Equal(t, want.URL, got.URL, "peer must retain the legacy constructor URL")
						if path != "" {
							require.Equal(t, "https://peer.invalid/proxy/custom", got.URL)
						}
					})
				}
			})
		}
	}
}
