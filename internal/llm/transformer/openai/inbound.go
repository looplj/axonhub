package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	transformer "github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// InboundTransformer implements transformer.Inbound for OpenAI format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new OpenAI InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

func (t *InboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIChatCompletion
}

// TransformRequest transforms HTTP request to ChatCompletionRequest.
func (t *InboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	// Check content type
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = httpReq.Headers.Get("Content-Type")
	}

	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var chatReq llm.Request

	err := json.Unmarshal(httpReq.Body, &chatReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode openai request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if chatReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
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
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks, DefaultTransformChunk)
}

// TransformError transforms LLM error response to HTTP error response.
func (t *InboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       []byte(`{"error":{"message":"An unexpected error occurred","type":"unexpected_error"}}`),
		}
	}

	if httpErr, ok := xerrors.As[*httpclient.Error](rawErr); ok {
		return httpErr
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body: []byte(
				fmt.Sprintf(
					`{"error":{"message":"%s","type":"invalid_request_error"}}`,
					strings.TrimPrefix(rawErr.Error(), transformer.ErrInvalidRequest.Error()+": "),
				),
			),
		}
	}

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok {
		body, err := json.Marshal(llmErr)
		if err != nil {
			return &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Body:       []byte(`{"error":{"message":"internal server error","type":"internal_server_error"}}`),
			}
		}

		return &httpclient.Error{
			StatusCode: llmErr.StatusCode,
			Status:     http.StatusText(llmErr.StatusCode),
			Body:       body,
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       []byte(fmt.Sprintf(`{"error":{"message":"%s","type":"internal_server_error"}}`, rawErr.Error())),
	}
}
