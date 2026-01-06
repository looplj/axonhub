package stream

import (
	"context"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// EnsureUsage creates a decorator that ensures stream requests include usage information
// by setting IncludeUsage to true when stream mode is enabled.
func EnsureUsage() pipeline.Middleware {
	return pipeline.OnLlmRequest("stream-usage", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if request.Stream != nil && *request.Stream {
			if request.StreamOptions == nil {
				request.StreamOptions = &llm.StreamOptions{}
			}

			request.StreamOptions.IncludeUsage = true
		}

		return request, nil
	})
}
