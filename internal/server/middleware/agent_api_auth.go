package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/server/biz"
)

var agentAPIKeyAuthConfig = &APIKeyConfig{
	Headers:       []string{"Authorization"},
	RequireBearer: true,
}

// WithAgentAPIAuth authenticates runtime access for Agent API endpoints.
// Only API keys of type "agent" are allowed.
func WithAgentAPIAuth(auth *biz.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := ExtractAPIKeyFromRequest(c.Request, agentAPIKeyAuthConfig)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, err)
			return
		}

		apiKey, err := auth.AuthenticateAPIKey(c.Request.Context(), key)
		if err != nil {
			if ent.IsNotFound(err) || errors.Is(err, biz.ErrInvalidAPIKey) {
				AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			} else {
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate API key"))
			}
			return
		}

		if apiKey.Type != apikey.TypeAgent {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			return
		}

		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)
		if apiKey.Edges.Project != nil {
			ctx = contexts.WithProjectID(ctx, apiKey.Edges.Project.ID)
		}

		ctx, err = withAPIKeyPrincipal(ctx, apiKey)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid authentication context"))
			return
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
