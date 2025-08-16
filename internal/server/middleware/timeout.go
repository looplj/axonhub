package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

func responseTimeout(c *gin.Context) {
	c.JSON(http.StatusBadGateway, gin.H{
		"error": "Request timeout",
	})
}

func WithTimeout(ts time.Duration) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(ts),
		timeout.WithResponse(responseTimeout),
	)
}
