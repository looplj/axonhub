package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func TestWithAPIKeyConfig_RejectsNoAuthKeyWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(WithAPIKeyConfig(&biz.AuthService{}, nil))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+biz.NoAuthAPIKeyValue)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestWithAPIKeyConfig_AllowsMissingAuthorizationWhenNoAuthAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		key, err := ExtractAPIKeyFromRequest(c.Request, &APIKeyConfig{
			Headers:       []string{"Authorization"},
			RequireBearer: true,
		})
		if errors.Is(err, ErrAPIKeyRequired) {
			c.Status(http.StatusNoContent)
			c.Abort()

			return
		}

		if err != nil || key != "" {
			c.Status(http.StatusTeapot)
			c.Abort()

			return
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestWithAPIKeyConfig_DoesNotFallbackToNoAuthForInvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := &biz.ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](cacheConfig),
	}
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig:    cacheConfig,
		Ent:            client,
		ProjectService: projectService,
		KeyPrefix:      "ah",
	})
	t.Cleanup(apiKeyService.Stop)
	auth := &biz.AuthService{
		APIKeyService: apiKeyService,
		AllowNoAuth:   true,
	}

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	owner, err := client.User.Create().
		SetEmail("owner-" + uuid.NewString() + "@example.com").
		SetPassword("unused").
		SetFirstName("Owner").
		SetLastName("User").
		SetIsOwner(true).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	proj, err := client.Project.Create().
		SetName(uuid.NewString()).
		SetDescription("test").
		SetStatus(project.StatusActive).
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.APIKey.Create().
		SetName(biz.NoAuthAPIKeyName).
		SetKey(biz.NoAuthAPIKeyValue).
		SetUserID(owner.ID).
		SetProjectID(proj.ID).
		SetType(apikey.TypeNoauth).
		SetStatus(apikey.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	router := gin.New()
	router.Use(WithEntClient(client), WithAPIKeyConfig(auth, nil))
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer definitely-invalid")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
