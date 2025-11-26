package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func WithProjectID() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectIDStr := c.GetHeader("X-Project-ID")
		if projectIDStr == "" {
			c.Next()
			return
		}

		projectID, err := objects.ParseGUID(projectIDStr)
		if err != nil || projectID.Type != ent.TypeProject {
			c.AbortWithStatusJSON(http.StatusBadRequest, objects.ErrorResponse{
				Error: objects.Error{
					Type:    http.StatusText(http.StatusBadRequest),
					Message: "Invalid project ID",
				},
			})

			return
		}

		ctx := contexts.WithProjectID(c.Request.Context(), projectID.ID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
