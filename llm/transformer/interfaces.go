package transformer

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/llm"
)

// Inbound represents a transformer accpet the request from user and respond to use use the transformed response.
// e.g: OpenAPI transformer accepts the request from user with OpenAPI format and respond with OpenAI format.
type Inbound interface {
	// TransformRequest transforms HTTP request to ChatCompletionRequest.
	TransformRequest(ctx context.Context, rwReq *llm.GenericHttpRequest) (*llm.ChatCompletionRequest, error)

	// TransformResponse transforms ChatCompletionResponse to HTTP response.
	TransformResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error)
}

// Outbound represents a transformer that convert request to the undering provider format.
// And transform the response from the undering provider format to generic chat completion format.
type Outbound interface {
	// TransformRequest transforms ChatCompletionRequest to HTTP request.
	TransformRequest(ctx context.Context, chatReq *llm.ChatCompletionRequest) (*llm.GenericHttpRequest, error)

	// TransformResponse transforms ChatCompletionResponse to HTTP response.
	TransformResponse(ctx context.Context, chatResp *llm.GenericHttpResponse) (*llm.ChatCompletionResponse, error)
}

// Transformer converts HTTP requests to ChatCompletionRequest
type Transformer interface {
	TransformRequest(ctx context.Context, httpReq *http.Request) (*llm.ChatCompletionRequest, error)
	TransformResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error)
	TransformStreamResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error)
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
