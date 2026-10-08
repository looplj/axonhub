package codex

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// TestOutboundTransformer_AdditionalToolsPreserveHosts covers relays that front
// the official backend and are opted in through
// AXONHUB_CODEX_PRESERVE_ADDITIONAL_TOOLS_HOSTS: only an exact host match keeps
// the `additional_tools` input item, and the relay is still not treated as the
// official backend.
func TestOutboundTransformer_AdditionalToolsPreserveHosts(t *testing.T) {
	body := []byte(`{
		"model": "gpt-6-luna",
		"input": [
			{"type": "additional_tools", "role": "developer", "tools": [{"type": "custom", "name": "exec"}]},
			{"type": "message", "role": "user", "content": "Hello"}
		]
	}`)

	tests := []struct {
		name     string
		env      *string
		baseURL  string
		preserve bool
		official bool
	}{
		{name: "env unset", env: nil, baseURL: "http://10.50.1.28:8093/backend-api/codex#", preserve: false},
		{name: "env empty", env: new(""), baseURL: "http://10.50.1.28:8093/backend-api/codex#", preserve: false},
		{name: "env with only separators", env: new(" , ,"), baseURL: "http://10.50.1.28:8093/backend-api/codex#", preserve: false},
		{name: "env unset keeps official backend", env: nil, baseURL: "https://chatgpt.com/backend-api/codex#", preserve: true, official: true},
		{name: "ip host with port in base url", env: new("10.50.1.28"), baseURL: "http://10.50.1.28:8093/backend-api/codex#", preserve: true},
		{name: "port in env entry is ignored", env: new("10.50.1.28:9999"), baseURL: "http://10.50.1.28:8093/backend-api/codex#", preserve: true},
		{name: "one of several hosts", env: new("10.50.1.28, codex-image-shrink"), baseURL: "http://codex-image-shrink:8099/backend-api/codex#", preserve: true},
		{name: "spaces and empty entries", env: new(" , relay.example.com ,, "), baseURL: "https://relay.example.com/v1", preserve: true},
		{name: "case insensitive", env: new("Relay.Example.COM"), baseURL: "https://RELAY.example.com/v1", preserve: true},
		{name: "base url without scheme", env: new("codex-image-shrink"), baseURL: "codex-image-shrink:8099/backend-api/codex", preserve: true},
		{name: "base url without scheme or port", env: new("relay.example.com"), baseURL: "relay.example.com/v1", preserve: true},
		{name: "different host", env: new("relay.example.com"), baseURL: "https://other.example.com/v1", preserve: false},
		{name: "listed host only in path", env: new("relay.example.com"), baseURL: "https://other.example.com/relay.example.com/v1", preserve: false},
		{name: "subdomain of listed host", env: new("relay.example.com"), baseURL: "https://api.relay.example.com/v1", preserve: false},
		{name: "listed host as prefix of hostname", env: new("relay.example.com"), baseURL: "https://relay.example.com.evil.example/v1", preserve: false},
		{name: "listed parent domain", env: new("example.com"), baseURL: "https://relay.example.com/v1", preserve: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(codexPreserveAdditionalToolsHostsEnv, "")
			if tt.env == nil {
				require.NoError(t, os.Unsetenv(codexPreserveAdditionalToolsHostsEnv))
			} else {
				t.Setenv(codexPreserveAdditionalToolsHostsEnv, *tt.env)
			}

			outbound, err := NewOutboundTransformer(Params{
				BaseURL: tt.baseURL,
				TokenProvider: staticTokenGetter{creds: &oauth.OAuthCredentials{
					AccessToken: testAccessTokenWithAccountID(t),
					ExpiresAt:   time.Now().Add(time.Hour),
				}},
			})
			require.NoError(t, err)
			require.Equal(t, tt.official, outbound.isOfficialCodex())

			req, err := responses.NewInboundTransformer().TransformRequest(
				t.Context(), &httpclient.Request{Body: body})
			require.NoError(t, err)

			wire, err := outbound.TransformRequest(t.Context(), req)
			require.NoError(t, err)

			var payload struct {
				Input []json.RawMessage `json:"input"`
			}
			require.NoError(t, json.Unmarshal(wire.Body, &payload))

			if tt.preserve {
				require.Len(t, payload.Input, 2)
				require.Contains(t, string(payload.Input[0]), `"additional_tools"`)
				require.Contains(t, string(payload.Input[0]), `"exec"`)
				require.Contains(t, string(payload.Input[1]), "Hello")

				return
			}

			require.Len(t, payload.Input, 1)
			require.NotContains(t, string(wire.Body), "additional_tools")
			require.Contains(t, string(wire.Body), "Hello")
		})
	}
}
