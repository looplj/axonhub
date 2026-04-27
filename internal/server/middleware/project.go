package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/log"
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

		// Verify the authenticated user has access to the requested project.
		// Skip check for API key auth (project is bound to the key itself).
		if user, ok := contexts.GetUser(c.Request.Context()); ok && user != nil {
			client := ent.FromContext(c.Request.Context())
			if client != nil {
				exists, err := client.UserProject.Query().
					Where(userproject.UserID(user.ID), userproject.ProjectID(projectID.ID)).
					Exist(c.Request.Context())
				if err != nil {
					log.Error(c.Request.Context(), "failed to verify project access", log.Cause(err))
					AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to verify project access"))
					return
				}
				if !exists {
					AbortWithError(c, http.StatusForbidden, errors.New("Access denied: you do not have access to this project"))
					return
				}
			}
		}

		ctx := contexts.WithProjectID(c.Request.Context(), projectID.ID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
