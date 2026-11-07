package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestRequestHookFailuresMatchNoPlugin(t *testing.T) {
	scripts := []struct {
		name   string
		source string
		budget time.Duration
	}{
		{name: "throw", source: `export function onRequest() { throw new Error("boom"); }`},
		{name: "illegal", source: `export function onRequest() { return null; }`},
		{name: "timeout", source: `export async function onRequest() { await new Promise(() => {}); }`, budget: 200 * time.Millisecond},
	}
	for _, script := range scripts {
		t.Run(script.name, func(t *testing.T) {
			for _, format := range []string{"openai", "anthropic"} {
				for _, stream := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/stream=%t", format, stream), func(t *testing.T) {
						without := runHookPipeline(t, nil, format, stream, nil, 0, 0)
						manager := newTestManager(t, newPluginDB(t))
						if script.budget > 0 {
							manager.SetHookTimeoutForTest(script.budget)
						}
						installHook(t, manager, "acme.fail", true, false, script.source)
						with := runHookPipeline(t, manager, format, stream, nil, 0, 0)
						require.Equal(t, without.status, with.status)
						require.Equal(t, without.upstream, with.upstream)
						require.Equal(t, without.body, with.body)
						require.Equal(t, without.events, with.events)
					})
				}
			}
		})
	}
}

func TestRequestHookAttemptBodyDoesNotLeakIntoRetries(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installHook(t, manager, "acme.attempt", false, true, `
export function onAttempt(ctx, req) {
  const next = { ...req, messages: req.messages.map((message) => ({ ...message, content: String(message.content || "") + "|" + ctx.attempt.channel.id })) };
  const call = next.messages[0] && next.messages[0].tool_calls && next.messages[0].tool_calls[0];
  if (call) {
    const args = JSON.parse(call.function.arguments);
    args.nested.value += 1;
    next.messages[0] = { ...next.messages[0], tool_calls: [{ ...call, function: { ...call.function, arguments: JSON.stringify(args) } }] };
  }
  return next;
}
`)
	body := `{"model":"gpt-4","messages":[{"role":"assistant","content":"hi","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"nested\":{\"value\":1}}"}}]}]}`
	t.Run("failover regenerates the wire body", func(t *testing.T) {
		var first atomic.Int32
		result := runHookPipeline(t, manager, "openai", false, func(w http.ResponseWriter, r *http.Request, id int) {
			if id == 11 && first.Add(1) == 1 {
				http.Error(w, "busy", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, openAICompletion)
		}, 1, 0, body, 11, 22)
		require.NoError(t, result.err)
		require.Len(t, result.upstream, 2)
		require.Contains(t, result.upstream[0], `|11`)
		require.NotContains(t, result.upstream[0], `|22`)
		require.Contains(t, result.upstream[1], `|22`)
		require.NotContains(t, result.upstream[1], `|11`)
		require.Equal(t, float64(2), gjson.Get(gjson.Get(result.upstream[0], "messages.0.tool_calls.0.function.arguments").String(), "nested.value").Float())
		require.Equal(t, float64(2), gjson.Get(gjson.Get(result.upstream[1], "messages.0.tool_calls.0.function.arguments").String(), "nested.value").Float())
	})
	t.Run("same channel retry regenerates the wire body", func(t *testing.T) {
		var hits atomic.Int32
		result := runHookPipeline(t, manager, "openai", false, func(w http.ResponseWriter, r *http.Request, id int) {
			if hits.Add(1) == 1 {
				http.Error(w, "busy", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, openAICompletion)
		}, 0, 1, body, 11)
		require.NoError(t, result.err)
		require.Len(t, result.upstream, 2)
		for _, upstream := range result.upstream {
			require.Equal(t, "hi|11", gjson.Get(upstream, "messages.0.content").String())
			require.Equal(t, float64(2), gjson.Get(gjson.Get(upstream, "messages.0.tool_calls.0.function.arguments").String(), "nested.value").Float())
		}
	})
}

func TestRequestHookAttemptDropsModelOrStreamChange(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installHook(t, manager, "acme.drop", false, true, `
export function onAttempt(ctx, req) {
  return { ...req, model: "other-model", messages: [{ role: "user", content: "changed" }], stream: false, temperature: 0.9 };
}
`)
	result := runHookPipeline(t, manager, "openai", true, nil, 0, 0)
	require.NoError(t, result.err)
	require.NotEmpty(t, result.upstream)
	require.Equal(t, "gpt-4", gjson.Get(result.upstream[0], "model").String())
	require.Equal(t, "hi", gjson.Get(result.upstream[0], "messages.0.content").String())
	require.True(t, gjson.Get(result.upstream[0], "stream").Bool())
	require.False(t, gjson.Get(result.upstream[0], "temperature").Exists())
	require.NotEmpty(t, result.events)
}

func TestRequestHookOnRequestOverloadDoesNotCallUpstream(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installed := installHook(t, manager, "acme.request-capacity", true, false, `
export function onRequest() { return new Promise(() => {}); }
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
				Args:           []byte(`[{}]`),
				State:          []byte(`{}`),
				Timeout:        time.Minute,
			})
		})
	}
	require.Eventually(t, func() bool { return manager.VMCount() == 16 }, 5*time.Second, 10*time.Millisecond)
	before := manager.ConsecutiveFailures(installed.ID)
	var hits atomic.Int32
	result := runHookPipeline(t, manager, "openai", false, func(w http.ResponseWriter, r *http.Request, id int) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, openAICompletion)
	}, 1, 1)
	require.Error(t, result.err)
	require.Equal(t, http.StatusTooManyRequests, result.status)
	require.Zero(t, hits.Load())
	require.Empty(t, result.upstream)
	require.Equal(t, before, manager.ConsecutiveFailures(installed.ID))
	require.Equal(t, int64(1), manager.hookCounter(installed.ID).overloaded)
}

func TestRequestHookAttemptOverloadDoesNotSwitchChannels(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installed := installHook(t, manager, "acme.attempt-capacity", false, true, `
export function onAttempt(ctx, req) { return new Promise(() => {}); }
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
				Export:         onAttemptExport,
				Args:           []byte(`[{}]`),
				State:          []byte(`{}`),
				Timeout:        time.Minute,
			})
		})
	}
	require.Eventually(t, func() bool { return manager.VMCount() == 16 }, 5*time.Second, 10*time.Millisecond)
	before := manager.ConsecutiveFailures(installed.ID)
	var hits atomic.Int32
	result := runHookPipeline(t, manager, "openai", false, func(w http.ResponseWriter, r *http.Request, id int) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, openAICompletion)
	}, 1, 1, "", 11, 22)
	require.Error(t, result.err)
	require.Equal(t, http.StatusTooManyRequests, result.status)
	require.Zero(t, hits.Load())
	require.Empty(t, result.upstream)
	require.Equal(t, before, manager.ConsecutiveFailures(installed.ID))
	require.Equal(t, int64(1), manager.hookCounter(installed.ID).overloaded)
}

type hookPipelineResult struct {
	status   int
	body     string
	events   []string
	upstream []string
	err      error
}

func runHookPipeline(t *testing.T, manager *Manager, format string, stream bool, handler func(http.ResponseWriter, *http.Request, int), channelRetries, sameRetries int, bodies ...any) hookPipelineResult {
	t.Helper()
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	ids := []int{11}
	if len(bodies) > 0 {
		if text, ok := bodies[0].(string); ok && text != "" {
			body = text
		}
		next := make([]int, 0)
		for _, item := range bodies {
			if id, ok := item.(int); ok {
				next = append(next, id)
			}
		}
		if len(next) > 0 {
			ids = next
		}
	}
	if format == "anthropic" && !gjson.Get(body, "max_tokens").Exists() {
		body = body[:len(body)-1] + `,"max_tokens":16}`
	}
	if stream {
		body = mustStreamBody(t, body)
	}
	var mu sync.Mutex
	var upstream []string
	servers := make([]*httptest.Server, len(ids))
	candidates := make([]*orchestrator.ChannelModelsCandidate, len(ids))
	for i, id := range ids {
		index := i
		channelID := id
		servers[index] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			mu.Lock()
			upstream = append(upstream, string(raw))
			mu.Unlock()
			if handler != nil {
				handler(w, r, channelID)
				return
			}
			if stream {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, streamResponse(format))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, completionResponse(format))
		}))
	}
	t.Cleanup(func() {
		for _, server := range servers {
			server.Close()
		}
	})
	for i, id := range ids {
		candidates[i] = hookCandidate(t, format, servers[i].URL, id)
	}
	state := &orchestrator.PersistenceState{ChannelModelsCandidates: candidates}
	ctx := contexts.EnsureContainer(pluginCtx())
	var middlewares []pipeline.Middleware
	var hooks orchestrator.RequestHooks
	var hook pipeline.Middleware
	if manager != nil {
		hooks = testRequestHooks(t, manager, ctx)
		hook = hooks.Request
		if hook != nil {
			middlewares = append(middlewares, hook)
		}
	}
	var inbound transformer.Inbound
	if format == "anthropic" {
		inbound = anthropic.NewInboundTransformer()
	} else {
		inbound = openai.NewInboundTransformer()
	}
	persistentInbound, outbound := orchestrator.NewPersistentTransformers(state, inbound, middlewares...)
	if hook != nil {
		middlewares = append(middlewares, orchestrator.ApplyRequestHookAttempt(outbound, hooks.Attempt))
	}
	pipe := pipeline.NewFactory(httpclient.NewHttpClient()).Pipeline(
		persistentInbound,
		outbound,
		pipeline.WithRetry(channelRetries, sameRetries, 0),
		pipeline.WithMiddlewares(middlewares...),
	)
	result, err := pipe.Process(ctx, &httpclient.Request{
		Method:  http.MethodPost,
		Path:    hookPath(format),
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(body),
	})
	if hook != nil {
		require.True(t, hook.(*requestHookMiddleware).session.released, "pipeline owns cleanup on success, overload, and before returning a stream")
	}
	out := hookPipelineResult{err: err, upstream: append([]string(nil), upstream...)}
	if err != nil {
		out.status = hookErrorStatus(format, ctx, err)
		return out
	}
	if result.Response != nil {
		out.status = result.Response.StatusCode
		out.body = string(result.Response.Body)
	}
	if result.EventStream != nil {
		for result.EventStream.Next() {
			event := result.EventStream.Current()
			if event != nil {
				out.events = append(out.events, string(event.Data))
			}
		}
		out.err = result.EventStream.Err()
		_ = result.EventStream.Close()
	}
	return out
}

func hookCandidate(t *testing.T, format, serverURL string, id int) *orchestrator.ChannelModelsCandidate {
	t.Helper()
	var outbound transformer.Outbound
	var err error
	var kind channel.Type
	var apiFormat llm.APIFormat
	if format == "anthropic" {
		outbound, err = anthropic.NewOutboundTransformer(serverURL, "test-key")
		kind = channel.TypeAnthropic
		apiFormat = llm.APIFormatAnthropicMessage
	} else {
		outbound, err = openai.NewOutboundTransformer(serverURL, "test-key")
		kind = channel.TypeOpenai
		apiFormat = llm.APIFormatOpenAIChatCompletion
	}
	require.NoError(t, err)
	return &orchestrator.ChannelModelsCandidate{
		Channel:   &biz.Channel{Channel: &ent.Channel{ID: id, Name: format, Type: kind}, Outbound: outbound},
		APIFormat: string(apiFormat),
		Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"}},
	}
}

func hookPath(format string) string {
	if format == "anthropic" {
		return "/v1/messages"
	}
	return "/v1/chat/completions"
}

func mustStreamBody(t *testing.T, body string) string {
	t.Helper()
	require.True(t, gjson.Valid(body))
	if gjson.Get(body, "stream").Exists() {
		return body
	}
	return body[:len(body)-1] + `,"stream":true}`
}

func completionResponse(format string) string {
	if format == "anthropic" {
		return `{"id":"msg_1","type":"message","role":"assistant","model":"gpt-4","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`
	}
	return openAICompletion
}

func streamResponse(format string) string {
	if format == "anthropic" {
		return "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"gpt-4\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":1}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	}
	return "data: {\"id\":\"test\",\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\ndata: {\"id\":\"test\",\"model\":\"gpt-4\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n"
}

func hookErrorStatus(format string, ctx context.Context, err error) int {
	if format == "anthropic" {
		if httpErr := anthropic.NewInboundTransformer().TransformError(ctx, err); httpErr != nil {
			return httpErr.StatusCode
		}
	}
	if httpErr := openai.NewInboundTransformer().TransformError(ctx, err); httpErr != nil {
		return httpErr.StatusCode
	}
	return 0
}

const openAICompletion = `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

// A plugin must not change how the orchestrator handles private Go values or
// updates the inbound request's actual model. Only onRequest's public projection
// and each attempt's wire body cross the JSON boundary.
func TestRequestHooksPreserveUnifiedRequestBehavior(t *testing.T) {
	for _, mode := range []struct {
		name             string
		request, attempt bool
	}{
		{name: "no hooks"}, {name: "request only", request: true}, {name: "attempt only", attempt: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			manager := newTestManager(t, newPluginDB(t))
			if mode.request || mode.attempt {
				installHook(t, manager, "acme.identity", mode.request, mode.attempt, `
export function onRequest(ctx, req) { return undefined; }
export function onAttempt(ctx, body) { return [{op:"add",path:"/temperature",value:0.2}]; }
`)
			}
			ctx := contexts.EnsureContainer(pluginCtx())
			request := hookRequest(t, "keep")
			// This belongs to the host and cannot survive a whole-request JSON round trip.
			opaque := make(chan int)
			request.RawRequest.TransformerMetadata = map[string]any{"opaque": opaque}
			raw := request.RawRequest
			candidate := hookCandidate(t, "openai", "https://example.com", 11)
			candidate.Models[0].ActualModel = "actual-model"
			state := &orchestrator.PersistenceState{LlmRequest: request, ChannelModelsCandidates: []*orchestrator.ChannelModelsCandidate{candidate}}
			var middlewares []pipeline.Middleware
			hooks := testRequestHooks(t, manager, ctx)
			hook := hooks.Request
			if hook != nil {
				defer hook.OnProcessEnd()
				middlewares = append(middlewares, hook)
				got, err := hook.OnInboundLlmRequest(ctx, request)
				require.NoError(t, err)
				require.Same(t, request, got)
			}
			_, outbound := orchestrator.NewPersistentTransformers(state, openai.NewInboundTransformer(), middlewares...)
			for range 2 {
				wire, err := outbound.TransformRequest(ctx, request)
				require.NoError(t, err)
				require.False(t, gjson.GetBytes(wire.Body, "temperature").Exists())
				if hook != nil {
					wire, err = orchestrator.ApplyRequestHookAttempt(outbound, hooks.Attempt).OnOutboundRawRequest(ctx, wire)
					require.NoError(t, err)
				}
				require.Equal(t, mode.attempt, gjson.GetBytes(wire.Body, "temperature").Exists())
				require.Same(t, request, state.LlmRequest)
				require.Equal(t, "actual-model", state.LlmRequest.Model)
				require.Same(t, raw, request.RawRequest)
				require.Equal(t, opaque, request.RawRequest.TransformerMetadata["opaque"])
				require.Nil(t, request.Temperature)
			}
		})
	}
}

// Cover a transformer that supplies a JSON body but leaves its optional wire
// APIFormat empty. The hook and ctx.attempt must use the same resolved format.
func TestAttemptHookUsesResolvedOutboundFormat(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		want       string
		matched    bool
	}{
		{"candidate fallback", "", "openai/chat_completions", true},
		{"wire overrides candidate", "anthropic/messages", "anthropic/messages", true},
		{"unmatched wire format", "gemini/contents", "gemini/contents", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := newTestManager(t, newPluginDB(t))
			installed := installHook(t, manager, "acme.resolved", false, true, `export function onAttempt(ctx, body) {
     return [{op:"add",path:"/resolved_format",value:ctx.attempt.apiFormat}];
   }`)
			ctx := contexts.EnsureContainer(pluginCtx())
			hooks := testRequestHooks(t, manager, ctx)
			hook := hooks.Request
			require.NotNil(t, hook)
			defer hook.OnProcessEnd()
			request := hookRequest(t, "keep")
			_, err := hook.OnInboundLlmRequest(ctx, request)
			require.NoError(t, err)
			candidate := hookCandidate(t, "openai", "https://example.com", 11)
			candidate.Channel.Outbound = &wireFormatOverride{Outbound: candidate.Channel.Outbound, format: tc.wire}
			state := &orchestrator.PersistenceState{LlmRequest: request, ChannelModelsCandidates: []*orchestrator.ChannelModelsCandidate{candidate}}
			_, outbound := orchestrator.NewPersistentTransformers(state, openai.NewInboundTransformer(), hook)
			wire, err := outbound.TransformRequest(ctx, request)
			require.NoError(t, err)
			require.Equal(t, tc.want, wire.APIFormat)
			wire, err = orchestrator.ApplyRequestHookAttempt(outbound, hooks.Attempt).OnOutboundRawRequest(ctx, wire)
			require.NoError(t, err)
			resolved := gjson.GetBytes(wire.Body, "resolved_format")
			require.Equal(t, tc.matched, resolved.Exists())
			if tc.matched {
				require.Equal(t, tc.want, resolved.String())
			}
			require.Zero(t, manager.ConsecutiveFailures(installed.ID))
		})
	}
}

type wireFormatOverride struct {
	transformer.Outbound

	format string
}

func (w *wireFormatOverride) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	raw, err := w.Outbound.TransformRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	raw.APIFormat = w.format
	return raw, nil
}
