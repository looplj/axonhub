package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/aisdk"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
)

type AiSdkHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	ModelService    *biz.ModelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type AiSDKHandlers struct {
	ChatCompletionHandler *ChatCompletionHandlers
}

func NewAiSDKHandlers(params AiSdkHandlersParams) *AiSDKHandlers {
	return &AiSDKHandlers{
		ChatCompletionHandler: &ChatCompletionHandlers{
			ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
				params.ChannelService,
				params.ModelService,
				params.RequestService,
				params.HttpClient,
				aisdk.NewDataStreamTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
			StreamWriter: WriteJSONStream,
		},
	}
}

func (handlers *AiSDKHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandler.ChatCompletion(c)
}

// WriteJSONStream writes stream events as plain JSON text stream.
func WriteJSONStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	// Set text stream headers
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Vercel-AI-Data-Stream", "v1")

	for {
		select {
		case <-ctx.Done():
			clientDisconnected = true

			log.Warn(ctx, "Client disconnected, stop streaming")

			return
		default:
			if stream.Next() {
				cur := stream.Current()
				_, _ = c.Writer.Write(cur.Data)
				log.Debug(ctx, "write stream event", log.Any("event", cur))
				c.Writer.Flush()
			} else {
				if err := stream.Err(); err != nil {
					log.Error(ctx, "Error in stream", log.Cause(err))
					_, _ = c.Writer.Write([]byte("3:" + `"` + err.Error() + `"` + "\n"))
				}

				return
			}
		}
	}
}
