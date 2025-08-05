package maxtoken

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
)

// ChatCompletionDecoratorImpl implements ChatCompletionDecorator interface.
type ChatCompletionDecoratorImpl struct {
	name               string
	defaultTemperature *float64
	defaultMaxTokens   *int64
}

// NewChatCompletionDecoratorImpl creates a new ChatCompletionDecoratorImpl.
func NewChatCompletionDecoratorImpl(name string) decorator.Decorator {
	return &ChatCompletionDecoratorImpl{
		name: name,
	}
}

// NewChatCompletionDecorator creates a new ChatCompletionDecoratorImpl (alias for compatibility).
func NewChatCompletionDecorator(name string) decorator.Decorator {
	return NewChatCompletionDecoratorImpl(name)
}

// Decorate modifies the chat completion request.
func (d *ChatCompletionDecoratorImpl) Decorate(
	ctx context.Context,
	request *llm.Request,
) (*llm.Request, error) {
	// Set default values if not specified
	if request.Temperature == nil {
		temp := 0.7
		request.Temperature = &temp
	}

	if request.MaxTokens == nil {
		request.MaxTokens = d.defaultMaxTokens
	}

	return request, nil
}

// Name returns the name of the decorator.
func (d *ChatCompletionDecoratorImpl) Name() string {
	return d.name
}

// SetDefaultTemperature sets the default temperature.
func (d *ChatCompletionDecoratorImpl) SetDefaultTemperature(temperature float64) {
	d.defaultTemperature = &temperature
}

// SetDefaultMaxTokens sets the default max tokens.
func (d *ChatCompletionDecoratorImpl) SetDefaultMaxTokens(maxTokens int64) {
	d.defaultMaxTokens = &maxTokens
}
