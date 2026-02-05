package shared

import (
	"context"
)

// sessionContextKey is the key used to store and retrieve the session ID from the context.
type sessionContextKey struct{}

// WithSessionID sets the session ID in the context.
// This is essential for features that require cross-request state, such as:
// 1. Prompt Caching: Providers like Anthropic use session/trace IDs to optimize cache hits.
// 2. Tracing: It allows linking the transformation pipeline with the unified tracing system (AH-Trace-Id).
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sessionID)
}

// GetSessionID retrieves the session ID from the context.
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(sessionContextKey{}).(string)
	return sessionID, ok
}
