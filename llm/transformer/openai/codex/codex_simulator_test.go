package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/simulator"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

type staticTokenGetter struct {
	creds *oauth.OAuthCredentials
}

const testChatAccountID = "acct_test"

func (g staticTokenGetter) Get(ctx context.Context) (*oauth.OAuthCredentials, error) {
	return g.creds, nil
}

func testAccessTokenWithAccountID(t *testing.T) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": testChatAccountID,
		},
	})

	signed, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	return signed
}

func TestCodexOutbound_MinimalIdentityHeaders(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)
	sim := newCodexSimulatorWithToken(t, accessToken)
	req := newCodexChatCompletionRequest(t)
	req.Header.Set("Conversation_id", "legacy-conversation")
	req.Header.Set("Openai-Beta", "responses=experimental")
	req.Header.Set("Session_id", "provided-session")
	req.Header.Set("Version", "9.9.9")

	finalReq, err := sim.Simulate(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, finalReq)

	assert.Equal(t, codexAPIURL, finalReq.URL.String())
	assert.Equal(t, "application/json", finalReq.Header.Get("Content-Type"))
	assert.Equal(t, AxonHubOriginator, finalReq.Header.Get("Originator"))
	assert.Equal(t, "axonhub/1.0", finalReq.Header.Get("User-Agent"))
	assert.Equal(t, "provided-session", finalReq.Header.Get("Session_id"))
	assert.Equal(t, testChatAccountID, finalReq.Header.Get("Chatgpt-Account-Id"))
	assert.Equal(t, "Bearer "+accessToken, finalReq.Header.Get("Authorization"))
	assert.Equal(t, "legacy-conversation", finalReq.Header.Get("Conversation_id"))
	assert.Equal(t, "responses=experimental", finalReq.Header.Get("Openai-Beta"))
	assert.Equal(t, "9.9.9", finalReq.Header.Get("Version"))
}

func TestCodexOutbound_AllowsInboundIdentityOverrides(t *testing.T) {
	ctx := context.Background()
	sim := newCodexSimulator(t)
	req := newCodexChatCompletionRequest(t)
	req.Header.Set("Originator", legacyCodexOriginator())
	req.Header.Set("User-Agent", legacyCodexUserAgent())

	finalReq, err := sim.Simulate(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, legacyCodexOriginator(), finalReq.Header.Get("Originator"))
	assert.Equal(t, legacyCodexUserAgent(), finalReq.Header.Get("User-Agent"))
	assert.Contains(t, strings.ToLower(finalReq.Header.Get("User-Agent")), legacyCodexOriginator())
}

func TestCodexOutbound_PassthroughModernCodexHeaders(t *testing.T) {
	ctx := context.Background()
	sim := newCodexSimulator(t)
	req := newCodexChatCompletionRequest(t)
	req.Header.Set("X-Codex-Turn-Metadata", `{"session_id":"turn-session","turn_id":"turn-123"}`)
	req.Header.Set("X-Codex-Window-Id", "window-123")
	req.Header.Set("X-Client-Request-Id", "request-123")
	req.Header.Set("X-Codex-Beta-Features", "js_repl")

	finalReq, err := sim.Simulate(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, `{"session_id":"turn-session","turn_id":"turn-123"}`, finalReq.Header.Get("X-Codex-Turn-Metadata"))
	assert.Equal(t, "window-123", finalReq.Header.Get("X-Codex-Window-Id"))
	assert.Equal(t, "request-123", finalReq.Header.Get("X-Client-Request-Id"))
	assert.Equal(t, "js_repl", finalReq.Header.Get("X-Codex-Beta-Features"))
}

func TestCodexOutbound_SessionIDPrecedence(t *testing.T) {
	t.Run("inbound Session_id header is used", func(t *testing.T) {
		ctx := shared.WithSessionID(context.Background(), "context-session")
		sim := newCodexSimulator(t)
		req := newCodexChatCompletionRequest(t)
		req.Header.Set("Session_id", "header-session")

		finalReq, err := sim.Simulate(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "header-session", finalReq.Header.Get("Session_id"))
	})

	t.Run("no inbound Session_id uses session_id from X-Codex-Turn-Metadata", func(t *testing.T) {
		ctx := shared.WithSessionID(context.Background(), "context-session")
		sim := newCodexSimulator(t)
		req := newCodexChatCompletionRequest(t)
		req.Header.Set("X-Codex-Turn-Metadata", `{"session_id":"turn-session","turn_id":"turn-123"}`)

		finalReq, err := sim.Simulate(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "turn-session", finalReq.Header.Get("Session_id"))
	})

	t.Run("no inbound header but context has session uses context", func(t *testing.T) {
		ctx := shared.WithSessionID(context.Background(), "context-session")
		sim := newCodexSimulator(t)
		req := newCodexChatCompletionRequest(t)

		finalReq, err := sim.Simulate(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "context-session", finalReq.Header.Get("Session_id"))
	})

	t.Run("invalid X-Codex-Turn-Metadata falls back to context session", func(t *testing.T) {
		ctx := shared.WithSessionID(context.Background(), "context-session")
		sim := newCodexSimulator(t)
		req := newCodexChatCompletionRequest(t)
		req.Header.Set("X-Codex-Turn-Metadata", `{"session_id":`)

		finalReq, err := sim.Simulate(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "context-session", finalReq.Header.Get("Session_id"))
	})

	t.Run("no inbound no context generates uuid", func(t *testing.T) {
		sim := newCodexSimulator(t)
		req := newCodexChatCompletionRequest(t)

		finalReq, err := sim.Simulate(context.Background(), req)
		require.NoError(t, err)

		sessionID := finalReq.Header.Get("Session_id")
		assert.NotEmpty(t, sessionID)
		_, parseErr := uuid.Parse(sessionID)
		assert.NoError(t, parseErr)
	})
}

func newCodexSimulator(t *testing.T) *simulator.Simulator {
	t.Helper()

	return newCodexSimulatorWithToken(t, testAccessTokenWithAccountID(t))
}

func newCodexSimulatorWithToken(t *testing.T, accessToken string) *simulator.Simulator {
	t.Helper()

	inbound := openai.NewInboundTransformer()
	outbound, err := NewOutboundTransformer(Params{
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	return simulator.NewSimulator(inbound, outbound)

}

func newCodexChatCompletionRequest(t *testing.T) *http.Request {
	t.Helper()

	bodyBytes, err := json.Marshal(map[string]any{
		"model": "gpt-5-codex",
		"messages": []map[string]any{{
			"role":    "user",
			"content": "Hello",
		}},
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	return req
}
