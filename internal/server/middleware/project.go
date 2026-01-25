package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func WithProjectID() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectIDStr := c.GetHeader("X-Project-ID")
		if projectIDStr == "" {
			projectIDStr = c.Query("project_id")
		}

		if projectIDStr == "" {
			c.Next()
			return
		}

		var projectID int
		var parseErr error

		if parsedID, err := objects.ParseGUID(projectIDStr); err == nil && parsedID.Type == ent.TypeProject {
			projectID = parsedID.ID
		} else {
			projectID, parseErr = strconv.Atoi(projectIDStr)
			if parseErr != nil {
				AbortWithError(c, http.StatusBadRequest, errors.New("Invalid project ID"))
				return
			}
		}

		ctx := contexts.WithProjectID(c.Request.Context(), projectID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
