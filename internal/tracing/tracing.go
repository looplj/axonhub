package tracing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Config struct {
	TraceHeader string `conf:"trace_header"`
}

// ContextKey 定义 context key 类型.
type ContextKey string

const (
	// TraceIDContextKey 用于在 context 中存储 trace id.
	TraceIDContextKey ContextKey = "trace_id"
)

// TraceID represents a trace identifier.
type TraceID string

// String returns the string representation of the trace ID.
func (t TraceID) String() string {
	return string(t)
}

// WithTraceID 将 trace id 存储到 context 中.
func WithTraceID(ctx context.Context, traceID TraceID) context.Context {
	return context.WithValue(ctx, TraceIDContextKey, traceID)
}

// GetTraceID 从 context 中获取 trace id.
func GetTraceID(ctx context.Context) (TraceID, bool) {
	traceID, ok := ctx.Value(TraceIDContextKey).(TraceID)
	return traceID, ok
}

// GenerateTraceID 生成一个新的 trace id，格式为 at-{{uuid}}.
func GenerateTraceID() TraceID {
	id := uuid.New()
	return TraceID(fmt.Sprintf("at-%s", id.String()))
}
