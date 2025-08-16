package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/log"
)

func WithTimeout(ts time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Info(c, "WithTimeout", log.String("timeout", ts.String()))

		ctx, cancel := context.WithTimeout(c.Request.Context(), ts)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
