package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestCodexOutbound_StreamAcceptHeader(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)
	capturedHeaders := make(chan http.Header, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: server.URL,
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, false)
	executor := httpclient.NewHttpClientWithClient(server.Client())

	stream, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	defer func() {
		_ = stream.Close()
	}()

	var headers http.Header
	select {
	case headers = <-capturedHeaders:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for captured stream request")
	}

	assert.Equal(t, "text/event-stream", headers.Get("Accept"))
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, AxonHubOriginator, headers.Get("Originator"))
	assert.Equal(t, "axonhub/1.0", headers.Get("User-Agent"))
	assert.Equal(t, testChatAccountID, headers.Get("Chatgpt-Account-Id"))
	assert.Equal(t, "Bearer "+accessToken, headers.Get("Authorization"))
}

func TestCodexOutbound_StreamAllowsDownstreamIdentityOverrides(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)
	capturedHeaders := make(chan http.Header, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: server.URL,
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, true)
	executor := httpclient.NewHttpClientWithClient(server.Client())

	stream, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	defer func() {
		_ = stream.Close()
	}()

	var headers http.Header
	select {
	case headers = <-capturedHeaders:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for captured stream request")
	}

	assert.Equal(t, legacyCodexOriginator(), headers.Get("Originator"))
	assert.Equal(t, legacyCodexUserAgent(), headers.Get("User-Agent"))
	assert.Contains(t, strings.ToLower(headers.Get("User-Agent")), legacyCodexOriginator())
	assert.Equal(t, testChatAccountID, headers.Get("Chatgpt-Account-Id"))
	assert.Equal(t, "Bearer "+accessToken, headers.Get("Authorization"))
}

func TestCodexOutbound_CustomizeExecutorAggregatesNonStreamRequests(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, false)
	executor := outbound.CustomizeExecutor(&mockCodexExecutor{
		streamEvents: []*httpclient.StreamEvent{
			{Type: "response.created", Data: []byte(`{"type":"response.created","sequence_number":0,"response":{"id":"resp_test_123","object":"response","created_at":1700000000,"model":"gpt-5-codex","status":"in_progress","output":[]}}`)},
			{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"msg_test_456","type":"message","status":"in_progress","role":"assistant"}}`)},
			{Type: "response.content_part.added", Data: []byte(`{"type":"response.content_part.added","sequence_number":2,"item_id":"msg_test_456","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`)},
			{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","sequence_number":3,"item_id":"msg_test_456","output_index":0,"content_index":0,"delta":"Hello"}`)},
			{Type: "response.output_text.done", Data: []byte(`{"type":"response.output_text.done","sequence_number":4,"item_id":"msg_test_456","output_index":0,"content_index":0,"text":"Hello"}`)},
			{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","sequence_number":5,"output_index":0,"item":{"id":"msg_test_456","type":"message","status":"completed","role":"assistant"}}`)},
			{Type: "response.completed", Data: []byte(`{"type":"response.completed","sequence_number":6,"response":{"id":"resp_test_123","object":"response","created_at":1700000000,"model":"gpt-5-codex","status":"completed","output":[]}}`)},
		},
	})

	response, err := executor.Do(ctx, request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "application/json", response.Headers.Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body, &body))
	assert.Equal(t, "resp_test_123", body["id"])
	assert.Equal(t, "completed", body["status"])
	assert.Equal(t, "gpt-5-codex", body["model"])
}

var _ pipeline.ChannelCustomizedExecutor = (*OutboundTransformer)(nil)

type mockCodexExecutor struct {
	streamEvents []*httpclient.StreamEvent
}

func (m *mockCodexExecutor) Do(_ context.Context, _ *httpclient.Request) (*httpclient.Response, error) {
	return nil, assert.AnError
}

func (m *mockCodexExecutor) DoStream(_ context.Context, _ *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.SliceStream(m.streamEvents), nil
}

func TestCodexOutbound_DoesNotInjectCLIInstructions(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Stream: lo.ToPtr(true),
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)

	_, hasInstructions := body["instructions"]
	assert.False(t, hasInstructions)
	assert.NotContains(t, string(hreq.Body), "You are a coding agent running in the Codex CLI")
	assert.NotContains(t, string(hreq.Body), "You are Codex")
	assert.Equal(t, false, body["store"])
}

func TestCodexOutbound_PreservesMinimalCompatTransforms(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)
	store := true
	parallelToolCalls := false
	maxTokens := int64(128)
	maxCompletionTokens := int64(256)
	topP := 0.8
	serviceTier := "flex"
	reasoningSummary := "detailed"

	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Tools: []llm.Tool{{
			Type: "function",
			Function: llm.Function{
				Name:       "shell",
				Parameters: []byte(`{"type":"object","properties":{}}`),
			},
		}},
		Store:               &store,
		ParallelToolCalls:   &parallelToolCalls,
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		TopP:                &topP,
		ServiceTier:         &serviceTier,
		ReasoningSummary:    &reasoningSummary,
		Metadata:            map[string]string{"source": "caller"},
		TransformerMetadata: map[string]any{},
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)

	assert.Equal(t, false, body["store"])
	assert.Equal(t, true, body["stream"])
	assert.NotContains(t, body, "max_output_tokens")
	assert.Equal(t, true, body["parallel_tool_calls"])
	assert.Equal(t, topP, body["top_p"])
	assert.Equal(t, serviceTier, body["service_tier"])
	assert.NotContains(t, body, "metadata")
	assert.Equal(t, []any{"reasoning.encrypted_content"}, body["include"])

	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, reasoningSummary, reasoning["summary"])

	assert.NotContains(t, string(hreq.Body), "You are a coding agent running in the Codex CLI")
	assert.NotContains(t, string(hreq.Body), "You are Codex")
}

func TestCodexOutbound_AppliesReasoningDefaultsWhenMissing(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Tools: []llm.Tool{{
			Type: "function",
			Function: llm.Function{
				Name:       "shell",
				Parameters: []byte(`{"type":"object","properties":{}}`),
			},
		}},
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)
	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, true, body["parallel_tool_calls"])
	assert.Equal(t, []any{"reasoning.encrypted_content"}, body["include"])
	assert.Equal(t, "auto", reasoning["summary"])
	assert.NotContains(t, body, "metadata")
}

func newTestCodexOutbound(t *testing.T) *OutboundTransformer {
	t.Helper()

	accessToken := testAccessTokenWithAccountID(t)

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	return outbound
}

func decodeCodexRequestBody(t *testing.T, hreq *httpclient.Request) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.Unmarshal(hreq.Body, &body))

	return body
}

func buildCodexStreamRequest(t *testing.T, ctx context.Context, outbound *OutboundTransformer, withInboundIdentity bool) *httpclient.Request {
	t.Helper()

	bodyBytes, err := json.Marshal(map[string]any{
		"model":  "gpt-5-codex",
		"stream": true,
		"messages": []map[string]any{{
			"role":    "user",
			"content": "Hello",
		}},
	})
	require.NoError(t, err)

	rawReq, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	rawReq.Header.Set("Accept", "application/json")
	rawReq.Header.Set("Connection", "keep-alive")
	rawReq.Header.Set("Content-Type", "application/json")
	rawReq.Header.Set("Conversation_id", "legacy-conversation")
	rawReq.Header.Set("Openai-Beta", "responses=experimental")
	rawReq.Header.Set("Session_id", "provided-session")
	rawReq.Header.Set("Version", "9.9.9")
	if withInboundIdentity {
		rawReq.Header.Set("Originator", legacyCodexOriginator())
		rawReq.Header.Set("User-Agent", legacyCodexUserAgent())
	}

	inbound := openai.NewInboundTransformer()
	inboundRequest, err := httpclient.ReadHTTPRequest(rawReq)
	require.NoError(t, err)

	llmReq, err := inbound.TransformRequest(ctx, inboundRequest)
	require.NoError(t, err)
	llmReq.RawRequest = inboundRequest

	outboundRequest, err := outbound.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	outboundRequest = httpclient.MergeInboundRequest(outboundRequest, inboundRequest)
	outboundRequest, err = httpclient.FinalizeAuthHeaders(outboundRequest)
	require.NoError(t, err)

	return outboundRequest
}

func legacyCodexOriginator() string {
	return "codex" + "_cli_rs"
}

func legacyCodexUserAgent() string {
	return legacyCodexOriginator() + "/0.50.0 (macOS 14.0.0; arm64) Terminal"
}
