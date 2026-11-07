package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/looplj/axonhub/internal/ent"
)

func TestInvokeBorrowDoesNotCompile(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	compiles := 0
	manager.compileModule = func(data []byte) (*moejs.Module, error) {
		compiles++
		return compilePackageModule(data)
	}
	installed := installJS(t, manager, "acme.borrow", `
let calls = 0;
export function onEvent(ctx, op) {
  if (op.void) {
    ctx.state.marker = "kept";
    return;
  }
  calls += 1;
  ctx.state.calls = calls;
  return calls;
}

`)
	reads := manager.PackageReads()
	compiled := compiles
	extracts := 0
	manager.extract = func([]byte, string) ([]string, error) {
		extracts++
		return nil, fmt.Errorf("invoke unzipped the package")
	}

	first, err := invokeCall(t, manager, installed.ID, map[string]any{}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(1), decodeValue(t, first.Value))
	require.Equal(t, map[string]any{"calls": float64(1)}, decodeState(t, first.State))
	require.Equal(t, 1, manager.VMCount())

	second, err := invokeCall(t, manager, installed.ID, map[string]any{}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(2), decodeValue(t, second.Value))
	require.Equal(t, 1, manager.VMCount())

	voided, err := invokeCall(t, manager, installed.ID, map[string]any{"void": true}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Nil(t, voided.Value)
	require.Equal(t, map[string]any{"marker": "kept"}, decodeState(t, voided.State))

	require.Equal(t, compiled, compiles)
	require.Equal(t, reads, manager.PackageReads())
	require.Zero(t, extracts)
}

func TestInvokePromiseDoesNotCommitState(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	installed := installJS(t, manager, "acme.promise", `
export function onEvent(ctx, op) {
  if (op.hang) {
    ctx.state.dirty = true;
    return new Promise(function () {});
  }
  if (op.resolve) {
    ctx.state.dirty = true;
    return Promise.resolve({ ok: true, dirty: ctx.state.dirty === true, keep: ctx.state.keep });
  }
  if (op.reject) {
    ctx.state.dirty = true;
    return Promise.reject(new Error("rejected"));
  }
  return { dirty: ctx.state.dirty === true, keep: ctx.state.keep };
}
`)
	started := time.Now()
	_, err := invokeCall(t, manager, installed.ID, map[string]any{"hang": true}, map[string]any{"keep": true}, nil, 200*time.Millisecond)
	require.GreaterOrEqual(t, time.Since(started), 150*time.Millisecond)
	require.Less(t, time.Since(started), time.Second)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotErrorIs(t, err, ErrVMUnavailable)
	require.Zero(t, manager.VMCount())

	resolved, err := invokeCall(t, manager, installed.ID, map[string]any{"resolve": true}, map[string]any{"keep": true}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"ok": true, "dirty": true, "keep": true}, decodeValue(t, resolved.Value))
	require.Equal(t, map[string]any{"dirty": true, "keep": true}, decodeState(t, resolved.State))

	_, err = invokeCall(t, manager, installed.ID, map[string]any{"reject": true}, map[string]any{"keep": true}, nil, time.Second)
	require.Error(t, err)
	require.ErrorContains(t, err, "rejected")
	require.NotErrorIs(t, err, context.DeadlineExceeded)

	result, err := invokeCall(t, manager, installed.ID, map[string]any{}, map[string]any{"keep": true}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"dirty": false, "keep": true}, decodeValue(t, result.Value))
	require.Equal(t, map[string]any{"keep": true}, decodeState(t, result.State))
}

func TestInvokeInterruptedVMIsDiscarded(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	installed := installJS(t, manager, "acme.interrupt", `
let calls = 0;
export function onEvent(ctx, op) {
  if (op.block) { for (;;) {} }
  calls += 1;
  ctx.state.seen = (ctx.state.seen || 0) + 1;
  return { calls: calls, seen: ctx.state.seen };
}
`)
	first, err := invokeCall(t, manager, installed.ID, map[string]any{}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"calls": float64(1), "seen": float64(1)}, decodeValue(t, first.Value))
	require.Equal(t, 1, manager.VMCount())

	started := time.Now()
	_, err = invokeCall(t, manager, installed.ID, map[string]any{"block": true}, decodeState(t, first.State), nil, 300*time.Millisecond)
	require.Less(t, time.Since(started), 2*time.Second)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotErrorIs(t, err, ErrVMUnavailable)
	require.Zero(t, manager.VMCount())

	next, err := invokeCall(t, manager, installed.ID, map[string]any{}, decodeState(t, first.State), nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"calls": float64(1), "seen": float64(2)}, decodeValue(t, next.Value))
	require.Equal(t, map[string]any{"seen": float64(2)}, decodeState(t, next.State))
	require.Equal(t, 1, manager.VMCount())
}

func TestInvokeStateIsolatedAcrossRequests(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	installed := installJS(t, manager, "acme.state", `
let boots = 0;
export function onEvent(ctx, op) {
  if (op.block) { for (;;) {} }
  boots += 1;
  const prev = ctx.state.owner || "";
  if (op.owner) {
    ctx.state.owner = op.owner;
    ctx.state.n = (ctx.state.n || 0) + 1;
  }
  return { boots: boots, owner: ctx.state.owner || "", n: ctx.state.n || 0, prev: prev };
}
`)
	first, err := invokeCall(t, manager, installed.ID, map[string]any{"owner": "a"}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"boots": float64(1), "owner": "a", "n": float64(1), "prev": ""}, decodeValue(t, first.Value))

	second, err := invokeCall(t, manager, installed.ID, map[string]any{"owner": "b"}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"boots": float64(2), "owner": "b", "n": float64(1), "prev": ""}, decodeValue(t, second.Value))
	require.Equal(t, map[string]any{"owner": "b", "n": float64(1)}, decodeState(t, second.State))

	third, err := invokeCall(t, manager, installed.ID, map[string]any{"owner": "a"}, decodeState(t, first.State), nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"boots": float64(3), "owner": "a", "n": float64(2), "prev": "a"}, decodeValue(t, third.Value))

	_, err = invokeCall(t, manager, installed.ID, map[string]any{"block": true}, decodeState(t, third.State), nil, 300*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Zero(t, manager.VMCount())

	fourth, err := invokeCall(t, manager, installed.ID, map[string]any{"owner": "a"}, decodeState(t, third.State), nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"boots": float64(1), "owner": "a", "n": float64(3), "prev": "a"}, decodeValue(t, fourth.Value))
	require.Equal(t, map[string]any{"owner": "a", "n": float64(3)}, decodeState(t, fourth.State))
}

func TestInvokeKV(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	now := time.Date(2026, 11, 6, 3, 0, 0, 0, time.UTC)
	manager.kvNow = func() time.Time { return now }
	installed := installJS(t, manager, "acme.kv", kvHostScript)
	other := installJS(t, manager, "acme.kv-other", kvHostScript)
	id := installed.ID

	require.Nil(t, kvOp(t, manager, id, map[string]any{"op": "get", "key": "missing"}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "empty", "value": "",
	}))
	require.Equal(t, "", kvOp(t, manager, id, map[string]any{"op": "get", "key": "empty"}))
	require.Equal(t, false, kvOp(t, manager, id, map[string]any{"op": "delete", "key": "missing"}))
	require.Equal(t, true, kvOp(t, manager, id, map[string]any{"op": "delete", "key": "empty"}))
	require.Equal(t, false, kvOp(t, manager, id, map[string]any{"op": "delete", "key": "empty"}))

	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "cursor", "value": "first",
		"options": map[string]any{"ttlSeconds": 60},
	}))
	now = now.Add(30 * time.Second)
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "cursor", "value": "reset",
		"options": map[string]any{"ttlSeconds": 60},
	}))
	now = now.Add(45 * time.Second)
	require.Equal(t, "reset", kvOp(t, manager, id, map[string]any{"op": "get", "key": "cursor"}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "cursor", "value": "forever",
	}))
	now = now.Add(2 * time.Minute)
	require.Equal(t, "forever", kvOp(t, manager, id, map[string]any{"op": "get", "key": "cursor"}))

	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "soon", "value": "gone",
		"options": map[string]any{"ttlSeconds": 1},
	}))
	now = now.Add(time.Second)
	require.Nil(t, kvOp(t, manager, id, map[string]any{"op": "get", "key": "soon"}))
	require.Equal(t, false, kvOp(t, manager, id, map[string]any{"op": "delete", "key": "soon"}))

	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "c", "value": "cee", "options": map[string]any{"namespace": "custom"},
	}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "a.b", "value": "ab", "options": map[string]any{"namespace": "custom"},
	}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "a.b.c", "value": "abc", "options": map[string]any{"namespace": "custom"},
	}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "ab", "value": "nope", "options": map[string]any{"namespace": "custom"},
	}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "a_b", "value": "under", "options": map[string]any{"namespace": "custom"},
	}))
	require.Equal(t, "set-ok", kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "axb", "value": "wild", "options": map[string]any{"namespace": "custom"},
	}))

	listed := kvPageOp(t, manager, id, map[string]any{"namespace": "custom"})
	require.Equal(t, []string{"a.b", "a.b.c", "a_b", "ab", "axb", "c"}, keysOfPage(listed))
	require.Nil(t, listed.NextCursor)
	require.Nil(t, listed.Entries[0].ExpiresAt)
	require.NotEmpty(t, listed.Entries[0].UpdatedAt)
	_, err := time.Parse(time.RFC3339Nano, listed.Entries[0].UpdatedAt)
	require.NoError(t, err)

	prefixed := kvPageOp(t, manager, id, map[string]any{"namespace": "custom", "prefix": "a.b"})
	require.Equal(t, []string{"a.b", "a.b.c"}, keysOfPage(prefixed))
	literal := kvPageOp(t, manager, id, map[string]any{"namespace": "custom", "prefix": "a_"})
	require.Equal(t, []string{"a_b"}, keysOfPage(literal))

	page := kvPageOp(t, manager, id, map[string]any{"namespace": "custom", "first": 2})
	require.Equal(t, []string{"a.b", "a.b.c"}, keysOfPage(page))
	require.Equal(t, "a.b.c", *page.NextCursor)
	next := kvPageOp(t, manager, id, map[string]any{"namespace": "custom", "first": 2, "after": *page.NextCursor})
	require.Equal(t, []string{"a_b", "ab"}, keysOfPage(next))
	rest := kvPageOp(t, manager, id, map[string]any{"namespace": "custom", "first": 2, "after": *next.NextCursor})
	require.Equal(t, []string{"axb", "c"}, keysOfPage(rest))
	require.Nil(t, rest.NextCursor)

	empty := kvPageOp(t, manager, id, map[string]any{"namespace": "custom", "prefix": "missing"})
	require.Empty(t, empty.Entries)
	require.Nil(t, empty.NextCursor)

	for i := range 101 {
		_, err := manager.KVSet(pluginCtx(), "acme.kv", id, "", fmt.Sprintf("k%03d", i), "v", 0)
		require.NoError(t, err)
	}
	filled := kvPageOp(t, manager, id, map[string]any{"prefix": "k"})
	require.Len(t, filled.Entries, 100)
	require.Equal(t, "k000", filled.Entries[0].Key)
	require.Equal(t, "k099", *filled.NextCursor)
	tail := kvPageOp(t, manager, id, map[string]any{"prefix": "k", "after": *filled.NextCursor})
	require.Equal(t, []string{"k100"}, keysOfPage(tail))
	require.Nil(t, tail.NextCursor)

	badFirst := kvOp(t, manager, id, map[string]any{"op": "list", "options": map[string]any{"first": 0}}).(map[string]any)
	require.Equal(t, "PluginHostError", badFirst["name"])
	require.Equal(t, "INVALID_ARGUMENT", badFirst["code"])
	overFirst := kvOp(t, manager, id, map[string]any{"op": "list", "options": map[string]any{"first": 101}}).(map[string]any)
	require.Equal(t, "LIMIT_EXCEEDED", overFirst["code"])
	fraction := kvOp(t, manager, id, map[string]any{"op": "list", "options": map[string]any{"first": 1.5}}).(map[string]any)
	require.Equal(t, "INVALID_ARGUMENT", fraction["code"])

	badKey := kvOp(t, manager, id, map[string]any{"op": "set", "key": "bad key", "value": "v"}).(map[string]any)
	require.Equal(t, "PluginHostError", badKey["name"])
	require.Equal(t, "INVALID_ARGUMENT", badKey["code"])
	require.NotContains(t, badKey["message"], "s3cret")
	huge := kvOp(t, manager, id, map[string]any{
		"op": "set", "key": "huge", "value": strings.Repeat("s3cret", kvMaxValueBytes),
	}).(map[string]any)
	require.Equal(t, "LIMIT_EXCEEDED", huge["code"])
	require.NotContains(t, huge["message"], "s3cret")
	require.Equal(t, "set-ok", kvOp(t, manager, other.ID, map[string]any{
		"op": "set", "key": "cursor", "value": "other",
	}))
	require.Equal(t, "forever", kvOp(t, manager, id, map[string]any{"op": "get", "key": "cursor"}))
	require.Nil(t, kvOp(t, manager, other.ID, map[string]any{"op": "get", "key": "empty"}))
	require.Equal(t, "other", kvOp(t, manager, other.ID, map[string]any{"op": "get", "key": "cursor"}))
}

func TestInvokeCircuitBreaker(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	now := time.Date(2026, 11, 6, 3, 0, 0, 0, time.UTC)
	manager.callNow = func() time.Time { return now }
	installed := installJS(t, manager, "acme.breaker", `
export function onEvent(ctx, op) {
  const n = Number(ctx.kv.get("hits") || "0") + 1;
  ctx.kv.set("hits", String(n));
  if (op.fail) throw new Error("boom");
  return n;
}
`)
	for range circuitFailureLimit {
		_, err := invokeCall(t, manager, installed.ID, map[string]any{"fail": true}, map[string]any{}, nil, time.Second)
		require.ErrorContains(t, err, "boom")
		require.NotErrorIs(t, err, ErrCircuitOpen)
	}
	require.Equal(t, "20", kvStored(t, manager, installed.PluginID, installed.ID, "hits"))

	_, err := invokeCall(t, manager, installed.ID, map[string]any{"fail": true}, map[string]any{}, nil, time.Second)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.Equal(t, "20", kvStored(t, manager, installed.PluginID, installed.ID, "hits"))

	now = now.Add(circuitPause - time.Nanosecond)
	_, err = invokeCall(t, manager, installed.ID, map[string]any{"fail": true}, map[string]any{}, nil, time.Second)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.Equal(t, "20", kvStored(t, manager, installed.PluginID, installed.ID, "hits"))

	now = now.Add(time.Nanosecond)
	result, err := invokeCall(t, manager, installed.ID, map[string]any{}, map[string]any{}, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(21), decodeValue(t, result.Value))

	for range circuitFailureLimit - 1 {
		_, err := invokeCall(t, manager, installed.ID, map[string]any{"fail": true}, map[string]any{}, nil, time.Second)
		require.ErrorContains(t, err, "boom")
	}
	_, err = invokeCall(t, manager, installed.ID, map[string]any{"fail": true}, map[string]any{}, nil, time.Second)
	require.ErrorContains(t, err, "boom")
	_, err = invokeCall(t, manager, installed.ID, map[string]any{}, map[string]any{}, nil, time.Second)
	require.ErrorIs(t, err, ErrCircuitOpen)
}

func TestInvokeHostObjects(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	installed := installJS(t, manager, "acme.host", `
let saved = null;
export function onEvent(ctx, op) {
  if (op.op === "config") {
    let threw = false;
    let nestedThrew = false;
    try { ctx.config.token = "mutated"; } catch (e) { threw = true; }
    try { ctx.config.nested.token = "mutated"; } catch (e) { nestedThrew = true; }
    return { token: ctx.config.token, nested: ctx.config.nested.token, threw: threw, nestedThrew: nestedThrew };
  }
  if (op.op === "save") {
    saved = ctx;
    return "saved";
  }
  if (op.op === "reuse") {
    try {
      saved.kv.get("k");
      return "reused";
    } catch (e) {
      return { name: e.name, code: e.code };
    }
  }
  if (op.op === "log") {
    try {
      ctx.log[op.level](op.message, op.fields);
      return "logged";
    } catch (e) {
      return { name: e.name, code: e.code, message: e.message };
    }
  }
  return null;
}
`)
	config := map[string]any{
		"token":  "s3cret-token",
		"nested": map[string]any{"token": "s3cret-token"},
	}
	result, err := invokeCall(t, manager, installed.ID, map[string]any{"op": "config"}, map[string]any{}, config, time.Second)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"token":       "s3cret-token",
		"nested":      "s3cret-token",
		"threw":       true,
		"nestedThrew": true,
	}, decodeValue(t, result.Value))
	require.Equal(t, "s3cret-token", config["token"])
	require.NotContains(t, manager.Current().ByID(installed.ID).Config, "token")

	logged := decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{
		"op": "log", "level": "info", "message": "hello",
		"fields": map[string]any{"plugin_id": "evil", "detail": "kept"},
	}, nil))
	require.Equal(t, "logged", logged)
	badLog := decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{
		"op": "log", "level": "error", "message": 12,
	}, config)).(map[string]any)
	require.Equal(t, "PluginHostError", badLog["name"])
	require.Equal(t, "INVALID_ARGUMENT", badLog["code"])
	require.NotContains(t, badLog["message"], "s3cret-token")

	require.Equal(t, "saved", decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{"op": "save"}, nil)))
	reused := decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{"op": "reuse"}, nil)).(map[string]any)
	require.Equal(t, "PluginHostError", reused["name"])
	require.Equal(t, "INVALID_STATE", reused["code"])
}

func TestPoolCanceledWaiterPassesWakeup(t *testing.T) {
	exec := &pluginExec{borrowed: vmPoolSize}
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	secondCtx, cancelSecond := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelSecond()
	startBorrow := func(ctx context.Context) <-chan error {
		result := make(chan error, 1)
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					result <- fmt.Errorf("panic: %v", recovered)
				}
			}()
			_, err := exec.borrow(ctx, nil)
			result <- err
		}()
		return result
	}
	waitForQueue := func(count int) {
		t.Helper()
		require.Eventually(t, func() bool {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			return len(exec.waiters) == count
		}, time.Second, time.Millisecond)
	}
	first := startBorrow(firstCtx)
	waitForQueue(1)
	second := startBorrow(secondCtx)
	waitForQueue(2)

	// Hold the lock until the first waiter is both canceled and signaled.
	// No other borrower releases capacity to rescue a lost wakeup.
	exec.mu.Lock()
	cancelFirst()
	exec.borrowed--
	exec.idle = []*pluginVM{{}}
	exec.wake()
	exec.mu.Unlock()

	require.ErrorIs(t, <-first, ErrVMUnavailable)
	require.NoError(t, <-second)
}

func TestInvokePoolExhaustionIsNotAFailure(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	installed := installJS(t, manager, "acme.pool", `
export function onEvent(ctx, op) {
  if (op.block) { for (;;) {} }
  if (op.fail) throw new Error("boom");
  return 1;
}
`)
	errCh := make(chan error, vmPoolSize)
	cancels := make([]context.CancelFunc, vmPoolSize)
	blocked := invokeRequest(t, installed.ID, map[string]any{"block": true}, map[string]any{}, nil, 0)
	for i := range vmPoolSize {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					errCh <- fmt.Errorf("panic: %v", recovered)
				}
			}()
			_, err := manager.Invoke(ctx, blocked)
			errCh <- err
		}()
	}
	require.Eventually(t, func() bool { return manager.VMCount() == vmPoolSize }, 5*time.Second, 10*time.Millisecond)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer waitCancel()
	_, err := manager.Invoke(waitCtx, invokeRequest(t, installed.ID, map[string]any{"block": true}, map[string]any{}, nil, 0))
	require.ErrorIs(t, err, ErrVMUnavailable)

	for _, cancel := range cancels {
		cancel()
	}
	for range vmPoolSize {
		select {
		case got := <-errCh:
			require.ErrorIs(t, got, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("blocked plugin call did not return")
		}
	}
	require.Zero(t, manager.VMCount())

	for range 5 {
		_, err := invokeCall(t, manager, installed.ID, map[string]any{"fail": true}, map[string]any{}, nil, time.Second)
		require.ErrorContains(t, err, "boom")
		require.NotErrorIs(t, err, ErrCircuitOpen)
	}
}

func TestPluginLogFieldsKeepIdentity(t *testing.T) {
	fields := pluginLogFields("acme.one", 9, "onEvent", map[string]any{
		"plugin_id":       "evil",
		"installation_id": "evil-install",
		"export":          "nope",
		"detail":          "kept",
	})
	seen := map[string]int{}
	for _, field := range fields {
		seen[field.Key]++
	}
	require.Equal(t, 1, seen["plugin_id"])
	require.Equal(t, 1, seen["installation_id"])
	require.Equal(t, 1, seen["export"])
	require.Equal(t, "acme.one", zapString(t, fields, "plugin_id"))
	require.Equal(t, int64(9), zapInt(t, fields, "installation_id"))
	require.Equal(t, "onEvent", zapString(t, fields, "export"))
	require.Equal(t, "kept", zapString(t, fields, "detail"))
}

const kvHostScript = `
export function onEvent(ctx, op) {
  try {
    if (op.op === "get") return ctx.kv.get(op.key, op.options);
    if (op.op === "set") {
      ctx.kv.set(op.key, op.value, op.options);
      return "set-ok";
    }
    if (op.op === "delete") return ctx.kv.delete(op.key, op.options);
    if (op.op === "list") return ctx.kv.list(op.options);
    return { name: "PluginHostError", code: "INVALID_ARGUMENT", message: "unknown op" };
  } catch (e) {
    return { name: e.name, code: e.code, message: e.message };
  }
}
`

func installJS(t *testing.T, manager *Manager, id, source string) *ent.Plugin {
	t.Helper()
	installed := installUI(t, manager, uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = id
		manifest["name"] = id
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

func invokeCall(t *testing.T, manager *Manager, id int, arg, state any, config map[string]any, timeout time.Duration) (InvokeResult, error) {
	t.Helper()
	return manager.Invoke(context.Background(), invokeRequest(t, id, arg, state, config, timeout))
}

func mustInvoke(t *testing.T, manager *Manager, id int, arg any, config map[string]any) json.RawMessage {
	t.Helper()
	result, err := invokeCall(t, manager, id, arg, map[string]any{}, config, time.Second)
	require.NoError(t, err)
	return result.Value
}

func invokeRequest(t *testing.T, id int, arg, state any, config map[string]any, timeout time.Duration) InvokeRequest {
	t.Helper()
	rawArgs, err := json.Marshal([]any{arg})
	require.NoError(t, err)
	rawState, err := json.Marshal(state)
	require.NoError(t, err)
	return InvokeRequest{
		InstallationID: id,
		Export:         "onEvent",
		Args:           rawArgs,
		State:          rawState,
		Config:         config,
		Timeout:        timeout,
	}
}

func decodeValue(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	if raw == nil {
		return nil
	}
	var value any
	require.NoError(t, json.Unmarshal(raw, &value))
	return value
}

func decodeState(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	value := decodeValue(t, raw)
	state, ok := value.(map[string]any)
	require.True(t, ok)
	return state
}

func kvOp(t *testing.T, manager *Manager, id int, op map[string]any) any {
	t.Helper()
	result, err := invokeCall(t, manager, id, op, map[string]any{}, nil, 10*time.Second)
	require.NoError(t, err)
	return decodeValue(t, result.Value)
}

func kvPageOp(t *testing.T, manager *Manager, id int, options map[string]any) kvPageJSON {
	t.Helper()
	value := kvOp(t, manager, id, map[string]any{"op": "list", "options": options})
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	var page kvPageJSON
	require.NoError(t, json.Unmarshal(raw, &page))
	return page
}

type kvPageJSON struct {
	Entries []struct {
		Namespace string  `json:"namespace"`
		Key       string  `json:"key"`
		Value     string  `json:"value"`
		ExpiresAt *string `json:"expiresAt"`
		UpdatedAt string  `json:"updatedAt"`
	} `json:"entries"`
	NextCursor *string `json:"nextCursor"`
}

func keysOfPage(page kvPageJSON) []string {
	keys := make([]string, 0, len(page.Entries))
	for _, entry := range page.Entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func kvStored(t *testing.T, manager *Manager, manifestID string, installationID int, key string) string {
	t.Helper()
	entry, err := manager.KVGet(pluginCtx(), manifestID, installationID, "", key)
	require.NoError(t, err)
	require.NotNil(t, entry)
	return entry.Value
}

func zapString(t *testing.T, fields []zap.Field, key string) string {
	t.Helper()
	for _, field := range fields {
		if field.Key == key && field.Type == zapcore.StringType {
			return field.String
		}
	}
	t.Fatalf("string field %s not found", key)
	return ""
}

func zapInt(t *testing.T, fields []zap.Field, key string) int64 {
	t.Helper()
	for _, field := range fields {
		if field.Key == key && field.Type == zapcore.Int64Type {
			return field.Integer
		}
	}
	t.Fatalf("int field %s not found", key)
	return 0
}

func TestInvokePoolReleasedAfterRetirement(t *testing.T) {
	for _, action := range []string{"disable", "delete"} {
		t.Run(action, func(t *testing.T) {
			manager := newTestManager(t, newPluginDB(t))
			installed := installJS(t, manager, "acme.retire", `export function onEvent() { return 1; }`)
			gen := manager.Current()
			require.NoError(t, gen.Acquire())
			defer gen.Release()
			_, err := invokeCall(t, manager, installed.ID, nil, nil, nil, time.Second)
			require.NoError(t, err)
			require.Equal(t, 1, manager.VMCount())
			if action == "disable" {
				require.NoError(t, manager.Disable(context.Background(), installed.ID))
			} else {
				require.NoError(t, manager.Delete(context.Background(), installed.ID))
			}
			// An acquired old generation keeps its runtime until it drains.
			require.Equal(t, 1, manager.VMCount())
			gen.Release()
			require.Zero(t, manager.VMCount())
			require.Empty(t, manager.execs)
			if action == "disable" {
				require.NoError(t, manager.Enable(context.Background(), installed.ID, nil, nil))
				_, err = invokeCall(t, manager, installed.ID, nil, nil, nil, time.Second)
				require.NoError(t, err)
				require.Equal(t, 1, manager.VMCount())
			}
		})
	}
}

func TestRetiredResourceKeepsNewModulePool(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	installed := installJS(t, manager, "acme.upgradepool", `export function onEvent() { return 1; }`)
	old := manager.Current()
	require.NoError(t, old.Acquire())
	defer old.Release()
	_, err := invokeCall(t, manager, installed.ID, nil, nil, nil, time.Second)
	require.NoError(t, err)
	_, err = manager.Install(pluginCtx(), uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.upgradepool"
		manifest["version"] = "2.0.0"
	}, map[string]string{
		"ui/page.html": "<html></html>",
		"plugin.js":    `let calls = 0; export function onEvent() { return ++calls; }`,
	}), nil, nil)
	require.NoError(t, err)
	result, err := invokeCall(t, manager, installed.ID, nil, nil, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(1), decodeValue(t, result.Value))
	old.Release()
	require.Equal(t, 1, manager.VMCount())
	result, err = invokeCall(t, manager, installed.ID, nil, nil, nil, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(2), decodeValue(t, result.Value))
}
