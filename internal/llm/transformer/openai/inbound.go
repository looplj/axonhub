package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// InboundTransformer implements transformer.Inbound for OpenAI format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new OpenAI InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

// Name returns the name of the transformer.
func (t *InboundTransformer) Name() string {
	return "openai/chat_completions"
}

// TransformRequest transforms HTTP request to ChatCompletionRequest.
func (t *InboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("http request is nil")
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = httpReq.Headers.Get("Content-Type")
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	var chatReq llm.Request

	err := json.Unmarshal(httpReq.Body, &chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to decode openai request: %w", err)
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	return &chatReq, nil
}

// TransformResponse transforms ChatCompletionResponse to Response.
func (t *InboundTransformer) TransformResponse(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	// Create generic response
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func (t *InboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.MapErr(stream, func(chunk *llm.Response) (*httpclient.StreamEvent, error) {
		return t.TransformStreamChunk(ctx, chunk)
	}), nil
}

func (t *InboundTransformer) TransformStreamChunk(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.StreamEvent, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	if chatResp.Object == "[DONE]" {
		return &httpclient.StreamEvent{
			Data: []byte("[DONE]"),
		}, nil
	}

	// For OpenAI, we keep the original response format as the event data
	eventData, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	return &httpclient.StreamEvent{
		Type: "",
		Data: eventData,
	}, nil
}

func (t *InboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, error) {
	return AggregateStreamChunks(ctx, chunks)
}
