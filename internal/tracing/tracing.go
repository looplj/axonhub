package tracing

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/contexts"
)

type Config struct {
	ThreadHeader string `conf:"thread_header" yaml:"thread_header" json:"thread_header"`
	TraceHeader  string `conf:"trace_header" yaml:"trace_header" json:"trace_header"`
}

// GenerateTraceID generate trace id, format as at-{{uuid}}.
func GenerateTraceID() string {
	id := uuid.New()
	return fmt.Sprintf("at-%s", id.String())
}

// WithTraceID store trace id to context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return contexts.WithTraceID(ctx, traceID)
}

// GetTraceID get trace id from context.
func GetTraceID(ctx context.Context) (string, bool) {
	return contexts.GetTraceID(ctx)
}

// WithOperationName store operation name to context.
func WithOperationName(ctx context.Context, name string) context.Context {
	return contexts.WithOperationName(ctx, name)
}

// GetOperationName get operation name from context.
func GetOperationName(ctx context.Context) (string, bool) {
	return contexts.GetOperationName(ctx)
}
