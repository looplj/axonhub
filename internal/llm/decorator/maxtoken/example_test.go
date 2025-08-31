package maxtoken

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/decorator"
)

// Example of how to use the EnsureMaxTokens decorator.
func ExampleEnsureMaxTokens() {
	// Create a new decorator chain
	chain := decorator.NewChain()

	// Add the max token decorator to the chain with a default value of 150
	chain.Add(EnsureMaxTokens(150))

	content := "Hello, world!"
	req := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
		// MaxTokens is nil initially
	}

	// Execute the decorator chain
	result, err := chain.ExecuteRequest(context.Background(), req)
	if err != nil {
		panic(err)
	}

	// The MaxTokens should now be set to the default value
	fmt.Printf("MaxTokens: %d\n", *result.MaxTokens)
	// Output: MaxTokens: 150
}
