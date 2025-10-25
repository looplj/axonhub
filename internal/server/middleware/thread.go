package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
)

// WithThread is a middleware that extracts the X-Thread-ID header and
// gets or creates the corresponding thread entity in the database.
func WithThread(threadService *biz.ThreadService) gin.HandlerFunc {
	return func(c *gin.Context) {
		threadID := c.GetHeader("X-Thread-ID")
		if threadID == "" {
			c.Next()
			return
		}

		// Get project ID from context
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		if !ok {
			// If no project ID in context, skip thread creation
			c.Next()
			return
		}

		// Get or create thread (errors are logged but don't block the request)
		thread, err := threadService.GetOrCreateThread(c.Request.Context(), projectID, threadID)
		if err != nil {
			// Log error but continue - thread is optional
			c.Next()
			return
		}

		// Store thread in context
		ctx := contexts.WithThread(c.Request.Context(), thread)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
