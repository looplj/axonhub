package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/tracing"
)

func getTraceIDFromHeader(c *gin.Context, config tracing.Config) string {
	// Use the configured trace header name, or default to "AH-Trace-Id"
	primaryTraceHeader := config.TraceHeader
	if primaryTraceHeader == "" {
		primaryTraceHeader = "AH-Trace-Id"
	}

	traceID := c.GetHeader(primaryTraceHeader)
	if traceID != "" {
		return traceID
	}

	for _, header := range config.ExtraTraceHeaders {
		traceID = c.GetHeader(header)
		if traceID != "" {
			return traceID
		}
	}

	return ""
}

// WithTrace is a middleware that extracts the X-Trace-ID header and
// gets or creates the corresponding trace entity in the database.
func WithTrace(config tracing.Config, traceService *biz.TraceService) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := getTraceIDFromHeader(c, config)
		if traceID == "" {
			c.Next()
			return
		}

		// Get project ID from context
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		// Get thread ID from context if available
		var threadID *int
		if thread, ok := contexts.GetThread(c.Request.Context()); ok && thread != nil {
			threadID = &thread.ID
		}

		// Get or create trace (errors are logged but don't block the request)
		trace, err := traceService.GetOrCreateTrace(c.Request.Context(), projectID, traceID, threadID)
		if err != nil {
			log.Warn(c.Request.Context(), "Failed to get or create trace", log.Cause(err))
			c.Next()

			return
		}

		// Store trace in context
		if log.DebugEnabled(c.Request.Context()) {
			log.Debug(c.Request.Context(), "Trace created", log.Any("trace", trace))
		}

		ctx := contexts.WithTrace(c.Request.Context(), trace)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
