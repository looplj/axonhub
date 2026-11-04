package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
	"github.com/stretchr/testify/require"
)

func TestDisableAPIKeyGraphQLErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scopes []string
		code   string
	}{
		{"archived", []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, "VALIDATION_FAILED"},
		{"foreign", []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, "NOT_FOUND"},
		{"missing", []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, "NOT_FOUND"},
		{"self", []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, "VALIDATION_FAILED"},
		{"wrong_guid", []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, "VALIDATION_FAILED"},
		{"missing_read", []string{string(scopes.ScopeWriteAPIKeys)}, "FORBIDDEN"},
		{"missing_write", []string{string(scopes.ScopeReadAPIKeys)}, "FORBIDDEN"},
		{"storage_failure", []string{string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys)}, "INTERNAL_SERVER_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mr, fx, ctx, client := setupOpenAPI(t, tc.scopes)
			id := fx.targetKey.ID
			kind := "APIKey"
			switch tc.name {
			case "archived":
				_, err := client.APIKey.UpdateOneID(id).SetStatus(apikey.StatusArchived).Save(authz.WithTestBypass(ctx))
				require.NoError(t, err)
			case "foreign":
				id = fx.otherKey.ID
			case "missing":
				id = 999999
			case "self":
				id = fx.serviceAccount.ID
			case "wrong_guid":
				kind = "Project"
			case "storage_failure":
				client.APIKey.Use(func(next ent.Mutator) ent.Mutator {
					return ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) {
						return nil, errors.New("test storage unavailable")
					})
				})
			}
			auth := biz.NewAuthService(biz.AuthServiceParams{Ent: client, APIKeyService: mr.apiKeyService})
			router := gin.New()
			server := NewGraphqlHandlers(Dependencies{Ent: client, APIKeyService: mr.apiKeyService, APIKeyProfileTemplateService: mr.apiKeyProfileTemplateService, QuotaService: mr.quotaService}).Graphql
			router.POST("/graphql", middleware.WithOpenAPIAuth(auth), func(c *gin.Context) { server.ServeHTTP(c.Writer, c.Request) })
			body, err := json.Marshal(map[string]string{"query": fmt.Sprintf(`mutation { disableAPIKey(id: "gid://axonhub/%s/%d") { id status } }`, kind, id)})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(body)))
			req.Header.Set("Authorization", "Bearer "+fx.serviceAccount.Key)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			var response struct {
				Errors []struct {
					Path       []any `json:"path"`
					Extensions struct {
						Code string `json:"code"`
					} `json:"extensions"`
				} `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Len(t, response.Errors, 1, rec.Body.String())
			require.Equal(t, tc.code, response.Errors[0].Extensions.Code)
			require.Equal(t, []any{"disableAPIKey"}, response.Errors[0].Path)
			key, err := client.APIKey.Get(authz.WithTestBypass(ctx), fx.targetKey.ID)
			require.NoError(t, err)
			want := apikey.StatusEnabled
			if tc.name == "archived" {
				want = apikey.StatusArchived
			}
			require.Equal(t, want, key.Status)
		})
	}
}
