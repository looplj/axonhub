package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"github.com/looplj/axonhub/internal/ent"
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
	c.JSON(http.StatusOK, gin.H{
		"data": h.oidc.GetProviders(c.Request.Context()),
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
	user, _ := c.Get("user")
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := user.(*ent.User).ID

	// Get the base URL (priority: config public URL > request host)
	baseURL := h.getBaseURL(c)

	authURL, state, err := h.oidc.GetLinkAuthorizeURL(c.Request.Context(), provider, baseURL, userID)
	if err != nil {
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
				provider = providers[0].Name
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

	exchangeCode, intent, err := h.oidc.Callback(c.Request.Context(), provider, code, state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	baseURL := h.publicURL
	if baseURL == "" {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}
	return strings.TrimSuffix(baseURL, "/")
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := h.auth.GenerateJWTToken(c.Request.Context(), user)
	if err != nil {
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
