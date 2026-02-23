package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	doubao "github.com/looplj/axonhub/llm/transformer/doubao"
)

type SeedanceVideoHandlersParams struct {
	fx.In

	VideoService    *biz.VideoService
	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	PromptService   *biz.PromptService
	QuotaService    *biz.QuotaService
	HttpClient      *httpclient.HttpClient
}

type SeedanceVideoHandlers struct {
	VideoService       *biz.VideoService
	CreateOrchestrator *orchestrator.ChatCompletionOrchestrator
	InboundTransformer *doubao.VideoInboundTransformer
}

func NewSeedanceVideoHandlers(params SeedanceVideoHandlersParams) *SeedanceVideoHandlers {
	inbound := doubao.NewVideoInboundTransformer()

	return &SeedanceVideoHandlers{
		VideoService: params.VideoService,
		CreateOrchestrator: orchestrator.NewChatCompletionOrchestrator(
			params.ChannelService,
			params.ModelService,
			params.RequestService,
			params.HttpClient,
			inbound,
			params.SystemService,
			params.UsageLogService,
			params.PromptService,
			params.QuotaService,
		),
		InboundTransformer: inbound,
	}
}

func (h *SeedanceVideoHandlers) CreateTask(c *gin.Context) {
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
		log.Error(ctx, "Error processing seedance create", log.Cause(err))

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

func (h *SeedanceVideoHandlers) GetTask(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	requestID, err := parseRequestIDSeedance(idStr)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	video, err := h.VideoService.GetTask(ctx, requestID)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	llmResp := &llm.Response{
		ID:          video.ID,
		Object:      "video.task",
		Model:       video.Model,
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatSeedanceVideo,
		Video:       video,
		Choices:     []llm.Choice{},
	}

	httpResp, err := h.InboundTransformer.TransformResponse(ctx, llmResp)
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

func (h *SeedanceVideoHandlers) DeleteTask(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	requestID, err := parseRequestIDSeedance(idStr)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	if err := h.VideoService.DeleteTask(ctx, requestID); err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func parseRequestIDSeedance(s string) (int, error) {
	if gid, err := objects.ParseGUID(s); err == nil && gid.ID > 0 {
		return gid.ID, nil
	}

	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}

	return id, nil
}
