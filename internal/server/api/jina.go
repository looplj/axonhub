package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/jina"
)

type JinaHandlersParams struct {
	fx.In

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
	LiveStreamRegistry *biz.LiveStreamRegistry
	ChannelProbeService         *biz.ChannelProbeService
=======
	HistoricalWeight            float64 `name:"historical_weight"`
	RealtimeWeight              float64 `name:"realtime_weight"`
>>>>>>> 0e3d84a9 (feat: implement performance-aware load balancing strategy)
}

func NewJinaHandlers(params JinaHandlersParams) *JinaHandlers {
	return &JinaHandlers{
		RerankHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				jina.NewRerankInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
				params.ChannelProbeService,
				params.HistoricalWeight,
				params.RealtimeWeight,
			),
		},
		EmbeddingHandlers: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.DefaultSelector,
				params.RequestService,
				params.HttpClient,
				jina.NewEmbeddingInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
				params.PromptService,
				params.QuotaService,
				params.PromptProtectionRuleService,
				params.LiveStreamRegistry,
				params.ChannelProbeService,
				params.HistoricalWeight,
				params.RealtimeWeight,
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
