package openai

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// EmbeddingOutboundTransformer handles only OpenAI embeddings requests.
type EmbeddingOutboundTransformer struct {
	config *Config
}

func NewEmbeddingOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if _, err := NewOutboundTransformerWithConfig(config); err != nil {
		return nil, err
	}

	return &EmbeddingOutboundTransformer{config: config}, nil
}

func (t *EmbeddingOutboundTransformer) transformError(
	ctx context.Context,
	rawErr *httpclient.Error,
) *llm.ResponseError {
	base := &OutboundTransformer{config: t.config}
	return base.TransformError(ctx, rawErr)
}

func (t *EmbeddingOutboundTransformer) TransformError(
	ctx context.Context,
	rawErr *httpclient.Error,
) *llm.ResponseError {
	return t.transformError(ctx, rawErr)
}

func (t *EmbeddingOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIEmbedding
}

func (t *EmbeddingOutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if llmReq.RequestType != llm.RequestTypeEmbedding {
		return nil, fmt.Errorf("%w: %s is not supported by %s outbound transformer", transformer.ErrInvalidRequest, llmReq.RequestType, t.APIFormat())
	}

	return transformEmbeddingRequest(ctx, t.config, llmReq)
}

func (t *EmbeddingOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	return transformEmbeddingResponse(ctx, httpResp, t.transformError)
}

func (t *EmbeddingOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, transformer.ErrInvalidRequest
}

func (t *EmbeddingOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, transformer.ErrInvalidRequest
}
