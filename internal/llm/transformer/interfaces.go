package transformer

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Inbound represents a transformer accpet the request from user and respond to use the transformed response.
// e.g: OpenAPI transformer accepts the request from user with OpenAPI format and respond with OpenAI format.
type Inbound interface {
	// TransformRequest transforms HTTP request to the unified request format.
	TransformRequest(ctx context.Context, request *httpclient.Request) (*llm.Request, error)

	// TransformResponse transforms the unified response format to HTTP response.
	TransformResponse(ctx context.Context, response *llm.Response) (*httpclient.Response, error)

	// TransformStream transforms the unified stream response format to HTTP response.
	TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error)

	// AggregateStreamChunks aggregates streaming response chunks into a complete response.
	// This method handles unified-specific streaming formats and converts the chunks to a the user request format complete response.
	// e.g: the user request with OpenAI format, but the provider response with Claude format, the chunks is the unified response format, the AggregateStreamChunks will convert
	// the chunks to the OpenAI response format.
	AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, error)
}

// Outbound represents a transformer that convert the generic Request to the undering provider format.
// And transform the response from the undering provider format to generic Response format.
type Outbound interface {
	// TransformRequest transforms the generic request to HTTP request.
	TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error)

	// TransformResponse transforms the HTTP response to the unified response format.
	TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error)

	// TransformStream transforms the HTTP stream response to the unified response format.
	TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error)

	// AggregateStreamChunks aggregates streaming response chunks into a complete response.
	// This method handles provider-specific streaming formats and converts the chunks to a original provider format complete response.
	// e.g: the user request with OpenAI format, but the provider response with Claude format, the chunks is the Claude response format, the AggregateStreamChunks will convert
	// the chunks to the Claude response format.
	AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, error)
}
