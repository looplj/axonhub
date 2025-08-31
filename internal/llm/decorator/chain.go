package decorator

import (
	"context"
	"sync"

	"github.com/looplj/axonhub/internal/llm"
)

// Chain implements DecoratorChain interface.
type Chain struct {
	mu         sync.RWMutex
	decorators []Decorator
}

// NewChain creates a new decorator chain.
func NewChain() DecoratorChain {
	return &Chain{
		decorators: make([]Decorator, 0),
	}
}

// NewDecoratorChain creates a new decorator chain (alias for compatibility).
func NewDecoratorChain() DecoratorChain {
	return NewChain()
}

// Add adds a decorator to the chain.
func (c *Chain) Add(decorator Decorator) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.decorators = append(c.decorators, decorator)
}

// Remove removes a decorator from the chain by name.
func (c *Chain) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, decorator := range c.decorators {
		if decorator.Name() == name {
			// Remove the decorator at index i
			c.decorators = append(c.decorators[:i], c.decorators[i+1:]...)
			break
		}
	}
}

// Execute applies all decorators in the chain to the request.
func (c *Chain) ExecuteRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	currentRequest := request

	var err error

	// Apply each decorator in sequence
	for _, decorator := range c.decorators {
		currentRequest, err = decorator.DecorateRequest(ctx, currentRequest)
		if err != nil {
			return currentRequest, err
		}
	}

	return currentRequest, nil
}

// ExecuteResponse applies all decorators in the chain to the response.
func (c *Chain) ExecuteResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	currentResponse := response

	var err error

	// Apply each decorator in sequence
	for _, decorator := range c.decorators {
		currentResponse, err = decorator.DecorateResponse(ctx, currentResponse)
		if err != nil {
			return currentResponse, err
		}
	}

	return currentResponse, nil
}

// List returns all decorators in the chain.
func (c *Chain) List() []Decorator {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]Decorator, len(c.decorators))
	copy(result, c.decorators)

	return result
}

// Clear removes all decorators from the chain.
func (c *Chain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.decorators = c.decorators[:0]
}

// Size returns the number of decorators in the chain.
func (c *Chain) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.decorators)
}

// Count returns the number of decorators in the chain (alias for Size).
func (c *Chain) Count() int {
	return c.Size()
}
