package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// AlphaSearchInboundTransformer accepts the Codex/CPA alpha search envelope
// while preserving all provider-specific fields for transparent forwarding.
type AlphaSearchInboundTransformer struct{}

func NewAlphaSearchInboundTransformer() *AlphaSearchInboundTransformer {
	return &AlphaSearchInboundTransformer{}
}

func (t *AlphaSearchInboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}
	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(httpReq.Body, &envelope); err != nil || envelope == nil {
		return nil, fmt.Errorf("%w: failed to decode alpha search request: %v", transformer.ErrInvalidRequest, err)
	}
	var model string
	if raw, ok := envelope["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("%w: model is required for alpha search routing", transformer.ErrInvalidRequest)
	}

	return &llm.Request{
		Model:       model,
		Messages:    []llm.Message{},
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
		AlphaSearch: &llm.AlphaSearchRequest{Body: append([]byte(nil), httpReq.Body...)},
	}, nil
}

func (t *AlphaSearchInboundTransformer) TransformResponse(ctx context.Context, resp *llm.Response) (*httpclient.Response, error) {
	if resp == nil || resp.AlphaSearch == nil || len(resp.AlphaSearch.Body) == 0 {
		return nil, fmt.Errorf("alpha search response is empty")
	}
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       append([]byte(nil), resp.AlphaSearch.Body...),
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (t *AlphaSearchInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: alpha search does not support streaming", transformer.ErrInvalidRequest)
}

func (t *AlphaSearchInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("alpha search does not support streaming")
}

func (t *AlphaSearchInboundTransformer) TransformError(ctx context.Context, err error) *httpclient.Error {
	return NewInboundTransformer().TransformError(ctx, err)
}
