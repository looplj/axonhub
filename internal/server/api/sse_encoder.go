package api

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type StreamErrorEncoder func(context.Context, error) (*httpclient.StreamEvent, error)

type streamAdapterFactory func(context.Context, streams.Stream[*httpclient.StreamEvent], *biz.SystemService) (streams.Stream[*httpclient.StreamEvent], StreamErrorEncoder)

func encodeStreamError(formatErr StreamErrorFormatter) StreamErrorEncoder {
	if formatErr == nil {
		formatErr = FormatStreamError
	}
	return func(ctx context.Context, err error) (*httpclient.StreamEvent, error) {
		data, marshalErr := json.Marshal(formatErr(ctx, err))
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &httpclient.StreamEvent{Type: "error", Data: data}, nil
	}
}

func (h *ChatCompletionHandlers) writeSSEStream(c *gin.Context, stream streams.Stream[*httpclient.StreamEvent]) {
	encodeErr := encodeStreamError(FormatStreamError)
	if h.streamAdapterFactory != nil {
		var systemService *biz.SystemService
		if h.ChatCompletionOrchestrator != nil {
			systemService = h.ChatCompletionOrchestrator.SystemService
		}
		stream, encodeErr = h.streamAdapterFactory(c.Request.Context(), stream, systemService)
	}
	writeEncodedSSEStream(c, stream, encodeErr, h.sseKeepAlive, h.sseHeartbeatFormat)
}
