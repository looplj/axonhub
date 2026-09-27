package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

const adminGraphqlTestSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type adminGraphqlAuthFixture struct {
	auth    *biz.AuthService
	client  *ent.Client
	jwt     string
	saKey   string
	userKey string
}

func setupAdminGraphqlAuth(t *testing.T) adminGraphqlAuthFixture {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	hashed, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	owner, err := client.User.Create().
		SetEmail(fmt.Sprintf("owner-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashed).
		SetFirstName("Owner").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	proj, err := client.Project.Create().
		SetName(fmt.Sprintf("project-%d", time.Now().UnixNano())).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	mustKey := func(name string, keyType apikey.Type) string {
		value, err := biz.GenerateAPIKey("ah")
		require.NoError(t, err)

		client.APIKey.Create().
			SetName(name).
			SetKey(value).
			SetUserID(owner.ID).
			SetProjectID(proj.ID).
			SetType(keyType).
			SetStatus(apikey.StatusEnabled).
			SaveX(ctx)

		return value
	}

	cacheCfg := xcache.Config{Mode: xcache.ModeMemory}
	projectSvc := &biz.ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](cacheCfg),
	}
	apiKeySvc := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig: cacheCfg, Ent: client, ProjectService: projectSvc, KeyPrefix: "ah",
	})
	t.Cleanup(apiKeySvc.Stop)
	userSvc := biz.NewUserService(biz.UserServiceParams{
		CacheConfig: cacheCfg, Ent: client, APIKeyService: apiKeySvc,
	})
	systemSvc := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheCfg, Ent: client})

	authSvc := biz.NewAuthService(biz.AuthServiceParams{
		SystemService: systemSvc, APIKeyService: apiKeySvc, UserService: userSvc, Ent: client,
	})

	require.NoError(t, systemSvc.SetSecretKey(ctx, adminGraphqlTestSecret))

	jwt, err := authSvc.GenerateJWTToken(ctx, owner)
	require.NoError(t, err)

	return adminGraphqlAuthFixture{
		auth:    authSvc,
		client:  client,
		jwt:     jwt,
		saKey:   mustKey("service-account", apikey.TypeServiceAccount),
		userKey: mustKey("user-key", apikey.TypeUser),
	}
}

func TestWithAdminGraphqlAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixture := setupAdminGraphqlAuth(t)

	var principalType authz.PrincipalType

	engine := gin.New()
	engine.Use(WithEntClient(fixture.client))
	engine.POST("/admin/graphql", WithAdminGraphqlAuth(fixture.auth), func(c *gin.Context) {
		principal, ok := authz.GetPrincipal(c.Request.Context())
		require.True(t, ok)
		principalType = principal.Type
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name       string
		bearer     string
		wantStatus int
		wantType   authz.PrincipalType
	}{
		{"user jwt", fixture.jwt, http.StatusOK, authz.PrincipalTypeUser},
		{"service account api key", fixture.saKey, http.StatusOK, authz.PrincipalTypeAPIKey},
		{"user api key rejected", fixture.userKey, http.StatusForbidden, authz.PrincipalTypeUnknown},
		{"unknown token rejected", "not-a-jwt-nor-an-api-key", http.StatusUnauthorized, authz.PrincipalTypeUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			principalType = authz.PrincipalTypeUnknown

			req := httptest.NewRequest(http.MethodPost, "/admin/graphql", nil)
			req.Header.Set("Authorization", "Bearer "+tc.bearer)

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantStatus, recorder.Code)
			require.Equal(t, tc.wantType, principalType)
		})
	}

	t.Run("missing authorization rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/graphql", nil)

		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}
