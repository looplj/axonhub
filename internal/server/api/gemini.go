package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm/transformer/gemini"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type GeminiHandlersParams struct {
	fx.In

	ChannelService  *biz.ChannelService
	RequestService  *biz.RequestService
	SystemService   *biz.SystemService
	UsageLogService *biz.UsageLogService
	HttpClient      *httpclient.HttpClient
}

type GeminiHandlers struct {
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewGeminiHandlers(params GeminiHandlersParams) *GeminiHandlers {
	return &GeminiHandlers{
		ChatCompletionHandlers: NewChatCompletionHandlers(
			chat.NewChatCompletionProcessor(
				params.ChannelService,
				params.RequestService,
				params.HttpClient,
				gemini.NewInboundTransformer(),
				params.SystemService,
				params.UsageLogService,
			),
		),
	}
}

func (handlers *GeminiHandlers) GenerateContent(c *gin.Context) {
	alt := c.Query("alt")
	switch alt {
	case "sse":
		log.Info(c.Request.Context(), "Using SSE")
		handlers.ChatCompletionHandlers.ChatCompletion(c)
	default:
		log.Info(c.Request.Context(), "Using JSON")
		handlers.ChatCompletionHandlers.WithStreamWriter(WriteGeminiStream).ChatCompletion(c)
	}
}

func WriteGeminiStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	ctx := c.Request.Context()
	clientDisconnected := false

	defer func() {
		if clientDisconnected {
			log.Warn(ctx, "Client disconnected")
		}
	}()

	clientGone := c.Writer.CloseNotify()

	_, _ = c.Writer.Write([]byte("["))

	first := true

	for {
		select {
		case <-clientGone:
			clientDisconnected = true

			log.Warn(ctx, "Client disconnected, stop streaming")

			return
		default:
			if stream.Next() {
				cur := stream.Current()

				if !first {
					_, _ = c.Writer.Write([]byte(","))
				}

				_, _ = c.Writer.Write(cur.Data)
				first = false

				log.Debug(ctx, "write stream event", log.Any("event", cur))
				c.Writer.Flush()
			} else {
				if err := stream.Err(); err != nil {
					log.Error(ctx, "Error in stream", log.Cause(err))
					_, _ = c.Writer.Write([]byte("3:" + `"` + err.Error() + `"` + "\n"))
				}

				_, _ = c.Writer.Write([]byte("]"))

				return
			}
		}
	}
}
