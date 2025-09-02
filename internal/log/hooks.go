package log

import (
	"context"

	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/tracing"
)

type Hook interface {
	Apply(ctx context.Context, msg string, fields ...Field) []Field
}

type HookFunc func(context.Context, string, ...Field) []Field

func (h HookFunc) Apply(ctx context.Context, msg string, fields ...Field) []Field {
	return h(ctx, msg, fields...)
}

type fieldsHook struct {
	fields []Field
}

func (f *fieldsHook) Apply(ctx context.Context, msg string, fields ...Field) []Field {
	return append(f.fields, fields...)
}

func contextFields(ctx context.Context, msg string, fields ...Field) []Field {
	if ctx == nil {
		return nil
	}

	if ctx.Err() != nil {
		fields = append(fields, NamedError("context_error", ctx.Err()))
	}

	if ts, ok := ctx.Deadline(); ok {
		fields = append(fields, Time("context_deadline", ts))
	}

	return fields
}

// Apply adds trace ID to log entries if it exists in the context.
func traceFields(ctx context.Context, msg string, fields ...zap.Field) []zap.Field {
	if ctx == nil {
		return fields
	}

	// Try to get trace ID from context
	if traceID, ok := tracing.GetTraceID(ctx); ok {
		// Add trace ID to fields
		fields = append(fields, zap.String("trace_id", traceID.String()))
	}

	return fields
}
