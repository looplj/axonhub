package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/server/biz"
)

type OIDCHandlers struct {
	oidc *biz.OIDCService
	auth *biz.AuthService
}

func NewOIDCHandlers(oidc *biz.OIDCService, auth *biz.AuthService) *OIDCHandlers {
	return &OIDCHandlers{
		oidc: oidc,
		auth: auth,
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
		c.Error(fmt.Errorf("%s", "Provider is required"))
		return
	}

	// Get the base URL from the request
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	authURL, state, err := h.oidc.GetAuthorizeURL(c.Request.Context(), provider, baseURL)
	if err != nil {
		c.Error(err)
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
			c.Error(fmt.Errorf("%s", "Provider is required"))
			return
		}
	}

	code := c.Query("code")
	state := c.Query("state")
	errorDesc := c.Query("error")

	if errorDesc != "" {
		c.Error(fmt.Errorf("%s", c.Query("error_description")))
		return
	}

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code and state are required"})
		return
	}

	exchangeCode, err := h.oidc.Callback(c.Request.Context(), provider, code, state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusFound, "/oauth/oidc/idp-callback?code="+exchangeCode)
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
