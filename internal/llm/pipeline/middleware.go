package pipeline

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// Middleware modifies chat completion requests before they are sent to the provider.
type Middleware interface {
	// Name returns the name of the middleware
	Name() string

	// OnLlmRequest execute after inbound transform http request to llm request and before outbound transform llm request to http request.
	OnLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)

	// OnRawRequest execute after outbound transform llm request to http request and before send request to the provider.
	OnRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)
}

func OnLlmRequest(name string, handler func(ctx context.Context, request *llm.Request) (*llm.Request, error)) Middleware {
	return &simpleMiddleware{
		name:           name,
		requestHandler: handler,
	}
}

func OnRawRequest(name string, handler func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)) Middleware {
	return &simpleMiddleware{
		name:              name,
		rawRequestHandler: handler,
	}
}

type simpleMiddleware struct {
	name              string
	requestHandler    func(ctx context.Context, request *llm.Request) (*llm.Request, error)
	rawRequestHandler func(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)
}

func (d *simpleMiddleware) Name() string {
	return d.name
}

func (d *simpleMiddleware) OnLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if d.requestHandler == nil {
		return request, nil
	}

	return d.requestHandler(ctx, request)
}

func (d *simpleMiddleware) OnRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	if d.rawRequestHandler == nil {
		return request, nil
	}

	return d.rawRequestHandler(ctx, request)
}
