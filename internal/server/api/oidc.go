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
	group := r.Group("/auth/oidc")
	group.GET("/providers", h.GetProviders)
	group.GET("/authorize/:provider", h.GetAuthorizeURL)
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

	authURL, state, err := h.oidc.GetAuthorizeURL(c.Request.Context(), provider)
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
		c.Error(fmt.Errorf("%s", "Provider is required"))
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	errorDesc := c.Query("error")
	
	if errorDesc != "" {
		c.Error(fmt.Errorf("%s", c.Query("error_description")))
		return
	}

	if code == "" || state == "" {
		c.Error(fmt.Errorf("%s", "Code and state are required"))
		return
	}

	exchangeCode, err := h.oidc.Callback(c.Request.Context(), provider, code, state)
	if err != nil {
		c.Error(err)
		return
	}
	
	c.Redirect(http.StatusFound, "/auth/oidc/callback?code="+exchangeCode)
}

func (h *OIDCHandlers) Exchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(fmt.Errorf("%s", err.Error()))
		return
	}

	user, err := h.oidc.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		c.Error(err)
		return
	}

	token, err := h.auth.GenerateJWTToken(c.Request.Context(), user)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}
