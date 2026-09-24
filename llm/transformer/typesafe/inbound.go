package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type SystemOneInboundTransformer struct{}

func NewSystemOneInboundTransformer() *SystemOneInboundTransformer {
	return &SystemOneInboundTransformer{}
}

func (t *SystemOneInboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	decoder := json.NewDecoder(bytes.NewReader(httpReq.Body))
	decoder.UseNumber()
	var wireReq systemOneWireRequest
	if err := decoder.Decode(&wireReq); err != nil {
		return nil, fmt.Errorf("%w: failed to decode systemone request: %w", transformer.ErrInvalidRequest, err)
	}

	if wireReq.Stream != nil && *wireReq.Stream {
		return nil, fmt.Errorf("%w: streaming is not supported for systemone requests", transformer.ErrInvalidRequest)
	}

	if wireReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}
	if wireReq.State == nil {
		return nil, fmt.Errorf("%w: state is required", transformer.ErrInvalidRequest)
	}
	if len(wireReq.Questions) == 0 {
		return nil, fmt.Errorf("%w: questions are required", transformer.ErrInvalidRequest)
	}

	llmReq := &llm.Request{
		Model:       wireReq.Model,
		RequestType: llm.RequestTypeSystemOne,
		APIFormat:   llm.APIFormatTypeSafeSystemOne,
		SystemOne: &llm.SystemOneRequest{
			State:     wireReq.State,
			Questions: wireReq.Questions,
		},
		RawRequest: httpReq,
	}

	return llmReq, nil
}

func (t *SystemOneInboundTransformer) TransformResponse(
	ctx context.Context,
	llmResp *llm.Response,
) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("llm response is nil")
	}

	if llmResp.SystemOne == nil {
		return nil, fmt.Errorf("systemone response is nil")
	}

	wireResp := systemOneWireResponse{
		Model:   llmResp.Model,
		Answers: llmResp.SystemOne.Answers,
	}

	if llmResp.Usage != nil {
		wireResp.Usage = &systemOneWireUsage{
			InputTokens:  llmResp.Usage.PromptTokens,
			OutputTokens: llmResp.Usage.CompletionTokens,
		}
	}

	body, err := json.Marshal(wireResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal systemone response: %w", err)
	}

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}, nil
}

func (t *SystemOneInboundTransformer) TransformError(
	ctx context.Context,
	err error,
) *httpclient.Error {
	if err == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte(`{"error":{"message":"Internal server error","type":"api_error"}}`),
		}
	}

	var respErr *llm.ResponseError
	if errors.As(err, &respErr) {
		errorBody := map[string]any{
			"error": respErr.Detail,
		}
		body, marshalErr := json.Marshal(errorBody)
		if marshalErr != nil {
			return &httpclient.Error{
				StatusCode: respErr.StatusCode,
				Body:       []byte(`{"error":{"message":"Failed to marshal error","type":"api_error"}}`),
			}
		}
		return &httpclient.Error{
			StatusCode: respErr.StatusCode,
			Body:       body,
		}
	}

	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Body:       []byte(fmt.Sprintf(`{"error":{"message":%q,"type":"api_error"}}`, err.Error())),
	}
}

func (t *SystemOneInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("systemone does not support streaming")
}

func (t *SystemOneInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("systemone does not support streaming")
}
