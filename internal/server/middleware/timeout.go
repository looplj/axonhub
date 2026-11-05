package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/llm/httpclient"
)

func WithTimeout(ts time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), ts)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// WithLLMTimeout permits disabling the total deadline while preserving cancellation.
func WithLLMTimeout(timeout time.Duration) gin.HandlerFunc {
	if timeout != 0 {
		return WithTimeout(timeout)
	}
	return func(c *gin.Context) {
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		writer := &llmResponseWriter{ResponseWriter: c.Writer, timeout: llmWriteTimeout}
		c.Writer = writer
		writer.setDeadline(time.Time{})
		// net/http may flush buffered headers/body after Gin returns.
		defer func() { writer.setDeadline(time.Now().Add(writer.timeout)) }()
		c.Next()
	}
}

// WithStreamIdleTimeout configures upstream reads without counting downstream keep-alives.
func WithStreamIdleTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := httpclient.WithStreamIdleTimeout(c.Request.Context(), timeout)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
