package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/tracing"
)

// WithLoggingTracing save the trace ID to the request context.
// So the logger can log the traace ID in the next logs.
func WithLoggingTracing(config tracing.Config) gin.HandlerFunc {
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

		ctx := tracing.WithTraceID(c.Request.Context(), traceID)
		if !strings.HasSuffix(c.FullPath(), "/graphql") {
			operationName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
			ctx = tracing.WithOperationName(ctx, operationName)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
