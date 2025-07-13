package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/llm/client"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func NewOpenAIHandlers() *OpenAIHandlers {
	return &OpenAIHandlers{}
}

type OpenAIHandlers struct {
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	processor := NewChatCompletionProcessor(
		openai.NewInboundTransformer(),
		openai.NewOutboundTransformer(),
		client.NewHttpClient(30*time.Second),
	)
	if err := processor.Process(c); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
