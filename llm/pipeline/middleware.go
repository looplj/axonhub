package pipeline

import (
	"context"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// Middleware defines the interface for pipeline decorators.
//
// Execution concepts:
// - Request: A single call to pipeline.Process(...) (e.g., one chat completion request from client).
// - Attempt: A single outbound execution (request to provider). A request may contain multiple attempts due to retries or channel switching.
type Middleware interface {
	// Name returns the name of the middleware
	Name() string

	// OnInboundLlmRequest executes after inbound transformation (Provider -> Unified) and before any outbound logic.
	// Timing: Once per Request, before any attempts.
	// Order: Forward (first registered executes first).
	OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)

	// OnInboundRawResponse executes after the final unified response is transformed back to provider format (Unified -> Provider).
	// Timing: Once per successful non-streaming Request.
	// Order: Forward.
	OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)

	// OnOutboundRawRequest executes after outbound transformation (Unified -> Provider) and before sending the request.
	// Timing: Once per Attempt (will repeat on retries/switches).
	// Order: Forward.
	OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)

	// OnOutboundRawError executes if the provider request fails (network error or status code >= 400).
	// Timing: Once per failed Attempt.
	// Order: Reverse (last registered executes first).
	OnOutboundRawError(ctx context.Context, err error)

	// OnOutboundRawResponse executes after a successful provider response is received.
	// Timing: Once per successful non-streaming Attempt.
	// Order: Reverse.
	OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)

	// OnOutboundLlmResponse executes after the provider response is transformed to unified format.
	// Timing: Once per successful non-streaming Attempt.
	// Order: Reverse.
	OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error)

	// OnOutboundRawStream executes after a successful provider stream is established.
	// Timing: Once per successful streaming Attempt. The middleware can wrap the stream to process individual chunks.
	// Order: Reverse.
	OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error)

	// OnOutboundLlmStream executes after the provider stream is transformed to unified format.
	// Timing: Once per successful streaming Attempt.
	// Order: Reverse.
	OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
}

func OnLlmRequest(name string, handler func(ctx context.Context, request *llm.Request) (*llm.Request, error)) Middleware {
	return &simpleMiddleware{
		name:                  name,
		inboundRequestHandler: handler,
	}
}

func OnRawRequest(name string, handler func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)) Middleware {
	return &simpleMiddleware{
		name:                      name,
		outboundRawRequestHandler: handler,
	}
}

type simpleMiddleware struct {
	name                            string
	inboundRequestHandler           func(ctx context.Context, request *llm.Request) (*llm.Request, error)
	inboundRawResponseHandler       func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)
	outboundRawRequestHandler       func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)
	outboundRawResponseHandler      func(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)
	outboundRawStreamHandler        func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error)
	outboundRawErrorResponseHandler func(ctx context.Context, err error)
	outboundLlmResponseHandler      func(ctx context.Context, response *llm.Response) (*llm.Response, error)
	outboundLlmStreamHandler        func(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
}

func (d *simpleMiddleware) Name() string {
	return d.name
}

func (d *simpleMiddleware) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if d.inboundRequestHandler == nil {
		return request, nil
	}

	return d.inboundRequestHandler(ctx, request)
}

func (d *simpleMiddleware) OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	if d.inboundRawResponseHandler == nil {
		return response, nil
	}

	return d.inboundRawResponseHandler(ctx, response)
}

func (d *simpleMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	if d.outboundRawRequestHandler == nil {
		return request, nil
	}

	return d.outboundRawRequestHandler(ctx, request)
}

func (d *simpleMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	if d.outboundRawErrorResponseHandler == nil {
		return
	}

	d.outboundRawErrorResponseHandler(ctx, err)
}

func (d *simpleMiddleware) OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	if d.outboundRawResponseHandler == nil {
		return response, nil
	}

	return d.outboundRawResponseHandler(ctx, response)
}

func (d *simpleMiddleware) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	if d.outboundLlmResponseHandler == nil {
		return response, nil
	}

	return d.outboundLlmResponseHandler(ctx, response)
}

func (d *simpleMiddleware) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	if d.outboundRawStreamHandler == nil {
		return stream, nil
	}

	return d.outboundRawStreamHandler(ctx, stream)
}

func (d *simpleMiddleware) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	if d.outboundLlmStreamHandler == nil {
		return stream, nil
	}

	return d.outboundLlmStreamHandler(ctx, stream)
}

type DummyMiddleware struct {
	name string
}

func (d *DummyMiddleware) Name() string {
	return d.name
}

func (d *DummyMiddleware) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	return request, nil
}

func (d *DummyMiddleware) OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	return response, nil
}

func (d *DummyMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	return request, nil
}

func (d *DummyMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	// Do nothing
}

func (d *DummyMiddleware) OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error) {
	return response, nil
}

func (d *DummyMiddleware) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	return response, nil
}

func (d *DummyMiddleware) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return stream, nil
}

func (d *DummyMiddleware) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	return stream, nil
}
