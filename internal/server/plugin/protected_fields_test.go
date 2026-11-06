package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestAttemptProtectionByProtocol(t *testing.T) {
	for _, tc := range []struct {
		name               string
		format             llm.APIFormat
		before, after, url string
		fields             []string
	}{
		{name: "chat absent false equivalent", format: llm.APIFormatOpenAIChatCompletion, before: `{"model":"m"}`, after: `{"model":"m","stream":false}`},
		{name: "chat true removal", format: llm.APIFormatOpenAIChatCompletion, before: `{"model":"m","stream":true}`, after: `{"model":"m"}`, fields: []string{"stream"}},
		{name: "model removal", format: llm.APIFormatAnthropicMessage, before: `{"model":"m"}`, after: `{}`, fields: []string{"model"}},
		{name: "anthropic business fields", format: llm.APIFormatAnthropicMessage, before: `{"model":"m","type":"a"}`, after: `{"model":"m","type":"b","generate":false,"stream_format":"custom"}`},
		{name: "gemini business fields", format: llm.APIFormatGeminiContents, before: `{"contents":[],"type":"a"}`, after: `{"contents":[],"type":"b","generate":false}`, url: "https://example.com/models/m:generateContent"},
		{name: "gemini conflicting model", format: llm.APIFormatGeminiContents, before: `{"contents":[]}`, after: `{"contents":[],"model":"other"}`, fields: []string{"model"}},
		{name: "gemini consistent model", format: llm.APIFormatGeminiContents, before: `{"contents":[]}`, after: `{"contents":[],"model":"models/m"}`},
		{name: "gemini stream URL", format: llm.APIFormatGeminiContents, before: `{}`, after: `{"stream":false}`, url: "https://example.com/models/m:streamGenerateContent?alt=sse", fields: []string{"stream"}},
		{name: "gemini consistent stream", format: llm.APIFormatGeminiContents, before: `{}`, after: `{"stream":true}`, url: "https://example.com/models/m:streamGenerateContent?alt=sse"},
		{name: "embedding batch model", format: llm.APIFormatGeminiEmbedding, before: `{"requests":[{"model":"models/m"}]}`, after: `{"requests":[{"model":"models/other"}]}`, fields: []string{"requests/*/model"}},
		{name: "ollama missing means true", format: llm.APIFormatOllamaChat, before: `{"model":"m","stream":false}`, after: `{"model":"m"}`, fields: []string{"stream"}},
		{name: "ollama true equivalent", format: llm.APIFormatOllamaChat, before: `{"model":"m","stream":true}`, after: `{"model":"m"}`},
		{name: "ws defaults", format: llm.APIFormatOpenAIResponseWebSocket, before: `{"model":"m"}`, after: `{"model":"m","generate":true,"type":"response.create"}`},
		{name: "ws warmup", format: llm.APIFormatOpenAIResponseWebSocket, before: `{"model":"m"}`, after: `{"model":"m","generate":false}`, fields: []string{"generate"}},
		{name: "ws frame type", format: llm.APIFormatOpenAIResponseWebSocket, before: `{"model":"m"}`, after: `{"model":"m","type":"other"}`, fields: []string{"type"}},
		{name: "speech default audio", format: llm.APIFormatOpenAISpeech, before: `{"model":"m","stream_format":"audio"}`, after: `{"model":"m"}`},
		{name: "speech SSE removal", format: llm.APIFormatOpenAISpeech, before: `{"model":"m","stream_format":"sse"}`, after: `{"model":"m"}`, fields: []string{"stream_format"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := protectedAttemptFields(&httpclient.Request{Body: []byte(tc.before), APIFormat: string(tc.format), URL: tc.url}, []byte(tc.after), "m")
			require.ElementsMatch(t, tc.fields, changed)
		})
	}
}

func TestProtectedDropsObservableWithoutFailureOrStateCommit(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installed := installHook(t, m, "acme.protected-drops", true, true, `
export function onRequest(ctx, req) { ctx.state.leaked=true; return {...req,stream:true}; }
export function onAttempt(ctx, body) { ctx.state.leaked=true; return body.patch; }
`)
	ctx := contexts.WithRequestID(contexts.EnsureContainer(pluginCtx()), "protected-drop-request")
	h := testRequestHooks(t, m, ctx).Request.(*requestHookMiddleware)
	defer h.OnProcessEnd()
	m.noteCallFailure(installed.ID)
	got, err := h.OnInboundLlmRequest(ctx, hookRequest(t, "keep"))
	require.NoError(t, err)
	require.False(t, streaming(got.Stream))
	require.Equal(t, int64(1), m.hookCounter(installed.ID).protectedDrops)
	require.Equal(t, 1, m.ConsecutiveFailures(installed.ID))
	require.Empty(t, h.session.states[installed.ID])
	_, err = h.OnOutboundLlmRequest(ctx, got, llm.APIFormatOpenAIChatCompletion)
	require.NoError(t, err)
	returns := []string{
		`{"model":"other","stream":true}`,
		`[{"op":"remove","path":"/model"}]`,
		`[{"op":"replace","path":"","value":{"model":"other","stream":true}}]`,
		`[{"op":"copy","from":"/other","path":"/model"}]`,
		`[{"op":"move","from":"/other","path":"/model"}]`,
	}
	for i, ret := range returns {
		body := []byte(`{"model":"gpt-4","stream":false,"other":"secret-model-value","patch":` + ret + `}`)
		wire := &httpclient.Request{Body: body, APIFormat: string(llm.APIFormatOpenAIChatCompletion)}
		result, err := h.OnAttemptRawRequest(ctx, wire)
		require.NoError(t, err)
		require.JSONEq(t, string(body), string(result.Body))
		require.Equal(t, int64(i+2), m.hookCounter(installed.ID).protectedDrops)
		require.Empty(t, h.session.states[installed.ID])
		require.Equal(t, 1, m.ConsecutiveFailures(installed.ID))
	}
	statuses := m.RuntimeStatusViews()
	require.Len(t, statuses, 1)
	require.Equal(t, 6, statuses[0].ProtectedDrops)
	// Invalid patches count as failures, not protected drops.
	_, err = h.OnAttemptRawRequest(ctx, &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIChatCompletion), Body: []byte(`{"model":"gpt-4","patch":[{"op":"remove","path":"/missing"}]}`)})
	require.NoError(t, err)
	require.Equal(t, int64(6), m.hookCounter(installed.ID).protectedDrops)
	require.Equal(t, 2, m.ConsecutiveFailures(installed.ID))
}

func TestProtectedFieldsValidatePatchFinalResult(t *testing.T) {
	before := json.RawMessage(`{"model":"m","stream":false}`)
	after, err := applyHookReturn(before, json.RawMessage(`[{"op":"replace","path":"/model","value":"temporary"},{"op":"replace","path":"/model","value":"m"}]`))
	require.NoError(t, err)
	require.Empty(t, protectedAttemptFields(&httpclient.Request{APIFormat: string(llm.APIFormatOpenAIChatCompletion), Body: before}, after, "m"))
}

func TestAttemptProtectionCoversAllAPIFormats(t *testing.T) {
	formats := llm.AllAPIFormats()
	require.Len(t, attemptProtocolControls, len(formats))
	for _, format := range formats {
		require.Contains(t, attemptProtocolControls, format, "new API formats require an explicit protection policy")
	}
}
