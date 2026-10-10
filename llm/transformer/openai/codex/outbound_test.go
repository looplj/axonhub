package codex

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// TestCodexOutbound_DefaultUserAgentHidesInboundClientUA pins the ownership of
// the outbound User-Agent (issue #2635): the transformer always presents the
// Codex-client-like default and never copies the inbound client's UA. Forwarding
// the client UA is opt-in through the orchestrator's user-agent pass-through
// middleware, which runs after this transformer and overwrites the default when
// the switch is enabled.
func TestCodexOutbound_DefaultUserAgentHidesInboundClientUA(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	const clientUA = "ZCode/3.14.4 (darwin arm64)"

	body := []byte(`{"model":"gpt-5-codex","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	rawReq, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(body))
	require.NoError(t, err)
	rawReq.Header.Set("Content-Type", "application/json")
	rawReq.Header.Set("User-Agent", clientUA)

	inboundRequest, err := httpclient.ReadHTTPRequest(rawReq)
	require.NoError(t, err)

	llmReq, err := openai.NewInboundTransformer().TransformRequest(ctx, inboundRequest)
	require.NoError(t, err)
	llmReq.RawRequest = inboundRequest

	req, err := outbound.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	assert.Equal(t, codexDefaultUserAgent, req.Headers.Get("User-Agent"))
	assert.NotEqual(t, clientUA, req.Headers.Get("User-Agent"))

	// The generic inbound-header merge must not reintroduce the client UA either:
	// User-Agent is a blocked header, so the transformer-owned value survives.
	merged := httpclient.MergeInboundRequest(req, inboundRequest)
	assert.Equal(t, codexDefaultUserAgent, merged.Headers.Get("User-Agent"))
	assert.NotEqual(t, clientUA, merged.Headers.Get("User-Agent"))
}

// TestCodexOutbound_DefaultUserAgentIsCodexLike keeps the fake identity
// consistent with the Version header the transformer fabricates.
func TestCodexOutbound_DefaultUserAgentIsCodexLike(t *testing.T) {
	assert.Equal(t, "codex_cli_rs/"+codexDefaultVersion, codexDefaultUserAgent)
}

// TestCodexOutbound_ScrubsClientFingerprintHeaders pins the codex-scoped half of
// issue #2635: client-private attribution headers must not reach the ChatGPT
// Codex backend. The scrub is deliberately scoped to this channel rather than
// added to httpclient's global blockedHeaders, because Http-Referer and X-Title
// are legitimate app-attribution headers for OpenRouter-style channels.
func TestCodexOutbound_ScrubsClientFingerprintHeaders(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	body := []byte(`{"model":"gpt-5-codex","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	rawReq, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(body))
	require.NoError(t, err)
	rawReq.Header.Set("Content-Type", "application/json")
	rawReq.Header.Set("User-Agent", "ZCode/3.14.4 (darwin arm64)")
	// Client fingerprint headers from the issue report.
	rawReq.Header.Set("Http-Referer", "https://editor.example/session")
	rawReq.Header.Set("X-Title", "ZCode")
	rawReq.Header.Set("X-Platform", "darwin")
	rawReq.Header.Set("X-Zcode-Session", "zcode-session")
	rawReq.Header.Set("X-Zcode-Client", "ZCode/3.14.4")
	rawReq.Header.Set("X-Os-Name", "macOS")
	// A non-fingerprint custom header must still be forwarded.
	rawReq.Header.Set("X-Custom", "client-value")
	// Originator is genuine Codex identity and must still be honoured.
	rawReq.Header.Set("Originator", "codex_cli_rs")

	inboundRequest, err := httpclient.ReadHTTPRequest(rawReq)
	require.NoError(t, err)

	llmReq, err := openai.NewInboundTransformer().TransformRequest(ctx, inboundRequest)
	require.NoError(t, err)
	llmReq.RawRequest = inboundRequest

	req, err := outbound.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	// MergeInboundRequest is what actually forwards leftover inbound headers, so
	// assert on the merged request, not just the transformer output.
	merged := httpclient.MergeInboundRequest(req, inboundRequest)
	for _, name := range []string{"Http-Referer", "X-Title", "X-Platform", "X-Zcode-Session", "X-Zcode-Client", "X-Os-Name"} {
		assert.Empty(t, merged.Headers.Get(name), "%s must not leak to the Codex upstream", name)
	}

	assert.Equal(t, "client-value", merged.Headers.Get("X-Custom"))
	assert.Equal(t, "codex_cli_rs", merged.Headers.Get("Originator"))
}
