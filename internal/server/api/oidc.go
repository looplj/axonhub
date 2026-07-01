package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

type OIDCHandlers struct {
	oidc      *biz.OIDCService
	auth      *biz.AuthService
	publicURL string
}

type OIDCHandlerParams struct {
	fx.In

	OIDCService *biz.OIDCService
	AuthService *biz.AuthService
	PublicURL   string `name:"public_url"`
}

func NewOIDCHandlers(params OIDCHandlerParams) *OIDCHandlers {
	if params.OIDCService.CountProviders() > 0 && params.PublicURL == "" {
		log.Warn(contexts.WithUser(context.Background(), &ent.User{IsOwner: true}), "OIDC is enabled but server.public_url is not configured. This is insecure and can lead to Host header injection attacks in production.")
	}

	return &OIDCHandlers{
		oidc:      params.OIDCService,
		auth:      params.AuthService,
		publicURL: params.PublicURL,
	}
}

func (h *OIDCHandlers) RegisterRoutes(r gin.IRouter) {
	group := r.Group("/oidc")
	group.GET("/providers", h.GetProviders)
	group.GET("/authorize/:provider", h.GetAuthorizeURL)
	group.GET("/callback", h.Callback)
	group.GET("/callback/:provider", h.Callback)
	group.POST("/exchange", h.Exchange)
}

func (h *OIDCHandlers) GetProviders(c *gin.Context) {
	ctx := c.Request.Context()

	// Try to extract user from Authorization header if present for is_linked check
	authHeader := c.GetHeader("Authorization")
	if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		if u, err := h.auth.AuthenticateJWTToken(ctx, token); err == nil {
			ctx = contexts.WithUser(ctx, u)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": h.oidc.GetProviders(ctx),
	})
}

func (h *OIDCHandlers) GetAuthorizeURL(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider is required"})
		return
	}

	// Get the base URL (priority: config public URL > request host)
	baseURL := h.getBaseURL(c)

	authURL, state, err := h.oidc.GetAuthorizeURL(c.Request.Context(), provider, baseURL)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"url":   authURL,
			"state": state,
		},
	})
}

func (h *OIDCHandlers) GetLinkAuthorizeURL(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider is required"})
		return
	}

	// This assumes the route is protected by AuthMiddleware so contexts.GetUser should succeed.
	user, ok := contexts.GetUser(c.Request.Context())
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID := user.ID

	// Get the base URL (priority: config public URL > request host)
	baseURL := h.getBaseURL(c)

	authURL, state, err := h.oidc.GetLinkAuthorizeURL(c.Request.Context(), provider, baseURL, userID)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"url":   authURL,
			"state": state,
		},
	})
}

func (h *OIDCHandlers) Callback(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		if h.oidc.CountProviders() == 1 {
			// If only one provider, we don't need the parameter
			providers := h.oidc.GetProviders(c.Request.Context())
			if len(providers) > 0 {
				provider = providers[0].ID
			}
		}

		if provider == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Provider is required"})
			return
		}
	}

	code := c.Query("code")
	state := c.Query("state")
	errorDesc := c.Query("error")

	if errorDesc != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": c.Query("error_description")})
		return
	}

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code and state are required"})
		return
	}

	exchangeCode, intent, err := h.oidc.Callback(c.Request.Context(), provider, code, state, h.getBaseURL(c))
	if err != nil {
		baseURL := h.getBaseURL(c)
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/oauth/oidc/idp-callback?error=auth_failed&error_description=%s", baseURL, url.QueryEscape(err.Error())))

		return
	}

	baseURL := h.getBaseURL(c)

	if intent == "link" {
		c.Redirect(http.StatusFound, baseURL+"/settings/profile?oidc_link=success")
		return
	}

	c.Redirect(http.StatusFound, baseURL+"/oauth/oidc/idp-callback?code="+exchangeCode)
}

func (h *OIDCHandlers) getBaseURL(c *gin.Context) string {
	if h.publicURL != "" {
		return strings.TrimSuffix(h.publicURL, "/")
	}

	// Fallback to request host if publicURL is not configured.
	// NOTE: In production, it's highly recommended to configure public_url to prevent Host header attacks.
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func (h *OIDCHandlers) Exchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.oidc.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		// Map common exchange errors to 400 Bad Request
		if strings.Contains(err.Error(), "invalid or expired") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	token, err := h.auth.GenerateJWTToken(c.Request.Context(), user)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}
