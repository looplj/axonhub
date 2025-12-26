# Unified API Reference

## Overview

AxonHub provides a unified API gateway that supports both OpenAI Chat Completions and Anthropic Messages APIs through a single interface. This allows you to use existing OpenAI or Anthropic client SDKs while seamlessly accessing models from multiple providers. The platform automatically handles API format translation, enabling you to use one API format to access models from any supported provider.

## Key Benefits

- **API Interoperability**: Use OpenAI Chat Completions API to call Anthropic models, or use Anthropic Messages API to call OpenAI models
- **Zero Code Changes**: Keep using your existing OpenAI or Anthropic client SDKs without modification
- **Automatic Translation**: AxonHub automatically converts between API formats when needed
- **Provider Flexibility**: Access any supported AI provider regardless of which API format you prefer

## Supported API Formats

### OpenAI Chat Completions API

AxonHub fully supports the OpenAI Chat Completions API specification, allowing you to use any OpenAI-compatible client SDK.

**Endpoints:**
- `POST /v1/chat/completions` - Text generation
- `GET /v1/models` - List available models

**Example Request:**
```go
import (
    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)

// Create OpenAI client with AxonHub configuration
client := openai.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
    
)

// Call Anthropic model using OpenAI API format
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Hello, Claude!"),
    },
    Model: openai.ChatModel("claude-3-5-sonnet"),
},
    option.WithHeader("AH-Trace-Id", "trace-example-123"),
    option.WithHeader("AH-Thread-Id", "thread-example-abc"))
if err != nil {
    // Handle error appropriately
    panic(err)
}

// Access the response content
responseText := completion.Choices[0].Message.Content
fmt.Println(responseText)
```

### OpenAI Responses API

AxonHub provides partial support for the OpenAI Responses API. This API offers a simplified interface for single-turn interactions.

**Endpoints:**
- `POST /v1/responses` - Generate a response

**Limitations:**
- ❌ `previous_response_id` is **not supported** - conversation history must be managed client-side
- ✅ Basic response generation is fully functional
- ✅ Streaming responses are supported

**Example Request:**
```go
import (
    "context"
    "fmt"

    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
    "github.com/openai/openai-go/v3/responses"
    "github.com/openai/openai-go/v3/shared"
)

// Create OpenAI client with AxonHub configuration
client := openai.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
)

ctx := context.Background()

// Generate a response (previous_response_id not supported)
params := responses.ResponseNewParams{
    Model: shared.ResponsesModel("gpt-4o"),
    Input: responses.ResponseNewParamsInputUnion{
        OfString: openai.String("Hello, how are you?"),
    },
}

response, err := client.Responses.New(ctx, params,
        option.WithHeader("AH-Trace-Id", "trace-example-123"),
        option.WithHeader("AH-Thread-Id", "thread-example-abc"))
if err != nil {
    panic(err)
}

fmt.Println(response.OutputText())
```

**Example: Streaming Response**
```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
    "github.com/openai/openai-go/v3/responses"
    "github.com/openai/openai-go/v3/shared"
)

client := openai.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
)

ctx := context.Background()

params := responses.ResponseNewParams{
    Model: shared.ResponsesModel("gpt-4o"),
    Input: responses.ResponseNewParamsInputUnion{
        OfString: openai.String("Tell me a short story about a robot."),
    },
}

stream := client.Responses.NewStreaming(ctx, params,
        option.WithHeader("AH-Trace-Id", "trace-example-123"),
        option.WithHeader("AH-Thread-Id", "thread-example-abc"))

var fullContent strings.Builder
for stream.Next() {
    event := stream.Current()
    if event.Type == "response.output_text.delta" && event.Delta != "" {
        fullContent.WriteString(event.Delta)
        fmt.Print(event.Delta) // Print as it streams
    }
}

if err := stream.Err(); err != nil {
    panic(err)
}

fmt.Println("\nComplete response:", fullContent.String())
```

### Anthropic Messages API

AxonHub also supports the native Anthropic Messages API for applications that prefer Anthropic's specific features and response format.

**Endpoints:**
- `POST /anthropic/v1/messages` - Text generation
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

### Gemini API

AxonHub provides native support for the Gemini API, enabling access to Gemini's powerful multi-modal capabilities.

**Endpoints:**
- `POST /gemini/v1beta/models/{model}:generateContent` - Text and multi-modal content generation

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

AxonHub automatically translates between API formats, enabling these powerful scenarios:

### Use OpenAI SDK with Anthropic Models
```go
// OpenAI SDK calling Anthropic model
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Tell me about artificial intelligence"),
    },
    Model: openai.ChatModel("claude-3-5-sonnet"),  // Anthropic model
})

// Access response
responseText := completion.Choices[0].Message.Content
fmt.Println(responseText)
// AxonHub automatically translates OpenAI format → Anthropic format
```

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

### Embedding API

AxonHub provides comprehensive support for text and multimodal embedding generation through OpenAI-compatible and Jina AI-specific APIs.

**Endpoints:**
- `POST /v1/embeddings` - OpenAI-compatible embedding API
- `POST /jina/v1/embeddings` - Jina AI-specific embedding API

**Supported Input Types:**
- Single text string
- Array of text strings
- Token arrays (integers)
- Multiple token arrays

**Supported Encoding Formats:**
- `float` - Default, returns embedding vectors as float arrays
- `base64` - Returns embeddings as base64-encoded strings

#### Request Format

```json
{
  "input": "The text to embed",
  "model": "text-embedding-3-small",
  "encoding_format": "float",
  "dimensions": 1536,
  "user": "user-id"
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `input` | string \| string[] \| number[] \| number[][] | ✅ | The text(s) to embed. Can be a single string, array of strings, token array, or multiple token arrays. |
| `model` | string | ✅ | The model to use for embedding generation. |
| `encoding_format` | string | ❌ | Format to return embeddings in. Either `float` or `base64`. Default: `float`. |
| `dimensions` | integer | ❌ | Number of dimensions for the output embeddings. |
| `user` | string | ❌ | Unique identifier for the end-user. |

**Jina-Specific Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task` | string | ❌ | Task type for Jina embeddings. Options: `text-matching`, `retrieval.query`, `retrieval.passage`, `separation`, `classification`, `none`. |

#### Response Format

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.123, 0.456, ...],
      "index": 0
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 4,
    "total_tokens": 4
  }
}
```

#### Examples

**OpenAI SDK (Python):**
```python
import openai

client = openai.OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

response = client.embeddings.create(
    input="Hello, world!",
    model="text-embedding-3-small"
)

print(response.data[0].embedding[:5])  # First 5 dimensions
```

**OpenAI SDK (Go):**
```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
)

func main() {
    client := openai.NewClient(
        option.WithAPIKey("your-axonhub-api-key"),
        option.WithBaseURL("http://localhost:8090/v1"),
    )

    embedding, err := client.Embeddings.New(context.TODO(), openai.EmbeddingNewParams{
        Input: openai.Union[string](openai.String("Hello, world!")),
        Model: openai.String("text-embedding-3-small"),
        option.WithHeader("AH-Trace-Id", "trace-example-123"),
        option.WithHeader("AH-Thread-Id", "thread-example-abc"),
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Embedding dimensions: %d\n", len(embedding.Data[0].Embedding))
    fmt.Printf("First 5 values: %v\n", embedding.Data[0].Embedding[:5])
}
```

**Multiple Texts:**
```python
response = client.embeddings.create(
    input=["Hello, world!", "How are you?"],
    model="text-embedding-3-small"
)

for i, data in enumerate(response.data):
    print(f"Text {i}: {data.embedding[:3]}...")
```

**Jina-Specific Task:**
```python
import requests

response = requests.post(
    "http://localhost:8090/jina/v1/embeddings",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "input": "What is machine learning?",
        "model": "jina-embeddings-v2-base-en",
        "task": "retrieval.query"
    }
)

result = response.json()
print(result["data"][0]["embedding"][:5])
```

### Rerank API

AxonHub supports document reranking through Jina AI rerank API, allowing you to reorder documents based on relevance to a query.

**Endpoints:**
- `POST /v1/rerank` - Jina-compatible rerank API (convenience endpoint)
- `POST /jina/v1/rerank` - Jina AI-specific rerank API

> **Note**: OpenAI does not provide a native rerank API. Both endpoints use Jina's rerank format.

#### Request Format

```json
{
  "model": "jina-reranker-v1-base-en",
  "query": "What is machine learning?",
  "documents": [
    "Machine learning is a subset of artificial intelligence...",
    "Deep learning uses neural networks...",
    "Statistics involves data analysis..."
  ],
  "top_n": 2,
  "return_documents": true
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | ✅ | The model to use for reranking (e.g., `jina-reranker-v1-base-en`). |
| `query` | string | ✅ | The search query to compare documents against. |
| `documents` | string[] | ✅ | List of documents to rerank. Minimum 1 document. |
| `top_n` | integer | ❌ | Number of most relevant documents to return. If not specified, returns all documents. |
| `return_documents` | boolean | ❌ | Whether to return the original documents in the response. Default: false. |

#### Response Format

```json
{
  "model": "jina-reranker-v1-base-en",
  "object": "list",
  "results": [
    {
      "index": 0,
      "relevance_score": 0.95,
      "document": {
        "text": "Machine learning is a subset of artificial intelligence..."
      }
    },
    {
      "index": 1,
      "relevance_score": 0.87,
      "document": {
        "text": "Deep learning uses neural networks..."
      }
    }
  ],
  "usage": {
    "prompt_tokens": 45,
    "total_tokens": 45
  }
}
```

#### Examples

**Python Example:**
```python
import requests

response = requests.post(
    "http://localhost:8090/v1/rerank",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "model": "jina-reranker-v1-base-en",
        "query": "What is machine learning?",
        "documents": [
            "Machine learning is a subset of artificial intelligence that enables computers to learn without being explicitly programmed.",
            "Deep learning uses neural networks with many layers.",
            "Statistics is the study of data collection and analysis."
        ],
        "top_n": 2
    }
)

result = response.json()
for item in result["results"]:
    print(f"Score: {item['relevance_score']:.3f} - {item['document']['text'][:50]}...")
```

**Jina Endpoint (Python):**
```python
import requests

# Jina-specific rerank request
response = requests.post(
    "http://localhost:8090/jina/v1/rerank",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "model": "jina-reranker-v1-base-en",
        "query": "What are the benefits of renewable energy?",
        "documents": [
            "Solar power generates electricity from sunlight.",
            "Coal mining provides jobs but harms the environment.",
            "Wind turbines convert wind energy into electricity.",
            "Fossil fuels are non-renewable and contribute to climate change."
        ],
        "top_n": 3,
        "return_documents": True
    }
)

result = response.json()
print("Reranked documents:")
for i, item in enumerate(result["results"]):
    print(f"{i+1}. Score: {item['relevance_score']:.3f}")
    print(f"   Text: {item['document']['text']}")
```

**Go Example:**
```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type RerankRequest struct {
    Model     string   `json:"model,omitempty"`
    Query     string   `json:"query"`
    Documents []string `json:"documents"`
    TopN      *int     `json:"top_n,omitempty"`
}

type RerankResponse struct {
    Model   string `json:"model"`
    Object  string `json:"object"`
    Results []struct {
        Index          int     `json:"index"`
        RelevanceScore float64 `json:"relevance_score"`
        Document       *struct {
            Text string `json:"text"`
        } `json:"document,omitempty"`
    } `json:"results"`
}

func main() {
    req := RerankRequest{
        Model: "jina-reranker-v1-base-en",
        Query: "What is artificial intelligence?",
        Documents: []string{
            "AI refers to machines performing tasks that typically require human intelligence.",
            "Machine learning is a subset of AI.",
            "Deep learning uses neural networks.",
        },
        TopN: &[]int{2}[0], // pointer to 2
    }

    jsonData, _ := json.Marshal(req)
    
    httpReq, _ := http.NewRequestWithContext(
        context.TODO(),
        "POST",
        "http://localhost:8090/v1/rerank",
        bytes.NewBuffer(jsonData),
    )
    httpReq.Header.Set("Authorization", "Bearer your-axonhub-api-key")
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("AH-Trace-Id", "trace-example-123")
    httpReq.Header.Set("AH-Thread-Id", "thread-example-abc")

    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var result RerankResponse
    json.Unmarshal(body, &result)

    for _, item := range result.Results {
        fmt.Printf("Score: %.3f, Text: %s\n", 
            item.RelevanceScore, 
            item.Document.Text[:50]+"...")
    }
}
```


| Provider               | Status     | Supported Models             | Compatible APIs |
| ---------------------- | ---------- | ---------------------------- | --------------- |
| **OpenAI**             | ✅ Done    | GPT-4, GPT-4o, GPT-5, etc.   | OpenAI, Anthropic, Gemini, Embedding |
| **Anthropic**          | ✅ Done    | Claude 3.5, Claude 3.0, etc. | OpenAI, Anthropic, Gemini |
| **Zhipu AI**           | ✅ Done    | GLM-4.5, GLM-4.5-air, etc.   | OpenAI, Anthropic, Gemini |
| **Moonshot AI (Kimi)** | ✅ Done    | kimi-k2, etc.                | OpenAI, Anthropic, Gemini |
| **DeepSeek**           | ✅ Done    | DeepSeek-V3.1, etc.          | OpenAI, Anthropic, Gemini |
| **ByteDance Doubao**   | ✅ Done    | doubao-1.6, etc.             | OpenAI, Anthropic, Gemini |
| **Gemini**             | ✅ Done    | Gemini 2.5, etc.             | OpenAI, Anthropic, Gemini |
| **Jina AI**            | ✅ Done    | Embeddings, Reranker, etc.   | Jina Embedding, Jina Rerank |
| **AWS Bedrock**        | 🔄 Testing | Claude on AWS                | OpenAI, Anthropic, Gemini |
| **Google Cloud**       | 🔄 Testing | Claude on GCP                | OpenAI, Anthropic, Gemini |

## Authentication

Both API formats use the same authentication system:

- **OpenAI API**: Use `Authorization: Bearer <your-api-key>` header
- **Anthropic API**: Use `X-API-Key: <your-api-key>` header

The API keys are managed through AxonHub's API Key management system and provide the same permissions regardless of which API format you use.

## Streaming Support

Both API formats support streaming responses:

### OpenAI Streaming
```go
// OpenAI SDK streaming
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Write a short story about AI"),
    },
    Model:  openai.ChatModel("claude-3-5-sonnet"),
    Stream: openai.Bool(true),
})
if err != nil {
    panic(err)
}

// Iterate over streaming chunks
for completion.Next() {
    chunk := completion.Current()
    if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
        fmt.Print(chunk.Choices[0].Delta.Content)
    }
}

if err := completion.Err(); err != nil {
    panic(err)
}
```

### Anthropic Streaming
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

Both API formats return standardized error responses:

### OpenAI Format Error
```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

### Anthropic Format Error
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

AxonHub supports **function tools** (custom function calling) across all API formats. However, provider-specific tools are **not supported**:

| Tool Type | Support Status | Notes |
| --------- | -------------- | ----- |
| **Function Tools** | ✅ Supported | Custom function definitions work across all providers |
| **Web Search** | ❌ Not Supported | Provider-specific (OpenAI, Anthropic, etc.) |
| **Code Interpreter** | ❌ Not Supported | Provider-specific (OpenAI, Anthropic, etc.) |
| **File Search** | ❌ Not Supported | Provider-specific |
| **Computer Use** | ❌ Not Supported | Anthropic-specific |

> **Note**: Only generic function tools that can be translated across providers are supported. Provider-specific tools like web search, code interpreter, and computer use require direct access to the provider's infrastructure and cannot be proxied through AxonHub.

## Best Practices

1. **Choose Your Preferred API**: Use the API format that best fits your application's needs and existing codebase
2. **Consistent Authentication**: Use the same API key across both API formats
3. **Model Selection**: Specify the target model explicitly in your requests
4. **Error Handling**: Implement proper error handling for both API formats
5. **Streaming**: Use streaming for better user experience with long responses
6. **Use Function Tools**: For tool calling, use generic function tools instead of provider-specific tools

## Migration Guide

### From OpenAI to AxonHub
```go
// Before: Direct OpenAI
client := openai.NewClient(
    option.WithAPIKey("openai-key"),
)

// After: AxonHub with OpenAI API
client := openai.NewClient(
    option.WithAPIKey("axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
)
// Your existing code continues to work!
```

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

