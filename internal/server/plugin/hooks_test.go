package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestHookReturnContract(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installed := installHook(t, manager, "acme.request-contract", true, false, `
export function onRequest(ctx, req) {
  if (req.user === "throw") throw new Error("boom");
  if (req.user === "null") return null;
  if (req.user === "number") return 1;
  if (req.user === "array") return [req];
  if (req.user === "undefined") { req.temperature = 0.9; return; }
  if (req.user === "reject") { ctx.reject(403, "blocked"); return req; }
  if (req.user === "catch-reject") {
    try { ctx.reject(400, "bad request"); } catch (e) {}
    return { ...req, temperature: 0.8 };
  }
  if (req.user === "illegal-reject") {
    try { ctx.reject(500, "nope"); } catch (e) {}
    return { ...req, temperature: 0.2 };
  }
  if (req.user === "blank-reject") {
    try { ctx.reject(403, "   "); } catch (e) {}
    return { ...req, temperature: 0.3 };
  }
  if (req.user === "stream") return { ...req, stream: true, temperature: 0.1 };
  if (req.user === "state-fail") {
    ctx.state.n = 1;
    throw new Error("discard");
  }
  if (req.user === "state-read") return { ...req, user: String(ctx.state.n || 0) };
  if (req.user === "commit") {
    ctx.state.n = (ctx.state.n || 0) + 1;
    return { ...req, user: String(ctx.state.n), temperature: 0.4 };
  }
  return {
    ...req,
    model: "rewritten",
    temperature: 0.4,
    raw_request: { body: "aGFja2Vk" },
    request_type: "embedding",
    api_format: "anthropic/messages",
  };
}
`)
	ctx := contexts.WithRequestID(contexts.EnsureContainer(pluginCtx()), "req-contract")
	middleware := testRequestHooks(t, manager, ctx).Request
	require.NotNil(t, middleware)
	t.Cleanup(middleware.OnProcessEnd)
	hook := middleware.(*requestHookMiddleware)

	t.Run("applies public fields and restores internal fields", func(t *testing.T) {
		request := hookRequest(t, "keep")
		got, err := hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Same(t, request, got)
		require.Equal(t, "rewritten", got.Model)
		require.InDelta(t, 0.4, lo.FromPtr(got.Temperature), 0.0001)
		require.Equal(t, llm.RequestTypeChat, got.RequestType)
		require.Equal(t, llm.APIFormatOpenAIChatCompletion, got.APIFormat)
		require.True(t, lo.FromPtr(got.TransformOptions.ArrayInstructions))
		require.Equal(t, "kept", got.TransformerMetadata["marker"])
		require.Equal(t, []byte(`{"model":"gpt-4"}`), got.RawRequest.Body)
		require.Equal(t, "secret", got.ProviderExtensions.OpenAIResponses.Request.ReasoningContext)
		require.Equal(t, []int{2, 4}, got.ProviderExtensions.OpenAIResponses.Request.OmittedInputItemIndices)
	})

	t.Run("undefined keeps the request", func(t *testing.T) {
		request := hookRequest(t, "undefined")
		got, err := hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Nil(t, got.Temperature)
		require.Equal(t, "gpt-4", got.Model)
	})

	for _, user := range []string{"null", "number", "array", "throw"} {
		t.Run(user, func(t *testing.T) {
			before := manager.ConsecutiveFailures(installed.ID)
			request := hookRequest(t, user)
			got, err := hook.OnInboundLlmRequest(ctx, request)
			require.NoError(t, err)
			require.Equal(t, "gpt-4", got.Model)
			require.Nil(t, got.Temperature)
			require.Equal(t, before+1, manager.ConsecutiveFailures(installed.ID))
		})
	}

	t.Run("legal reject stops the request", func(t *testing.T) {
		before := manager.ConsecutiveFailures(installed.ID)
		request := hookRequest(t, "reject")
		_, err := hook.OnInboundLlmRequest(ctx, request)
		var response *llm.ResponseError
		require.ErrorAs(t, err, &response)
		require.Equal(t, http.StatusForbidden, response.StatusCode)
		require.Equal(t, "blocked", response.Detail.Message)
		require.Equal(t, before, manager.ConsecutiveFailures(installed.ID))
	})

	t.Run("caught reject still rejects", func(t *testing.T) {
		request := hookRequest(t, "catch-reject")
		_, err := hook.OnInboundLlmRequest(ctx, request)
		var response *llm.ResponseError
		require.ErrorAs(t, err, &response)
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
		require.Nil(t, request.Temperature)
	})

	t.Run("illegal reject does not reject", func(t *testing.T) {
		request := hookRequest(t, "illegal-reject")
		got, err := hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.InDelta(t, 0.2, lo.FromPtr(got.Temperature), 0.0001)
		require.Equal(t, "gpt-4", got.Model)
	})

	t.Run("blank reject does not reject", func(t *testing.T) {
		request := hookRequest(t, "blank-reject")
		got, err := hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.InDelta(t, 0.3, lo.FromPtr(got.Temperature), 0.0001)
	})

	t.Run("stream change drops the edit", func(t *testing.T) {
		before := manager.ConsecutiveFailures(installed.ID)
		request := hookRequest(t, "stream")
		request.Stream = lo.ToPtr(false)
		got, err := hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.False(t, lo.FromPtr(got.Stream))
		require.Nil(t, got.Temperature)
		require.Equal(t, before, manager.ConsecutiveFailures(installed.ID))
	})

	t.Run("failed call discards state", func(t *testing.T) {
		request := hookRequest(t, "state-fail")
		got, err := hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Equal(t, "state-fail", lo.FromPtr(got.User))
		request = hookRequest(t, "state-read")
		got, err = hook.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Equal(t, "0", lo.FromPtr(got.User))
	})

	t.Run("successful call commits state", func(t *testing.T) {
		first := hookRequest(t, "commit")
		got, err := hook.OnInboundLlmRequest(ctx, first)
		require.NoError(t, err)
		require.Equal(t, "1", lo.FromPtr(got.User))
		second := hookRequest(t, "commit")
		got, err = hook.OnInboundLlmRequest(ctx, second)
		require.NoError(t, err)
		require.Equal(t, "2", lo.FromPtr(got.User))
	})
}

func TestRequestHookPassThroughAccounting(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	modelOnly := installHook(t, manager, "acme.model-only", true, true, `
export function onRequest(ctx, req) { return { ...req, model: "mapped" }; }
export function onAttempt(ctx, req) { return { ...req, model: req.model }; }
`)
	rewriter := installHook(t, manager, "acme.rewriter", true, true, `
export function onRequest(ctx, req) { return { ...req, model: "mapped", temperature: 0.2 }; }
export function onAttempt(ctx, req) { return { ...req, user: "attempt" }; }
`)
	attemptOnly := installHook(t, manager, "acme.attempt-only", false, true, `
export function onAttempt(ctx, body) {
  ctx.state.calls = (ctx.state.calls || 0) + 1;
  return { ...body, attempt_only: true };
}
`)
	ctx := contexts.EnsureContainer(pluginCtx())
	middleware := testRequestHooks(t, manager, ctx).Request
	require.NotNil(t, middleware)
	t.Cleanup(middleware.OnProcessEnd)
	hook := middleware.(*requestHookMiddleware)
	request := hookRequest(t, "keep")
	_, err := hook.OnInboundLlmRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "mapped", request.Model)

	for attempt := 1; attempt <= 2; attempt++ {
		previousState := append(json.RawMessage(nil), hook.session.states[attemptOnly.ID]...)
		_, err = hook.OnOutboundLlmRequest(ctx, request, llm.APIFormatOpenAIChatCompletion)
		require.NoError(t, err)
		require.Equal(t, previousState, hook.session.states[attemptOnly.ID], "metadata callback must not execute onAttempt")

		manager.PassThroughApplied(ctx, 9)
		require.Equal(t, int64(0), manager.hookCounter(modelOnly.ID).overridden)
		require.Equal(t, int64(attempt), manager.hookCounter(rewriter.ID).overridden)
		require.Equal(t, int64(0), manager.hookCounter(attemptOnly.ID).overridden)

		wire, err := hook.OnAttemptRawRequest(ctx, &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
			Body:      []byte(`{"model":"mapped","temperature":0.7}`),
		})
		require.NoError(t, err)
		var body struct {
			AttemptOnly bool   `json:"attempt_only"`
			User        string `json:"user"`
		}
		require.NoError(t, json.Unmarshal(wire.Body, &body))
		require.True(t, body.AttemptOnly)
		require.Equal(t, "attempt", body.User)
		var state struct {
			Calls int `json:"calls"`
		}
		require.NoError(t, json.Unmarshal(hook.session.states[attemptOnly.ID], &state))
		require.Equal(t, attempt, state.Calls)

		manager.PassThroughApplied(ctx, 9)
		require.Equal(t, int64(attempt), manager.hookCounter(rewriter.ID).overridden, "count at most once per attempt")
		require.Equal(t, int64(0), manager.hookCounter(attemptOnly.ID).overridden, "wire-body edits are not pass-through overrides")
	}
}

func TestRequestHookOverloadDoesNotCountFailure(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installed := installHook(t, manager, "acme.overload", true, false, `
export function onRequest(ctx, req) { return new Promise(() => {}); }
`)
	attempt := installHook(t, manager, "acme.attempt-overload", false, true, `
export function onAttempt() { return new Promise(() => {}); }
`)
	blockCtx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); workers.Wait() })
	for range 16 {
		workers.Go(func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("blocked invoke panicked: %v", recovered)
				}
			}()
			_, _ = manager.Invoke(blockCtx, InvokeRequest{
				InstallationID: installed.ID,
				Export:         onRequestExport,
				Args:           json.RawMessage(`[{}]`),
				State:          json.RawMessage(`{}`),
				Timeout:        time.Minute,
			})
		})
	}
	require.Eventually(t, func() bool { return manager.VMCount() == 16 }, 5*time.Second, 10*time.Millisecond)

	before := manager.ConsecutiveFailures(installed.ID)
	ctx := contexts.WithRequestID(contexts.EnsureContainer(pluginCtx()), "req-overload")
	middleware := testRequestHooks(t, manager, ctx).Request
	require.NotNil(t, middleware)
	t.Cleanup(middleware.OnProcessEnd)
	_, err := middleware.(*requestHookMiddleware).OnInboundLlmRequest(ctx, hookRequest(t, "keep"))
	var response *llm.ResponseError
	require.ErrorAs(t, err, &response)
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
	require.Equal(t, "plugin_overloaded", response.Detail.Code)
	require.Equal(t, "rate_limit_error", response.Detail.Type)
	require.Equal(t, "plugin acme.overload is at capacity; please retry shortly", response.Detail.Message)
	require.Equal(t, before, manager.ConsecutiveFailures(installed.ID))
	require.Equal(t, int64(1), manager.hookCounter(installed.ID).overloaded)

	blockAttempt, cancelAttempt := context.WithCancel(context.Background())
	var attemptWorkers sync.WaitGroup
	t.Cleanup(func() { cancelAttempt(); attemptWorkers.Wait() })
	for range 16 {
		attemptWorkers.Go(func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("blocked invoke panicked: %v", recovered)
				}
			}()
			_, _ = manager.Invoke(blockAttempt, InvokeRequest{
				InstallationID: attempt.ID,
				Export:         onAttemptExport,
				Args:           json.RawMessage(`[{}]`),
				State:          json.RawMessage(`{}`),
				Timeout:        time.Minute,
			})
		})
	}
	require.Eventually(t, func() bool { return manager.VMCount() == 32 }, 5*time.Second, 10*time.Millisecond)
	before = manager.ConsecutiveFailures(attempt.ID)
	_, err = middleware.(*requestHookMiddleware).OnOutboundLlmRequest(ctx, hookRequest(t, "keep"), llm.APIFormatOpenAIChatCompletion)
	require.NoError(t, err)
	_, err = middleware.(*requestHookMiddleware).OnAttemptRawRequest(ctx, &httpclient.Request{Body: []byte(`{"model":"gpt-4"}`), APIFormat: string(llm.APIFormatOpenAIChatCompletion)})
	var overloaded *orchestrator.PluginOverloadedError
	require.ErrorAs(t, err, &overloaded)
	require.ErrorAs(t, err, &response)
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
	require.False(t, errors.Is(err, context.DeadlineExceeded))
	require.Equal(t, before, manager.ConsecutiveFailures(attempt.ID))
	require.Equal(t, int64(1), manager.hookCounter(attempt.ID).overloaded)
}

func installHook(t *testing.T, manager *Manager, id string, request, attempt bool, source string) *ent.Plugin {
	t.Helper()
	installed := installUI(t, manager, uiZip(t, func(t *testing.T, manifest map[string]any) {
		manifest["id"] = id
		manifest["name"] = id
		hooksOf(t, manifest)["request"] = map[string]any{"enabled": request, "requestTypes": []string{"chat"}}
		hooksOf(t, manifest)["attempt"] = map[string]any{"enabled": attempt, "apiFormats": []string{"openai/chat_completions", "anthropic/messages"}}
	}, map[string]string{
		"ui/page.html": "<html></html>",
		"plugin.js":    source,
	}))
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, nil, nil))
	current := manager.Current().ByID(installed.ID)
	require.NotNil(t, current)
	require.Equal(t, RuntimeStatusActive, current.Status, current.Reason)
	return installed
}

func hookRequest(t *testing.T, user string) *llm.Request {
	t.Helper()
	var request llm.Request
	require.NoError(t, json.Unmarshal([]byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`), &request))
	request.User = lo.ToPtr(user)
	request.RequestType = llm.RequestTypeChat
	request.APIFormat = llm.APIFormatOpenAIChatCompletion
	request.TransformOptions.ArrayInstructions = lo.ToPtr(true)
	request.TransformerMetadata = map[string]any{"marker": "kept"}
	request.RawRequest = &httpclient.Request{Body: []byte(`{"model":"gpt-4"}`)}
	request.ProviderExtensions = &llm.ProviderExtensions{
		OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
			Request: &llm.OpenAIResponsesRequestExtensions{
				ReasoningContext:        "secret",
				OmittedInputItemIndices: []int{2, 4},
			},
		},
	}
	return &request
}

func testRequestHooks(t *testing.T, manager *Manager, ctx context.Context) orchestrator.RequestHooks {
	t.Helper()
	hooks, err := manager.Middleware(ctx)
	require.NoError(t, err)
	return hooks
}
