package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
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

func TestSafeAdminRequest(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		origin string
		site   string
		want   bool
	}{
		{"same-origin", http.MethodPost, "https://example.com", "same-origin", true},
		{"cross-origin", http.MethodPost, "https://attacker.example", "cross-site", false},
		{"sibling-subdomain", http.MethodPost, "https://evil.example.com", "same-site", false},
		{"no-origin", http.MethodPost, "", "", false},
		{"read", http.MethodGet, "", "cross-site", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "https://example.com/admin/graphql", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Sec-Fetch-Site", tt.site)
			if got := safeAdminRequest(req); got != tt.want {
				t.Fatalf("safeAdminRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdminCookieOriginRequiresSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/signin", WithAdminCookieOrigin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, tt := range []struct {
		name   string
		origin string
		want   int
	}{
		{"missing", "", http.StatusForbidden},
		{"cross-origin", "https://other.example", http.StatusForbidden},
		{"same-origin", "https://example.com", http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "https://example.com/auth/signin", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}

func TestAdminCookieSessionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := enttest.NewEntClient(t, "sqlite3", "file:admin-cookie?mode=memory&_fk=1")
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	_, err := client.System.Create().SetKey(biz.SystemKeySecretKey).SetValue("test-secret-key").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u, err := client.User.Create().SetEmail("cookie@example.com").SetPassword("test-password").SetStatus(user.StatusActivated).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	auth := &biz.AuthService{
		SystemService: &biz.SystemService{Cache: xcache.NewFromConfig[ent.System](cacheConfig)},
		UserService:   &biz.UserService{UserCache: xcache.NewFromConfig[ent.User](cacheConfig)},
	}
	now := time.Now().Truncate(time.Second)
	token, err := auth.GenerateJWTTokenAt(ctx, u, now.Add(-25*24*time.Hour), now.Add(-25*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ent.NewContext(c.Request.Context(), client))
	})
	router.GET("/admin/probe", WithJWTAuth(auth), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/admin/probe", WithJWTAuth(auth), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	get := httptest.NewRequest(http.MethodGet, "/admin/probe", nil)
	get.AddCookie(&http.Cookie{Name: biz.AdminSessionCookieName, Value: token})
	get.Header.Set("X-Admin-Activity", "1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, get)
	if recorder.Code != http.StatusNoContent || len(recorder.Result().Cookies()) != 1 {
		t.Fatalf("renewal response = %d, cookies = %d", recorder.Code, len(recorder.Result().Cookies()))
	}
	if cookie := recorder.Result().Cookies()[0]; !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("renewal cookie flags = %+v", cookie)
	}

	noActivity := httptest.NewRequest(http.MethodGet, "/admin/probe", nil)
	noActivity.AddCookie(&http.Cookie{Name: biz.AdminSessionCookieName, Value: token})
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, noActivity)
	if recorder.Code != http.StatusNoContent || len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("background request renewed cookie: status = %d, cookies = %d", recorder.Code, len(recorder.Result().Cookies()))
	}

	post := httptest.NewRequest(http.MethodPost, "https://example.com/admin/probe", nil)
	post.AddCookie(&http.Cookie{Name: biz.AdminSessionCookieName, Value: token})
	post.Header.Set("Origin", "https://attacker.example")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, post)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin response = %d", recorder.Code)
	}
}
