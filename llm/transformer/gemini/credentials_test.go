// 职责: 验证 Gemini 的逐请求认证解析在聊天、嵌入和图像路径保持配对。
// 关联文件:
//
//	llm/transformer/gemini/outbound.go
//	llm/transformer/gemini/embedding.go
//	llm/transformer/gemini/image.go
package gemini

import (
	"context"
	"errors"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/require"
)

type credentialTestProvider func(context.Context) string

func (p credentialTestProvider) Get(ctx context.Context) string { return p(ctx) }

func TestCredentialResolverPairsAuthenticationAcrossRequestTypes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request *llm.Request
		suffix  string
	}{
		{"chat", &llm.Request{Model: "model", Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: new("hello")}}}}, ":generateContent"},
		{"stream", &llm.Request{Model: "model", Stream: new(true), Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: new("hello")}}}}, ":streamGenerateContent?alt=sse"},
		{"embedding", &llm.Request{Model: "model", RequestType: llm.RequestTypeEmbedding, Embedding: &llm.EmbeddingRequest{Input: llm.EmbeddingInput{StringArray: []string{"hello"}}}}, ":embedContent"},
		{"image", &llm.Request{Model: "model", RequestType: llm.RequestTypeImage, Image: &llm.ImageRequest{Prompt: "hello"}}, ":generateContent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, endpointPath := range []string{"", "/custom"} {
				calls := 0
				out, err := NewOutboundTransformerWithConfig(Config{
					BaseURL: "https://wrong.invalid", PlatformType: PlatformVertex, EndpointPath: endpointPath,
					APIKeyProvider: credentialTestProvider(func(context.Context) string { t.Fatal("resolver must replace key selection"); return "" }),
					CredentialResolver: func(context.Context) (string, *httpclient.AuthConfig, error) {
						calls++
						return "https://selected.invalid/v1/projects/selected/locations/global", &httpclient.AuthConfig{Type: "bearer", APIKey: "synthetic-token"}, nil
					},
				})
				require.NoError(t, err)
				request, err := out.TransformRequest(t.Context(), tc.request)
				require.NoError(t, err)
				require.Equal(t, 1, calls)
				require.Equal(t, &httpclient.AuthConfig{Type: "bearer", APIKey: "synthetic-token"}, request.Auth)
				if endpointPath != "" {
					require.Equal(t, "https://selected.invalid/v1/projects/selected/locations/global/custom", request.URL)
				} else {
					require.Equal(t, "https://selected.invalid/v1/projects/selected/locations/global/publishers/google/models/model"+tc.suffix, request.URL)
				}
			}
		})
	}
}

func TestCredentialResolverErrorDoesNotFallBack(t *testing.T) {
	want := errors.New("synthetic token failure")
	out, err := NewOutboundTransformerWithConfig(Config{
		APIKeyProvider:     credentialTestProvider(func(context.Context) string { t.Fatal("unexpected fallback"); return "" }),
		CredentialResolver: func(context.Context) (string, *httpclient.AuthConfig, error) { return "", nil, want },
	})
	require.NoError(t, err)
	request, err := out.TransformRequest(t.Context(), &llm.Request{Model: "model"})
	require.ErrorIs(t, err, want)
	require.Nil(t, request)
}
