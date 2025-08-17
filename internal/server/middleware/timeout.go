package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

func WithTimeout(ts time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), ts)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
