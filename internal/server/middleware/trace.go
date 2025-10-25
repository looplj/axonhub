package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/tracing"
)

// WithTrace is a middleware that extracts the X-Trace-ID header and
// gets or creates the corresponding trace entity in the database.
func WithTrace(config tracing.Config, traceService *biz.TraceService) gin.HandlerFunc {
	// Use the configured trace header name, or default to "AH-Trace-Id"
	traceHeader := config.TraceHeader
	if traceHeader == "" {
		traceHeader = "AH-Trace-Id"
	}

	return func(c *gin.Context) {
		traceID := c.GetHeader(traceHeader)
		if traceID == "" {
			c.Next()
			return
		}

		// Get project ID from context
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		if !ok {
			// If no project ID in context, skip trace creation
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
			// Log error but continue - trace is optional
			c.Next()
			return
		}

		// Store trace in context
		ctx := contexts.WithTrace(c.Request.Context(), trace)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
