package bus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

func WithLogging(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			start := time.Now()
			logger.Debug("bus: processing event",
				"topic", event.Topic,
				"type", event.Type,
				"event_id", event.Metadata.ID,
			)

			err := next(ctx, event)

			if err != nil {
				logger.Error("bus: event processing failed",
					"topic", event.Topic,
					"type", event.Type,
					"event_id", event.Metadata.ID,
					"duration", time.Since(start),
					"error", err,
				)
			} else {
				logger.Debug("bus: event processed",
					"topic", event.Topic,
					"type", event.Type,
					"event_id", event.Metadata.ID,
					"duration", time.Since(start),
				)
			}

			return err
		}
	}
}

func WithRecover(logger *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, event Event) (retErr error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("bus: handler panicked",
						"topic", event.Topic,
						"event_id", event.Metadata.ID,
						"panic", r,
					)
					retErr = fmt.Errorf("bus: handler panicked: %v", r)
				}
			}()
			return next(ctx, event)
		}
	}
}

// WithTracing returns a middleware that propagates trace context.
// If the event metadata has no TraceID, a new one is generated and
// injected into both the event metadata and the context.
func WithTracing() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			if event.Metadata.TraceID == "" {
				event.Metadata.TraceID = uuid.New().String()
			}

			ctx = ContextWithMetadata(ctx, event.Metadata)

			return next(ctx, event)
		}
	}
}

// WithFilter returns a middleware that skips handler invocation when the
// accept predicate returns false for the event.
func WithFilter(accept func(Event) bool) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, event Event) error {
			if !accept(event) {
				return nil
			}
			return next(ctx, event)
		}
	}
}
