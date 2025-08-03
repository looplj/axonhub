package transformer

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// Inbound represents a transformer accpet the request from user and respond to use use the transformed response.
// e.g: OpenAPI transformer accepts the request from user with OpenAPI format and respond with OpenAI format.
type Inbound interface {
	// TransformRequest transforms HTTP request to the unified request format.
	TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error)

	// TransformResponse transforms the unified response format to HTTP response.
	TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error)

	// TransformStreamChunk transforms the unified stream chunk format to HTTP response.
	TransformStreamChunk(ctx context.Context, response *llm.Response) (*httpclient.StreamEvent, error)
}

// Outbound represents a transformer that convert the generic Request to the undering provider format.
// And transform the response from the undering provider format to generic Response format.
type Outbound interface {
	// TransformRequest transforms the generic request to HTTP request.
	TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error)

	// TransformResponse transforms the HTTP response to the unified response format.
	TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error)

	// TransformStreamChunks transforms generic stream event to the unified response format.
	TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error)

	// AggregateStreamChunks aggregates streaming response chunks into a complete response.
	// This method handles provider-specific streaming formats and converts them to a unified response.
	AggregateStreamChunks(ctx context.Context, chunks [][]byte) (*llm.Response, error)
}
