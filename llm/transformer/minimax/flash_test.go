package minimax

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestFlashRequestBoundary(t *testing.T) {
	for _, host := range []string{"https://api.minimax.io", "https://api.minimaxi.com"} {
		for _, protocol := range []string{"openai", "anthropic"} {
			for _, effort := range []string{"", "low", "medium", "high", "xhigh", "max", "none"} {
				t.Run(host+"/"+protocol+"/"+effort, func(t *testing.T) {
					var outbound transformer.Outbound
					var err error
					path := "/v1/chat/completions"
					if protocol == "openai" {
						outbound, err = NewOutboundTransformer(host+"/v1", "test-key")
					} else {
						path = "/anthropic/v1/messages"
						outbound, err = anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
							Type: anthropic.PlatformDirect, BaseURL: host + "/anthropic",
							APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
						})
					}
					require.NoError(t, err)
					req := &llm.Request{
						Model: "MiniMax-M3.1-Flash", ReasoningEffort: effort,
						Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}}},
					}
					wire, err := outbound.TransformRequest(t.Context(), req)
					require.NoError(t, err)
					require.Equal(t, host+path, wire.URL)

					// Capture the final request after HTTP-client authentication and serialization.
					var body map[string]any
					var header http.Header
					var capturedPath string
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						header, capturedPath = r.Header.Clone(), r.URL.Path
						if json.NewDecoder(r.Body).Decode(&body) != nil {
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{}`))
					}))
					defer server.Close()
					wire.URL = server.URL + path
					_, err = httpclient.NewHttpClientWithClient(server.Client()).Do(t.Context(), wire)
					require.NoError(t, err)
					require.Equal(t, path, capturedPath)
					require.True(t, header.Get("Authorization") == "Bearer test-key" || header.Get("X-API-Key") == "test-key")
					require.Equal(t, req.Model, body["model"])
					require.NotContains(t, body, "service_tier")
					if effort == "" {
						require.NotContains(t, body, "thinking")
						require.NotContains(t, body, "reasoning_effort")
						require.NotContains(t, body, "output_config")
					} else if protocol == "openai" {
						require.Equal(t, effort, body["reasoning_effort"])
					} else if effort == "none" {
						// Explicit unsupported choices remain caller-owned; no silent rewrite.
						require.Equal(t, "disabled", body["thinking"].(map[string]any)["type"])
					} else {
						require.Equal(t, effort, body["output_config"].(map[string]any)["effort"])
						require.Equal(t, "adaptive", body["thinking"].(map[string]any)["type"])
						require.NotContains(t, body["thinking"], "budget_tokens")
					}
					require.Equal(t, effort, req.ReasoningEffort)
				})
			}
		}
	}
}

func TestFlashNativeEffortAndLegacyIsolation(t *testing.T) {
	for _, platform := range []anthropic.PlatformType{anthropic.PlatformDirect, anthropic.PlatformDeepSeek} {
		for _, model := range []string{"MiniMax-M3.1-Flash", "MiniMax-M3", "MiniMax-M2.7", "saved-custom-alias"} {
			t.Run(string(platform)+"/"+model, func(t *testing.T) {
				outbound, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
					Type: platform, BaseURL: "https://example.com/anthropic",
					APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
				})
				require.NoError(t, err)
				wire, err := outbound.TransformRequest(t.Context(), &llm.Request{
					Model:               model,
					Messages:            []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("Hi")}}},
					TransformerMetadata: map[string]any{anthropic.TransformerMetadataKeyOutputConfigEffort: "max"},
				})
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, json.Unmarshal(wire.Body, &body))
				require.Equal(t, model, body["model"])
				require.Equal(t, "max", body["output_config"].(map[string]any)["effort"])
				require.False(t, strings.Contains(string(wire.Body), "service_tier"))
			})
		}
	}
}
