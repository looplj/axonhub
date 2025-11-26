package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("panic recovery with custom middleware", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery())

		router.GET("/panic", func(c *gin.Context) {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 500 {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("normal request without panic", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery())

		router.GET("/ok", func(c *gin.Context) {
			c.String(200, "OK")
		})

		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "OK") {
			t.Errorf("expected body to contain 'OK', got %s", w.Body.String())
		}
	})

	t.Run("panic with nil value", func(t *testing.T) {
		router := gin.New()
		router.Use(Recovery())

		router.GET("/panic-nil", func(c *gin.Context) {
			panic(nil)
		})

		req := httptest.NewRequest(http.MethodGet, "/panic-nil", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != 500 {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}
