package anthropic

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
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// InboundTransformer implements transformer.Inbound for Anthropic format.
type InboundTransformer struct{}

// NewInboundTransformer creates a new Anthropic InboundTransformer.
func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

func (t *InboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAnthropicMessage
}

// TransformRequest transforms Anthropic HTTP request to ChatCompletionRequest.
//
//nolint:maintidx
func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
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

	var anthropicReq MessageRequest

	err := json.Unmarshal(httpReq.Body, &anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode anthropic request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if anthropicReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(anthropicReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	if anthropicReq.MaxTokens <= 0 {
		return nil, fmt.Errorf("%w: max_tokens is required and must be positive", transformer.ErrInvalidRequest)
	}

	// Validate system prompt format
	if anthropicReq.System != nil {
		if anthropicReq.System.Prompt == nil && len(anthropicReq.System.MultiplePrompts) > 0 {
			// Validate that all system prompts are text type
			for _, prompt := range anthropicReq.System.MultiplePrompts {
				if prompt.Type != "text" {
					return nil, fmt.Errorf(
						"%w: system prompt array must contain only text type elements",
						transformer.ErrInvalidRequest,
					)
				}
			}
		}
	}

	return convertToLLMRequest(&anthropicReq)
}

// TransformResponse transforms ChatCompletionResponse to Anthropic HTTP response.
func (t *InboundTransformer) TransformResponse(ctx context.Context, chatResp *llm.Response) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to Anthropic response format
	anthropicResp := convertToAnthropicResponse(chatResp)

	body, err := json.Marshal(anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"no-cache"},
		},
	}, nil
}

func (t *InboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}

// TransformError transforms LLM error response to HTTP error response in Anthropic format.
func (t *InboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	if rawErr == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       []byte(`{"message":"internal server error","request_id":""}`),
		}
	}

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok {
		aErr := &AnthropicErr{
			StatusCode: llmErr.StatusCode,
			Message:    llmErr.Detail.Message,
			RequestID:  llmErr.Detail.RequestID,
		}

		body, err := json.Marshal(aErr)
		if err != nil {
			return &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Body:       []byte(`{"message":"internal server error","type":"internal_server_error"}`),
			}
		}

		return &httpclient.Error{
			StatusCode: llmErr.StatusCode,
			Status:     http.StatusText(llmErr.StatusCode),
			Body:       body,
		}
	}

	if httpErr, ok := xerrors.As[*httpclient.Error](rawErr); ok {
		return httpErr
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		aErr := &AnthropicErr{
			StatusCode: http.StatusBadRequest,
			Message:    strings.TrimPrefix(rawErr.Error(), transformer.ErrInvalidRequest.Error()+": "),
			RequestID:  "",
			Type:       "invalid_request_error",
		}

		body, err := json.Marshal(aErr)
		if err != nil {
			return &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     http.StatusText(http.StatusInternalServerError),
				Body:       []byte(`{"message":"internal server error","type":"internal_server_error"}`),
			}
		}

		return &httpclient.Error{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body:       body,
		}
	}

	aErr := &AnthropicErr{
		StatusCode: http.StatusInternalServerError,
		Message:    rawErr.Error(),
		RequestID:  "",
	}

	body, err := json.Marshal(aErr)
	if err != nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Status:     http.StatusText(http.StatusInternalServerError),
			Body:       []byte(`{"message":"internal server error","type":"internal_server_error"}`),
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Status:     http.StatusText(http.StatusInternalServerError),
		Body:       body,
	}
}
