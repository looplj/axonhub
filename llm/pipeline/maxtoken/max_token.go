package maxtoken

import (
	"context"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// EnsureMaxTokens creates a decorator that ensures requests have a max tokens value
// by setting it to the provided default when not already specified.
func EnsureMaxTokens(defaultValue int64) pipeline.Middleware {
	return pipeline.OnLlmRequest("max-tokens", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		if request.MaxTokens == nil {
			request.MaxTokens = &defaultValue
		}

		if *request.MaxTokens > defaultValue {
			request.MaxTokens = &defaultValue
		}

		return request, nil
	})
}
