package watcher

import "context"

// Watcher provides a best-effort cross-goroutine / cross-instance watch stream.
//
// It is designed for cache invalidation / reload signals rather than durable delivery:
// implementations may drop events when subscribers are slow or disconnected.
//
// Watch is reference-counted via the returned stop function; callers must call stop
// exactly once to avoid resource leaks (e.g. goroutines, Redis pubsub connections).
type Watcher[T any] interface {
	// Watch subscribes to the watch stream and returns:
	//   - a channel that emits events
	//   - a stop function to unsubscribe (must be called once)
	Watch() (<-chan T, func())
}

// Notifier is a Watcher that can also publish events.
//
// Typical usage:
//   - writer side (mutations): call Notify(...) after updating the source of truth
//   - reader side (live cache): depend only on Watcher to trigger reload
type Notifier[T any] interface {
	Watcher[T]

	// Notify broadcasts the value to all subscribers.
	Notify(ctx context.Context, v T) error
}
