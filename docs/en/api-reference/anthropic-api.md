# Anthropic API Reference

## Overview

AxonHub supports the native Anthropic Messages API for applications that prefer Anthropic's specific features and response format. You can use the Anthropic SDK to access not only Claude models but also OpenAI, Gemini, and other supported models.

## Key Benefits

- **API Interoperability**: Use Anthropic Messages API to call OpenAI, Gemini, and other supported models
- **Zero Code Changes**: Continue using your existing Anthropic client SDK without modification
- **Automatic Translation**: AxonHub automatically converts between API formats when needed
- **Provider Flexibility**: Access any supported AI provider using the Anthropic API format

## Supported Endpoints

**Endpoints:**
- `POST /anthropic/v1/messages` - Text generation
- `POST /v1/messages` - Text generation (alternative)
- `GET /anthropic/v1/models` - List available models

**Example Request:**
```go
import (
    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

// Create Anthropic client with AxonHub configuration
client := anthropic.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/anthropic"),
    
)

// Call OpenAI model using Anthropic API format
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, GPT!")),
}

response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gpt-4o"),
    Messages:  messages,
    MaxTokens: 1024,
})
if err != nil {
    // Handle error appropriately
    panic(err)
}

// Extract text content from response
responseText := ""
for _, block := range response.Content {
    if textBlock := block.AsText(); textBlock != nil {
        responseText += textBlock.Text
    }
}
fmt.Println(responseText)
```

## API Translation Capabilities

AxonHub automatically translates between API formats, enabling powerful scenarios:

### Use Anthropic SDK with OpenAI Models
```go
// Anthropic SDK calling OpenAI model
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("What is machine learning?")),
}

response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gpt-4o"),  // OpenAI model
    Messages:  messages,
    MaxTokens: 1024,
})

// Access response
for _, block := range response.Content {
    if textBlock := block.AsText(); textBlock != nil {
        fmt.Println(textBlock.Text)
    }
}
// AxonHub automatically translates Anthropic format → OpenAI format
```

### Use Anthropic SDK with Gemini Models
```go
// Anthropic SDK calling Gemini model
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("Explain quantum computing")),
}

response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gemini-2.5"),  // Gemini model
    Messages:  messages,
    MaxTokens: 1024,
})

// Access response
for _, block := range response.Content {
    if textBlock := block.AsText(); textBlock != nil {
        fmt.Println(textBlock.Text)
    }
}
// AxonHub automatically translates Anthropic format → Gemini format
```

## Authentication

The Anthropic API format uses the following authentication:

- **Header**: `X-API-Key: <your-api-key>`

The API keys are managed through AxonHub's API Key management system and provide the same permissions regardless of which API format you use.

## Streaming Support

Anthropic API format supports streaming responses:

```go
// Anthropic SDK streaming
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("Count to five")),
}

stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gpt-4o"),
    Messages:  messages,
    MaxTokens: 1024,
})

// Collect streamed content
var content string
for stream.Next() {
    event := stream.Current()
    switch event := event.(type) {
    case anthropic.ContentBlockDeltaEvent:
        if event.Type == "content_block_delta" {
            content += event.Delta.Text
            fmt.Print(event.Delta.Text) // Print as it streams
        }
    }
}

if err := stream.Err(); err != nil {
    panic(err)
}

fmt.Println("\nComplete response:", content)
```

## Error Handling

Anthropic format error responses:

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key"
  }
}
```

## Tool Support

AxonHub supports **function tools** (custom function calling) through the Anthropic API format. However, provider-specific tools are **not supported**:

| Tool Type | Support Status | Notes |
| --------- | -------------- | ----- |
| **Function Tools** | ✅ Supported | Custom function definitions work across all providers |
| **Web Search** | ❌ Not Supported | Provider-specific (OpenAI, Anthropic, etc.) |
| **Code Interpreter** | ❌ Not Supported | Provider-specific (OpenAI, Anthropic, etc.) |
| **File Search** | ❌ Not Supported | Provider-specific |
| **Computer Use** | ❌ Not Supported | Anthropic-specific |

> **Note**: Only generic function tools that can be translated across providers are supported. Provider-specific tools like web search, code interpreter, and computer use require direct access to the provider's infrastructure and cannot be proxied through AxonHub.

## Best Practices

1. **Use Tracing Headers**: Include `AH-Trace-Id` and `AH-Thread-Id` headers for better observability
2. **Model Selection**: Specify the target model explicitly in your requests
3. **Error Handling**: Implement proper error handling for API responses
4. **Streaming**: Use streaming for better user experience with long responses
5. **Use Function Tools**: For tool calling, use generic function tools instead of provider-specific tools

## Migration Guide

### From Anthropic to AxonHub
```go
// Before: Direct Anthropic
client := anthropic.NewClient(
    option.WithAPIKey("anthropic-key"),
)

// After: AxonHub with Anthropic API
client := anthropic.NewClient(
    option.WithAPIKey("axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/anthropic"),
)
// Your existing code continues to work!
```
