package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/tracing"
)

// WithThread is a middleware that extracts the X-Thread-ID header and
// gets or creates the corresponding thread entity in the database.
func WithThread(config tracing.Config, threadService *biz.ThreadService) gin.HandlerFunc {
	// Use the configured thread header name, or default to "AH-Thread-Id"
	threadHeader := config.ThreadHeader
	if threadHeader == "" {
		threadHeader = "AH-Thread-Id"
	}

	return func(c *gin.Context) {
		threadID := c.GetHeader(threadHeader)
		if threadID == "" {
			c.Next()
			return
		}

		// Get project ID from context
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		// Get or create thread (errors are logged but don't block the request)
		thread, err := threadService.GetOrCreateThread(c.Request.Context(), projectID, threadID)
		if err != nil {
			log.Warn(c.Request.Context(), "failed to get or create thread", log.String("thread_id", threadID), log.Int("project_id", projectID), log.Cause(err))
			c.Next()

			return
		}

		// Store thread in context
		ctx := contexts.WithThread(c.Request.Context(), thread)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
