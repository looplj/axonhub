package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/metrics"
)

// WithMetrics adds metrics collection to HTTP requests
func WithMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics after request is processed
		duration := time.Since(start).Seconds()

		metrics.Metrics.RecordHTTPRequest(
			c.Request.Context(),
			c.Request.Method,
			c.FullPath(),
			c.Writer.Status(),
			duration,
		)
	}
}
