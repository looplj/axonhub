package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm/httpclient"
)

const failoverGuardScript = `
export function onEvent(ctx, event) {
  const ch = event.subject.channel;
  const n = ctx.counter.add("failover:" + ch.id + ":" + event.subject.model, 300);
  if (n === ctx.config.threshold) {
    ctx.webhook.send(ctx.config.target, { message: ch.name + " failover " + n });
    ctx.axonhub.channels.disable(ch.id, { reason: "failover threshold" });
  }
  ctx.kv.set("n", String(n));
}
`

func TestEventDeliveryFollowsManifest(t *testing.T) {
	client := newPluginDB(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	manager := newEventManager(t, client, xcache.Config{Mode: xcache.ModeMemory}, bus, nil, nil)

	installed := installEventPlugin(t, manager, "acme.events", []string{"request.failover"}, nil, `
export function onEvent(ctx, event) {
  const prev = ctx.kv.get("seen") || "";
  ctx.kv.set("seen", prev + event.type + "\n");
}
`)
	require.True(t, bus.HasSubscribers(events.TypeRequestFailover))
	require.False(t, bus.HasSubscribers(events.TypeRequestCompleted))

	bus.Publish(context.Background(), events.Event{ID: "completed", Type: events.TypeRequestCompleted, OccurredAt: time.Now().UTC()})
	bus.Publish(context.Background(), events.Event{
		ID: "self", Type: events.TypeRequestFailover, OccurredAt: time.Now().UTC(), Origin: "plugin:acme.events",
	})
	bus.Publish(context.Background(), events.Event{
		ID: "ok", Type: events.TypeRequestFailover, OccurredAt: time.Now().UTC(),
	})
	waitKV(t, manager, "acme.events", installed.ID, "seen", "request.failover\n")

	require.NoError(t, manager.UpdateSettings(pluginCtx(), installed.ID, 0, map[string]any{}))
	bus.Publish(context.Background(), events.Event{
		ID: "again", Type: events.TypeRequestFailover, OccurredAt: time.Now().UTC(),
	})
	waitKV(t, manager, "acme.events", installed.ID, "seen", "request.failover\nrequest.failover\n")

	require.NoError(t, manager.Disable(pluginCtx(), installed.ID))
	require.False(t, bus.HasSubscribers(events.TypeRequestFailover))
	bus.Publish(context.Background(), events.Event{
		ID: "after-disable", Type: events.TypeRequestFailover, OccurredAt: time.Now().UTC(),
	})
	require.Equal(t, "request.failover\nrequest.failover\n", kvValue(t, manager, "acme.events", installed.ID, "seen"))

	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, nil, nil))
	require.True(t, bus.HasSubscribers(events.TypeRequestFailover))
	require.NoError(t, manager.Stop(context.Background()))
	require.False(t, bus.HasSubscribers(events.TypeRequestFailover))
}

func TestEventPermissionDeniedHasNoSideEffect(t *testing.T) {
	client := newPluginDB(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	channels := newEventChannels(t, client, bus)
	manager := newEventManager(t, client, xcache.Config{Mode: xcache.ModeMemory}, bus, channels, nil)
	ch := createEventChannel(t, client, "denied-channel", []string{"sk-denied"})

	installed := installEventPlugin(t, manager, "acme.denied", []string{"request.failover"}, []string{"write_channels"}, `
export function onEvent(ctx, event) {
  try {
    ctx.axonhub.channels.disable(event.subject.channel.id, { reason: "nope" });
    ctx.kv.set("result", "wrote");
  } catch (e) {
    ctx.kv.set("result", e.name + ":" + e.code);
  }
}
`)
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, nil, nil))
	publishFailover(bus, ch, "", 0)
	waitKV(t, manager, "acme.denied", installed.ID, "result", "PluginHostError:PERMISSION_DENIED")

	stored, err := client.Channel.Get(pluginCtx(), ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, stored.Status)
}

func TestFailoverThresholdDisablesChannel(t *testing.T) {
	t.Run("memory", func(t *testing.T) {
		runFailoverThreshold(t, xcache.Config{Mode: xcache.ModeMemory}, nil)
	})
	t.Run("miniredis", func(t *testing.T) {
		mr := miniredis.RunT(t)
		runFailoverThreshold(t, xcache.Config{
			Mode:  xcache.ModeTwoLevel,
			Redis: xredis.Config{Addr: mr.Addr()},
		}, mr)
	})
}

func TestCredentialDisablePublishesPluginOrigin(t *testing.T) {
	client := newPluginDB(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	channels := newEventChannels(t, client, bus)
	manager := newEventManager(t, client, xcache.Config{Mode: xcache.ModeMemory}, bus, channels, nil)
	secret := "sk-credential-plugin-test"
	ch := createEventChannel(t, client, "cred-channel", []string{secret, secret})
	fingerprint := events.Fingerprint(secret)

	received := make(chan events.Event, 1)
	bus.Subscribe("test-credential", []string{events.TypeChannelCredentialDisabled}, func(_ context.Context, event events.Event) {
		received <- event
	})
	installed := installEventPlugin(t, manager, "acme.cred", []string{"request.failover"}, []string{"write_channels"}, fmt.Sprintf(`
export function onEvent(ctx, event) {
  try {
    ctx.axonhub.channels.disableCredential(1, "oauth");
    ctx.kv.set("oauth", "wrote");
  } catch (e) {
    ctx.kv.set("oauth", e.code);
  }
  ctx.axonhub.channels.disableCredential(event.subject.channel.id, "%s", { reason: "plugin action" });
  ctx.kv.set("done", "yes");
}
`, fingerprint))
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, []string{"write_channels"}, nil))
	publishFailover(bus, ch, "", 0)
	waitKV(t, manager, "acme.cred", installed.ID, "done", "yes")
	require.Equal(t, "INVALID_ARGUMENT", kvValue(t, manager, "acme.cred", installed.ID, "oauth"))

	select {
	case event := <-received:
		require.Equal(t, "plugin:acme.cred", event.Origin)
		require.Equal(t, 1, event.Depth)
		require.Equal(t, fingerprint, event.Subject.Credential.Fingerprint)
		require.Equal(t, "plugin action", event.Data["reason"])
		require.Nil(t, event.Data["expiresAt"])
		require.Equal(t, 0, event.Data["errorCode"])
		encoded := fmt.Sprintf("%#v", event.Map())
		require.NotContains(t, encoded, secret)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for credential event")
	}

	stored, err := client.Channel.Get(pluginCtx(), ch.ID)
	require.NoError(t, err)
	require.Len(t, stored.DisabledAPIKeys, 1)
	require.Equal(t, secret, stored.DisabledAPIKeys[0].Key)
}

func TestCredentialDisableTakesOverAQuotaOwnedRecord(t *testing.T) {
	client := newPluginDB(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	channels := newEventChannels(t, client, bus)
	manager := newEventManager(t, client, xcache.Config{Mode: xcache.ModeMemory}, bus, channels, nil)
	secret := "sk-credential-takeover"
	ch := createEventChannel(t, client, "takeover-channel", []string{secret})
	fingerprint := events.Fingerprint(secret)

	// A provider quota observation parked the key.
	expiredAt := time.Now().Add(-24 * time.Hour)
	_, err := client.Channel.UpdateOneID(ch.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{
			Key:           secret,
			DisabledAt:    expiredAt,
			Reason:        "Provider quota exhausted",
			QuotaDisabled: true,
		}}).
		Save(pluginCtx())
	require.NoError(t, err)

	installed := installEventPlugin(t, manager, "acme.takeover", []string{"request.failover"}, []string{"write_channels"}, fmt.Sprintf(`
export function onEvent(ctx, event) {
  const runs = ctx.counter.add("runs", 300);
  ctx.axonhub.channels.disableCredential(event.subject.channel.id, "%s", { reason: "plugin policy" });
  ctx.kv.set("once", "done");
  ctx.kv.set("runs", String(runs));
}
`, fingerprint))
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, []string{"write_channels"}, nil))
	publishFailover(bus, ch, "", 0)
	waitKV(t, manager, "acme.takeover", installed.ID, "runs", "1")

	stored, err := client.Channel.Get(pluginCtx(), ch.ID)
	require.NoError(t, err)
	require.Len(t, stored.DisabledAPIKeys, 1)
	require.False(t, stored.DisabledAPIKeys[0].QuotaDisabled, "the plugin disable takes ownership away from the quota reconciler")
	require.Equal(t, "plugin policy", stored.DisabledAPIKeys[0].Reason)

	// A second disable of the now plugin-owned key is a genuine duplicate and
	// must stay idempotent: the record and its reason are untouched. The runs
	// counter only reaches two when the second handler has actually run, so the
	// assertions below cannot pass off a duplicate that never happened.
	publishFailover(bus, ch, "", 0)
	waitKV(t, manager, "acme.takeover", installed.ID, "runs", "2")

	again, err := client.Channel.Get(pluginCtx(), ch.ID)
	require.NoError(t, err)
	require.Len(t, again.DisabledAPIKeys, 1)
	require.False(t, again.DisabledAPIKeys[0].QuotaDisabled)
	require.Equal(t, "plugin policy", again.DisabledAPIKeys[0].Reason)
	require.Equal(t, stored.DisabledAPIKeys[0].DisabledAt, again.DisabledAPIKeys[0].DisabledAt, "the idempotent disable keeps the original record")
}

func TestInstallRejectsClosedEventsWithoutWrite(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	for _, name := range []string{"channel.auto_disabled", "provider.quota.warning", "provider.quota.exhausted"} {
		t.Run(name, func(t *testing.T) {
			raw := eventZip(t, "acme.closed", []string{name}, nil, "export function onEvent() {}")
			_, err := manager.Install(pluginCtx(), raw, nil, nil)
			require.ErrorContains(t, err, name)
			require.ErrorContains(t, err, "is not enabled")
			require.Zero(t, pluginCount(t, client))
		})
	}

	t.Run("plugin.js required", func(t *testing.T) {
		raw := eventZip(t, "acme.closed", []string{"request.failover"}, nil, "")
		_, err := manager.Install(pluginCtx(), raw, nil, nil)
		require.ErrorContains(t, err, "plugin.js is required")
		require.Zero(t, pluginCount(t, client))
	})

	t.Run("async onEvent can be installed", func(t *testing.T) {
		raw := eventZip(t, "acme.closed", []string{"request.failover"}, nil, "export async function onEvent() { return 1 }")
		installed, err := manager.Install(pluginCtx(), raw, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, installed.Plugin)
	})

	t.Run("top-level await", func(t *testing.T) {
		before := pluginCount(t, client)
		raw := eventZip(t, "acme.closed", []string{"request.failover"}, nil, "await Promise.resolve(1)\nexport function onEvent() {}")
		_, err := manager.Install(pluginCtx(), raw, nil, nil)
		require.ErrorContains(t, err, "top-level await is not allowed")
		require.Equal(t, before, pluginCount(t, client))
	})
}

func TestWriteLimitHasNoSideEffect(t *testing.T) {
	client := newPluginDB(t)
	manager := newTestManager(t, client)
	now := time.Date(2026, 11, 6, 4, 0, 0, 0, time.UTC)
	manager.kvNow = func() time.Time { return now }
	installed := installJS(t, manager, "acme.writes", `
export function onEvent(ctx, op) {
  try {
    if (op.op === "burst") {
      for (let i = 0; i < op.n; i++) ctx.kv.set("w" + i, "v");
      return "ok";
    }
    ctx.kv.set(op.key, "v");
    return "ok";
  } catch (e) {
    return e.code;
  }
}
`)
	require.Equal(t, "RATE_LIMITED", decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{"op": "burst", "n": 11}, nil)))
	require.Equal(t, "v", kvValue(t, manager, "acme.writes", installed.ID, "w0"))
	require.Empty(t, kvValue(t, manager, "acme.writes", installed.ID, "w10"))
	require.Equal(t, "ok", decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{"key": "w10"}, nil)))

	now = now.Add(time.Minute + time.Second)
	for range maxWritesPerMinute {
		require.True(t, manager.allowWrite("acme.writes"))
	}
	require.Equal(t, "RATE_LIMITED", decodeValue(t, mustInvoke(t, manager, installed.ID, map[string]any{"key": "minute"}, nil)))
	require.Empty(t, kvValue(t, manager, "acme.writes", installed.ID, "minute"))
	require.Len(t, manager.writes.calls["acme.writes"], maxWritesPerMinute)
}

func runFailoverThreshold(t *testing.T, cache xcache.Config, mr *miniredis.Miniredis) {
	t.Helper()
	client := newPluginDB(t)
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	var hits atomic.Int32
	var body atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		body.Store(string(raw))
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	channels := newEventChannels(t, client, bus)
	require.NoError(t, channels.SystemService.SetWebhookNotifierConfig(pluginCtx(), &biz.WebhookNotifierConfig{
		Targets: []biz.WebhookTarget{{
			Name:      "duty",
			Enabled:   true,
			URL:       server.URL,
			TimeoutMs: 1000,
			Body:      "{{.Message}}",
		}},
	}))
	notifier := biz.NewWebhookNotifier(channels.SystemService, httpclient.NewHttpClient())
	channels.WebhookNotifier = notifier
	manager := newEventManager(t, client, cache, bus, channels, notifier)
	ch := createEventChannel(t, client, "guard-channel", []string{"sk-guard"})

	installed := installEventPlugin(t, manager, "acme.guard", []string{"request.failover"}, []string{"write_channels", "webhook"}, failoverGuardScript)
	require.NoError(t, manager.Enable(pluginCtx(), installed.ID, []string{"write_channels", "webhook"}, nil))
	require.NoError(t, manager.UpdateSettings(pluginCtx(), installed.ID, 0, map[string]any{
		"threshold": 10,
		"target":    "duty",
	}))

	for i := 1; i <= 10; i++ {
		publishFailover(bus, ch, "", 0)
	}
	waitKV(t, manager, "acme.guard", installed.ID, "n", "10")
	require.Equal(t, int32(1), hits.Load())
	require.Equal(t, "guard-channel failover 10", body.Load())
	stored, err := client.Channel.Get(pluginCtx(), ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, stored.Status)

	publishFailover(bus, ch, "", 0)
	waitKV(t, manager, "acme.guard", installed.ID, "n", "11")
	require.Equal(t, int32(1), hits.Load())
	stored, err = client.Channel.Get(pluginCtx(), ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, stored.Status)

	if mr != nil {
		found := false
		for _, key := range mr.Keys() {
			if strings.HasPrefix(key, pluginCounterPrefix) && strings.Contains(key, ":300}:") {
				found = true
			}
		}
		require.True(t, found, "counter keys: %v", mr.Keys())
	}
}

func newEventManager(t *testing.T, client *ent.Client, cache xcache.Config, bus *events.Bus, channels *biz.ChannelService, hooks *biz.WebhookNotifier) *Manager {
	t.Helper()
	manager, err := newManager(ManagerParams{
		Client:   client,
		Cache:    cache,
		Bus:      bus,
		Channels: channels,
		Webhooks: hooks,
	}, managerConfig{
		refreshInterval: time.Hour,
		debounce:        time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, manager.Stop(context.Background()))
	})
	return manager
}

func newEventChannels(t *testing.T, client *ent.Client, bus *events.Bus) *biz.ChannelService {
	t.Helper()
	channels := biz.NewChannelServiceForTest(client)
	channels.EventBus = bus
	channels.WebhookNotifier = biz.NewWebhookNotifier(channels.SystemService, httpclient.NewHttpClient())
	t.Cleanup(channels.Stop)
	return channels
}

func createEventChannel(t *testing.T, client *ent.Client, name string, keys []string) *ent.Channel {
	t.Helper()
	ch, err := client.Channel.Create().
		SetName(name).
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKeys: keys}).
		SetSupportedModels([]string{"gpt-5"}).
		SetDefaultTestModel("gpt-5").
		SetStatus(channel.StatusEnabled).
		Save(pluginCtx())
	require.NoError(t, err)
	return ch
}

func installEventPlugin(t *testing.T, manager *Manager, id string, eventNames, permissions []string, source string) *ent.Plugin {
	t.Helper()
	installed := installUI(t, manager, eventZip(t, id, eventNames, permissions, source))
	if permissions == nil {
		require.NoError(t, manager.Enable(pluginCtx(), installed.ID, nil, nil))
		current := manager.Current().ByID(installed.ID)
		require.NotNil(t, current)
		require.Equal(t, RuntimeStatusActive, current.Status, current.Reason)
	}
	return installed
}

func eventZip(t *testing.T, id string, eventNames, permissions []string, source string) []byte {
	t.Helper()
	return uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = id
		manifest["name"] = id
		manifest["events"] = stringSliceAny(eventNames)
		if permissions != nil {
			manifest["permissions"] = stringSliceAny(permissions)
		}
	}, eventZipFiles(source))
}

func eventZipFiles(source string) map[string]string {
	files := map[string]string{"ui/page.html": "<html></html>"}
	if source != "" {
		files["plugin.js"] = source
	}
	return files
}

func stringSliceAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func publishFailover(bus *events.Bus, ch *ent.Channel, origin string, depth int) {
	bus.Publish(context.Background(), events.Event{
		ID:         fmt.Sprintf("failover-%d-%d", ch.ID, time.Now().UnixNano()),
		Type:       events.TypeRequestFailover,
		Severity:   events.SeverityWarning,
		OccurredAt: time.Now().UTC(),
		Origin:     origin,
		Depth:      depth,
		Subject: events.Subject{
			Channel: &events.ChannelRef{
				ID:      ch.ID,
				Name:    ch.Name,
				Type:    ch.Type.String(),
				BaseURL: ch.BaseURL,
				Status:  ch.Status.String(),
			},
			Model: "gpt-5",
		},
	})
}

func waitKV(t *testing.T, manager *Manager, pluginID string, installationID int, key, want string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		last = kvValue(t, manager, pluginID, installationID, key)
		if last == want {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("kv %s = %q, want %q", key, last, want)
}

func kvValue(t *testing.T, manager *Manager, pluginID string, installationID int, key string) string {
	t.Helper()
	entry, err := manager.KVGet(pluginCtx(), pluginID, installationID, "", key)
	require.NoError(t, err)
	if entry == nil {
		return ""
	}
	return entry.Value
}

func pluginCount(t *testing.T, client *ent.Client) int {
	t.Helper()
	count, err := client.Plugin.Query().Count(pluginCtx())
	require.NoError(t, err)
	return count
}
