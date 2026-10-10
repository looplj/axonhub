package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
)

type CodexCompatibilityHandlers struct {
	System   *biz.SystemService
	Channels *biz.ChannelService
	Models   *biz.ModelService
}

func NewCodexCompatibilityHandlers(system *biz.SystemService, channels *biz.ChannelService, models *biz.ModelService) *CodexCompatibilityHandlers {
	return &CodexCompatibilityHandlers{System: system, Channels: channels, Models: models}
}

func (h *CodexCompatibilityHandlers) Enabled(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := authz.RunWithSystemBypass(c.Request.Context(), "codex-compatibility-switch", func(bypassCtx context.Context) (*biz.CodexCompatibilitySettings, error) {
		return h.System.CodexCompatibilitySettings(bypassCtx)
	})
	if err != nil {
		middleware.AbortWithError(c, 500, errors.New("failed to read Codex compatibility settings"))
		return
	}
	if !settings.Enabled {
		middleware.AbortWithError(c, 404, errors.New("Codex compatibility is disabled"))
		return
	}
	c.Next()
}

func (h *CodexCompatibilityHandlers) Whoami(c *gin.Context) {
	key, ok := contexts.GetAPIKey(c.Request.Context())
	if !ok || key == nil {
		middleware.AbortWithError(c, 401, errors.New("Invalid API key"))
		return
	}
	id := fmt.Sprintf("axonhub-key-%d", key.ID)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"email": key.Name, "chatgpt_user_id": id, "chatgpt_account_id": id, "chatgpt_plan_type": "plus", "chatgpt_account_is_fedramp": false})
}

func (h *CodexCompatibilityHandlers) ListModels(c *gin.Context) {
	ctx := c.Request.Context()
	c.Header("Cache-Control", "no-store")
	catalog, err := authz.RunWithSystemBypass(ctx, "codex-catalog-source", func(bypassCtx context.Context) (*biz.CodexCatalog, error) {
		settings, err := h.System.CodexCompatibilitySettings(bypassCtx)
		if err != nil {
			return nil, err
		}
		if settings.ChannelID == nil {
			return nil, &biz.CodexCatalogError{Status: 503, Message: "Codex catalog source is unavailable"}
		}
		return h.Channels.FetchCodexCatalog(bypassCtx, *settings.ChannelID, c.Query("client_version"))
	})
	if err != nil {
		var catalogErr *biz.CodexCatalogError
		if errors.As(err, &catalogErr) {
			middleware.AbortWithError(c, catalogErr.Status, catalogErr)
		} else {
			middleware.AbortWithError(c, 500, errors.New("failed to read Codex catalog settings"))
		}
		return
	}
	models, err := h.Models.ListEnabledModels(ctx)
	if err != nil {
		status := 500
		if errors.Is(err, privacy.Deny) {
			status = 403
		}
		middleware.AbortWithError(c, status, errors.New("failed to list visible models"))
		return
	}
	ids := make([]string, len(models))
	for i, model := range models {
		ids[i] = model.ID
	}
	body, err := catalog.Intersect(ids)
	if err != nil {
		middleware.AbortWithError(c, 500, errors.New("failed to encode Codex catalog"))
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}
