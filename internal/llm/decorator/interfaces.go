package decorator

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
)

// Decorator modifies chat completion requests before they are sent to the provider.
type Decorator interface {
	// Name returns the name of the decorator
	Name() string

	// DecorateRequest modifies the request and returns the modified request or an error
	DecorateRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)

	// DecorateResponse modifies the response and returns the modified response or an error
	DecorateResponse(ctx context.Context, response *llm.Response) (*llm.Response, error)
}

// DecoratorChain manages a chain of decorators.
type DecoratorChain interface {
	Add(decorator Decorator)
	Remove(name string)
	ExecuteRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)
	ExecuteResponse(ctx context.Context, response *llm.Response) (*llm.Response, error)
	List() []Decorator
	Clear()
}
