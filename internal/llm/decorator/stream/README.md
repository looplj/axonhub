# Stream Package

This package contains decorators related to stream functionality for LLM requests.

## Usage

To ensure that stream requests include usage information, use the `EnsureUsage` decorator:

```go
import "github.com/looplj/axonhub/internal/llm/stream"

// Create a new decorator chain
chain := decorator.NewChain()

// Add the stream usage decorator to the chain
chain.Add(stream.EnsureUsage())

// Create a request with stream enabled
streamEnabled := true
req := &llm.Request{
    Stream: &streamEnabled,
}

// Execute the decorator chain
result, err := chain.ExecuteRequest(context.Background(), req)
if err != nil {
    // handle error
}

// The StreamOptions will now have IncludeUsage set to true
```

## Benefits

Using the functional approach with `stream.EnsureUsage()` provides:

1. Cleaner, more focused code
2. Better separation of concerns
3. Easier testing and maintenance
4. Consistent with other functional decorators in the codebase