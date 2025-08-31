package stream

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
)

// EnsureUsage creates a decorator that ensures stream requests include usage information
// by setting IncludeUsage to true when stream mode is enabled.
func EnsureUsage() decorator.Decorator {
	return decorator.RequestDecorator("stream-usage", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		// Only apply if stream mode is enabled
		if request.Stream != nil && *request.Stream {
			// Initialize StreamOptions if nil
			if request.StreamOptions == nil {
				request.StreamOptions = &llm.StreamOptions{}
			}
			// Ensure IncludeUsage is true
			request.StreamOptions.IncludeUsage = true
		}

		return request, nil
	})
}
