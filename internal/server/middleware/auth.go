package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

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

// WithGraphQLAuthForLLMAPIKey allows API key auth for createLLMAPIKey only.
func WithGraphQLAuthForLLMAPIKey(auth *biz.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer JWT auth for all operations.
		if tryJWTAuth(c, auth) {
			c.Next()
			return
		}

		allowAPIKey, err := isCreateLLMAPIKeyMutation(c)
		if err != nil {
			AbortWithError(c, http.StatusBadRequest, err)
			return
		}
		if !allowAPIKey {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid token"))
			return
		}

		key, err := ExtractAPIKeyFromRequest(c.Request, nil)
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

func tryJWTAuth(c *gin.Context, auth *biz.AuthService) bool {
	token, err := ExtractAPIKeyFromRequest(c.Request, &APIKeyConfig{
		Headers:       []string{"Authorization"},
		RequireBearer: true,
	})
	if err != nil {
		return false
	}

	user, err := auth.AuthenticateJWTToken(c.Request.Context(), token)
	if err != nil {
		return false
	}

	ctx := contexts.WithUser(c.Request.Context(), user)
	c.Request = c.Request.WithContext(ctx)
	return true
}

type graphQLPayload struct {
	Query         string `json:"query"`
	OperationName string `json:"operationName"`
}

func isCreateLLMAPIKeyMutation(c *gin.Context) (bool, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false, err
	}
	if len(body) == 0 {
		return false, errors.New("empty GraphQL request body")
	}
	defer func() {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	}()

	var payload graphQLPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, err
	}
	if payload.Query == "" {
		return false, errors.New("missing GraphQL query")
	}

	doc, err := parser.ParseQuery(&ast.Source{Input: payload.Query})
	if err != nil {
		return false, err
	}

	for _, op := range doc.Operations {
		if payload.OperationName != "" && op.Name != payload.OperationName {
			continue
		}
		if op.Operation != ast.Mutation {
			continue
		}
		for _, selection := range op.SelectionSet {
			field, ok := selection.(*ast.Field)
			if !ok {
				continue
			}
			if field.Name == "createLLMAPIKey" {
				return true, nil
			}
		}
	}

	return false, nil
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
