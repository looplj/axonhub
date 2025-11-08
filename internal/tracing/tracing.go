package tracing

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/contexts"
)

type Config struct {
	// ThreadHeader is the header name for thread ID.
	// Default to "AH-Thread-Id".
	ThreadHeader string `conf:"thread_header" yaml:"thread_header" json:"thread_header"`

	// TraceHeader is the header name for trace ID.
	// Default to "AH-Trace-Id".
	TraceHeader string `conf:"trace_header" yaml:"trace_header" json:"trace_header"`

	// ExtraTraceHeaders is the extra header names for trace ID.
	// It will use if primary trace header is not found in request headers.
	// e.g. set it to []string{"Sentry-Trace"} to trace claude-code or any other product using sentry.
	// Default to nil.
	ExtraTraceHeaders []string `conf:"extra_trace_headers" yaml:"extra_trace_headers" json:"extra_trace_headers"`

	// ClaudeCodeTraceEnabled enables extracting trace IDs from Claude Code request metadata.
	// Default to false.
	ClaudeCodeTraceEnabled bool `conf:"claude_code_trace_enabled" yaml:"claude_code_trace_enabled" json:"claude_code_trace_enabled"`
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
