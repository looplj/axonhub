# Provider Architecture

## Overview

The Provider architecture replaces the previous `OutboundTransformer` + `HTTPClient` pattern with a unified `Provider` interface that handles both request transformation and HTTP communication for specific LLM providers.

## Key Benefits

1. **Simplified Architecture**: Combines transformation and HTTP logic into a single component
2. **Better Abstraction**: Each provider encapsulates all provider-specific logic
3. **Improved Streaming**: Native support for `Stream[ChatCompletionResponse]` return type
4. **Modern Libraries**: Uses provider-specific SDKs (e.g., `sashabaranov/go-openai`)
5. **Easier Testing**: Self-contained providers are easier to mock and test

## Architecture

```
HTTP Request → InboundTransformer → ChatCompletionRequest → Provider → ChatCompletionResponse
                                                        ↓
                                                   Stream[ChatCompletionResponse]
```

### Core Interfaces

#### Provider Interface
```go
type Provider interface {
    ChatCompletion(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
    ChatCompletionStream(ctx context.Context, request *types.ChatCompletionRequest) (Stream[*types.ChatCompletionResponse], error)
    SupportsModel(model string) bool
    GetConfig() types.ProviderConfig
    SetConfig(config types.ProviderConfig)
}
```

#### Stream Interface
```go
type Stream[T any] interface {
    ResponseChannel() <-chan T
    ErrorChannel() <-chan error
}
```

#### Provider Registry
```go
type ProviderRegistry interface {
    RegisterProvider(name string, provider Provider) error
    GetProvider(name string) (Provider, error)
    GetProviderForModel(model string) (Provider, error)
    ListProviders() []string
    UnregisterProvider(name string) error
    MapModelToProvider(model, providerName string) error
}
```

## Available Providers

### OpenAI Provider
- **Location**: `llm/provider/openai/`
- **Library**: `sashabaranov/go-openai`
- **Supported Models**: `gpt-3.5-turbo`, `gpt-4`, `gpt-4-turbo`, etc.
- **Features**: 
  - Native streaming support
  - Multi-modal content (text + images)
  - Tool calling
  - Function calling

### DeepSeek Provider
- **Location**: `llm/provider/deepseek/`
- **Library**: Custom HTTP client
- **Supported Models**: `deepseek-chat`, `deepseek-coder`, `deepseek-reasoner`
- **Features**:
  - Streaming support
  - Multi-modal content
  - Reasoning content support
  - Tool calling

## Usage Example

```go
package main

import (
    "context"
    "log"
    
    "github.com/September-1/axonhub/llm/client"
    "github.com/September-1/axonhub/llm/provider"
    "github.com/September-1/axonhub/llm/provider/openai"
    "github.com/September-1/axonhub/llm/provider/deepseek"
    "github.com/September-1/axonhub/llm/types"
)

func main() {
    // Create provider registry
    registry := provider.NewProviderRegistry()
    
    // Register OpenAI provider
    openaiProvider := openai.NewProvider()
    registry.RegisterProvider("openai", openaiProvider)
    registry.MapModelToProvider("gpt-3.5-turbo", "openai")
    
    // Register DeepSeek provider
    httpClient := client.NewHttpClient()
    deepseekProvider := deepseek.NewProvider(httpClient)
    registry.RegisterProvider("deepseek", deepseekProvider)
    registry.MapModelToProvider("deepseek-chat", "deepseek")
    
    // Use provider for chat completion
    ctx := context.Background()
    request := &types.ChatCompletionRequest{
        Model: "gpt-3.5-turbo",
        Messages: []types.ChatCompletionMessage{
            {
                Role: "user",
                Content: types.ChatCompletionMessageContent{
                    Content: stringPtr("Hello, how are you?"),
                },
            },
        },
    }
    
    // Get provider for model
    prov, err := registry.GetProviderForModel(request.Model)
    if err != nil {
        log.Fatal(err)
    }
    
    // Non-streaming completion
    response, err := prov.ChatCompletion(ctx, request)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Response: %+v", response)
    
    // Streaming completion
    request.Stream = true
    stream, err := prov.ChatCompletionStream(ctx, request)
    if err != nil {
        log.Fatal(err)
    }
    
    for {
        select {
        case resp, ok := <-streams.ResponseChannel():
            if !ok {
                return // Stream ended
            }
            log.Printf("Stream response: %+v", resp)
        case err := <-streams.ErrorChannel():
            if err != nil {
                log.Printf("Stream error: %v", err)
                return
            }
        }
    }
}

func stringPtr(s string) *string {
    return &s
}
```

## Migration Guide

### From OutboundTransformer + HTTPClient to Provider

#### Before (Old Architecture)
```go
type ChatCompletionProcessor struct {
    InboundTransformer  transformer.Transformer
    OutboundTransformer transformer.OutboundTransformer
    HTTPClient          client.HttpClient
}

// Usage
genericReq, err := processor.OutboundTransformer.Transform(ctx, chatReq)
httpResp, err := processor.HTTPClient.DoRequest(ctx, genericReq)
chatResp, err := processor.OutboundTransformer.TransformResponse(ctx, httpResp, chatReq)
```

#### After (New Architecture)
```go
type ChatCompletionProcessor struct {
    InboundTransformer transformer.Transformer
    ProviderRegistry   provider.ProviderRegistry
}

// Usage
prov, err := processor.ProviderRegistry.GetProviderForModel(chatReq.Model)
chatResp, err := prov.ChatCompletion(ctx, chatReq)
// OR for streaming
stream, err := prov.ChatCompletionStream(ctx, chatReq)
```

### Key Changes

1. **Remove OutboundTransformer**: No longer needed, logic moved to Provider
2. **Remove HTTPClient dependency**: Each provider manages its own HTTP communication
3. **Add ProviderRegistry**: Central registry for managing providers
4. **Update streaming**: Now returns `Stream[ChatCompletionResponse]` instead of channels
5. **Model-to-Provider mapping**: Registry handles routing requests to appropriate providers

### Breaking Changes

1. `OutboundTransformer` interface is deprecated
2. `HTTPClient.DoStream()` return type changed from `*GenericHttpResponse` to `Stream[ChatCompletionResponse]`
3. Constructor signatures changed to accept `ProviderRegistry` instead of `OutboundTransformer` + `HTTPClient`
4. Provider-specific configuration now handled through `Provider.SetConfig()`

## Adding New Providers

To add a new provider:

1. Create a new directory under `llm/provider/`
2. Implement the `Provider` interface
3. Handle request/response transformations internally
4. Support both streaming and non-streaming modes
5. Register the provider in your application setup

### Example Provider Structure
```
llm/provider/newprovider/
├── provider.go          # Main provider implementation
├── client.go           # HTTP client wrapper (if needed)
├── types.go            # Provider-specific types (if needed)
└── README.md           # Provider-specific documentation
```

## Testing

Providers can be easily mocked for testing:

```go
type MockProvider struct{}

func (m *MockProvider) ChatCompletion(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
    return &types.ChatCompletionResponse{
        ID: "test-response",
        Choices: []types.ChatCompletionChoice{
            {
                Message: &types.ChatCompletionMessage{
                    Role: "assistant",
                    Content: types.ChatCompletionMessageContent{
                        Content: stringPtr("Mock response"),
                    },
                },
            },
        },
    }, nil
}

// Implement other Provider methods...
```

## Configuration

Providers can be configured through the `ProviderConfig` type:

```go
config := types.ProviderConfig{
    Name:    "openai",
    BaseURL: "https://api.openai.com",
    APIKey:  "your-api-key",
    Timeout: 30,
}
provider.SetConfig(config)
```