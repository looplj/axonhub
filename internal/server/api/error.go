package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// JSONError returns a JSON error response and adds the error to gin context for access logging.
func JSONError(c *gin.Context, status int, err error) {
	_ = c.Error(err)
	if c.Writer.Written() {
		// Headers were already flushed (e.g. an SSE stream was committed and a
		// later error surfaced). Writing another status would trigger gin's
		// "superfluous response.WriteHeader" warning and emit garbage, so only
		// record the late error in logs.
		log.Warn(c.Request.Context(), "Skipping JSON error response, headers already written",
			log.Int("status", status),
			log.Cause(err),
		)
		return
	}
	c.JSON(status, objects.ErrorResponse{
		Error: objects.Error{
			Type:    http.StatusText(status),
			Message: err.Error(),
		},
	})
}
