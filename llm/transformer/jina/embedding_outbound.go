package jina

import (
	"context"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// EmbeddingOutboundTransformer handles only Jina embeddings requests.
type EmbeddingOutboundTransformer struct {
	*OutboundTransformer
}

func NewEmbeddingOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	base, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		return nil, err
	}

	return &EmbeddingOutboundTransformer{OutboundTransformer: base}, nil
}

func (t *EmbeddingOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatJinaEmbedding
}

func (t *EmbeddingOutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	if llmReq.RequestType != llm.RequestTypeEmbedding {
		return nil, fmt.Errorf("%w: %s is not supported by %s outbound transformer", transformer.ErrInvalidRequest, llmReq.RequestType, t.APIFormat())
	}

	return t.transformEmbeddingRequest(ctx, llmReq)
}

func (t *EmbeddingOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= http.StatusBadRequest {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	return t.transformEmbeddingResponse(ctx, httpResp)
}

func (t *EmbeddingOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("embedding does not support streaming")
}

func (t *EmbeddingOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("embedding does not support streaming")
}
