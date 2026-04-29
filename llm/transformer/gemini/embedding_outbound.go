package gemini

import (
	"context"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// EmbeddingOutboundTransformer handles only Gemini embedding requests.
type EmbeddingOutboundTransformer struct {
	config Config
}

func NewEmbeddingOutboundTransformerWithConfig(config Config) (transformer.Outbound, error) {
	config = clenupConfig(config)

	return &EmbeddingOutboundTransformer{
		config: config,
	}, nil
}

func (t *EmbeddingOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatGeminiEmbedding
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

	base := &OutboundTransformer{config: t.config}

	return base.transformEmbeddingRequest(ctx, llmReq)
}

func (t *EmbeddingOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	base := &OutboundTransformer{config: t.config}

	return base.transformEmbeddingResponse(ctx, httpResp)
}

func (t *EmbeddingOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, transformer.ErrInvalidRequest
}

func (t *EmbeddingOutboundTransformer) TransformError(
	ctx context.Context,
	rawErr *httpclient.Error,
) *llm.ResponseError {
	base := &OutboundTransformer{config: t.config}

	return base.TransformError(ctx, rawErr)
}

func (t *EmbeddingOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, &llm.ResponseError{
		StatusCode: http.StatusBadRequest,
		Detail: llm.ErrorDetail{
			Message: "embedding does not support streaming",
			Type:    "invalid_request_error",
		},
	}
}

func (t *EmbeddingOutboundTransformer) buildEmbeddingURL(model string, isBatch bool) string {
	base := &OutboundTransformer{config: t.config}

	return base.buildEmbeddingURL(model, isBatch)
}
