package pipeline

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Middleware modifies chat completion requests before they are sent to the provider.
type Middleware interface {
	// Name returns the name of the middleware
	Name() string

	// OnInboundLlmRequest execute after inbound transform http request to llm request and before outbound transform llm request to http request.
	OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)

	// OnOutboundRawRequest execute after outbound transform llm request to http request and before send request to the provider.
	OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)

	// OnOutboundRawErrorResponse execute after send request to the provider and before outbound transform http response to llm response.
	OnOutboundRawErrorResponse(ctx context.Context, err error)

	// OnOutboundRawResponse execute after send request to the provider and before outbound transform http response to llm response.
	// Only execute if the request is not a stream.
	OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)

	// OnOutboundLlmResponse execute after outbound transform http response to llm response and before send response to the client.
	// Only execute if the request is not a stream.
	OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error)

	// OnOutboundRawStream execute after send request to the provider and before outbound transform http stream to llm stream.
	// Only execute if the request is a stream.
	OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error)

	// OnOutboundLlmStream execute after outbound transform http stream to llm stream and before send stream to the client.
	// Only execute if the request is a stream.
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
		name:                   name,
		outboundRequestHandler: handler,
	}
}

type simpleMiddleware struct {
	name                            string
	inboundRequestHandler           func(ctx context.Context, request *llm.Request) (*llm.Request, error)
	outboundRequestHandler          func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)
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

func (d *simpleMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	if d.outboundRequestHandler == nil {
		return request, nil
	}

	return d.outboundRequestHandler(ctx, request)
}

func (d *simpleMiddleware) OnOutboundRawErrorResponse(ctx context.Context, err error) {
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

func (d *DummyMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	return request, nil
}

func (d *DummyMiddleware) OnOutboundRawErrorResponse(ctx context.Context, err error) {
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
