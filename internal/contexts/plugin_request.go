// 职责: 在请求内传递插件元数据及候选 thread 和 trace ID。
// 关联文件:
//   internal/server/plugin/server_middleware.go
//   internal/server/plugin/hostapi.go
//   internal/server/middleware/thread.go
//   internal/server/middleware/trace.go

package contexts

import (
	"context"
	"encoding/json"
)

const pluginRequestContextKey ContextKey = "plugin_request_context"

// PluginRequestContext is an immutable snapshot published by server middleware.
type PluginRequestContext struct {
	JSON     json.RawMessage
	ThreadID string
	TraceID  string
}

func WithPluginRequestContext(ctx context.Context, metadata PluginRequestContext) context.Context {
	return context.WithValue(ctx, pluginRequestContextKey, metadata)
}

func GetPluginRequestContext(ctx context.Context) PluginRequestContext {
	metadata, _ := ctx.Value(pluginRequestContextKey).(PluginRequestContext)
	return metadata
}
