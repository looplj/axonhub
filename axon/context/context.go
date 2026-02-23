package axoncontext

import "context"

type contextKey string

const (
	threadIDKey contextKey = "thread_id"
	traceIDKey  contextKey = "trace_id"
)

// WithThreadID returns a new context with the given thread ID.
func WithThreadID(ctx context.Context, threadID string) context.Context {
	return context.WithValue(ctx, threadIDKey, threadID)
}

// ThreadID returns the thread ID from the context, or empty string if not set.
func ThreadID(ctx context.Context) string {
	v, _ := ctx.Value(threadIDKey).(string)
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
