package shared

import (
	"context"
)

type sessionContextKey struct{}

// WithSessionID sets the session ID in the context.
// Some provider will use the session ID to handle the prompt cache.
// We should set the session ID in the context if we want to use the cache.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sessionID)
}

// GetSessionID retrieves the session ID from the context.
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(sessionContextKey{}).(string)
	return sessionID, ok
}
