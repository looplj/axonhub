package decorator

import (
	"context"
	"sync"

	"github.com/looplj/axonhub/internal/llm"
)

// Chain implements DecoratorChain interface
type Chain struct {
	mu         sync.RWMutex
	decorators []ChatCompletionDecorator
}

// NewChain creates a new decorator chain
func NewChain() DecoratorChain {
	return &Chain{
		decorators: make([]ChatCompletionDecorator, 0),
	}
}

// NewDecoratorChain creates a new decorator chain (alias for compatibility)
func NewDecoratorChain() DecoratorChain {
	return NewChain()
}

// Add adds a decorator to the chain
func (c *Chain) Add(decorator ChatCompletionDecorator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decorators = append(c.decorators, decorator)
}

// Remove removes a decorator from the chain by name
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

// Execute applies all decorators in the chain to the request
func (c *Chain) Execute(ctx context.Context, request *llm.ChatCompletionRequest) (*llm.ChatCompletionRequest, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	currentRequest := request
	var err error

	// Apply each decorator in sequence
	for _, decorator := range c.decorators {
		currentRequest, err = decorator.Decorate(ctx, currentRequest)
		if err != nil {
			return currentRequest, err
		}
	}

	return currentRequest, nil
}

// List returns all decorators in the chain
func (c *Chain) List() []ChatCompletionDecorator {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]ChatCompletionDecorator, len(c.decorators))
	copy(result, c.decorators)
	return result
}

// Clear removes all decorators from the chain
func (c *Chain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decorators = c.decorators[:0]
}

// Size returns the number of decorators in the chain
func (c *Chain) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.decorators)
}

// Count returns the number of decorators in the chain (alias for Size)
func (c *Chain) Count() int {
	return c.Size()
}
