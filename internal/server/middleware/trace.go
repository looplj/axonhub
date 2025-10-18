package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/tracing"
)

// WithTracing 中间件用于处理 trace ID.
// 如果请求头中包含配置的 trace header，则使用该 ID，否则生成一个新的 trace ID.
func WithTracing(config tracing.Config) gin.HandlerFunc {
	// Use the configured trace header name, or default to "AH-Trace-Id"
	traceHeader := config.TraceHeader
	if traceHeader == "" {
		traceHeader = "AH-Trace-Id"
	}

	return func(c *gin.Context) {
		// Use the trace header from the request first.
		traceID := c.GetHeader(traceHeader)
		if traceID == "" {
			traceID = tracing.GenerateTraceID()
		}

		c.Header(traceHeader, traceID)
		ctx := tracing.WithTraceID(c.Request.Context(), traceID)

		if !strings.HasSuffix(c.FullPath(), "/graphql") {
			operationName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
			ctx = tracing.WithOperationName(ctx, operationName)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
