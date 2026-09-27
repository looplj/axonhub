package middleware

import (
	"errors"
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

		projectID, parseErr := objects.ParseGUID(projectIDStr)
		if parseErr != nil || projectID.Type != ent.TypeProject {
			AbortWithError(c, http.StatusBadRequest, errors.New("Invalid project ID"))
			return
		}

		// An API key is pinned to its own project. The header selects a project
		// for user JWTs, but must not let a key read another project's data.
		if apiKey, ok := contexts.GetAPIKey(c.Request.Context()); ok && apiKey != nil && apiKey.ProjectID != projectID.ID {
			AbortWithError(c, http.StatusForbidden, errors.New("Project ID is not allowed for this API key"))
			return
		}

		ctx := contexts.WithProjectID(c.Request.Context(), projectID.ID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
