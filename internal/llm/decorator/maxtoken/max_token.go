package maxtoken

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
)

// EnsureMaxTokens creates a decorator that ensures requests have a max tokens value
// by setting it to the provided default when not already specified.
func EnsureMaxTokens(defaultValue int64) decorator.Decorator {
	return decorator.RequestDecorator("max-tokens", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if request.MaxTokens == nil {
			request.MaxTokens = &defaultValue
		}

		if *request.MaxTokens > defaultValue {
			request.MaxTokens = &defaultValue
		}

		return request, nil
	})
}
