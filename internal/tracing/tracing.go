package tracing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Config struct {
	TraceHeader string `conf:"trace_header" yaml:"trace_header" json:"trace_header"`
}

// ContextKey 定义 context key 类型.
type ContextKey string

const (
	// TraceIDContextKey 用于在 context 中存储 trace id.
	TraceIDContextKey ContextKey = "trace_id"
	// OperationNameContextKey 用于在 context 中存储 operation name.
	OperationNameContextKey ContextKey = "operation_name"
)

// WithTraceID 将 trace id 存储到 context 中.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDContextKey, traceID)
}

// GetTraceID 从 context 中获取 trace id.
func GetTraceID(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(TraceIDContextKey).(string)
	return traceID, ok
}

// GenerateTraceID 生成一个新的 trace id，格式为 at-{{uuid}}.
func GenerateTraceID() string {
	id := uuid.New()
	return fmt.Sprintf("at-%s", id.String())
}

func WithOperationName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, OperationNameContextKey, name)
}

func GetOperationName(ctx context.Context) (string, bool) {
	operationName, ok := ctx.Value(OperationNameContextKey).(string)
	return operationName, ok
}
