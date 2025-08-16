package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type AuthHandlersParams struct {
	fx.In

	AuthService *biz.AuthService
}

func NewAuthHandlers(params AuthHandlersParams) *AuthHandlers {
	return &AuthHandlers{
		AuthService: params.AuthService,
	}
}

type AuthHandlers struct {
	AuthService *biz.AuthService
}

// SignInRequest 登录请求.
type SignInRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SignInResponse 登录响应.
type SignInResponse struct {
	User  objects.UserInfo `json:"user"`
	Token string           `json:"token"`
}

// SignIn handles user authentication.
func (h *AuthHandlers) SignIn(c *gin.Context) {
	var req SignInRequest

	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: "Invalid request format",
		})

		return
	}

	// Authenticate user
	user, err := h.AuthService.AuthenticateUser(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, objects.ErrorResponse{
			Error: "Invalid email or password",
		})

		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateJWTToken(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, objects.ErrorResponse{
			Error: "Failed to generate token",
		})

		return
	}

	response := SignInResponse{
		User: objects.UserInfo{
			Email:          user.Email,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
			IsOwner:        user.IsOwner,
			PreferLanguage: user.PreferLanguage,
			Scopes:         user.Scopes,
			Roles:          []objects.Role{}, // TODO: Load user roles
		},
		Token: token,
	}

	c.JSON(http.StatusOK, response)
}
