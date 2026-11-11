// 职责: 确认插件 UI 资源不经过带凭证的全局 CORS。
// 关联文件:
//
//	internal/server/routes.go
//	internal/server/plugin/ui.go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPluginUISkipsCredentialedCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:5173"}
	config.AllowCredentials = true
	credentialedCORS := skipCredentialedCORS(cors.New(config))

	engine := gin.New()
	engine.Use(credentialedCORS)
	engine.OPTIONS("*any", credentialedCORS)
	engine.GET(adminGroupPath+pluginUIRoutePath, func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Plugin-UI", "reached")
		c.Status(http.StatusOK)
	})
	engine.GET("/admin/system/status", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	t.Run("opaque origin reaches the plugin handler", func(t *testing.T) {
		response := performCORS(engine, http.MethodGet, adminGroupPath+pluginUIPath+"token/ui/live/app.js", "null")
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "reached", response.Header().Get("X-Plugin-Ui"))
		require.Equal(t, "*", response.Header().Get("Access-Control-Allow-Origin"))
		require.Empty(t, response.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("opaque origin stays rejected on other routes", func(t *testing.T) {
		response := performCORS(engine, http.MethodGet, "/admin/system/status", "null")
		require.Equal(t, http.StatusForbidden, response.Code)
		require.Empty(t, response.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("allowlisted origin keeps credentialed cors", func(t *testing.T) {
		response := performCORS(engine, http.MethodGet, "/admin/system/status", "http://localhost:5173")
		require.Equal(t, http.StatusNoContent, response.Code)
		require.Equal(t, "http://localhost:5173", response.Header().Get("Access-Control-Allow-Origin"))
		require.Equal(t, "true", response.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("lookalike path stays on the global policy", func(t *testing.T) {
		prefix := adminGroupPath + pluginUIPath
		require.False(t, isPluginUIResource(strings.TrimSuffix(prefix, "/")+"-extra/token/app.js"))
		require.False(t, isPluginUIResource(strings.TrimSuffix(prefix, "/")))
		require.True(t, isPluginUIResource(prefix+"token/ui/live/app.js"))
	})
}

func performCORS(engine *gin.Engine, method, path, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Origin", origin)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}
