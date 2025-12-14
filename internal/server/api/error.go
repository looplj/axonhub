package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/objects"
)

// JSONError returns a JSON error response and adds the error to gin context for access logging.
func JSONError(c *gin.Context, status int, err error) {
	_ = c.Error(err)
	c.JSON(status, objects.ErrorResponse{
		Error: objects.Error{
			Type:    http.StatusText(status),
			Message: err.Error(),
		},
	})
}
