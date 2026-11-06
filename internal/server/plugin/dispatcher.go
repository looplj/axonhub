package plugin

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/events"
)

type pluginEventKey struct{}

func withPluginEvent(ctx context.Context, event events.Event) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, pluginEventKey{}, event)
}

func pluginEventFrom(ctx context.Context) (events.Event, bool) {
	if ctx == nil {
		return events.Event{}, false
	}
	event, ok := ctx.Value(pluginEventKey{}).(events.Event)
	return event, ok
}

type pluginSubscription struct {
	key   string
	unsub func()
}

type pluginSubSpec struct {
	installationID int
	pluginID       string
	types          []string
	key            string
}

// eventDispatcher keeps one bus subscription per enabled installation.
// The parent context has no script deadline. It is canceled only when the manager stops.
type eventDispatcher struct {
	manager  *Manager
	bus      *events.Bus
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	subs     map[int]pluginSubscription
	inflight sync.WaitGroup
}

func newEventDispatcher(manager *Manager, bus *events.Bus) *eventDispatcher {
	if manager == nil || bus == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &eventDispatcher{
		manager: manager,
		bus:     bus,
		ctx:     ctx,
		cancel:  cancel,
		subs:    map[int]pluginSubscription{},
	}
}

func (d *eventDispatcher) sync(gen *Generation) {
	if d == nil {
		return
	}
	desired := map[int]pluginSubSpec{}
	if gen != nil {
		for _, item := range gen.Plugins() {
			if !eventPlugin(item) {
				continue
			}
			desired[item.ID] = pluginSubSpec{
				installationID: item.ID,
				pluginID:       item.PluginID,
				types:          slices.Clone(item.Manifest.Events),
				key:            eventListKey(item.PluginID, item.Manifest.Events),
			}
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	for id, sub := range d.subs {
		spec, ok := desired[id]
		if ok && spec.key == sub.key {
			delete(desired, id)
			continue
		}
		// Upgrade edge case intentionally deferred: unsubscribe does not wait for
		// an in-flight handler. A changed event list can briefly run the old and
		// replacement workers concurrently, including their KV read-modify-writes.
		sub.unsub()
		delete(d.subs, id)
	}
	for id, spec := range desired {
		unsub := d.bus.Subscribe(pluginSubscriptionName(spec.installationID), spec.types, func(_ context.Context, event events.Event) {
			d.deliver(spec.installationID, spec.pluginID, event)
		})
		d.subs[id] = pluginSubscription{key: spec.key, unsub: unsub}
	}
}

func (d *eventDispatcher) close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	d.cancel()
	subs := d.subs
	d.subs = map[int]pluginSubscription{}
	d.mu.Unlock()
	for _, sub := range subs {
		sub.unsub()
	}
	d.inflight.Wait()
}

func (d *eventDispatcher) deliver(installationID int, pluginID string, event events.Event) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.inflight.Add(1)
	d.mu.Unlock()
	defer d.inflight.Done()

	if event.Origin == "plugin:"+pluginID {
		return
	}
	if err := d.ctx.Err(); err != nil {
		return
	}
	// Upgrade edge case intentionally deferred: Invoke acquires the current
	// generation again. A swap between here and Invoke can pair this generation's
	// subscription check and config with the new generation's code and grants.
	gen := d.manager.Current()
	if gen == nil {
		return
	}
	loaded := gen.ByID(installationID)
	if loaded == nil || loaded.PluginID != pluginID || !eventPlugin(loaded) {
		return
	}
	if !slices.Contains(loaded.Manifest.Events, event.Type) {
		return
	}

	payload, err := json.Marshal(event.Map())
	if err != nil {
		log.Warn(d.ctx, "plugin event encode failed",
			log.String("plugin_id", pluginID),
			log.Int("installation_id", installationID),
			log.String("event_type", event.Type),
			log.Cause(err),
		)
		return
	}
	args, err := json.Marshal([]json.RawMessage{payload})
	if err != nil {
		log.Warn(d.ctx, "plugin event encode failed",
			log.String("plugin_id", pluginID),
			log.Int("installation_id", installationID),
			log.String("event_type", event.Type),
			log.Cause(err),
		)
		return
	}

	_, err = d.manager.Invoke(withPluginEvent(d.ctx, event), InvokeRequest{
		InstallationID: installationID,
		Export:         onEventExport,
		Args:           args,
		Config:         loaded.Config,
		Timeout:        onEventTimeout,
	})
	if err == nil || isEventDeliverySkip(err) {
		return
	}
	log.Warn(d.ctx, "plugin event handler failed",
		log.String("plugin_id", pluginID),
		log.Int("installation_id", installationID),
		log.String("event_type", event.Type),
		log.String("event_id", event.ID),
		log.Cause(err),
	)
}

func isEventDeliverySkip(err error) bool {
	return xerrors.Is(err, ErrVMUnavailable) ||
		xerrors.Is(err, ErrCircuitOpen) ||
		xerrors.Is(err, ErrGenerationClosed) ||
		xerrors.Is(err, context.Canceled)
}

func eventPlugin(item *LoadedPlugin) bool {
	return item != nil && item.Enabled && item.Status == RuntimeStatusActive && len(item.Manifest.Events) > 0
}

func eventListKey(pluginID string, types []string) string {
	cloned := slices.Clone(types)
	slices.Sort(cloned)
	return pluginID + "\x00" + strings.Join(cloned, "\x00")
}

func pluginSubscriptionName(installationID int) string {
	return "plugin-install:" + strconv.Itoa(installationID)
}
