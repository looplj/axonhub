package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
)

func TestWithProjectID_APIKeyIsPinnedToItsProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const keyProject = 3

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		// Reproduce what WithAdminGraphqlAuth injects for an API-key principal.
		ctx := contexts.WithAPIKey(c.Request.Context(), &ent.APIKey{ID: 1, ProjectID: keyProject})
		ctx = contexts.WithProjectID(ctx, keyProject)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.Use(WithProjectID())
	engine.POST("/admin/graphql", func(c *gin.Context) {
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, projectID)
	})

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"no header keeps the key project", "", http.StatusOK},
		{"matching header accepted", "gid://axonhub/Project/3", http.StatusOK},
		{"other project rejected", "gid://axonhub/Project/4", http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/graphql", nil)
			if tc.header != "" {
				req.Header.Set("X-Project-Id", tc.header)
			}

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantStatus, recorder.Code)

			if tc.wantStatus == http.StatusOK {
				require.Equal(t, "3", recorder.Body.String())
			}
		})
	}
}

func TestWithProjectID_UserSelectsProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(WithProjectID())
	engine.POST("/admin/graphql", func(c *gin.Context) {
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, projectID)
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/graphql", nil)
	req.Header.Set("X-Project-Id", "gid://axonhub/Project/9")

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "9", recorder.Body.String())
}
