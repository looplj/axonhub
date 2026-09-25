package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// WithAPIKeyAuth 中间件用于验证 API key.
func WithAPIKeyAuth(auth *biz.AuthService) gin.HandlerFunc {
	return WithAPIKeyConfig(auth, nil)
}

// WithAPIKeyConfig 中间件用于验证 API key，支持自定义配置.
func WithAPIKeyConfig(auth *biz.AuthService, config *APIKeyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := ExtractAPIKeyFromRequest(c.Request, config)
		// DO NOT ALLOW USE NO AUTH API KEY DIRECTLY.
		if key == biz.NoAuthAPIKeyValue {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			return
		}

		var apiKey *ent.APIKey
		if err == nil {
			apiKey, err = auth.AuthenticateAPIKey(c.Request.Context(), key)
		}
		if err != nil {
			apiKey, err = auth.AuthenticateNoAuth(c.Request.Context())
		}
		if err != nil {
			if ent.IsNotFound(err) || errors.Is(err, biz.ErrInvalidAPIKey) {
				AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			} else {
				log.Error(c.Request.Context(), "Failed to validate API key", log.Cause(err))
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate API key"))
			}

			return
		}

		if len(apiKey.AllowedIps) > 0 {
			clientIPs := clientIPCandidates(c)
			if !isAnyAllowedIP(clientIPs, apiKey.AllowedIps) {
				AbortWithError(c, http.StatusForbidden, errors.New("IP address is not allowed for this API key"))
				return
			}
		}

		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)

		if apiKey.Edges.Project != nil {
			ctx = contexts.WithProjectID(ctx, apiKey.Edges.Project.ID)
		}

		ctx = withSessionScopeForAPIKey(ctx, apiKey)

		ctx, err = withAPIKeyPrincipal(ctx, apiKey)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid authentication context"))
			return
		}

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func WithJWTAuth(auth *biz.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, hasAuthorization := c.Request.Header["Authorization"]
		token, err := ExtractAPIKeyFromRequest(c.Request, &APIKeyConfig{
			Headers:       []string{"Authorization"},
			RequireBearer: true,
		})
		fromCookie := false
		if err != nil {
			if hasAuthorization {
				AbortWithError(c, http.StatusUnauthorized, err)
				return
			}
			cookie, cookieErr := c.Request.Cookie(biz.AdminSessionCookieName)
			if cookieErr != nil {
				AbortWithError(c, http.StatusUnauthorized, err)
				return
			}
			token = cookie.Value
			fromCookie = true
		}
		if fromCookie && (!safeAdminRequest(c.Request) || c.Request.Method == http.MethodGet && c.FullPath() == "/admin/graphql") {
			AbortWithError(c, http.StatusForbidden, errors.New("Cross-origin admin request"))
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

		ctx = shared.WithSessionScope(ctx, "user:"+strconv.Itoa(user.ID))

		ctx, err = withUserPrincipal(ctx, user)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid authentication context"))
			return
		}

		c.Request = c.Request.WithContext(ctx)

		if fromCookie && c.GetHeader("X-Admin-Activity") == "1" {
			if refreshedToken, refreshed, refreshErr := auth.RefreshJWTToken(ctx, token, time.Now()); refreshErr != nil {
				log.Warn(ctx, "failed to renew admin session cookie", log.Cause(refreshErr))
			} else if refreshed {
				SetAdminSessionCookie(c, refreshedToken)
			}
		}

		c.Next()
	}
}

func SetAdminSessionCookie(c *gin.Context, token string) {
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return
	}
	maxAge := int(time.Until(exp.Time).Seconds())
	if maxAge < 1 {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		biz.AdminSessionCookieName,
		token,
		maxAge,
		"/",
		"",
		requestIsSecure(c),
		true,
	)
}

func safeAdminRequest(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return true
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) {
		return false
	}
	if r.TLS != nil {
		return u.Scheme == "https"
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return u.Scheme == "https"
	}
	return u.Scheme == "http"
}

func WithAdminCookieOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !safeAdminRequest(c.Request) {
			AbortWithError(c, http.StatusForbidden, errors.New("Cross-origin admin request"))
			return
		}
		c.Next()
	}
}

func ClearAdminSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(biz.AdminSessionCookieName, "", -1, "/", "", requestIsSecure(c), true)
}

func requestIsSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

var apiKeyAuthConfig = &APIKeyConfig{
	Headers:       []string{"Authorization"},
	RequireBearer: true,
}

// WithOpenAPIAuth gates the OpenAPI GraphQL surface (/openapi/v1/graphql).
// It accepts only service_account API keys and injects the API key principal,
// project, and session scope so the ent privacy layer can enforce per-project,
// scope-gated access for every query and mutation.
func WithOpenAPIAuth(auth *biz.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := ExtractAPIKeyFromRequest(c.Request, apiKeyAuthConfig)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, err)
			return
		}

		apiKey, err := auth.AuthenticateAPIKey(c.Request.Context(), key)
		if err != nil {
			if ent.IsNotFound(err) || errors.Is(err, biz.ErrInvalidAPIKey) {
				AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			} else {
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate API key"))
			}

			return
		}

		if apiKey.Type != apikey.TypeServiceAccount {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid API key"))
			return
		}

		if len(apiKey.AllowedIps) > 0 {
			clientIPs := clientIPCandidates(c)
			if !isAnyAllowedIP(clientIPs, apiKey.AllowedIps) {
				AbortWithError(c, http.StatusForbidden, errors.New("IP address is not allowed for this API key"))
				return
			}
		}

		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)
		if apiKey.Edges.Project != nil {
			ctx = contexts.WithProjectID(ctx, apiKey.Edges.Project.ID)
		}

		ctx = withSessionScopeForAPIKey(ctx, apiKey)

		ctx, err = withAPIKeyPrincipal(ctx, apiKey)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid authentication context"))
			return
		}

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

		apiKey, err := auth.AuthenticateAPIKey(c.Request.Context(), key)
		if err != nil {
			if ent.IsNotFound(err) || errors.Is(err, biz.ErrInvalidAPIKey) {
				AbortWithError(c, http.StatusUnauthorized, biz.ErrInvalidAPIKey)
			} else {
				AbortWithError(c, http.StatusInternalServerError, errors.New("Failed to validate API key"))
			}

			return
		}

		if len(apiKey.AllowedIps) > 0 {
			clientIPs := clientIPCandidates(c)
			if !isAnyAllowedIP(clientIPs, apiKey.AllowedIps) {
				AbortWithError(c, http.StatusForbidden, errors.New("IP address is not allowed for this API key"))
				return
			}
		}

		// 将 API key entity 保存到 context 中
		ctx := contexts.WithAPIKey(c.Request.Context(), apiKey)

		if apiKey.Edges.Project != nil {
			ctx = contexts.WithProjectID(ctx, apiKey.Edges.Project.ID)
		}

		ctx = withSessionScopeForAPIKey(ctx, apiKey)

		ctx, err = withAPIKeyPrincipal(ctx, apiKey)
		if err != nil {
			AbortWithError(c, http.StatusUnauthorized, errors.New("Invalid authentication context"))
			return
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

func withSessionScopeForAPIKey(ctx context.Context, key *ent.APIKey) context.Context {
	scope := "api_key:" + strconv.Itoa(key.ID)
	if key.Edges.Project != nil {
		scope += ":project:" + strconv.Itoa(key.Edges.Project.ID)
	}
	return shared.WithSessionScope(ctx, scope)
}

func withUserPrincipal(ctx context.Context, user *ent.User) (context.Context, error) {
	principal := authz.Principal{Type: authz.PrincipalTypeUser, UserID: &user.ID}
	return authz.WithPrincipal(ctx, principal)
}

func withAPIKeyPrincipal(ctx context.Context, key *ent.APIKey) (context.Context, error) {
	principal := authz.Principal{Type: authz.PrincipalTypeAPIKey, APIKeyID: &key.ID}
	if key.Edges.Project != nil {
		projectID := key.Edges.Project.ID
		principal.ProjectID = &projectID
	}

	return authz.WithPrincipal(ctx, principal)
}
