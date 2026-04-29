package jina

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// RerankOutboundTransformer handles only Jina rerank requests.
type RerankOutboundTransformer struct {
	*OutboundTransformer
}

func NewRerankOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	base, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		return nil, err
	}

	return &RerankOutboundTransformer{OutboundTransformer: base}, nil
}

func (t *RerankOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatJinaRerank
}

func (t *RerankOutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("%w: llm request is nil", transformer.ErrInvalidRequest)
	}

	if llmReq.RequestType != llm.RequestTypeRerank {
		return nil, fmt.Errorf("%w: %s is not supported by %s outbound transformer", transformer.ErrInvalidRequest, llmReq.RequestType, t.APIFormat())
	}

	return t.transformRerankRequest(ctx, llmReq)
}

func (t *RerankOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	return t.transformRerankResponse(ctx, httpResp)
}

func (t *RerankOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, transformer.ErrInvalidRequest
}

func (t *RerankOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, transformer.ErrInvalidRequest
}
