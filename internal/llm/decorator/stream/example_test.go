package stream

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
)

// Example of how to use the new stream usage decorator.
func Example() {
	// Create a new decorator chain
	chain := decorator.NewChain()

	// Add the stream usage decorator to the chain
	chain.Add(EnsureUsage())

	// Create a request with stream enabled
	streamEnabled := true
	req := &llm.Request{
		Stream: &streamEnabled,
	}

	// Execute the decorator chain
	result, err := chain.ExecuteRequest(context.Background(), req)
	if err != nil {
		panic(err)
	}

	// The StreamOptions should now have IncludeUsage set to true
	fmt.Printf("IncludeUsage: %v\n", result.StreamOptions.IncludeUsage)
	// Output: IncludeUsage: true
}
