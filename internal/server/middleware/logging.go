package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/tracing"
)

// WithLoggingTracing save the trace ID and request ID to the request context.
// So the logger can log the trace ID and request ID in the next logs.
func WithLoggingTracing(config tracing.Config) gin.HandlerFunc {
	// Use the configured trace header name, or default to "AH-Trace-Id"
	traceHeader := config.TraceHeader
	if traceHeader == "" {
		traceHeader = "AH-Trace-Id"
	}

	// Use the configured request header name, or default to "AH-Request-Id"
	requestHeader := config.RequestHeader
	if requestHeader == "" {
		requestHeader = "AH-Request-Id"
	}

	return func(c *gin.Context) {
		// Use the trace header from the request first.
		traceID := c.GetHeader(traceHeader)
		if traceID == "" {
			traceID = tracing.GenerateTraceID()
		}

		// Generate request ID for each request
		requestID := tracing.GenerateRequestID()

		// Set request ID header in response
		c.Header(requestHeader, requestID)

		ctx := tracing.WithTraceID(c.Request.Context(), traceID)
		ctx = tracing.WithRequestID(ctx, requestID)

		if !strings.HasSuffix(c.FullPath(), "/graphql") {
			operationName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
			ctx = tracing.WithOperationName(ctx, operationName)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
