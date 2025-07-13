package transformer

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/llm/types"
)

// InboundTransformer converts HTTP requests to ChatCompletionRequest
type InboundTransformer interface {
	Transform(ctx context.Context, httpReq *http.Request) (*types.ChatCompletionRequest, error)
	TransformResponse(ctx context.Context, chatResp *types.ChatCompletionResponse, httpResp *http.Response) (*types.GenericHttpResponse, error)
	SupportsContentType(contentType string) bool
	Name() string
	Priority() int
}

// OutboundTransformer converts ChatCompletionRequest to provider-specific format
type OutboundTransformer interface {
	Transform(ctx context.Context, request *types.ChatCompletionRequest) (*types.GenericHttpRequest, error)
	TransformResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
	TransformStreamResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (<-chan *types.ChatCompletionStreamResponse, error)
	SupportsProvider(provider string) bool
	Name() string
	GetProviderConfig(provider string) (*types.ProviderConfig, error)
}

// TransformerRegistry manages inbound and outbound transformers
type TransformerRegistry interface {
	RegisterInboundTransformer(name string, transformer InboundTransformer)
	RegisterOutboundTransformer(name string, transformer OutboundTransformer)
	GetInboundTransformer(name string) (InboundTransformer, error)
	GetOutboundTransformer(name string) (OutboundTransformer, error)
	ListInboundTransformers() []string
	ListOutboundTransformers() []string
	UnregisterInboundTransformer(name string)
	UnregisterOutboundTransformer(name string)
	GetSupportedFormats() []string
}
