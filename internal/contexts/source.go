package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent/request"
)

const (
	// SourceContextKey 用于在 context 中存储请求来源.
	SourceContextKey ContextKey = "source"
)

// WithSource 将请求来源存储到 context 中.
func WithSource(ctx context.Context, source request.Source) context.Context {
	return context.WithValue(ctx, SourceContextKey, source)
}

// GetSource 从 context 中获取请求来源.
func GetSource(ctx context.Context) (request.Source, bool) {
	source, ok := ctx.Value(SourceContextKey).(request.Source)
	return source, ok
}

// GetSourceOrDefault 从 context 中获取请求来源，如果不存在则返回默认值.
func GetSourceOrDefault(ctx context.Context, defaultSource request.Source) request.Source {
	if source, ok := GetSource(ctx); ok {
		return source
	}

	return defaultSource
}
