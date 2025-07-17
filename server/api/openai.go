package api

import (
	"github.com/gin-gonic/gin"
)

type OpenAIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type OpenAIErrorHandler struct{}

func (e *OpenAIErrorHandler) HandlerError(c *gin.Context, err error) {
	c.JSON(500, &OpenAIError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

func (e *OpenAIErrorHandler) HandleStreamError(c *gin.Context, err error) {
	c.SSEvent("", &OpenAIError{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "internal_error",
			Message: err.Error(),
		},
	})
}

type OpenAIHandlers struct {
	ChatCompletionHandlers *ChatCompletionHandlers
}

func NewOpenAIHandlers(chatCompletionHandlers *ChatCompletionHandlers) *OpenAIHandlers {
	return &OpenAIHandlers{
		ChatCompletionHandlers: chatCompletionHandlers,
	}
}

func (handlers *OpenAIHandlers) ChatCompletion(c *gin.Context) {
	handlers.ChatCompletionHandlers.ChatCompletion(c)
}
