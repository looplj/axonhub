package bus

import (
	"context"
	"encoding/json"
	"fmt"
)

// Handler processes an event and returns an error if processing fails.
type Handler func(ctx context.Context, event Event) error

// Middleware wraps a handler to add cross-cutting concerns such as
// logging, recovery, tracing, or filtering.
type Middleware func(Handler) Handler

// Chain composes multiple middlewares into a single middleware.
// Middlewares are applied in order: the first middleware in the list
// is the outermost wrapper.
func Chain(middlewares ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// TypedHandler creates a Handler that extracts the event payload via type
// assertion into the specified type T before invoking fn.
// It also supports []byte payloads by falling back to JSON unmarshal.
func TypedHandler[T any](fn func(ctx context.Context, event Event, payload T) error) Handler {
	return func(ctx context.Context, event Event) error {
		v, ok := event.Payload.(T)
		if !ok {
			// Fall back to JSON unmarshal for []byte payloads.
			raw, isBytes := event.Payload.([]byte)
			if !isBytes {
				return fmt.Errorf("bus: unexpected payload type %T, want %T", event.Payload, v)
			}

			if err := json.Unmarshal(raw, &v); err != nil {
				return fmt.Errorf("bus: failed to unmarshal payload: %w", err)
			}
		}
		return fn(ctx, event, v)
	}
}
