package axoncontext

import (
	"context"
	"strings"
)

type contextKey string

const (
	threadIDKey contextKey = "thread_id"
	traceIDKey  contextKey = "trace_id"
	workspaceKey contextKey = "workspace"
)

// WithThreadID returns a new context with the given thread ID.
func WithThreadID(ctx context.Context, threadID string) context.Context {
	return context.WithValue(ctx, threadIDKey, threadID)
}

// ThreadID returns the thread ID from the context, or empty string if not set.
func ThreadID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(threadIDKey).(string)
	v = strings.TrimSpace(v)
	return v
}

// WithTraceID returns a new context with the given trace ID.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceID returns the trace ID from the context, or empty string if not set.
func TraceID(ctx context.Context) string {
	v, _ := ctx.Value(traceIDKey).(string)
	return v
}

// WithWorkspace returns a new context with the given workspace path.
func WithWorkspace(ctx context.Context, workspace string) context.Context {
	return context.WithValue(ctx, workspaceKey, workspace)
}

// Workspace returns the workspace path from the context, or empty string if not set.
func Workspace(ctx context.Context) string {
	v, _ := ctx.Value(workspaceKey).(string)
	return v
}
