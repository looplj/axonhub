package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
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
	var ctx = c.Request.Context()
	var req SignInRequest

	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: "Invalid request format",
		})

		return
	}

	// Authenticate user
	user, err := h.AuthService.AuthenticateUser(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, biz.ErrInvalidPassword) {
			c.JSON(http.StatusUnauthorized, objects.ErrorResponse{
				Error: "Invalid email or password",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, objects.ErrorResponse{
			Error: "Failed to authenticate user",
		})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateJWTToken(ctx, user)
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
			Roles: lo.Map(user.Edges.Roles, func(role *ent.Role, _ int) objects.Role {
				return objects.Role{
					Code: role.Code,
					Name: role.Name,
				}
			}),
		},
		Token: token,
	}

	c.JSON(http.StatusOK, response)
}
