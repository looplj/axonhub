package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/tracing"
)

// TracingConfig holds configuration for the tracing middleware.
type TracingConfig struct {
	// TraceHeader is the header name for trace ID.
	TraceHeader string
}

// WithTracing 中间件用于处理 trace ID.
// 如果请求头中包含配置的 trace header，则使用该 ID，否则生成一个新的 trace ID.
func WithTracing(config TracingConfig) gin.HandlerFunc {
	// Use the configured trace header name, or default to "AH-Trace-Id"
	traceHeader := config.TraceHeader
	if traceHeader == "" {
		traceHeader = "AH-Trace-Id"
	}

	return func(c *gin.Context) {
		var traceID tracing.TraceID

		// 检查请求头中是否包含 trace ID
		traceIDStr := c.GetHeader(traceHeader)
		if traceIDStr != "" {
			traceID = tracing.TraceID(traceIDStr)
		} else {
			// 生成新的 trace ID
			traceID = tracing.GenerateTraceID()
		}

		// 将 trace ID 添加到响应头中
		c.Header(traceHeader, traceID.String())

		// 将 trace ID 存储到 context 中
		ctx := tracing.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		// 继续处理请求
		c.Next()
	}
}