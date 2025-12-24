package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/jina"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
)

type JinaHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

func NewJinaHandlers(params JinaHandlersParams) *JinaHandlers {
	return &JinaHandlers{
		RerankHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				jina.NewRerankInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
		EmbeddingHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				jina.NewEmbeddingInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		},
	}
}

type JinaHandlers struct {
	RerankHandlers    *ChatCompletionHandlers
	EmbeddingHandlers *ChatCompletionHandlers
}

// Rerank handles rerank requests.
func (h *JinaHandlers) Rerank(c *gin.Context) {
	h.RerankHandlers.ChatCompletion(c)
}

func (h *JinaHandlers) CreateEmbedding(c *gin.Context) {
	h.EmbeddingHandlers.ChatCompletion(c)
}
