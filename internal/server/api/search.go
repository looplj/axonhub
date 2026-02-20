package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/search"
)

type SearchHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type SearchHandlers struct {
	SearchOrchestrator *orchestrator.ChatCompletionOrchestrator
}

func NewSearchHandlers(params SearchHandlersParams) *SearchHandlers {
	return &SearchHandlers{
		SearchOrchestrator: orchestrator.NewSearchOrchestrator(
			params.ChannelService,
			params.ModelService,
			params.RequestService,
			params.HttpClient,
			search.NewInboundTransformer(),
			params.SystemService,
			params.UsageLogService,
		),
	}
}

func (handlers *SearchHandlers) Search(c *gin.Context) {
	// Create temporary handler to delegate to orchestrator
	handler := &ChatCompletionHandlers{
		ChatCompletionOrchestrator: handlers.SearchOrchestrator,
	}
	handler.ChatCompletion(c)
}
