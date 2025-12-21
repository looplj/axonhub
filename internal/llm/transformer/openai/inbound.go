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

	// Parse into OpenAI-specific Request type
	var oaiReq Request

	err := json.Unmarshal(httpReq.Body, &oaiReq)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode openai request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if oaiReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if len(oaiReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	// Convert to unified llm.Request
	chatReq := oaiReq.ToLLMRequest()
	chatReq.RawRequest = httpReq
	chatReq.RequestType = llm.RequestTypeChat
	chatReq.APIFormat = llm.APIFormatOpenAIChatCompletion

	return chatReq, nil
}

// TransformResponse transforms ChatCompletionResponse to Response.
func (t *InboundTransformer) TransformResponse(
	ctx context.Context,
	chatResp *llm.Response,
) (*httpclient.Response, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("chat completion response is nil")
	}

	// Convert to OpenAI Response format
	oaiResp := ResponseFromLLM(chatResp)

	body, err := json.Marshal(oaiResp)
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
	return streams.NoNil(streams.MapErr(stream, func(chunk *llm.Response) (*httpclient.StreamEvent, error) {
		return t.TransformStreamChunk(ctx, chunk)
	})), nil
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

	// Skip events that only contain ReasoningSignature (used by Anthropic inbound)
	// OpenAI format doesn't support ReasoningSignature in streaming
	if isReasoningSignatureEvent(chatResp) {
		//nolint:nilnil // Skip this event
		return nil, nil
	}

	// Convert to OpenAI Response format
	oaiResp := ResponseFromLLM(chatResp)

	// For OpenAI, we keep the original response format as the event data
	eventData, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat completion response: %w", err)
	}

	return &httpclient.StreamEvent{
		Type: "",
		Data: eventData,
	}, nil
}

// isReasoningSignatureEvent checks if the response contains ReasoningSignature.
// This is a help func to adapt the Anthropic thinking signature to OpenAI format.
// In the real world, the signature will use a separate event, so if the response contains
// ReasoningSignature, it means the event is for signature_delta, we should ignore it.
func isReasoningSignatureEvent(resp *llm.Response) bool {
	if len(resp.Choices) != 1 {
		return false
	}

	delta := resp.Choices[0].Delta
	if delta == nil {
		return false
	}

	// Check if ReasoningSignature is set
	if delta.ReasoningSignature != nil && *delta.ReasoningSignature != "" {
		return true
	}

	return false
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

	if errors.Is(rawErr, transformer.ErrInvalidModel) {
		return &httpclient.Error{
			StatusCode: http.StatusUnprocessableEntity,
			Status:     http.StatusText(http.StatusUnprocessableEntity),
			Body: []byte(
				fmt.Sprintf(
					`{"error":{"message":"%s","type":"invalid_model_error"}}`,
					strings.TrimPrefix(rawErr.Error(), transformer.ErrInvalidModel.Error()+": "),
				),
			),
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
