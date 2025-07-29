package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/server/biz"
	"go.uber.org/fx"
)

type SystemHandlersParams struct {
	fx.In

	SystemService *biz.SystemService
	AuthService   *biz.AuthService
}

func NewSystemHandlers(params SystemHandlersParams) *SystemHandlers {
	return &SystemHandlers{
		SystemService: params.SystemService,
		AuthService:   params.AuthService,
	}
}

type SystemHandlers struct {
	SystemService *biz.SystemService
	AuthService   *biz.AuthService
}

// SystemStatusResponse 系统状态响应
type SystemStatusResponse struct {
	IsInitialized bool `json:"isInitialized"`
}

// InitializeSystemRequest 系统初始化请求
type InitializeSystemRequest struct {
	OwnerEmail    string `json:"ownerEmail" binding:"required,email"`
	OwnerPassword string `json:"ownerPassword" binding:"required,min=6"`
}

// InitializeSystemResponse 系统初始化响应
type InitializeSystemResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SignInRequest 登录请求
type SignInRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SignInResponse 登录响应
type SignInResponse struct {
	User  UserInfo `json:"user"`
	Token string   `json:"token"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	FirstName *string  `json:"firstName"`
	LastName  *string  `json:"lastName"`
	IsOwner   bool     `json:"isOwner"`
	Scopes    []string `json:"scopes"`
	Roles     []Role   `json:"roles"`
}

// Role 角色信息
type Role struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetSystemStatus returns the system initialization status
func (h *SystemHandlers) GetSystemStatus(c *gin.Context) {
	isInitialized, err := h.SystemService.IsInitialized(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check system status",
		})
		return
	}

	c.JSON(http.StatusOK, SystemStatusResponse{
		IsInitialized: isInitialized,
	})
}

// InitializeSystem initializes the system with owner credentials
func (h *SystemHandlers) InitializeSystem(c *gin.Context) {
	var req InitializeSystemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, InitializeSystemResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}

	// Check if system is already initialized
	isInitialized, err := h.SystemService.IsInitialized(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, InitializeSystemResponse{
			Success: false,
			Message: "Failed to check initialization status",
		})
		return
	}

	if isInitialized {
		c.JSON(http.StatusBadRequest, InitializeSystemResponse{
			Success: false,
			Message: "System is already initialized",
		})
		return
	}

	// Initialize system
	err = h.SystemService.Initialize(c.Request.Context(), &biz.InitializeSystemArgs{
		OwnerEmail:    req.OwnerEmail,
		OwnerPassword: req.OwnerPassword,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, InitializeSystemResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to initialize system: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, InitializeSystemResponse{
		Success: true,
		Message: "System initialized successfully",
	})
}

// SignIn handles user authentication
func (h *SystemHandlers) SignIn(c *gin.Context) {
	var req SignInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// Authenticate user
	user, err := h.AuthService.AuthenticateUser(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid email or password",
		})
		return
	}

	// Generate JWT token
	token, err := h.AuthService.GenerateJWTToken(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate token",
		})
		return
	}

	// Handle optional fields
	var firstName, lastName *string
	if user.FirstName != "" {
		firstName = &user.FirstName
	}
	if user.LastName != "" {
		lastName = &user.LastName
	}

	response := SignInResponse{
		User: UserInfo{
			ID:        fmt.Sprintf("%d", user.ID),
			Email:     user.Email,
			FirstName: firstName,
			LastName:  lastName,
			IsOwner:   user.IsOwner,
			Scopes:    user.Scopes,
			Roles:     []Role{}, // TODO: Load user roles
		},
		Token: token,
	}

	c.JSON(http.StatusOK, response)
}