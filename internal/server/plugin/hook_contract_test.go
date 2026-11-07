package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestHookPatchContract(t *testing.T) {
	before := json.RawMessage(`{"items":[1,2],"a/b":{"~key":"old"},"remove":true}`)
	patch := json.RawMessage(`[
 {"op":"test","path":"/items/0","value":1},
 {"op":"add","path":"/items/-","value":3},
 {"op":"replace","path":"/a~1b/~0key","value":"new"},
 {"op":"copy","from":"/a~1b","path":"/copy"},
 {"op":"move","from":"/items/0","path":"/items/1"},
 {"op":"remove","path":"/remove"}]`)
	got, err := applyHookReturn(before, patch)
	require.NoError(t, err)
	require.JSONEq(t, `{"items":[2,1,3],"a/b":{"~key":"new"},"copy":{"~key":"new"}}`, string(got))
	for _, bad := range []string{
		`[{"op":"add","path":"/added","value":true},{"op":"remove","path":"/missing"}]`,
		`[{"op":"test","path":"/items/0","value":99}]`,
		`[{"op":"replace","path":"/items/-1","value":0}]`,
		`[{"op":"replace","path":"","value":[]}]`,
		`[{"op":"add","path":"bad/path","value":0}]`,
		`[{"op":"add","path":"/~2","value":0}]`,
		`[{"op":"test","path":"/items"}]`,
		`null`, `1`,
	} {
		_, err := applyHookReturn(before, json.RawMessage(bad))
		require.Error(t, err)
	}
	require.JSONEq(t, `{"items":[1,2],"a/b":{"~key":"old"},"remove":true}`, string(before))
	got, err = applyHookReturn(before, json.RawMessage(`[{"op":"replace","path":"","value":{"new":1}}]`))
	require.NoError(t, err)
	require.JSONEq(t, `{"new":1}`, string(got))
}

func TestHookInvalidTypesAccumulateFailures(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installed := installHook(t, m, "acme.invalid-types", true, false, `export function onRequest(ctx, req) { ctx.state.changed=true; return {...req,temperature:"bad"}; }`)
	ctx := contexts.EnsureContainer(pluginCtx())
	h := testRequestHooks(t, m, ctx).Request.(*requestHookMiddleware)
	defer h.OnProcessEnd()
	for range 3 {
		_, err := h.OnInboundLlmRequest(ctx, hookRequest(t, "keep"))
		require.NoError(t, err)
	}
	require.Equal(t, 3, m.ConsecutiveFailures(installed.ID))
	require.Empty(t, h.session.states[installed.ID])
}

func TestRequestHookPatchUsesOriginalAndPreservesPayload(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installed := installHook(t, m, "acme.patch", true, false, `export function onRequest(ctx, req) {
 req.model="in-place"; ctx.state.ok=true;
 return [{op:"add",path:"/temperature",value:0.2}];
 }`)
	ctx := contexts.EnsureContainer(pluginCtx())
	h := testRequestHooks(t, m, ctx).Request.(*requestHookMiddleware)
	defer h.OnProcessEnd()
	req := hookRequest(t, "keep")
	req.Transcription = &llm.TranscriptionRequest{File: []byte("audio"), FileName: "sample.wav"}
	req.Image = &llm.ImageRequest{Images: [][]byte{[]byte("image")}, Mask: []byte("mask"), ModelSpecified: true}
	got, err := h.OnInboundLlmRequest(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "gpt-4", got.Model)
	require.Equal(t, 0.2, *got.Temperature)
	require.Equal(t, []byte("audio"), got.Transcription.File)
	require.Equal(t, "sample.wav", got.Transcription.FileName)
	require.Equal(t, []byte("image"), got.Image.Images[0])
	require.Equal(t, []byte("mask"), got.Image.Mask)
	require.JSONEq(t, `{"ok":true}`, string(h.session.states[installed.ID]))
}

func TestAttemptHookMatchesWireFormatAndIsAtomic(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installed := installHook(t, m, "acme.wire", false, true, `export function onAttempt(ctx, body) {
 ctx.state.calls=(ctx.state.calls||0)+1;
 if (body.fail) return [{op:"add",path:"/edited",value:true},{op:"remove",path:"/missing"}];
 if (body.protected) return {model:"changed",stream:false};
 return [{op:"add",path:"/vendor",value:ctx.attempt.apiFormat},{op:"add",path:"/calls",value:ctx.state.calls}];
 }`)
	ctx := contexts.EnsureContainer(pluginCtx())
	h := testRequestHooks(t, m, ctx).Request.(*requestHookMiddleware)
	defer h.OnProcessEnd()
	_, err := h.OnInboundLlmRequest(ctx, hookRequest(t, "keep"))
	require.NoError(t, err)
	_, err = h.OnOutboundLlmRequest(ctx, hookRequest(t, "keep"), llm.APIFormatOpenAIChatCompletion)
	require.NoError(t, err)
	raw := &httpclient.Request{APIFormat: string(llm.APIFormatAnthropicMessage), Body: []byte(`{"model":"gpt-4","stream":false,"system":"wire-only"}`)}
	got, err := h.OnAttemptRawRequest(ctx, raw)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-4","stream":false,"system":"wire-only","vendor":"anthropic/messages","calls":1}`, string(got.Body))
	require.NotContains(t, string(raw.Body), "vendor")
	for _, body := range []string{`{"model":"gpt-4","stream":false,"fail":true}`, `{"model":"gpt-4","stream":false,"protected":true}`} {
		raw.Body = []byte(body)
		got, err = h.OnAttemptRawRequest(ctx, raw)
		require.NoError(t, err)
		require.Equal(t, body, string(got.Body))
		require.JSONEq(t, `{"calls":1}`, string(h.session.states[installed.ID]))
	}
	raw.APIFormat = string(llm.APIFormatGeminiContents)
	raw.Body = []byte(`{"contents":[]}`)
	got, err = h.OnAttemptRawRequest(ctx, raw)
	require.NoError(t, err)
	require.Same(t, raw, got)
	raw.APIFormat = string(llm.APIFormatAnthropicMessage)
	raw.ContentType = "multipart/form-data; boundary=test"
	got, err = h.OnAttemptRawRequest(ctx, raw)
	require.NoError(t, err)
	require.Same(t, raw, got)
}

func TestHookRegistrationValidation(t *testing.T) {
	for _, hooks := range []string{
		`{"request":true}`, `{"request":{"requestTypes":["chat"]}}`,
		`{"request":{"enabled":true,"requestTypes":[]}}`,
		`{"request":{"enabled":true,"requestTypes":["chat","chat"]}}`,
		`{"attempt":{"enabled":true,"apiFormats":["unknown"]}}`,
		`{"attempt":{"enabled":false,"apiFormats":["unknown"]}}`,
		`{"attempt":{"enabled":false,"extra":1}}`,
	} {
		raw := mutateManifest(t, func(t *testing.T, manifest map[string]any) {
			var h any
			require.NoError(t, json.Unmarshal([]byte(hooks), &h))
			manifest["hooks"] = h
		})
		_, err := ParseManifest(raw, "v1.0.0")
		require.Error(t, err, hooks)
	}
}

func TestHookScopesSkipSerializationAndRequireUpgradePreview(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installed := installHook(t, m, "acme.scope", true, false, `export function onRequest(ctx, req) { throw new Error("must not run"); }`)
	ctx := contexts.EnsureContainer(pluginCtx())
	h := testRequestHooks(t, m, ctx).Request.(*requestHookMiddleware)
	defer h.OnProcessEnd()
	req := hookRequest(t, "keep")
	req.RequestType = llm.RequestTypeEmbedding
	got, err := h.OnInboundLlmRequest(ctx, req)
	require.NoError(t, err)
	require.Same(t, req, got)
	require.Zero(t, m.ConsecutiveFailures(installed.ID))
	old := sampleManifest(t)
	next := sampleManifest(t)
	old.Hooks = &objects.PluginHooksSpec{Request: &objects.PluginRequestHook{Enabled: true, RequestTypes: []llm.RequestType{llm.RequestTypeChat}}}
	next.Hooks = &objects.PluginHooksSpec{Request: &objects.PluginRequestHook{Enabled: true, RequestTypes: []llm.RequestType{llm.RequestTypeChat, llm.RequestTypeEmbedding}}}
	require.Equal(t, []string{"request: chat, embedding"}, DiffManifest(&old, &next).Hooks)
	next.Hooks.Request.Enabled = false
	require.Empty(t, DiffManifest(&old, &next).Hooks)
	t.Run("disabled", func(t *testing.T) {
		disabled := newTestManager(t, newPluginDB(t))
		installHook(t, disabled, "acme.disabled", false, false, `export function onRequest() { throw new Error("disabled"); }`)
		middleware := testRequestHooks(t, disabled, ctx).Request
		if middleware != nil {
			defer middleware.OnProcessEnd()
		}
		require.Nil(t, middleware)
	})
}
