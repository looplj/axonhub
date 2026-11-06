package plugin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
)

const (
	fetchExact = "https://moderation.example.com/v1/check"
	fetchFast  = "https://moderation.example.com/fast"
	fetchSlow  = "https://moderation.example.com/slow"
)

func TestInvokeCancelsSynchronousKV(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			client := newPluginDB(t)
			manager := newTestManager(t, client)
			installed := installGrantedJS(t, manager, "acme.kv-cancel", `
export function onEvent(ctx, op) { return ctx.kv.get("key"); }
`, []string{})
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			observedCancellation := false
			client.PluginStorage.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
				return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
					if mode == "cancel" {
						cancel()
					}
					select {
					case <-ctx.Done():
						observedCancellation = true
						return nil, ctx.Err()
					case <-time.After(2 * time.Second):
						return nil, errors.New("KV query did not receive cancellation")
					}
				})
			}))
			req := invokeRequest(t, installed.ID, nil, map[string]any{}, nil, 100*time.Millisecond)
			_, err := manager.Invoke(parent, req)
			require.Error(t, err)
			require.True(t, observedCancellation, "blocked KV query must receive hook cancellation")
		})
	}
}

func TestFetchAsyncDoesNotLeakIntoNextCall(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	gate := &gateTripper{hold: map[string]chan struct{}{
		"/v1/check": make(chan struct{}),
		"/slow":     make(chan struct{}),
	}}
	manager.SetOutboundTransport(gate)
	installed := installGrantedJS(t, manager, "acme.fetch", fetchAsyncScript, []string{fetchExact, fetchFast, fetchSlow})

	fired, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "forget", "url": fetchExact}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"mark": float64(0)}, decodeValue(t, fired.Value))
	require.Equal(t, map[string]any{"early": true}, decodeState(t, fired.State))
	require.Zero(t, manager.VMCount())
	close(gate.hold["/v1/check"])
	time.Sleep(30 * time.Millisecond)

	next, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "read"}, decodeState(t, fired.State), nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"mark": float64(0), "late": false, "early": true, "earlyKeep": false}, decodeValue(t, next.Value))

	race, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "race", "fast": fetchFast, "slow": fetchSlow}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, "fast", decodeValue(t, race.Value))
	require.Zero(t, manager.VMCount())
	close(gate.hold["/slow"])
	time.Sleep(30 * time.Millisecond)
	afterRace, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "read"}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(0), decodeValue(t, afterRace.Value).(map[string]any)["mark"])

	ctx, cancel := context.WithCancel(context.Background())
	gate.setHold("/v1/check", make(chan struct{}))
	gate.mu.Lock()
	gate.entered = make(chan struct{})
	gate.mu.Unlock()
	done := make(chan error, 1)
	req := invokeRequest(t, installed.ID, map[string]any{"op": "wait", "url": fetchExact}, map[string]any{"keep": true}, nil, 0)
	go func() {
		_, err := manager.Invoke(ctx, req)
		done <- err
	}()
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("fetch did not start")
	}
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	require.Zero(t, manager.VMCount())
	kept, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "read"}, map[string]any{"keep": true}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, false, decodeValue(t, kept.Value).(map[string]any)["late"])
	require.Equal(t, true, decodeValue(t, kept.Value).(map[string]any)["earlyKeep"])

	release := make(chan struct{})
	entered := make(chan struct{})
	gate.setHold("/v1/check", release)
	gate.mu.Lock()
	gate.entered = entered
	gate.mu.Unlock()
	started := time.Now()
	timeoutReq := invokeRequest(t, installed.ID, map[string]any{"op": "wait", "url": fetchExact}, map[string]any{"keep": true}, nil, 2*time.Second)
	go func() {
		_, err := manager.Invoke(context.Background(), timeoutReq)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed-out fetch did not start")
	}
	err = <-done
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.GreaterOrEqual(t, time.Since(started), 1500*time.Millisecond)
	require.Zero(t, manager.VMCount())
	close(release)
	time.Sleep(30 * time.Millisecond)
	afterTimeout, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "read"}, map[string]any{"keep": true}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(0), decodeValue(t, afterTimeout.Value).(map[string]any)["mark"])
	require.Equal(t, false, decodeValue(t, afterTimeout.Value).(map[string]any)["late"])
}

func TestFetchOnRequestAndAttempt(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	manager.SetOutboundTransport(&gateTripper{})
	installed := installGrantedJS(t, manager, "acme.fetch-hooks", `
export function onEvent(ctx, op) {
  if (typeof fetch !== "undefined") return { globalFetch: true };
  return ctx.fetch(op.url).then(function (res) { return { from: "event", status: res.status, body: res.body }; });
}
export function onRequest(ctx, op) {
  if (typeof fetch !== "undefined") return { globalFetch: true };
  return ctx.fetch(op.url).then(function (res) { return { from: "request", status: res.status }; });
}
export function onAttempt(ctx, op) {
  if (typeof fetch !== "undefined") return { globalFetch: true };
  return ctx.fetch(op.url).then(function (res) { return { from: "attempt", status: res.status }; });
}
`, []string{fetchExact})

	for _, export := range []string{"onEvent", "onRequest", "onAttempt"} {
		req := invokeRequest(t, installed.ID, map[string]any{"url": fetchExact}, map[string]any{}, nil, time.Second)
		req.Export = export
		result, err := manager.Invoke(context.Background(), req)
		require.NoError(t, err)
		got := decodeValue(t, result.Value).(map[string]any)
		require.Equal(t, map[string]string{"onEvent": "event", "onRequest": "request", "onAttempt": "attempt"}[export], got["from"])
		require.Equal(t, float64(http.StatusOK), got["status"])
		require.NotContains(t, got, "globalFetch")
	}
	require.Equal(t, 1, manager.VMCount())
}

func TestFetchAsyncExportAwaits(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	manager.SetOutboundTransport(&gateTripper{})
	installed := installGrantedJS(t, manager, "acme.fetch-async", `
export async function onEvent(ctx, op) {
  const res = await ctx.fetch(op.url);
  return { status: res.status, body: res.body };
}
`, []string{fetchExact})
	result, err := invokeCall(t, manager, installed.ID, map[string]any{"url": fetchExact}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"status": float64(http.StatusOK), "body": "ok"}, decodeValue(t, result.Value))
}

func TestFetchDeniedDoesNotSend(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	gate := &gateTripper{}
	manager.SetOutboundTransport(gate)
	installed := installGrantedJS(t, manager, "acme.fetch-deny", `
export function onEvent(ctx, op) {
  return ctx.fetch(op.url).then(function () { return "sent"; }, function (err) { return err.code; });
}
`, []string{fetchExact})
	result, err := invokeCall(t, manager, installed.ID, map[string]any{"url": "https://other.example/v1"}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, "PERMISSION_DENIED", decodeValue(t, result.Value))
	require.Zero(t, gate.total())
}

func installGrantedJS(t *testing.T, manager *Manager, id, source string, network []string) *ent.Plugin {
	t.Helper()
	rules := make([]any, len(network))
	for i, rule := range network {
		rules[i] = rule
	}
	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = id
		manifest["name"] = id
		manifest["network"] = rules
	}, map[string]string{
		"ui/page.html": "<html></html>",
		"plugin.js":    source,
	}))
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, nil, network))
	current := manager.Current().ByID(installed.ID)
	require.NotNil(t, current)
	require.Equal(t, RuntimeStatusActive, current.Status, current.Reason)
	require.Equal(t, network, current.GrantedNetwork)
	return installed
}

const fetchAsyncScript = `
let mark = 0;
export function onEvent(ctx, op) {
  if (op.op === "forget") {
    ctx.fetch(op.url).then(function () { mark = 1; ctx.state.late = true; });
    ctx.state.early = true;
    return { mark: mark };
  }
  if (op.op === "race") {
    const slow = ctx.fetch(op.slow).then(function () { mark = 1; return "slow"; });
    const fast = ctx.fetch(op.fast).then(function () { return "fast"; });
    return Promise.race([slow, fast]);
  }
  if (op.op === "wait") {
    ctx.state.dirty = true;
    return ctx.fetch(op.url).then(function () { mark = 1; ctx.state.late = true; return "late"; });
  }
  return { mark: mark, late: ctx.state.late === true, early: ctx.state.early === true, earlyKeep: ctx.state.keep === true };
}
`

type gateTripper struct {
	mu      sync.Mutex
	hold    map[string]chan struct{}
	hits    map[string]int
	entered chan struct{}
}

func (g *gateTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	g.mu.Lock()
	if g.hits == nil {
		g.hits = map[string]int{}
	}
	g.hits[req.URL.Path]++
	hold := g.hold[req.URL.Path]
	if g.entered != nil {
		close(g.entered)
		g.entered = nil
	}
	g.mu.Unlock()
	if hold != nil {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-hold:
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("ok")),
		Request:    req,
	}, nil
}

func (g *gateTripper) setHold(path string, hold chan struct{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.hold == nil {
		g.hold = map[string]chan struct{}{}
	}
	g.hold[path] = hold
}

func (g *gateTripper) total() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, hits := range g.hits {
		n += hits
	}
	return n
}
