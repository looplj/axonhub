// 职责: 验证实时会话入口及 Qwen Base URL 前缀经过同一组路由中间件。
// 关联文件:
//   internal/server/routes.go
//   internal/server/api/realtime.go

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/realtime"
)

func TestRealtimeRoutesShareMiddleware(t *testing.T) {
	router := gin.New()
	group := router.Group("/", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	registerRealtimeRoutes(group, &api.RealtimeHandlers{}, realtime.Config{})

	for _, path := range []string{"/v1/realtime", "/v1/realtime/translations", "/qwen/v1/realtime"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, response.Code, path)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/realtime/qwen", nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Len(t, router.Routes(), 3)
}
