package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/llm/transformer/aisdk"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/chat"
)

type PlaygroundResponseError struct {
	Status int `json:"-"`
	Error  struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message"`
	} `json:"error"`
}

type PlaygroundHandlersParams struct {
	fx.In

	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

type PlaygroundHandlers struct {
	ChannelService *biz.ChannelService
	RequestService *biz.RequestService
	HttpClient     *httpclient.HttpClient
}

func NewPlaygroundHandlers(params PlaygroundHandlersParams) *PlaygroundHandlers {
	return &PlaygroundHandlers{
		ChannelService: params.ChannelService,
		RequestService: params.RequestService,
		HttpClient:     params.HttpClient,
	}
}

// tryExtractUpstreamErrorMessage attempts to extract a meaningful error message
// from a typical upstream error JSON payload. Supported shapes include:
// {"error": {"message": "..."}}, {"message": "..."}
// If nothing is extracted, returns an empty string.
func tryExtractUpstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 1) OpenAI / OpenRouter-like: {"error": {"message": "..."}}
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		if wrapped.Error.Message != "" {
			return wrapped.Error.Message
		}
	}

	// 2) Generic: {"message": "..."}
	var generic struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &generic); err == nil {
		if generic.Message != "" {
			return generic.Message
		}
	}

	// 3) Some providers may return: {"error": "..."}
	var alt struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &alt); err == nil {
		if alt.Error != "" {
			return alt.Error
		}
	}

	return ""
}

// HandleError handles raw errors and returns a PlaygroundResponseError.
func (handlers *PlaygroundHandlers) HandleError(rawErr error) *PlaygroundResponseError {
	if httpErr, ok := xerrors.As[*httpclient.Error](rawErr); ok {
		// Prefer upstream error message when available
		msg := tryExtractUpstreamErrorMessage(httpErr.Body)
		if msg == "" {
			msg = http.StatusText(httpErr.StatusCode)
		}

		return &PlaygroundResponseError{
			Status: httpErr.StatusCode,
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    httpErr.StatusCode,
				Message: msg,
			},
		}
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		return &PlaygroundResponseError{
			Status: http.StatusBadRequest,
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    http.StatusBadRequest,
				Message: http.StatusText(http.StatusBadRequest),
			},
		}
	}

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok {
		if llmErr.Detail.Message == "" {
			return &PlaygroundResponseError{
				Status: llmErr.StatusCode,
				Error: struct {
					Code    int    `json:"code,omitempty"`
					Message string `json:"message"`
				}{
					Code:    llmErr.StatusCode,
					Message: http.StatusText(llmErr.StatusCode),
				},
			}
		}

		// Try parse provider error code if present and numeric; otherwise use HTTP status.
		parsedCode, _ := strconv.Atoi(llmErr.Detail.Code)
		if parsedCode == 0 {
			parsedCode = llmErr.StatusCode
		}

		return &PlaygroundResponseError{
			Status: llmErr.StatusCode,
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    parsedCode,
				Message: llmErr.Detail.Message,
			},
		}
	}

	return &PlaygroundResponseError{
		Status: http.StatusInternalServerError,
		Error: struct {
			Code    int    `json:"code,omitempty"`
			Message string `json:"message"`
		}{
			Code:    http.StatusInternalServerError,
			Message: http.StatusText(http.StatusInternalServerError),
		},
	}
}

func (handlers *PlaygroundHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		log.Error(ctx, "Error reading HTTP request", log.Cause(err))
		c.JSON(http.StatusBadRequest, PlaygroundResponseError{
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			},
		})

		return
	}

	channelIDStr := c.Query("channel_id")
	if channelIDStr == "" {
		channelIDStr = c.GetHeader("X-Channel-ID")
	}

	log.Debug(ctx, "Received request", log.Any("request", genericReq), log.String("channel_id", channelIDStr))

	var processor *chat.ChatCompletionProcessor

	if channelIDStr != "" {
		channelID, err := objects.ParseGUID(channelIDStr)
		if err != nil {
			log.Error(ctx, "Error parsing channel ID", log.Cause(err))
			c.JSON(http.StatusBadRequest, PlaygroundResponseError{
				Error: struct {
					Code    int    `json:"code,omitempty"`
					Message string `json:"message"`
				}{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			})

			return
		}

		processor = chat.NewChatCompletionProcessorWithSelector(
			chat.NewSpecifiedChannelSelector(handlers.ChannelService, channelID),
			handlers.RequestService,
			handlers.HttpClient,
			aisdk.NewDataStreamTransformer(),
		)
	} else {
		// Use default processor with all available channels
		processor = chat.NewChatCompletionProcessor(
			handlers.ChannelService,
			handlers.RequestService,
			handlers.HttpClient,
			aisdk.NewDataStreamTransformer(),
		)
	}

	result, err := processor.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))
		errResponse := handlers.HandleError(err)
		c.JSON(errResponse.Status, errResponse)

		return
	}

	if result.ChatCompletion != nil {
		resp := result.ChatCompletion

		contentType := "application/json"
		if ct := resp.Headers.Get("Content-Type"); ct != "" {
			contentType = ct
		}

		c.Data(resp.StatusCode, contentType, resp.Body)

		return
	}

	if result.ChatCompletionStream != nil {
		defer func() {
			err := result.ChatCompletionStream.Close()
			if err != nil {
				log.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		// Set AI SDK data stream headers

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Vercel-AI-Data-Stream", "v1")
		c.Status(http.StatusOK)

		writeSSEStream(c, result.ChatCompletionStream)
	}
}
