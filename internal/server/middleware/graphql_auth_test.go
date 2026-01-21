package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
)

func setupTestGraphQLAuth(t *testing.T) (*gin.Engine, *ent.Client, *biz.AuthService, *ent.APIKey) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
		ProjectService: biz.NewProjectService(biz.ProjectServiceParams{
			CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
			Ent:         client,
		}),
	})

	authService := &biz.AuthService{
		APIKeyService: apiKeyService,
	}

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	hashedPassword, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	ownerUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	ownerProject, err := client.Project.Create().
		SetName("test-project").
		SetDescription("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	ownerAPIKey, err := client.APIKey.Create().
		SetName("Service Account").
		SetKey("ah-test-service-key").
		SetUserID(ownerUser.ID).
		SetProjectID(ownerProject.ID).
		SetType(apikey.TypeServiceAccount).
		SetStatus(apikey.StatusEnabled).
		SetScopes([]string{string(scopes.ScopeWriteAPIKeys)}).
		Save(ctx)
	require.NoError(t, err)

	router := gin.New()

	return router, client, authService, ownerAPIKey
}

func TestGraphQLAuth_AllowsCreateLLMAPIKeyWithAPIKey(t *testing.T) {
	router, client, authService, ownerAPIKey := setupTestGraphQLAuth(t)
	defer client.Close()

	router.POST("/admin/graphql", WithGraphQLAuthForLLMAPIKey(authService), func(c *gin.Context) {
		if _, ok := contexts.GetAPIKey(c.Request.Context()); !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "api key missing"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := []byte(`{"query":"mutation { createLLMAPIKey(name: \"llm-key\") { id } }"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(body))
	req.Header.Set("X-API-Key", ownerAPIKey.Key)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestGraphQLAuth_RejectsOtherMutationsWithAPIKey(t *testing.T) {
	router, client, authService, ownerAPIKey := setupTestGraphQLAuth(t)
	defer client.Close()

	router.POST("/admin/graphql", WithGraphQLAuthForLLMAPIKey(authService), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	body := []byte(`{"query":"mutation { createAPIKey { id } }"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(body))
	req.Header.Set("X-API-Key", ownerAPIKey.Key)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
