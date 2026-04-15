package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	doubao "github.com/looplj/axonhub/llm/transformer/doubao"
)

type DoubaoHandlersParams struct {
	fx.In

	VideoService    *biz.VideoService
	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	DefaultSelector *orchestrator.DefaultSelector
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	PromptService   *biz.PromptService
	PromptProtectionRuleService *biz.PromptProtectionRuleService
	QuotaService    *biz.QuotaService
	HttpClient      *httpclient.HttpClient
}

type DoubaoHandlers struct {
	VideoService       *biz.VideoService
	CreateOrchestrator *orchestrator.ChatCompletionOrchestrator
	InboundTransformer *doubao.VideoInboundTransformer
}

func NewDoubaoHandlers(params DoubaoHandlersParams) *DoubaoHandlers {
	inbound := doubao.NewVideoInboundTransformer()

	return &DoubaoHandlers{
		VideoService: params.VideoService,
		CreateOrchestrator: orchestrator.NewChatCompletionOrchestrator(
			params.ChannelService,
			params.DefaultSelector,
			params.RequestService,
			params.HttpClient,
			inbound,
			params.SystemService,
			params.UsageLogService,
			params.PromptService,
			params.QuotaService,
			params.PromptProtectionRuleService,
		),
		InboundTransformer: inbound,
	}
}

func (h *DoubaoHandlers) CreateTask(c *gin.Context) {
	ctx := c.Request.Context()

	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		httpErr := h.CreateOrchestrator.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))
		return
	}

	if len(genericReq.Body) == 0 {
		JSONError(c, http.StatusBadRequest, errors.New("Request body is empty"))
		return
	}

	result, err := h.CreateOrchestrator.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing doubao create", log.Cause(err))

		httpErr := h.CreateOrchestrator.Inbound.TransformError(ctx, err)
		c.JSON(httpErr.StatusCode, json.RawMessage(httpErr.Body))
		return
	}

	if result.ChatCompletion == nil {
		JSONError(c, http.StatusInternalServerError, biz.ErrInternal)
		return
	}

	resp := result.ChatCompletion
	contentType := "application/json"
	if ct := resp.Headers.Get("Content-Type"); ct != "" {
		contentType = ct
	}
	c.Data(resp.StatusCode, contentType, resp.Body)
}

func (h *DoubaoHandlers) GetTask(c *gin.Context) {
	ctx := c.Request.Context()

	externalID := c.Param("id")
	if externalID == "" {
		JSONError(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	resp, err := h.VideoService.GetTaskByExternalID(ctx, externalID)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	resp.Object = "video.task"
	resp.APIFormat = llm.APIFormatSeedanceVideo
	resp.Choices = []llm.Choice{}

	httpResp, err := h.InboundTransformer.TransformResponse(ctx, resp)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	contentType := "application/json"
	if ct := httpResp.Headers.Get("Content-Type"); ct != "" {
		contentType = ct
	}
	c.Data(httpResp.StatusCode, contentType, httpResp.Body)
}

func (h *DoubaoHandlers) DeleteTask(c *gin.Context) {
	ctx := c.Request.Context()

	externalID := c.Param("id")
	if externalID == "" {
		JSONError(c, http.StatusBadRequest, errors.New("invalid id"))
		return
	}

	if err := h.VideoService.DeleteTaskByExternalID(ctx, externalID); err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
}
