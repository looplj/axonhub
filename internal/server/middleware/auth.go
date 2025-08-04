package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/ent"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
)

// WithAPIKeyAuth 中间件用于验证 API key
func WithAPIKeyAuth(auth *biz.AuthService) gin.HandlerFunc {
	return WithAPIKeyConfig(auth, nil)
}

// WithAPIKeyConfig 中间件用于验证 API key，支持自定义配置
func WithAPIKeyConfig(auth *biz.AuthService, config *APIKeyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := ExtractAPIKeyFromRequest(c.Request, config)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 查询数据库验证 API key 是否存在
		apiKey, err := auth.ValidateAPIKey(c.Request.Context(), key)
		if err != nil {
			if ent.IsNotFound(err) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to validate API key",
				})
			}
			c.Abort()
			return
		}

		// 将 API key entity 保存到 context 中
		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)
		ctx = contexts.WithUser(ctx, apiKey.Edges.User)
		c.Request = c.Request.WithContext(ctx)

		// 继续处理请求
		c.Next()
	}
}

func WithJWTAuth(auth *biz.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := ExtractAPIKeyFromRequest(c.Request, &APIKeyConfig{
			Headers:       []string{"Authorization"},
			RequireBearer: true,
		})
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 验证 JWT token
		user, err := auth.ValidateJWTToken(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, biz.ErrInvalidJWT) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid JWT token",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to validate JWT token",
				})
			}
			c.Abort()
			return
		}
		ctx := contexts.WithUser(c.Request.Context(), user)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
