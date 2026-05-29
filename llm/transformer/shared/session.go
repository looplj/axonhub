package shared

import (
	"context"
)

// sessionContextKey is the key used to store and retrieve the session ID from the context.
type sessionContextKey struct{}

// sessionScopeContextKey is the key used to namespace client-provided session IDs.
type sessionScopeContextKey struct{}

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

// WithSessionScope sets a trusted server-side namespace for session-scoped upstream state.
func WithSessionScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, sessionScopeContextKey{}, scope)
}

// GetSessionScope retrieves the trusted server-side namespace for session-scoped upstream state.
func GetSessionScope(ctx context.Context) (string, bool) {
	scope, ok := ctx.Value(sessionScopeContextKey{}).(string)
	return scope, ok
}
