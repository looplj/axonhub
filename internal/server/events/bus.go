package events

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"

	"github.com/looplj/axonhub/internal/log"
)

const (
	// subscriberQueueCapacity is the bounded queue of one subscription.
	// A full queue drops that subscriber's copy.
	subscriberQueueCapacity = 1024

	// maxEventDepth is the deepest event that is still enqueued.
	// Depth greater than this is dropped and counted.
	maxEventDepth = 2
)

// Stats counts bus activity since NewBus.
//
// Published counts Publish calls that passed the depth check while the bus was open.
// Delivered counts event copies whose handler returned.
// DroppedDepth counts Publish calls dropped because Depth > 2.
// DroppedQueue counts per-subscriber copies dropped because that queue was full.
type Stats struct {
	Published    int64
	Delivered    int64
	DroppedDepth int64
	DroppedQueue int64
}

// Bus is an in-process, non-blocking event fan-out.
// Each subscription has one bounded queue and one worker goroutine.
type Bus struct {
	mu     sync.RWMutex
	subs   map[string]*subscription
	byType map[string]map[string]*subscription
	closed bool

	published    atomic.Int64
	delivered    atomic.Int64
	droppedDepth atomic.Int64
	droppedQueue atomic.Int64
}

type subscription struct {
	name     string
	types    map[string]struct{}
	queue    chan Event
	handler  func(context.Context, Event)
	stopCh   chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// NewBus returns an open bus. Close stops its workers.
func NewBus() *Bus {
	return &Bus{
		subs:   make(map[string]*subscription),
		byType: make(map[string]map[string]*subscription),
	}
}

// Publish enqueues a copy of event for each subscriber of event.Type.
// It does not block. Depth > 2 is dropped and counted, and is not enqueued.
// With no matching subscriber the call returns immediately.
// A full subscriber queue drops that subscriber's copy and counts it.
// Publish does not wait on ctx; a canceled caller still records the event.
func (b *Bus) Publish(ctx context.Context, event Event) {
	if event.Depth > maxEventDepth {
		b.droppedDepth.Add(1)
		return
	}

	subs, open := b.subscribers(event.Type)
	if !open {
		return
	}
	b.published.Add(1)
	for _, sub := range subs {
		copyEvent := cloneEvent(event)
		select {
		case sub.queue <- copyEvent:
		default:
			b.droppedQueue.Add(1)
		}
	}
}

// Subscribe registers handler for the exact event types.
// An empty type list subscribes to nothing.
//
// A duplicate name replaces the previous subscription: the previous worker is
// stopped, and the previous unsubscribe func stays idempotent without stopping
// the replacement. unsubscribe is idempotent and stops that subscription's worker.
func (b *Bus) Subscribe(name string, types []string, handler func(context.Context, Event)) (unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return func() {}
	}

	if prev := b.subs[name]; prev != nil {
		b.detachLocked(prev)
		prev.stop()
	}

	sub := newSubscription(name, types, handler)
	b.attachLocked(sub)
	go sub.run(&b.delivered)
	return func() {
		b.unsubscribe(sub)
	}
}

// HasSubscribers reports whether at least one live subscription includes eventType.
// The match is exact. An unsubscribed or replaced name is not live.
func (b *Bus) HasSubscribers(eventType string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.byType[eventType]) > 0
}

// Stats returns a point-in-time snapshot of the counters.
func (b *Bus) Stats() Stats {
	return Stats{
		Published:    b.published.Load(),
		Delivered:    b.delivered.Load(),
		DroppedDepth: b.droppedDepth.Load(),
		DroppedQueue: b.droppedQueue.Load(),
	}
}

// Close stops every worker and waits until each one exits or ctx ends.
// A second call is a no-op. Do not call Close from a subscription handler.
func (b *Bus) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	subs := make([]*subscription, 0, len(b.subs))
	for _, sub := range b.subs {
		subs = append(subs, sub)
	}
	b.subs = make(map[string]*subscription)
	b.byType = make(map[string]map[string]*subscription)
	b.mu.Unlock()

	for _, sub := range subs {
		sub.stop()
	}
	for _, sub := range subs {
		select {
		case <-sub.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (b *Bus) subscribers(eventType string) ([]*subscription, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, false
	}
	group := b.byType[eventType]
	subs := make([]*subscription, 0, len(group))
	for _, sub := range group {
		subs = append(subs, sub)
	}
	return subs, true
}

func (b *Bus) attachLocked(sub *subscription) {
	b.subs[sub.name] = sub
	for eventType := range sub.types {
		group := b.byType[eventType]
		if group == nil {
			group = make(map[string]*subscription)
			b.byType[eventType] = group
		}
		group[sub.name] = sub
	}
}

func (b *Bus) detachLocked(sub *subscription) {
	if current := b.subs[sub.name]; current == sub {
		delete(b.subs, sub.name)
	}
	for eventType := range sub.types {
		group := b.byType[eventType]
		if group[sub.name] == sub {
			delete(group, sub.name)
		}
		if len(group) == 0 {
			delete(b.byType, eventType)
		}
	}
}

func (b *Bus) unsubscribe(sub *subscription) {
	b.mu.Lock()
	if b.subs[sub.name] == sub {
		b.detachLocked(sub)
	}
	b.mu.Unlock()
	sub.stop()
}

func newSubscription(name string, types []string, handler func(context.Context, Event)) *subscription {
	set := make(map[string]struct{}, len(types))
	for _, eventType := range types {
		if eventType == "" {
			continue
		}
		set[eventType] = struct{}{}
	}
	if handler == nil {
		handler = func(context.Context, Event) {}
	}
	return &subscription{
		name:    name,
		types:   set,
		queue:   make(chan Event, subscriberQueueCapacity),
		handler: handler,
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (s *subscription) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *subscription) run(delivered *atomic.Int64) {
	defer close(s.done)
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(context.Background(), "event bus worker panicked",
				log.String("subscription", s.name),
				log.Any("panic", recovered),
				log.Stack("stack"),
			)
		}
	}()

	for {
		select {
		case <-s.stopCh:
			return
		case event := <-s.queue:
			s.deliver(delivered, event)
		}
	}
}

// deliver isolates a handler panic so one failure does not stop the worker.
// The subscription stays registered and later events are still delivered.
func (s *subscription) deliver(delivered *atomic.Int64, event Event) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(context.Background(), "event bus handler panicked",
				log.String("subscription", s.name),
				log.String("event_type", event.Type),
				log.Any("panic", recovered),
				log.Stack("stack"),
			)
		}
	}()
	s.handler(context.Background(), event)
	delivered.Add(1)
}

func cloneEvent(event Event) Event {
	event.Data = maps.Clone(event.Data)
	event.Subject = cloneSubject(event.Subject)
	return event
}

func cloneSubject(subject Subject) Subject {
	if subject.Channel != nil {
		channel := *subject.Channel
		subject.Channel = &channel
	}
	if subject.Credential != nil {
		credential := *subject.Credential
		subject.Credential = &credential
	}
	if subject.APIKey != nil {
		apiKey := *subject.APIKey
		subject.APIKey = &apiKey
	}
	if subject.Project != nil {
		project := *subject.Project
		subject.Project = &project
	}
	if subject.Request != nil {
		request := *subject.Request
		subject.Request = &request
	}
	return subject
}
