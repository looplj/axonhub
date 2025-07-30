package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/contexts"
	"github.com/looplj/axonhub/ent"
	"github.com/looplj/axonhub/ent/apikey"
	"github.com/looplj/axonhub/server/biz"
)

// WithAPIKeyAuth 中间件用于验证 API key
func WithAPIKeyAuth(client *ent.Client) gin.HandlerFunc {
	return WithAPIKeyConfig(client, nil)
}

// WithAPIKeyConfig 中间件用于验证 API key，支持自定义配置
func WithAPIKeyConfig(client *ent.Client, config *APIKeyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从多个可能的 headers 中获取 API key
		apiKeyValue, err := ExtractAPIKeyFromRequest(c.Request, config)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			c.Abort()
			return
		}

		// 查询数据库验证 API key 是否存在
		apiKeyEntity, err := client.APIKey.Query().
			Where(apikey.KeyEQ(apiKeyValue)).
			First(c.Request.Context())
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
		ctx := contexts.WithAPIKey(c.Request.Context(), apiKeyEntity)
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
