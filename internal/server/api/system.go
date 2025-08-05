package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"go.uber.org/fx"
)

type SystemHandlersParams struct {
	fx.In

	SystemService *biz.SystemService
}

func NewSystemHandlers(params SystemHandlersParams) *SystemHandlers {
	return &SystemHandlers{
		SystemService: params.SystemService,
	}
}

type SystemHandlers struct {
	SystemService *biz.SystemService
}

// SystemStatusResponse 系统状态响应.
type SystemStatusResponse struct {
	IsInitialized bool `json:"isInitialized"`
}

// InitializeSystemRequest 系统初始化请求.
type InitializeSystemRequest struct {
	OwnerEmail    string `json:"ownerEmail"    binding:"required,email"`
	OwnerPassword string `json:"ownerPassword" binding:"required,min=6"`
}

// InitializeSystemResponse 系统初始化响应.
type InitializeSystemResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetSystemStatus returns the system initialization status.
func (h *SystemHandlers) GetSystemStatus(c *gin.Context) {
	isInitialized, err := h.SystemService.IsInitialized(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, objects.ErrorResponse{
			Error: "Failed to check system status",
		})

		return
	}

	c.JSON(http.StatusOK, SystemStatusResponse{
		IsInitialized: isInitialized,
	})
}

// InitializeSystem initializes the system with owner credentials.
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
