package plugin

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestAcquireHookGenerationRetriesRefresh(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installHook(t, m, "acme.refresh", true, false, `export function onRequest(ctx, req) { return req; }`)
	old := m.current()
	first := true
	gen, err := acquireHookGeneration(t.Context(), func() *Generation {
		if first {
			first = false
			require.NoError(t, m.ForceRefresh(pluginCtx()))
			require.NotSame(t, old, m.current())
			return old // Refresh completed after the caller captured the old pointer.
		}
		return m.current()
	})
	require.NoError(t, err)
	require.Same(t, m.current(), gen)
	gen.Release()

	// A closed generation that remains current must return an error, not no hooks.
	m.retire(gen)
	_, err = acquireHookGeneration(t.Context(), m.current)
	require.ErrorIs(t, err, ErrGenerationClosed)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = acquireHookGeneration(ctx, m.current)
	require.ErrorIs(t, err, context.Canceled)
}

func TestHookGenerationReleasedOnEarlyPipelineFailure(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installHook(t, m, "acme.lifecycle", true, false, `export function onRequest(ctx, req) { return req; }`)
	ctx := contexts.EnsureContainer(pluginCtx())
	hooks := testRequestHooks(t, m, ctx)
	session := hooks.Request.(*requestHookMiddleware).session
	gen := session.gen
	pipe := pipeline.NewFactory(nil).Pipeline(openai.NewInboundTransformer(), nil,
		pipeline.WithMiddlewares(hooks.Request, hooks.Attempt))
	_, err := pipe.Process(ctx, &httpclient.Request{Body: []byte(`{`)})
	require.Error(t, err)
	require.True(t, session.released)
	require.Nil(t, session.gen)
	gen.mu.Lock()
	refs := gen.refs
	gen.mu.Unlock()
	require.Zero(t, refs)
}

func TestRequestHookSpeechTransportProtectionIsAtomic(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installed := installUI(t, m, uiZip(t, func(t *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.speech"
		hooksOf(t, manifest)["request"] = map[string]any{"enabled": true, "requestTypes": []string{"speech"}}
	}, map[string]string{"ui/page.html": "<html></html>", "plugin.js": `
 export function onRequest(ctx, req) {
  ctx.state.changed = true;
  const speech = {...req.speech, voice: "changed", stream_format: req.user};
  if (req.user === "delete") delete speech.stream_format;
  return {...req, speech};
 }`}))
	require.NoError(t, m.Enable(pluginCtx(), installed.ID, nil, nil))
	for _, tc := range []struct {
		before, after string
		allowed       bool
	}{
		{"audio", "sse", false},
		{"sse", "audio", false},
		{"audio", "delete", false},
		{"", "sse", false},
		{"audio", "audio", true},
		{"sse", "sse", true},
	} {
		t.Run(tc.before+"/"+tc.after, func(t *testing.T) {
			ctx := contexts.EnsureContainer(pluginCtx())
			hooks := testRequestHooks(t, m, ctx)
			defer hooks.Request.OnProcessEnd()
			req := &llm.Request{
				RequestType: llm.RequestTypeSpeech, APIFormat: llm.APIFormatOpenAISpeech,
				Model: "tts-1", Stream: lo.ToPtr(true), User: lo.ToPtr(tc.after),
				Speech: &llm.SpeechRequest{Input: "hello", Voice: "original", StreamFormat: tc.before},
			}
			got, err := hooks.Request.OnInboundLlmRequest(ctx, req)
			require.NoError(t, err)
			require.Equal(t, tc.before, got.Speech.StreamFormat)
			state := hooks.Request.(*requestHookMiddleware).session.states[installed.ID]
			if tc.allowed {
				require.Equal(t, "changed", got.Speech.Voice)
				require.JSONEq(t, `{"changed":true}`, string(state))
			} else {
				require.Equal(t, "original", got.Speech.Voice)
				require.Empty(t, state)
			}
		})
	}
}
