// 职责: 验证 HTTP 插件的执行、错误处理、热切换和请求路径开销。
// 关联文件:
//   internal/server/plugin/server_middleware.go
//   internal/server/plugin/manager.go
//   internal/server/plugin/hostapi.go
//   internal/server/plugin/manifest.go

package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestServerMiddlewareContextAndHeaders(t *testing.T) {
	m := newTestManager(t, newPluginDB(t))
	installServerMiddleware(t, m, "example.detect", []string{"badNamespace", "badHeader", "badValue", "detect", "amend"}, true, `
export function badNamespace() {
  return { headers: { set: { "x-bad-namespace": ["leaked"] } },
    context: { "example.other": { client: "bad" } } };
}
export function badHeader() {
  return { headers: { remove: ["Authorization"] },
    context: { axonhub: { traceId: "leaked" } } };
}
export function badValue() {
  return { headers: { set: { "x-null": [null] } },
    context: { axonhub: { traceId: "leaked" } } };
}
export function detect(ctx, request) {
  if (request.headers.authorization || request.headers.cookie ||
      request.headers["x-repeated-key"] || request.headers["x-bad-namespace"] ||
      request.projectId !== 11 || request.apiKeyId !== 17 ||
      request.method !== "POST" || request.path !== "/v1/chat/completions") throw new Error("invalid HTTP snapshot");
  return { headers: { set: { "X-Client": ["dsh", "v2"] }, remove: ["x-remove"] },
    context: { axonhub: { traceId: " t-1 " }, "example.detect": { client: "initial", keep: true } } };
}
export function amend(ctx, request) {
  if (request.headers["x-client"]?.[1] !== "v2" || request.headers["x-remove"] ||
      ctx.requestContext["example.detect"].client !== "initial") throw new Error("missing middleware updates");
  return { context: { axonhub: { traceId: "later", threadId: " thread-1 " },
    "example.detect": { client: { family: "dsh", claims: ["a", null] } } } };
}
export function onRequest(ctx, req) {
  if (ctx.requestContext["example.detect"].client.family !== "dsh") throw new Error("missing onRequest context");
  return { ...req, user: "from-server-middleware" };
}
export function onAttempt(ctx, req) {
  if (ctx.requestContext["example.detect"].client.claims[0] !== "a") throw new Error("missing onAttempt context");
  return { ...req, user: "from-attempt" };
}`)
	installServerMiddleware(t, m, "example.observe", []string{"observe"}, false, `
export function observe(ctx) {
  return { headers: { set: { "X-Seen": [ctx.requestContext["example.detect"].client.family] } } };
}`)

	ctx := contexts.WithProjectID(t.Context(), 11)
	ctx = contexts.WithAPIKey(ctx, &ent.APIKey{ID: 17, Key: "secret-synthetic"})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?key=secret-synthetic", strings.NewReader("original body")).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer secret-synthetic")
	req.Header.Set("Cookie", "secret=cookie")
	req.Header.Set("X-Repeated-Key", "custom secret-synthetic")
	req.Header.Set("X-Remove", "old")

	got, rejection := m.RunServerMiddleware(req)
	require.Nil(t, rejection)
	require.True(t, req.Body == got.Body)
	require.Equal(t, []string{"dsh", "v2"}, got.Header.Values("X-Client"))
	require.Equal(t, "dsh", got.Header.Get("X-Seen"))
	require.Empty(t, got.Header.Get("X-Remove"))
	require.Empty(t, got.Header.Get("X-Bad-Namespace"))
	require.Empty(t, got.Header.Values("X-Null"))
	require.Equal(t, "Bearer secret-synthetic", got.Header.Get("Authorization"))

	metadata := contexts.GetPluginRequestContext(got.Context())
	require.Equal(t, "t-1", metadata.TraceID)
	require.Equal(t, "thread-1", metadata.ThreadID)
	require.JSONEq(t, `{"axonhub":{"traceId":"t-1","threadId":"thread-1"},"example.detect":{"client":{"family":"dsh","claims":["a",null]},"keep":true}}`, string(metadata.JSON))
	require.Empty(t, contexts.GetPluginRequestContext(req.Context()).JSON)
	require.NoError(t, got.Context().Err())

	hooks := testRequestHooks(t, m, got.Context())
	defer hooks.Request.OnProcessEnd()
	unified, err := hooks.Request.OnInboundLlmRequest(got.Context(), &llm.Request{
		RequestType: llm.RequestTypeChat, Model: "chat-model",
	})
	require.NoError(t, err)
	require.NotNil(t, unified.User)
	require.Equal(t, "from-server-middleware", *unified.User)
	_, err = hooks.Request.(*requestHookMiddleware).OnOutboundLlmRequest(got.Context(), unified, llm.APIFormatOpenAIChatCompletion)
	require.NoError(t, err)
	attempt, err := hooks.Attempt.OnOutboundRawRequest(got.Context(), &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAIChatCompletion), Body: []byte(`{"model":"chat-model"}`),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"chat-model","user":"from-attempt"}`, string(attempt.Body))
}

func TestServerMiddlewareRejectsAndBoundsExecution(t *testing.T) {
	t.Run("explicit rejection short circuits", func(t *testing.T) {
		m := newTestManager(t, newPluginDB(t))
		installServerMiddleware(t, m, "example.reject", []string{"reject", "after"}, false, `
export function reject(ctx) {
  try { ctx.reject(403, "blocked"); } catch (_) {}
  return { headers: { set: { "X-Rejected": ["bad"] } } };
}
export function after() { return { headers: { set: { "X-After": ["bad"] } } }; }
`)
		req, rejection := m.RunServerMiddleware(httptest.NewRequest(http.MethodGet, "/", nil))
		require.NotNil(t, rejection)
		require.Equal(t, http.StatusForbidden, rejection.Status)
		require.EqualError(t, rejection.Err, "blocked")
		require.Empty(t, req.Header)
	})
	t.Run("deadline skips remaining handlers", func(t *testing.T) {
		m := newTestManager(t, newPluginDB(t))
		installServerMiddleware(t, m, "example.timeout", []string{"wait", "after"}, false, `
export async function wait() { await new Promise(() => {}); }
export function after() { return { headers: { set: { "X-After": ["bad"] } } }; }
`)
		start := time.Now()
		req, rejection := m.RunServerMiddleware(httptest.NewRequest(http.MethodGet, "/", nil))
		require.Less(t, time.Since(start), time.Second)
		require.Nil(t, rejection)
		require.Empty(t, req.Header)
		require.NoError(t, req.Context().Err())
	})
	t.Run("shared budget covers saturated pool", func(t *testing.T) {
		m := newTestManager(t, newPluginDB(t))
		id := installServerMiddleware(t, m, "example.saturated", []string{"run"}, false,
			`export function run(ctx) { ctx.reject(429, "ran"); }`)
		gen := m.serverMiddleware.Load()
		module := gen.ByID(id).resource.compiledModule()
		exec := m.execFor(id)
		for range vmPoolSize {
			vm, err := exec.borrow(t.Context(), module)
			require.NoError(t, err)
			t.Cleanup(func() { exec.release(vm, false) })
		}
		start := time.Now()
		req, rejected := m.RunServerMiddleware(httptest.NewRequest(http.MethodGet, "/", nil))
		require.Less(t, time.Since(start), time.Second)
		require.Nil(t, rejected)
		require.NoError(t, req.Context().Err())
		require.EqualValues(t, 1, m.hookCounter(id).overloaded)
	})
}

func TestServerMiddlewareCircuitIsolation(t *testing.T) {
	t.Run("HTTP success does not clear LLM failures", func(t *testing.T) {
		m := newTestManager(t, newPluginDB(t))
		id := installServerMiddleware(t, m, "example.llm-failure", []string{"handler"}, true, `
export function handler() { return { headers: { set: { "X-Healthy": ["yes"] } } }; }
export function onRequest() { throw new Error("broken LLM hook"); }
export function onAttempt() {}
`)
		for i := range circuitFailureLimit + 1 {
			ctx := contexts.EnsureContainer(t.Context())
			req, rejection := m.RunServerMiddleware(httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx))
			require.Nil(t, rejection)
			require.Equal(t, "yes", req.Header.Get("X-Healthy"))

			hooks := testRequestHooks(t, m, req.Context())
			_, err := hooks.Request.OnInboundLlmRequest(req.Context(), &llm.Request{
				Model: "model", RequestType: llm.RequestTypeChat,
			})
			hooks.Request.OnProcessEnd()
			require.NoError(t, err)
			if i+1 < circuitFailureLimit {
				require.Equal(t, i+1, m.ConsecutiveFailures(id))
			}
		}
		exec := m.execFor(id)
		require.True(t, exec.circuit.rejectCall(m.nowCall()))
		require.False(t, exec.serverCircuit("handler").rejectCall(m.nowCall()))
	})
	t.Run("failed HTTP handler pauses independently", func(t *testing.T) {
		m := newTestManager(t, newPluginDB(t))
		id := installServerMiddleware(t, m, "example.http-failure", []string{"broken", "healthy"}, true, `
export function broken() { throw new Error("broken HTTP handler"); }
export function healthy() { return { headers: { set: { "X-Healthy": ["yes"] } } }; }
export function onRequest(_ctx, request) { return { ...request, user: "healthy" }; }
export function onAttempt() {}
`)
		for range circuitFailureLimit + 1 {
			ctx := contexts.EnsureContainer(t.Context())
			req, rejection := m.RunServerMiddleware(httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx))
			require.Nil(t, rejection)
			require.Equal(t, "yes", req.Header.Get("X-Healthy"))

			hooks := testRequestHooks(t, m, req.Context())
			result, err := hooks.Request.OnInboundLlmRequest(req.Context(), &llm.Request{
				Model: "model", RequestType: llm.RequestTypeChat,
			})
			hooks.Request.OnProcessEnd()
			require.NoError(t, err)
			require.NotNil(t, result.User)
			require.Equal(t, "healthy", *result.User)
		}
		exec := m.execFor(id)
		require.True(t, exec.serverCircuit("broken").rejectCall(m.nowCall()))
		require.False(t, exec.serverCircuit("healthy").rejectCall(m.nowCall()))
		require.False(t, exec.circuit.rejectCall(m.nowCall()))
	})
}

func TestServerMiddlewareRefresh(t *testing.T) {
	db := newPluginDB(t)
	m := newTestManager(t, db)
	require.Nil(t, m.serverMiddleware.Load())
	first := installServerMiddleware(t, m, "example.first", []string{"trace"}, false,
		`export function trace() { return { context: { axonhub: { traceId: "first" } } }; }`)
	second := installServerMiddleware(t, m, "example.second", []string{"trace"}, false,
		`export function trace() { return { context: { axonhub: { traceId: "second" } } }; }`)
	run := func(manager *Manager) string {
		req, rejection := manager.RunServerMiddleware(httptest.NewRequest(http.MethodGet, "/", nil))
		require.Nil(t, rejection)
		return contexts.GetPluginRequestContext(req.Context()).TraceID
	}
	require.Equal(t, "first", run(m))
	started := newTestManager(t, db)
	require.NoError(t, started.Start(t.Context()))
	require.Equal(t, "first", run(started))
	require.NoError(t, started.Stop(t.Context()))

	t.Run("in-flight request keeps the captured generation", func(t *testing.T) {
		old := m.serverMiddleware.Load()
		require.NoError(t, old.Acquire())
		defer old.Release()
		packageReads := m.PackageReads()
		require.NoError(t, m.UpdateSettings(pluginCtx(), second, -1, map[string]any{}))
		require.Equal(t, packageReads, m.PackageReads())
		require.Equal(t, "second", run(m))

		upgraded := mustZip(t, map[string]string{
			"manifest.json": strings.Replace(serverMiddlewareManifest("example.second", []string{"trace"}, false),
				`"version":"1.0.0"`, `"version":"1.1.0"`, 1),
			"plugin.js": `export function trace() { return { context: { axonhub: { traceId: "upgraded" } } }; }`,
		})
		result, err := m.Install(pluginCtx(), upgraded, nil, nil)
		require.NoError(t, err)
		require.Nil(t, result.Preview)
		require.Equal(t, "upgraded", run(m))

		require.NoError(t, m.Disable(pluginCtx(), first))
		require.NoError(t, m.Delete(pluginCtx(), second))
		require.Nil(t, m.serverMiddleware.Load())
		require.Empty(t, run(m))

		invoked, err := m.Invoke(t.Context(), InvokeRequest{InstallationID: first, Export: "trace", Generation: old})
		require.NoError(t, err)
		require.JSONEq(t, `{"context":{"axonhub":{"traceId":"first"}}}`, string(invoked.Value))
	})

	require.NoError(t, m.Enable(pluginCtx(), first, nil, nil))
	require.Equal(t, "first", run(m))
	require.NoError(t, m.Stop(t.Context()))
	require.Nil(t, m.serverMiddleware.Load())
}

func TestServerMiddlewareManifestValidation(t *testing.T) {
	for _, test := range []struct {
		name, entries, source, wantErr string
	}{
		{"missing script", `[{"handler":"run"}]`, "", "plugin.js is required"},
		{"missing export", `[{"handler":"run"}]`, "export function another() {}", "must export run"},
		{"not callable", `[{"handler":"run"}]`, "export const run = 1", "export run must be a function"},
		{"unknown field", `[{"handler":"run","stage":"beforeTrace"}]`, "export function run() {}", "stage"},
		{"duplicate", `[{"handler":"run"},{"handler":"run"}]`, "export function run() {}", "duplicate handler"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{"manifest.json": strings.Replace(serverMiddlewareManifest("example.validate", []string{"run"}, false),
				`[{"handler":"run"}]`, test.entries, 1)}
			if test.source != "" {
				files["plugin.js"] = test.source
			}
			_, err := ParsePackage(mustZip(t, files))
			require.ErrorContains(t, err, test.wantErr)
		})
	}
	archive := mustZip(t, map[string]string{
		"manifest.json": serverMiddlewareManifest("example.validate", []string{"run"}, false),
		"plugin.js":     "export async function run() {}",
	})
	pkg, err := ParsePackage(archive)
	require.NoError(t, err)
	diff := DiffManifest(nil, &pkg.Manifest)
	require.True(t, diff.NeedsPreview())
	require.Equal(t, []string{"server.middleware: run"}, diff.Hooks)
}

func BenchmarkServerMiddleware(b *testing.B) {
	for _, test := range []struct {
		name     string
		handlers []string
	}{
		{"inactive", nil},
		{"one", []string{"trace"}},
		{"three", []string{"first", "trace", "last"}},
	} {
		b.Run(test.name, func(b *testing.B) {
			db := enttest.NewEntClient(b, "sqlite3", "file:server_middleware_bench_"+test.name+"?mode=memory&cache=shared&_fk=0")
			b.Cleanup(func() { require.NoError(b, db.Close()) })
			m, err := newManager(ManagerParams{Client: db, Cache: xcache.Config{Mode: xcache.ModeMemory}},
				managerConfig{refreshInterval: time.Hour, debounce: time.Hour})
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, m.Stop(context.Background())) })
			if len(test.handlers) != 0 {
				archive := zipFiles(b, map[string]string{
					"manifest.json": serverMiddlewareManifest("example.bench", test.handlers, false),
					"plugin.js": `
export function trace(_ctx, request) { return { context: { axonhub: { traceId: request.headers["x-dsh-session"][0] } } }; }
export function first() {}
export function last() {}
`,
				})
				installed, err := m.Install(pluginCtx(), archive, nil, nil)
				require.NoError(b, err)
				require.NoError(b, m.Enable(pluginCtx(), installed.Plugin.ID, nil, nil))
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			req.Header.Set("X-Dsh-Session", "abc")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer synthetic")
			warm, rejected := m.RunServerMiddleware(req)
			require.Nil(b, rejected)
			if len(test.handlers) != 0 {
				require.Equal(b, "abc", contexts.GetPluginRequestContext(warm.Context()).TraceID)
			}
			b.ReportAllocs()
			for b.Loop() {
				_, _ = m.RunServerMiddleware(req)
			}
		})
	}
}

func installServerMiddleware(t *testing.T, m *Manager, id string, handlers []string, llmHooks bool, source string) int {
	t.Helper()
	archive := mustZip(t, map[string]string{
		"manifest.json": serverMiddlewareManifest(id, handlers, llmHooks),
		"plugin.js":     source,
	})
	installed, err := m.Install(pluginCtx(), archive, nil, nil)
	require.NoError(t, err)
	require.NoError(t, m.Enable(pluginCtx(), installed.Plugin.ID, nil, nil))
	return installed.Plugin.ID
}

func serverMiddlewareManifest(id string, handlers []string, llmHooks bool) string {
	entries := make([]map[string]string, 0, len(handlers))
	for _, handler := range handlers {
		entries = append(entries, map[string]string{"handler": handler})
	}
	manifest := map[string]any{
		"schemaVersion": 1, "pluginApiVersion": 1, "axonhubVersion": "^1.0.0",
		"id": id, "name": id, "version": "1.0.0",
		"server": map[string]any{"middleware": entries},
	}
	if llmHooks {
		manifest["hooks"] = json.RawMessage(`{"request":{"enabled":true,"requestTypes":["chat"]},"attempt":{"enabled":true,"apiFormats":["openai/chat_completions"]}}`)
	}
	raw, _ := json.Marshal(manifest)
	return string(raw)
}
