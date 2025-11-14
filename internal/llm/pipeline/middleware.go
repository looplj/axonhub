package pipeline

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
)

// Middleware modifies chat completion requests before they are sent to the provider.
type Middleware interface {
	// Name returns the name of the middleware
	Name() string

	// BeforeRequest modifies the request and returns the modified request or an error
	BeforeRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)
}

func BeforeRequest(name string, handler func(ctx context.Context, request *llm.Request) (*llm.Request, error)) Middleware {
	return &simpleMiddleware{
		name:          name,
		requestHandle: handler,
	}
}

type simpleMiddleware struct {
	name          string
	requestHandle func(ctx context.Context, request *llm.Request) (*llm.Request, error)
}

func (d *simpleMiddleware) Name() string {
	return d.name
}

func (d *simpleMiddleware) BeforeRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if d.requestHandle == nil {
		return request, nil
	}

	return d.requestHandle(ctx, request)
}
