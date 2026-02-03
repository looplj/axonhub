# Gemini API Reference

## Overview

AxonHub provides native support for the Gemini API, enabling access to Gemini's powerful multi-modal capabilities. You can use the Gemini SDK to access not only Gemini models but also OpenAI, Anthropic, and other supported models.

## Key Benefits

- **API Interoperability**: Use Gemini API to call OpenAI, Anthropic, and other supported models
- **Zero Code Changes**: Continue using your existing Gemini client SDK without modification
- **Automatic Translation**: AxonHub automatically converts between API formats when needed
- **Multi-modal Support**: Access text and image capabilities through the Gemini API format

## Supported Endpoints

**Endpoints:**
- `POST /gemini/v1beta/models/{model}:generateContent` - Text and multi-modal content generation
- `POST /v1beta/models/{model}:generateContent` - Text and multi-modal content generation (alternative)
- `GET /gemini/v1beta/models` - List available models
- `GET /v1beta/models` - List available models (alternative)

**Example Request:**
```go
import (
    "context"
    "google.golang.org/genai"
)

// Create Gemini client with AxonHub configuration
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey:  "your-axonhub-api-key",
    Backend: genai.Backend(genai.APIBackendUnspecified), // Use default backend
    HTTPOptions: genai.HTTPOptions{
			BaseURL: "http://localhost:8090/gemini",
	},
})
if err != nil {
    // Handle error appropriately
    panic(err)
}

// Call OpenAI model using Gemini API format
modelName := "gpt-4o"  // OpenAI model accessed via Gemini API format
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("Hello, GPT!")},
    },
}

// Optional: Configure generation parameters
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.7)),
    MaxOutputTokens: genai.Ptr(int32(1024)),
}

response, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{content}, config)
if err != nil {
    // Handle error appropriately
    panic(err)
}

// Extract text from response
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    responseText := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*responseText)
}
```

**Example: Multi-turn Conversation**
```go
// Create a chat session with conversation history
modelName := "claude-3-5-sonnet"
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.5)),
}

chat, err := client.Chats.Create(ctx, modelName, config, nil)
if err != nil {
    panic(err)
}

// First message
response1, err := chat.SendMessage(ctx, genai.Part{Text: genai.Ptr("My name is Alice")})
if err != nil {
    panic(err)
}

// Follow-up message (model remembers context)
response2, err := chat.SendMessage(ctx, genai.Part{Text: genai.Ptr("What is my name?")})
if err != nil {
    panic(err)
}

// Extract response
if len(response2.Candidates) > 0 {
    text := response2.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)  // Should contain "Alice"
}
```

## API Translation Capabilities

AxonHub automatically translates between API formats, enabling powerful scenarios:

### Use Gemini SDK with OpenAI Models
```go
// Gemini SDK calling OpenAI model
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("Explain neural networks")},
    },
}

response, err := client.Models.GenerateContent(
    ctx,
    "gpt-4o",  // OpenAI model
    []*genai.Content{content},
    nil,
)

// Access response
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    text := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)
}
// AxonHub automatically translates Gemini format → OpenAI format
```

### Use Gemini SDK with Anthropic Models
```go
// Gemini SDK calling Anthropic model
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("What is artificial intelligence?")},
    },
}

response, err := client.Models.GenerateContent(
    ctx,
    "claude-3-5-sonnet",  // Anthropic model
    []*genai.Content{content},
    nil,
)

// Access response
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    text := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)
}
// AxonHub automatically translates Gemini format → Anthropic format
```

## Authentication

The Gemini API format uses the following authentication:

- **Header**: `X-Goog-API-Key: <your-api-key>`

The API keys are managed through AxonHub's API Key management system and provide the same permissions regardless of which API format you use.

## Streaming Support

Gemini API format supports streaming responses for real-time content generation.

## Error Handling

Gemini format error responses follow the standard Gemini API error format.

## Tool Support

AxonHub supports **function tools** (custom function calling) through the Gemini API format. However, provider-specific tools are **not supported**:

| Tool Type | Support Status | Notes |
| --------- | -------------- | ----- |
| **Function Tools** | ✅ Supported | Custom function definitions work across all providers |
| **Web Search** | ❌ Not Supported | Provider-specific |
| **Code Interpreter** | ❌ Not Supported | Provider-specific |
| **File Search** | ❌ Not Supported | Provider-specific |
| **Computer Use** | ❌ Not Supported | Anthropic-specific |

> **Note**: Only generic function tools that can be translated across providers are supported. Provider-specific tools require direct access to the provider's infrastructure and cannot be proxied through AxonHub.

## Best Practices

1. **Use Tracing Headers**: Include `AH-Trace-Id` and `AH-Thread-Id` headers for better observability
2. **Model Selection**: Specify the target model explicitly in your requests
3. **Error Handling**: Implement proper error handling for API responses
4. **Streaming**: Use streaming for better user experience with long responses
5. **Multi-modal Content**: Leverage Gemini API's multi-modal capabilities when working with images

## Migration Guide

### From Gemini to AxonHub
```go
// Before: Direct Gemini
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey: "gemini-api-key",
})

// After: AxonHub with Gemini API
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey: "your-axonhub-api-key",
    HTTPOptions: genai.HTTPOptions{
        BaseURL: "http://localhost:8090/gemini",
    },
})
// Your existing code continues to work!
```
