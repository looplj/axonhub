package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/server/biz"
)

// WithAPIKeyAuth 中间件用于验证 API key.
func WithAPIKeyAuth(auth *biz.AuthService) gin.HandlerFunc {
	return WithAPIKeyConfig(auth, nil)
}

// WithAPIKeyConfig 中间件用于验证 API key，支持自定义配置.
func WithAPIKeyConfig(auth *biz.AuthService, config *APIKeyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := ExtractAPIKeyFromRequest(c.Request, config)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, err)
			return
		}

		apiKey, err := auth.AnthenticateAPIKey(c.Request.Context(), key)
		if err != nil {
			if ent.IsNotFound(err) || errors.Is(err, biz.ErrInvalidAPIKey) {
				AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			} else {
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate API key"))
			}

			return
		}

		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)

		if apiKey.Edges.Project != nil {
			ctx = contexts.WithProjectID(ctx, apiKey.Edges.Project.ID)
		}

		c.Request = c.Request.WithContext(ctx)

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
			AbortWithError(c, http.StatusUnauthorized, err)
			return
		}

		user, err := auth.AuthenticateJWTToken(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, biz.ErrInvalidJWT) {
				AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid token"))
			} else {
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate token"))
			}

			return
		}

		ctx := contexts.WithUser(c.Request.Context(), user)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// WithGeminiKeyAuth be compatible with Gemini query key authentication.
// https://ai.google.dev/api/generate-content?hl=zh-cn#text_gen_text_only_prompt-SHELL
func WithGeminiKeyAuth(auth *biz.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		if key == "" {
			var err error

			key, err = ExtractAPIKeyFromRequest(c.Request, nil)
			if err != nil {
				AbortWithError(c, http.StatusUnauthorized, err)
				return
			}
		}

		apiKey, err := auth.AnthenticateAPIKey(c.Request.Context(), key)
		if err != nil {
			if ent.IsNotFound(err) || errors.Is(err, biz.ErrInvalidAPIKey) {
				AbortWithError(c, http.StatusUnauthorized, biz.ErrInvalidAPIKey)
			} else {
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate API key"))
			}

			return
		}

		// 将 API key entity 保存到 context 中
		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)

		if apiKey.Edges.Project != nil {
			ctx = contexts.WithProjectID(ctx, apiKey.Edges.Project.ID)
		}

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// WithSource 中间件用于设置请求来源到 context 中.
func WithSource(source request.Source) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := contexts.WithSource(c.Request.Context(), source)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
