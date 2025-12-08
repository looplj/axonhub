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
					return nil, fmt.Errorf("%w: system prompt must be text", transformer.ErrInvalidRequest)
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
	// InboundTransformer doesn't have platform type info, default to Direct (Anthropic official)
	return AggregateStreamChunks(ctx, chunks, PlatformDirect)
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

	if errors.Is(rawErr, transformer.ErrInvalidModel) {
		return &httpclient.Error{
			StatusCode: http.StatusUnprocessableEntity,
			Status:     http.StatusText(http.StatusUnprocessableEntity),
			Body: []byte(
				fmt.Sprintf(
					`{"message":"%s","type":"invalid_model_error"}`,
					strings.TrimPrefix(rawErr.Error(), transformer.ErrInvalidModel.Error()+": "),
				),
			),
		}
	}

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok {
		aErr := &AnthropicError{
			StatusCode: llmErr.StatusCode,
			RequestID:  llmErr.Detail.RequestID,
			Error:      ErrorDetail{Type: llmErr.Detail.Type, Message: llmErr.Detail.Message},
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
		aErr := &AnthropicError{
			StatusCode: http.StatusBadRequest,
			Error:      ErrorDetail{Type: "invalid_request_error", Message: strings.TrimPrefix(rawErr.Error(), transformer.ErrInvalidRequest.Error()+": ")},
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
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Body:       body,
		}
	}

	aErr := &AnthropicError{
		StatusCode: http.StatusInternalServerError,
		RequestID:  "",
		Error:      ErrorDetail{Type: "internal_server_error", Message: rawErr.Error()},
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
