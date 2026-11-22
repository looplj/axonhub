// 职责: 将插件的 HTTP 中间件接到网关请求链，并使用网关错误格式返回拒绝。
// 关联文件:
//   internal/server/plugin/server_middleware.go
//   internal/server/middleware/error.go
//   internal/server/routes.go
//   internal/server/codex_routes.go

package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/server/plugin"
)

func WithServerPluginMiddleware(manager *plugin.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, rejection := manager.RunServerMiddleware(c.Request)
		c.Request = req
		if rejection != nil {
			AbortWithError(c, rejection.Status, rejection.Err)
			return
		}
		c.Next()
	}
}
