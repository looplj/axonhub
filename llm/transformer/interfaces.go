package transformer

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/llm/types"
)

// Transformer converts HTTP requests to ChatCompletionRequest
type Transformer interface {
	TransformRequest(ctx context.Context, httpReq *http.Request) (*types.ChatCompletionRequest, error)
	TransformResponse(ctx context.Context, chatResp *types.ChatCompletionResponse) (*types.GenericHttpResponse, error)
	TransformStreamResponse(ctx context.Context, chatResp *types.ChatCompletionResponse) (*types.GenericHttpResponse, error)
	SupportsContentType(contentType string) bool
	Name() string
	Priority() int
}

type Stream[T any] interface {
	Next() bool
	Current() T
	Err() error
	Close() error
}

// TransformerRegistry manages inbound and outbound transformers
type TransformerRegistry interface {
	RegisterInboundTransformer(name string, transformer Transformer)
	GetInboundTransformer(name string) (Transformer, error)
	ListInboundTransformers() []string
	UnregisterInboundTransformer(name string)
	GetSupportedFormats() []string
}
