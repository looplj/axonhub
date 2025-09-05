package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
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

// HealthResponse 健康检查响应.
type HealthResponse struct {
	Status    string     `json:"status"`
	Timestamp time.Time  `json:"timestamp"`
	Version   string     `json:"version"`
	Build     build.Info `json:"build"`
	Uptime    string     `json:"uptime"`
}

// InitializeSystemRequest 系统初始化请求.
type InitializeSystemRequest struct {
	OwnerEmail     string `json:"ownerEmail"     binding:"required,email"`
	OwnerPassword  string `json:"ownerPassword"  binding:"required,min=6"`
	OwnerFirstName string `json:"ownerFirstName" binding:"required"`
	OwnerLastName  string `json:"ownerLastName"  binding:"required"`
	BrandName      string `json:"brandName"      binding:"required"`
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

// Health returns the application health status and build information.
func (h *SystemHandlers) Health(c *gin.Context) {
	buildInfo := build.GetBuildInfo()

	c.JSON(http.StatusOK, HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   build.Version,
		Build:     buildInfo,
		Uptime:    buildInfo.Uptime,
	})
}

// InitializeSystem initializes the system with owner credentials.
func (h *SystemHandlers) InitializeSystem(c *gin.Context) {
	var req InitializeSystemRequest

	err := c.ShouldBindJSON(&req)
	if err != nil {
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
		OwnerEmail:     req.OwnerEmail,
		OwnerPassword:  req.OwnerPassword,
		OwnerFirstName: req.OwnerFirstName,
		OwnerLastName:  req.OwnerLastName,
		BrandName:      req.BrandName,
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

// GetFavicon returns the system brand logo as favicon.
func (h *SystemHandlers) GetFavicon(c *gin.Context) {
	ctx := c.Request.Context()

	brandLogo, err := h.SystemService.BrandLogo(ctx)
	if err != nil {
		log.Error(ctx, "Failed to get brand logo", log.Cause(err))

		c.JSON(http.StatusInternalServerError, objects.ErrorResponse{
			Error: "Failed to get brand logo",
		})

		return
	}

	// 如果没有设置品牌标志，返回 404
	if brandLogo == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// 解析 base64 编码的图片数据
	// 假设格式为 "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA..."
	if !strings.HasPrefix(brandLogo, "data:") {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: "Invalid brand logo format",
		})

		return
	}

	// 提取 MIME 类型和 base64 数据
	parts := strings.Split(brandLogo, ",")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: "Invalid brand logo format",
		})

		return
	}

	// 提取 MIME 类型
	headerPart := parts[0] // "data:image/png;base64"
	mimeStart := strings.Index(headerPart, ":")

	mimeEnd := strings.Index(headerPart, ";")
	if mimeStart == -1 || mimeEnd == -1 {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: "Invalid brand logo format",
		})

		return
	}

	mimeType := headerPart[mimeStart+1 : mimeEnd]

	// 解码 base64 数据
	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		c.JSON(http.StatusBadRequest, objects.ErrorResponse{
			Error: "Failed to decode brand logo",
		})

		return
	}

	// 设置响应头
	c.Header("Content-Type", mimeType)
	c.Header("Cache-Control", "public, max-age=3600") // 缓存 1 小时
	c.Header("Content-Length", fmt.Sprintf("%d", len(imageData)))

	// 返回图片数据
	c.Data(http.StatusOK, mimeType, imageData)
}
