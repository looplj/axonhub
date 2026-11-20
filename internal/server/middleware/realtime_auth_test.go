// 职责: 验证实时入口使用的 Header 严格鉴权不会回退到 No-Auth。
// 关联文件:
//
//	internal/server/middleware/auth.go
//	internal/server/routes.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRealtimeStrictAuthorizationBeforeUpgrade(t *testing.T) {
	fixture := setupAdminGraphqlAuth(t)
	fixture.auth.AllowNoAuth = true
	router := gin.New()
	router.Use(WithEntClient(fixture.client))
	upgraded := false
	router.GET("/v1/realtime", WithStrictAPIKeyConfig(fixture.auth, &APIKeyConfig{Headers: []string{"Authorization"}, RequireBearer: true}), func(c *gin.Context) { upgraded = true; c.Status(http.StatusNoContent) })
	for _, tc := range []struct {
		authorization string
		status        int
	}{{"", 401}, {"Bearer invalid", 401}, {fixture.userKey, 401}, {"Bearer " + fixture.userKey, 204}} {
		upgraded = false
		req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=voice", nil)
		req.Header.Set("Authorization", tc.authorization)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		require.Equal(t, tc.status, res.Code)
		require.Equal(t, tc.status == 204, upgraded)
	}
}
