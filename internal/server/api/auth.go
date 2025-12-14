package api

import (
	"errors"
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
	User  *objects.UserInfo `json:"user"`
	Token string            `json:"token"`
}

// SignIn handles user authentication.
func (h *AuthHandlers) SignIn(c *gin.Context) {
	var (
		ctx = c.Request.Context()
		req SignInRequest
	)

	err := c.ShouldBindJSON(&req)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("Invalid request format"))
		return
	}

	// Authenticate user
	user, err := h.AuthService.AuthenticateUser(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, biz.ErrInvalidPassword) {
			JSONError(c, http.StatusUnauthorized, errors.New("Invalid email or password"))
			return
		}

		JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))

		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateJWTToken(ctx, user)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, errors.New("Internal server error"))
		return
	}

	response := SignInResponse{
		User:  biz.ConvertUserToUserInfo(ctx, user),
		Token: token,
	}

	c.JSON(http.StatusOK, response)
}
