package events

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/looplj/axonhub/internal/log"
)

// EventBroker manages event publication and subscription
type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	logger      *log.Logger
}

// Subscriber represents a client subscribed to events
type Subscriber struct {
	ID        string
	Topic     Topic
	ProjectID *int        // nil = all projects
	Events    chan *Event // buffered channel
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewEventBroker creates a new event broker
func NewEventBroker(logger *log.Logger) *EventBroker {
	return &EventBroker{
		subscribers: make(map[string]*Subscriber),
		logger:      logger,
	}
}

// Subscribe creates a new subscription to a topic
// projectID == nil subscribes to all projects (admin use case)
func (b *EventBroker) Subscribe(ctx context.Context, topic Topic, projectID *int) *Subscriber {
	subscriberID := uuid.New().String()

	subCtx, cancel := context.WithCancel(ctx)
	subscriber := &Subscriber{
		ID:        subscriberID,
		Topic:     topic,
		ProjectID: projectID,
		Events:    make(chan *Event, 100), // buffer 100 events
		ctx:       subCtx,
		cancel:    cancel,
	}

	b.mu.Lock()
	b.subscribers[subscriberID] = subscriber
	b.mu.Unlock()

	return subscriber
}

// Unsubscribe removes a subscriber and closes its channel
func (b *EventBroker) Unsubscribe(subscriberID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subscriber, exists := b.subscribers[subscriberID]; exists {
		subscriber.cancel()
		close(subscriber.Events)
		delete(b.subscribers, subscriberID)

	}
}

// Publish sends an event to all matching subscribers
// Non-blocking: uses select with default to avoid slow subscriber blocking
func (b *EventBroker) Publish(ctx context.Context, event *Event) {
	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	sent := 0
	dropped := 0
	for _, subscriber := range b.subscribers {
		// Filter by topic
		if subscriber.Topic != event.Topic {
			continue
		}

		// Filter by project ID if subscriber has project filter
		if subscriber.ProjectID != nil && !matchesProjectFilter(event, *subscriber.ProjectID) {
			continue
		}

		// Non-blocking send to avoid slow subscriber blocking others
		select {
		case subscriber.Events <- event:
			sent++
		default:
			// Channel full, drop event (subscriber too slow)
			dropped++
		}
	}
}

// matchesProjectFilter checks if event matches subscriber's project filter
func matchesProjectFilter(event *Event, projectID int) bool {
	switch event.Topic {
	case TopicRequests:
		if payload, ok := event.Payload.(*RequestEventPayload); ok {
			return payload.ProjectID == projectID
		}
		// Future: case TopicTraces, TopicChannels
	}
	return false
}

// Shutdown gracefully shuts down the broker
func (b *EventBroker) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id := range b.subscribers {
		if subscriber, exists := b.subscribers[id]; exists {
			subscriber.cancel()
			close(subscriber.Events)
			delete(b.subscribers, id)
		}
	}
}

// SubscriberCount returns the current number of subscribers
func (b *EventBroker) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
