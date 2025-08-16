package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/ent"
)

func WithEntClient(client *ent.Client) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(ent.NewContext(c.Request.Context(), client))
		c.Next()
	}
}
