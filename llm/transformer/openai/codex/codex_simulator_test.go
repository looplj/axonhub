package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/simulator"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

type staticTokenGetter struct {
	creds *oauth.OAuthCredentials
}

func (g staticTokenGetter) Get(ctx context.Context) (*oauth.OAuthCredentials, error) {
	return g.creds, nil
}

func TestCodexOutbound_WithSimulator_InboundHeadersCannotOverride(t *testing.T) {
	ctx := context.Background()

	inbound := openai.NewInboundTransformer()

	outbound, err := NewOutboundTransformer(Params{
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: "test-access-token",
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
		},
	})
	require.NoError(t, err)

	sim := simulator.NewSimulator(inbound, outbound)

	openAIReqBody := map[string]any{
		"model": "gpt-5-codex",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Hello",
			},
		},
	}
	bodyBytes, err := json.Marshal(openAIReqBody)
	require.NoError(t, err)

	tests := []struct {
		name             string
		inboundUA        string
		inboundVersion   string
		wantFinalUA      string
		wantFinalVer     string
		inboundSessionID string
		wantSessionID    string
	}{
		{
			name:             "non-codex UA uses defaults; session id preserved",
			inboundUA:        "axonhub-test/0.0.1",
			inboundVersion:   "9.9.9",
			wantFinalUA:      UserAgent,
			wantFinalVer:     codexDefaultVersion,
			inboundSessionID: "provided-session",
			wantSessionID:    "provided-session",
		},
		{
			name:             "codex_cli_rs UA and version are preserved; session id preserved",
			inboundUA:        "codex_cli_rs/0.50.0 (macOS 14.0.0; arm64) Terminal",
			inboundVersion:   "9.9.9",
			wantFinalUA:      "codex_cli_rs/0.50.0 (macOS 14.0.0; arm64) Terminal",
			wantFinalVer:     "9.9.9",
			inboundSessionID: "provided-session",
			wantSessionID:    "provided-session",
		},
		{
			name:           "missing session id gets generated",
			inboundUA:      "codex_cli_rs/0.50.0 (macOS 14.0.0; arm64) Terminal",
			inboundVersion: "9.9.9",
			wantFinalUA:    "codex_cli_rs/0.50.0 (macOS 14.0.0; arm64) Terminal",
			wantFinalVer:   "9.9.9",
			wantSessionID:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(bodyBytes))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", tt.inboundUA)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Openai-Beta", "hacked")
			req.Header.Set("Originator", "hacked")
			req.Header.Set("Version", tt.inboundVersion)

			if tt.inboundSessionID != "" {
				req.Header.Set("Session_id", tt.inboundSessionID)
			}

			finalReq, err := sim.Simulate(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, finalReq)

			assert.Equal(t, codexAPIURL, finalReq.URL.String())
			assert.Equal(t, "text/event-stream", finalReq.Header.Get("Accept"))
			assert.Equal(t, "responses=experimental", finalReq.Header.Get("Openai-Beta"))
			assert.Equal(t, "codex_cli_rs", finalReq.Header.Get("Originator"))
			assert.Equal(t, tt.wantFinalVer, finalReq.Header.Get("Version"))
			assert.Equal(t, tt.wantFinalUA, finalReq.Header.Get("User-Agent"))

			if tt.wantSessionID != "" {
				assert.Equal(t, tt.wantSessionID, finalReq.Header.Get("Session_id"))
			} else {
				assert.NotEmpty(t, finalReq.Header.Get("Session_id"))
			}

			assert.Equal(t, "Bearer test-access-token", finalReq.Header.Get("Authorization"))
		})
	}
}
