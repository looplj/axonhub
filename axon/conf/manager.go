package conf

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/axon/bus"
)

type LoadResult[T any] struct {
	Value      T
	ConfigFile string
}

type Loader[T any] interface {
	Load(ctx context.Context) (LoadResult[T], error)
}

type ValidatorFunc[T any] func(ctx context.Context, v T) error

type DifferFunc[T any] func(oldV, newV T) ChangeSet

type ApplyResult struct {
	EffectiveAt  string
	Attributes   map[string]string
	NeedsRebuild bool
}

type RuntimeApplier[T any] interface {
	Apply(ctx context.Context, oldV, newV T, changes ChangeSet) (ApplyResult, error)
}

type Manager[T any] struct {
	store     *Store[T]
	loader    Loader[T]
	validate  ValidatorFunc[T]
	diff      DifferFunc[T]
	applier   RuntimeApplier[T]
	bus       bus.EventBus
	topic     string
	logger    *slog.Logger
	reloadMu  sync.Mutex
	lastReqID string
}

type ManagerOptions[T any] struct {
	Store    *Store[T]
	Loader   Loader[T]
	Validate ValidatorFunc[T]
	Diff     DifferFunc[T]
	Applier  RuntimeApplier[T]
	Bus      bus.EventBus
	Topic    string
	Logger   *slog.Logger
}

func NewManager[T any](opts ManagerOptions[T]) (*Manager[T], error) {
	if opts.Store == nil {
		return nil, fmt.Errorf("conf: Store is required")
	}
	if opts.Loader == nil {
		return nil, fmt.Errorf("conf: Loader is required")
	}
	if opts.Applier == nil {
		return nil, fmt.Errorf("conf: Applier is required")
	}
	topic := opts.Topic
	if topic == "" {
		topic = TopicReloadEvent
	}
	l := opts.Logger
	if l == nil {
		l = slog.Default()
	}
	return &Manager[T]{
		store:    opts.Store,
		loader:   opts.Loader,
		validate: opts.Validate,
		diff:     opts.Diff,
		applier:  opts.Applier,
		bus:      opts.Bus,
		topic:    topic,
		logger:   l,
	}, nil
}

func (m *Manager[T]) Reload(ctx context.Context, source ReloadSource) error {
	reqID := uuid.NewString()
	m.reloadMu.Lock()
	m.lastReqID = reqID
	m.reloadMu.Unlock()

	requestedAt := time.Now()
	m.publish(ctx, ReloadEvent{
		RequestID:   reqID,
		Type:        ReloadEventRequested,
		Source:      source,
		RequestedAt: requestedAt,
	})

	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	oldV, oldVer := m.store.Get()

	m.publish(ctx, ReloadEvent{
		RequestID:   reqID,
		Type:        ReloadEventStarted,
		Source:      source,
		OldVersion:  oldVer,
		RequestedAt: requestedAt,
	})

	loaded, err := m.loader.Load(ctx)
	if err != nil {
		m.publishFailure(ctx, reqID, source, oldVer, oldVer, loaded.ConfigFile, requestedAt, err)
		return err
	}

	newV := loaded.Value

	if m.validate != nil {
		if err := m.validate(ctx, newV); err != nil {
			m.publishFailure(ctx, reqID, source, oldVer, oldVer, loaded.ConfigFile, requestedAt, err)
			return err
		}
	}

	changes := ChangeSet{}
	if m.diff != nil {
		changes = m.diff(oldV, newV)
	} else if reflect.DeepEqual(oldV, newV) {
		m.publish(ctx, ReloadEvent{
			RequestID:      reqID,
			Type:           ReloadEventNoop,
			Source:         source,
			ConfigFile:     loaded.ConfigFile,
			OldVersion:     oldVer,
			NewVersion:     oldVer,
			RequestedAt:    requestedAt,
			CompletedAt:    time.Now(),
			ProcessingTime: time.Since(requestedAt),
		})
		return nil
	}

	res, err := m.applier.Apply(ctx, oldV, newV, changes)
	if err != nil {
		m.publishFailure(ctx, reqID, source, oldVer, oldVer, loaded.ConfigFile, requestedAt, err)
		return err
	}

	newVer := m.store.Set(newV)

	m.publish(ctx, ReloadEvent{
		RequestID:      reqID,
		Type:           ReloadEventApplied,
		Source:         source,
		ConfigFile:     loaded.ConfigFile,
		OldVersion:     oldVer,
		NewVersion:     newVer,
		ChangedKeys:    changes.ChangedKeys,
		SensitiveKeys:  changes.SensitiveKeys,
		EffectiveAt:    res.EffectiveAt,
		Attributes:     res.Attributes,
		NeedsRebuild:   res.NeedsRebuild,
		RequestedAt:    requestedAt,
		CompletedAt:    time.Now(),
		ProcessingTime: time.Since(requestedAt),
	})

	return nil
}

func (m *Manager[T]) publishFailure(ctx context.Context, reqID string, source ReloadSource, oldVer uint64, newVer uint64, configFile string, requestedAt time.Time, err error) {
	m.publish(ctx, ReloadEvent{
		RequestID:      reqID,
		Type:           ReloadEventFailed,
		Source:         source,
		ConfigFile:     configFile,
		OldVersion:     oldVer,
		NewVersion:     newVer,
		Error:          err.Error(),
		RequestedAt:    requestedAt,
		CompletedAt:    time.Now(),
		ProcessingTime: time.Since(requestedAt),
	})
}

func (m *Manager[T]) publish(ctx context.Context, ev ReloadEvent) {
	if m.bus == nil {
		return
	}
	if err := m.bus.Publish(ctx, bus.Event{
		Topic:   m.topic,
		Type:    string(ev.Type),
		Payload: ev,
	}); err != nil {
		m.logger.Error("conf: failed to publish event", "error", err)
	}
}
