// 职责: 测试路径前缀的验证、路由、重定向和 WebSocket 升级。
// 关联文件:
//   internal/server/base_path.go

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
)

func TestNormalizeBasePath(t *testing.T) {
	for input, want := range map[string]string{
		"": "", "/": "", "/axonhub": "/axonhub", "/tools/axon_hub/": "/tools/axon_hub",
	} {
		got, err := NormalizeBasePath(input)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	for _, input := range []string{"axonhub", "//axonhub", "/axonhub//", "https://example.com", "/a/../b", "/a?b", "/a#b", "/a%2fb", "/a b"} {
		_, err := NormalizeBasePath(input)
		require.ErrorContains(t, err, "server.base_path", input)
	}
}

func TestMountBasePath(t *testing.T) {
	router := gin.New()
	router.GET("/v1/models", func(c *gin.Context) {
		c.String(http.StatusOK, "%s %s", contexts.GetBasePath(c.Request.Context()), c.Query("q"))
	})
	router.GET("/settings/:tab", func(c *gin.Context) {
		c.String(http.StatusOK, "%s %s", c.Request.URL.EscapedPath(), c.Param("tab"))
	})
	router.GET("/redirect", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/elsewhere")
	})

	for _, tc := range []struct {
		basePath string
		target   string
		status   int
		location string
		body     string
	}{
		{target: "/v1/models?q=root", status: http.StatusOK, body: " root"},
		{basePath: "/my_app", target: "/my_app/v1/models?q=mounted", status: http.StatusOK, body: "/my_app mounted"},
		{basePath: "/my_app", target: "/my_app/settings/a%20b", status: http.StatusOK, body: "/settings/a%20b a b"},
		{basePath: "/my_app", target: "/my_app?q=1", status: http.StatusPermanentRedirect, location: "/my_app/?q=1"},
		{basePath: "/my_app", target: "/my_app/v1/models/?q=1", status: http.StatusMovedPermanently, location: "/my_app/v1/models?q=1"},
		{basePath: "/my_app", target: "/my_app/redirect", status: http.StatusTemporaryRedirect, location: "/elsewhere"},
		{basePath: "/my_app", target: "/my_app2/v1/models", status: http.StatusNotFound},
		{basePath: "/my_app", target: "/v1/models", status: http.StatusNotFound},
	} {
		t.Run(tc.target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			if tc.basePath != "" {
				req.Header.Set("X-Forwarded-Prefix", "/client-supplied")
			}
			rec := httptest.NewRecorder()
			mountBasePath(tc.basePath, router).ServeHTTP(rec, req)
			require.Equal(t, tc.status, rec.Code)
			require.Equal(t, tc.location, rec.Header().Get("Location"))
			if tc.body != "" {
				require.Equal(t, tc.body, rec.Body.String())
			}
		})
	}
}

func TestMountBasePathPreservesWebSocketUpgrade(t *testing.T) {
	router := gin.New()
	router.GET("/v1/responses", func(c *gin.Context) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err == nil {
			defer conn.Close()
			_ = conn.WriteMessage(websocket.TextMessage, []byte("connected"))
		}
	})
	server := httptest.NewServer(mountBasePath("/axonhub", router))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/axonhub/v1/responses", nil)
	require.NoError(t, err)
	defer conn.Close()
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "connected", string(message))
}
