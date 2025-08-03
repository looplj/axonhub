package decorator

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
)

// ChatCompletionDecorator modifies chat completion requests before they are sent to the provider
type ChatCompletionDecorator interface {
	// Decorate modifies the request and returns the modified request or an error
	Decorate(ctx context.Context, request *llm.Request) (*llm.Request, error)

	// Name returns the name of the decorator
	Name() string
}

// DecoratorChain manages a chain of decorators
type DecoratorChain interface {
	Add(decorator ChatCompletionDecorator)
	Remove(name string)
	Execute(ctx context.Context, request *llm.Request) (*llm.Request, error)
	List() []ChatCompletionDecorator
	Clear()
}
