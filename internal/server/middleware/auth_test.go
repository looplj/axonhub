package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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

func TestWithAPIKeyConfig_RejectsMalformedAuthorizationWhenNoAuthAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(WithAPIKeyConfig(&biz.AuthService{AllowNoAuth: true}, &APIKeyConfig{
		Headers:       []string{"Authorization"},
		RequireBearer: true,
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "invalid-key")

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

func TestWithAPIKeyConfig_RejectsEmptyBearerWhenNoAuthAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(WithAPIKeyConfig(&biz.AuthService{AllowNoAuth: true}, &APIKeyConfig{
		Headers:       []string{"Authorization"},
		RequireBearer: true,
	}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}
