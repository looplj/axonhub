package transformer

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
)

// Inbound represents a transformer accpet the request from user and respond to use use the transformed response.
// e.g: OpenAPI transformer accepts the request from user with OpenAPI format and respond with OpenAI format.
type Inbound interface {
	// TransformRequest transforms HTTP request to ChatCompletionRequest.
	TransformRequest(ctx context.Context, rwReq *llm.GenericHttpRequest) (*llm.ChatCompletionRequest, error)

	// TransformResponse transforms ChatCompletionResponse to HTTP response.
	TransformResponse(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericHttpResponse, error)

	// TransformStreamChunk transforms ChatCompletionResponse to HTTP response.
	TransformStreamChunk(ctx context.Context, chatResp *llm.ChatCompletionResponse) (*llm.GenericStreamEvent, error)
}

// Outbound represents a transformer that convert request to the undering provider format.
// And transform the response from the undering provider format to generic chat completion format.
type Outbound interface {
	// TransformRequest transforms ChatCompletionRequest to HTTP request.
	TransformRequest(ctx context.Context, chatReq *llm.ChatCompletionRequest) (*llm.GenericHttpRequest, error)

	// TransformResponse transforms ChatCompletionResponse to HTTP response.
	TransformResponse(ctx context.Context, chatResp *llm.GenericHttpResponse) (*llm.ChatCompletionResponse, error)

	// TransformStreamChunks transforms generic HTTP response to ChatCompletionResponse.
	TransformStreamChunk(ctx context.Context, chatResp *llm.GenericHttpResponse) (*llm.ChatCompletionResponse, error)

	// AggregateStreamChunks aggregates streaming response chunks into a complete response.
	// This method handles provider-specific streaming formats and converts them to a unified response.
	AggregateStreamChunks(ctx context.Context, chunks [][]byte) (*llm.ChatCompletionResponse, error)
}
