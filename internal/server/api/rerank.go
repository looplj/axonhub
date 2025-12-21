package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/jina"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
)

type RerankHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

func NewRerankHandlers(params RerankHandlersParams) *RerankHandlers {
	return &RerankHandlers{
		ChatCompletionHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				jina.NewRerankInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
	}
}

type RerankHandlers struct {
	ChatCompletionHandlers *ChatCompletionHandlers
}

// Rerank handles rerank requests.
func (h *RerankHandlers) Rerank(c *gin.Context) {
	h.ChatCompletionHandlers.ChatCompletion(c)
}
