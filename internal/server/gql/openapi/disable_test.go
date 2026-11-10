package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/stretchr/testify/require"
)

func TestDisableAPIKeyBoundedAcrossReplicas(t *testing.T) {
	mr, fx, ctx, client := setupOpenAPI(t, []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)})
	ctx = authz.NewAPIKeyContext(ctx, fx.serviceAccount.ID, fx.project.ID)
	config := xcache.Config{Mode: xcache.ModeMemory}
	config.Memory.Expiration = time.Second
	replica := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: client, CacheConfig: config, ProjectService: mr.apiKeyService.ProjectService, KeyPrefix: "ah"})
	t.Cleanup(replica.Stop)
	auth := biz.NewAuthService(biz.AuthServiceParams{Ent: client, APIKeyService: replica})
	router := gin.New()
	router.GET("/llm", middleware.WithStrictAPIKeyConfig(auth, nil), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	httpServer := httptest.NewServer(router)
	t.Cleanup(httpServer.Close)
	call := func(key string) int {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/llm", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := httpServer.Client().Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode
	}
	require.Equal(t, http.StatusNoContent, call(fx.targetKey.Key))
	_, err := auth.AuthenticateAPIKey(context.Background(), fx.targetKey.Key)
	require.NoError(t, err)
	_, err = mr.apiKeyService.DisableAPIKey(ctx, fx.targetKey.ID)
	require.NoError(t, err)
	time.Sleep(1100 * time.Millisecond)
	_, err = auth.AuthenticateAPIKey(context.Background(), fx.targetKey.Key)
	require.ErrorIs(t, err, biz.ErrInvalidAPIKey)
	require.Equal(t, http.StatusUnauthorized, call(fx.targetKey.Key))
	restarted := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: client, CacheConfig: config, ProjectService: replica.ProjectService, KeyPrefix: "ah"})
	t.Cleanup(restarted.Stop)
	restartedAuth := biz.NewAuthService(biz.AuthServiceParams{Ent: client, APIKeyService: restarted})
	_, err = restartedAuth.AuthenticateAPIKey(context.Background(), fx.targetKey.Key)
	require.ErrorIs(t, err, biz.ErrInvalidAPIKey)
	_, err = auth.AuthenticateAPIKey(context.Background(), fx.serviceAccount.Key)
	require.NoError(t, err)
	require.NoError(t, client.Close())
	time.Sleep(1100 * time.Millisecond)
	_, err = auth.AuthenticateAPIKey(context.Background(), fx.serviceAccount.Key)
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, call(fx.serviceAccount.Key))
}

func TestDisableAPIKeyHTTPContract(t *testing.T) {
	mr, fx, ctx, client := setupOpenAPI(t, []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)})
	verifyDisableAPIKeyHTTP(t, mr, fx, ctx, client)
}

func verifyDisableAPIKeyHTTP(t *testing.T, mr *mutationResolver, fx fixtures, ctx context.Context, client *ent.Client) {
	t.Helper()
	auth := biz.NewAuthService(biz.AuthServiceParams{Ent: client, APIKeyService: mr.apiKeyService})
	router := gin.New()
	server := NewGraphqlHandlers(Dependencies{Ent: client, APIKeyService: mr.apiKeyService, APIKeyProfileTemplateService: mr.apiKeyProfileTemplateService, QuotaService: mr.quotaService}).Graphql
	router.POST("/graphql", middleware.WithOpenAPIAuth(auth), func(c *gin.Context) { server.ServeHTTP(c.Writer, c.Request) })
	for range 2 {
		query, err := json.Marshal(map[string]string{"query": fmt.Sprintf(`mutation { disableAPIKey(id: "gid://axonhub/APIKey/%d") { id projectID status } }`, fx.targetKey.ID)})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(query)))
		req.Header.Set("Authorization", "Bearer "+fx.serviceAccount.Key)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotContains(t, rec.Body.String(), `"errors"`)
		require.Contains(t, rec.Body.String(), `"status":"disabled"`)
		require.Contains(t, rec.Body.String(), fmt.Sprintf(`"projectID":%d`, fx.project.ID))
		require.NotContains(t, rec.Body.String(), fx.targetKey.Key)
	}
	bypass := authz.WithTestBypass(ctx)
	disabled, err := client.APIKey.Get(bypass, fx.targetKey.ID)
	require.NoError(t, err)
	require.Equal(t, fx.targetKey.Name, disabled.Name)
	require.Equal(t, fx.targetKey.Key, disabled.Key)
	require.Equal(t, fx.targetKey.Profiles, disabled.Profiles)
	_, err = mr.apiKeyService.UpdateAPIKeyStatus(bypass, disabled.ID, apikey.StatusEnabled)
	require.NoError(t, err)
	enabled, err := client.APIKey.Get(bypass, disabled.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, enabled.Status)
	_, err = mr.apiKeyService.UpdateAPIKeyStatus(bypass, disabled.ID, apikey.StatusArchived)
	require.NoError(t, err)
	body, err := json.Marshal(map[string]string{"query": fmt.Sprintf(`mutation { disableAPIKey(id: "gid://axonhub/APIKey/%d") { id status } }`, disabled.ID)})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+fx.serviceAccount.Key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":"VALIDATION_FAILED"`)
	archived, err := client.APIKey.Get(bypass, disabled.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusArchived, archived.Status)
}

func TestDisableAPIKeyAuthorization(t *testing.T) {
	for _, typ := range []apikey.Type{apikey.TypeServiceAccount, apikey.TypePersonal, apikey.TypeNoauth} {
		t.Run(string(typ), func(t *testing.T) {
			mr, fx, ctx, client := setupOpenAPI(t, []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)})
			ctx = authz.NewAPIKeyContext(ctx, fx.serviceAccount.ID, fx.project.ID)
			_, err := client.APIKey.UpdateOneID(fx.targetKey.ID).SetType(typ).Save(authz.WithTestBypass(ctx))
			require.NoError(t, err)
			for _, id := range []int{fx.targetKey.ID, fx.serviceAccount.ID, fx.otherKey.ID, 999999} {
				_, err = mr.apiKeyService.DisableAPIKey(ctx, id)
				require.Error(t, err)
			}
		})
	}
	for _, granted := range [][]string{nil, {string(scopes.ScopeReadAPIKeys)}, {string(scopes.ScopeWriteAPIKeys)}} {
		mr, fx, ctx, _ := setupOpenAPI(t, granted)
		ctx = authz.NewAPIKeyContext(ctx, fx.serviceAccount.ID, fx.project.ID)
		_, err := mr.apiKeyService.DisableAPIKey(ctx, fx.targetKey.ID)
		require.Error(t, err)
	}
}

func TestDisableAPIKeyRejectsInvalidCallerAndGUID(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)})
	ctx = authz.NewAPIKeyContext(ctx, fx.serviceAccount.ID, fx.project.ID)
	_, err := mr.DisableAPIKey(ctx, objects.GUID{Type: "Project", ID: fx.targetKey.ID})
	require.Error(t, err)
	_, err = mr.apiKeyService.DisableAPIKey(context.Background(), fx.targetKey.ID)
	require.Error(t, err)
	fx.serviceAccount.Type = apikey.TypeUser
	_, err = mr.apiKeyService.DisableAPIKey(ctx, fx.targetKey.ID)
	require.Error(t, err)
}
