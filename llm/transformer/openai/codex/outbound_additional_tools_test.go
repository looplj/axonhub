package codex

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestOutboundTransformer_AdditionalToolsScope(t *testing.T) {
	body := []byte(`{
		"model": "gpt-6-luna",
		"input": [
			{"type": "additional_tools", "role": "developer", "tools": [{"type": "custom", "name": "exec"}]},
			{"type": "message", "role": "user", "content": "Hello"}
		]
	}`)

	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "official backend", baseURL: "https://chatgpt.com/backend-api/codex#"},
		{name: "official backend without scheme", baseURL: "chatgpt.com/backend-api/codex"},
		{name: "compatible relay", baseURL: "https://relay.example.com/v1"},
		{name: "relay with the official host in its path", baseURL: "https://relay.example.com/chatgpt.com/v1"},
		{name: "relay whose hostname ends with the official host", baseURL: "https://chatgpt.com.relay.example/v1"},
		{name: "relay whose hostname starts with the official host", baseURL: "https://chatgpt.com-evil.example/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound, err := NewOutboundTransformer(Params{
				BaseURL: tt.baseURL,
				TokenProvider: staticTokenGetter{creds: &oauth.OAuthCredentials{
					AccessToken: testAccessTokenWithAccountID(t),
					ExpiresAt:   time.Now().Add(time.Hour),
				}},
			})
			require.NoError(t, err)

			req, err := responses.NewInboundTransformer().TransformRequest(
				t.Context(), &httpclient.Request{Body: body})
			require.NoError(t, err)

			wire, err := outbound.TransformRequest(t.Context(), req)
			require.NoError(t, err)

			var payload struct {
				Input []json.RawMessage `json:"input"`
			}
			require.NoError(t, json.Unmarshal(wire.Body, &payload))

			require.Len(t, payload.Input, 2)
			require.Contains(t, string(payload.Input[0]), `"additional_tools"`)
			require.Contains(t, string(payload.Input[0]), `"exec"`)
			require.Contains(t, string(payload.Input[1]), "Hello")
		})
	}
}
