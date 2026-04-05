package bus

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// EventBus defines the interface for an event bus.
// Implementations can be in-process (default), Kafka, Redis, NATS, etc.
type EventBus interface {
	// Publish publishes an event to all subscribers of the event's topic.
	Publish(ctx context.Context, event Event) error
	// Subscribe registers a handler for the given topic.
	Subscribe(topic string, handler Handler, middlewares ...Middleware) SubscriptionID
	// SubscribeWithFilter registers a handler with an event filter.
	SubscribeWithFilter(topic string, handler Handler, filter func(Event) bool, middlewares ...Middleware) SubscriptionID
	// Unsubscribe removes a subscription by its ID.
	Unsubscribe(id SubscriptionID)
	// Close closes the event bus and releases resources.
	Close() error
}

var _ EventBus = (*InProcessBus)(nil)

// InProcessBus is an in-process implementation of EventBus using Go channels.
// It is safe for concurrent use.
type InProcessBus struct {
	subscribers map[string][]subscription
	middlewares []Middleware
	mu          sync.RWMutex
	closed      bool
}

// SubscriptionID uniquely identifies a subscription, used for unsubscribing.
type SubscriptionID string

type subscription struct {
	id      SubscriptionID
	handler Handler
	filter  func(Event) bool
}

// New creates a new in-process EventBus with optional global middlewares.
func New(middlewares ...Middleware) EventBus {
	return NewInProcess(middlewares...)
}

// NewInProcess creates a new InProcessBus with optional global middlewares.
// Global middlewares are applied to every handler on publish.
func NewInProcess(middlewares ...Middleware) *InProcessBus {
	return &InProcessBus{
		subscribers: make(map[string][]subscription),
		middlewares: middlewares,
	}
}

// Publish publishes an event to all subscribers of the event's topic.
// If the event metadata ID is empty, a new UUID is generated.
// Returns the first error encountered from any subscriber handler.
func (b *InProcessBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return fmt.Errorf("bus: event bus is closed")
	}

	if event.Metadata.ID == "" {
		event.Metadata.ID = uuid.New().String()
	}

	subs := make([]subscription, len(b.subscribers[event.Topic]))
	copy(subs, b.subscribers[event.Topic])
	globalMiddlewares := make([]Middleware, len(b.middlewares))
	copy(globalMiddlewares, b.middlewares)
	b.mu.RUnlock()

	ctx = ContextWithEvent(ctx, event)
	ctx = ContextWithMetadata(ctx, event.Metadata)

	var chain Middleware
	if len(globalMiddlewares) > 0 {
		chain = Chain(globalMiddlewares...)
	}

	for _, sub := range subs {
		if sub.filter != nil && !sub.filter(event) {
			continue
		}

		h := sub.handler
		if chain != nil {
			h = chain(h)
		}

		if err := h(ctx, event); err != nil {
			return fmt.Errorf("bus: handler error on topic %q: %w", event.Topic, err)
		}
	}

	return nil
}

// Subscribe registers a handler for the given topic. Optional per-subscription
// middlewares are applied in addition to the global middlewares.
func (b *InProcessBus) Subscribe(topic string, handler Handler, middlewares ...Middleware) SubscriptionID {
	h := handler
	if len(middlewares) > 0 {
		h = Chain(middlewares...)(handler)
	}

	id := SubscriptionID(uuid.New().String())

	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[topic] = append(b.subscribers[topic], subscription{
		id:      id,
		handler: h,
	})
	return id
}

// SubscribeWithFilter registers a handler for the given topic that only
// receives events matching the provided filter predicate.
func (b *InProcessBus) SubscribeWithFilter(topic string, handler Handler, filter func(Event) bool, middlewares ...Middleware) SubscriptionID {
	h := handler
	if len(middlewares) > 0 {
		h = Chain(middlewares...)(handler)
	}

	id := SubscriptionID(uuid.New().String())

	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[topic] = append(b.subscribers[topic], subscription{
		id:      id,
		handler: h,
		filter:  filter,
	})
	return id
}

// Unsubscribe removes a subscription by its ID across all topics.
func (b *InProcessBus) Unsubscribe(id SubscriptionID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for topic, subs := range b.subscribers {
		for i, sub := range subs {
			if sub.id == id {
				b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// Close closes the event bus, preventing further publishes and clearing all subscribers.
func (b *InProcessBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	b.subscribers = make(map[string][]subscription)

	return nil
}
