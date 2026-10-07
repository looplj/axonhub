# OpenAI API Reference

## Overview

AxonHub provides full support for the OpenAI API specification, allowing you to use any OpenAI-compatible client SDK to access models from multiple providers.

## Key Benefits

- **API Interoperability**: Use OpenAI Chat Completions API to call Anthropic, Gemini, and other supported models
- **Zero Code Changes**: Continue using your existing OpenAI client SDK without modification
- **Automatic Translation**: AxonHub automatically converts between API formats when needed
- **Provider Flexibility**: Access any supported AI provider using the OpenAI API format

## Supported Endpoints

### OpenAI Chat Completions API

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

**Capabilities:**
- ✅ `previous_response_id` passthrough is supported for continued Responses conversations on the same upstream channel
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

// Generate a response (previous_response_id passthrough on the same upstream channel)
params := responses.ResponseNewParams{
    Model: shared.ResponsesModel("gpt-4o"),
    PreviousResponseID: openai.String("resp_prev_123"),
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

### OpenAI Decisions API

AxonHub supports the native OpenAI Decisions contract as a dedicated,
non-streaming endpoint. The request is sent to `POST /v1/decisions` and keeps
the Decisions JSON shape instead of converting it to Chat Completions,
Responses, or SystemOne.

**Endpoint:**

- `POST /v1/decisions` - Evaluate shared input with typed questions

**Channel endpoint configuration:**

Add a custom endpoint with `api_format: openai/decisions` to a channel that
should receive Decisions requests. The endpoint may inherit the channel
`base_url`, or provide its own `base_url`. An optional `path` replaces the
default path and must start with `/`, for example `/v1/decisions`. Decisions
does not fall back to a Chat endpoint. A channel must have a compatible
`openai/decisions` endpoint to receive this request type.

```json
{
  "api_format": "openai/decisions",
  "base_url": "https://decisions.example.test/api",
  "path": "/v1/decisions"
}
```

**Request contract:**

The native request requires `model`, `input`, and a non-empty `questions`
array. `input` may be a non-empty string or an array of supported message
items. Text parts use `input_text`; image parts use `input_image`; a `message`
item may contain text or nested content parts. Question and answer objects are
kept as native JSON, including unknown fields and typed choice values.

```json
{
  "model": "example-decisions-model",
  "input": "A synthetic customer report says the package arrived intact.",
  "questions": [
    {
      "type": "choice",
      "name": "route",
      "instructions": "Choose the best synthetic routing label.",
      "choices": [
        { "value": "review", "description": "Needs manual review." },
        { "value": "complete", "description": "No follow-up is needed." }
      ]
    }
  ]
}
```

**Response and limits:**

- A successful response is returned as native Decisions JSON with an
  `answers` array. Answers may be typed results such as `choice`, `predicate`,
  or `score`, and may include a `refusal` answer.
- Decisions is non-streaming in phase one. Set `stream` to `false` or omit it.
  A request with `"stream": true` is rejected locally with an invalid request
  error. SSE clients and streaming SDK helpers are not supported for this
  endpoint.
- If the response contains `usage`, only `usage.input_tokens` is mapped to
  AxonHub prompt usage. Output, total, cache, and reasoning token fields are
  not mapped by this contract. A missing or null usage object remains absent.
- Prompt protection applies the configured prompt-protection rules for a
  request: a matching rule may mask the matched text or reject the request.
  Masking has a primary path and a conditional raw-replay path:
  - Primary path: a string `input`, and the `text` of `input[].content[]` parts
    whose `type` is `input_text`.
  - Conditional raw-replay path: only when body pass-through is enabled and
    applied, matched mask rules from the primary path are available, and the
    extra fields themselves match those rules and their scopes, masking also
    applies to standalone `input[].text` and string `message.content`. With
    pass-through disabled, only the primary string `input` and nested
    `input_text` fields are masked. On their own, the extra forms produce no
    primary match and are returned unchanged.
  Inline image values and unrelated provider fields are preserved, while trace
  data does not expose inline image bytes.
- The native Decisions request and response are not converted to SystemOne.
  SystemOne endpoint configuration is a separate protocol boundary and is not
  a fallback or compatibility path for `/v1/decisions`.

**Synthetic response example:**

```json
{
  "model": "example-decisions-model",
  "answers": [
    { "type": "choice", "name": "route", "choice": "complete" },
    { "type": "refusal", "name": "safety_check", "refusal": "not evaluated" }
  ],
  "usage": { "input_tokens": 12, "output_tokens": 99 }
}
```

**Synthetic error example:**

```json
{
  "error": {
    "message": "decision rate limit",
    "type": "rate_limit_error",
    "code": "rate_limit",
    "x_beta": null
  }
}
```

**Synthetic conditional prompt-protection example:**

A rule masking `secret-*` matches the nested `input_text` part, so the primary
path masks it. If body pass-through is enabled and applied, that matched mask
rule is available for raw replay, and the standalone `input[].text` also matches
the rule and its scope, the conditional raw-replay path masks it too:

```json
{
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "secret-nested" }]
    },
    { "type": "input_text", "text": "secret-standalone" }
  ]
}
```

Under these conditions, both instances of `secret-*` are masked. With
pass-through disabled, only the nested `input_text` field is masked. With only the standalone
`input[].text` present, no primary field matches, so the request is returned
unchanged.

## API Translation Capabilities

AxonHub automatically translates between API formats, enabling powerful scenarios:

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

### Use OpenAI SDK with Gemini Models
```go
// OpenAI SDK calling Gemini model
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Explain neural networks"),
    },
    Model: openai.ChatModel("gemini-2.5"),  // Gemini model
})

// Access response
responseText := completion.Choices[0].Message.Content
fmt.Println(responseText)
// AxonHub automatically translates OpenAI format → Gemini format
```

## Moderations API

AxonHub supports OpenAI-compatible content moderation through a standalone endpoint.

**Endpoints:**
- `POST /v1/moderations` - Classify text and/or image inputs with models such as `omni-moderation-latest`

**Request parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `input` | string \| string[] \| multimodal object[] | ✅ | Content to classify. Multimodal items use `{ "type": "text", "text": "..." }` or `{ "type": "image_url", "image_url": { "url": "..." } }`. |
| `model` | string | ❌ | Moderation model. If omitted, AxonHub defaults to `omni-moderation-latest`. The model must be available on a channel that exposes the `openai/moderations` endpoint. |

**Standalone request example:**

```json
{
  "model": "omni-moderation-latest",
  "input": "...text to classify goes here..."
}
```

Multimodal input (text + image) is also supported:

```json
{
  "model": "omni-moderation-latest",
  "input": [
    { "type": "text", "text": "...text to classify goes here..." },
    {
      "type": "image_url",
      "image_url": { "url": "https://example.com/image.png" }
    }
  ]
}
```

**Notes:**
- Chat Completions / Responses do not model the optional inline `moderation` request field. Use this standalone endpoint instead.
- OpenAI and several OpenAI-compatible channel types include `openai/moderations` in their default endpoint set. Channels that do not support upstream `/moderations` may return provider errors; remove or override the endpoint if needed.
- When body pass-through is enabled and inbound/outbound formats match, model mapping still patches the top-level `model` field for moderations requests.

## Codex Alpha Search API

AxonHub can proxy the Codex/CPA-compatible alpha search endpoint without interpreting the provider-specific search payload.

**Endpoint:**
- `POST /v1/alpha/search`

The request must include a `model` so AxonHub can select a channel. The remaining JSON, including `commands.search_query`, is forwarded unchanged apart from the mapped top-level `model`. The upstream response is returned unchanged. This endpoint is not part of the public OpenAI API; configure an `openai/alpha_search` channel endpoint only for an upstream that implements `/alpha/search` (for example CPA). The built-in Codex channel includes it by default; all other channels, including Fenno, OpenAI, and OpenAI Responses, must opt in explicitly.

## Embedding API

AxonHub provides comprehensive support for text and multimodal embedding generation through OpenAI-compatible API.

**Endpoints:**
- `POST /v1/embeddings` - OpenAI-compatible embedding API

**Supported Input Types:**
- Single text string
- Array of text strings
- Token arrays (integers)
- Multiple token arrays

**Supported Encoding Formats:**
- `float` - Default, returns embedding vectors as float arrays
- `base64` - Returns embeddings as base64-encoded strings

### Request Format

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

### Response Format

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

### Examples

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

## Models API

AxonHub provides an enhanced `/v1/models` endpoint that lists available models with optional extended metadata.

### Supported Endpoints

**Endpoints:**
- `GET /v1/models` - List available models

### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `include` | string | ❌ | Comma-separated list of fields to include, or "all" for all extended fields |

### Available Fields for Include

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name of the model |
| `description` | string | Model description |
| `context_length` | integer | Maximum context length in tokens |
| `max_output_tokens` | integer | Maximum output tokens |
| `modalities` | object | Model supported input/output types (input, output) |
| `capabilities` | object | Model capabilities (vision, tool_call, reasoning) |
| `pricing` | object | Pricing information (input, output, cache_read, cache_write) |
| `icon` | string | Model icon URL |
| `type` | string | Model type (chat, embedding, image, rerank, moderation, tts, stt) |

### Response Format (Basic - Default)

When called without the `include` parameter, the endpoint returns only basic fields:

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1686935002,
      "owned_by": "openai"
    }
  ]
}
```

**Fields:**
- `id` - Model identifier
- `object` - Always "model"
- `created` - Unix timestamp of model creation
- `owned_by` - Organization that owns the model

### Response Format (Extended)

When using `?include=all` or selective fields, the response includes extended metadata:

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1686935002,
      "owned_by": "openai",
      "name": "GPT-4",
      "description": "GPT-4 model with advanced reasoning capabilities",
      "context_length": 8192,
      "max_output_tokens": 4096,
      "modalities": {
        "input": ["text", "image"],
        "output": ["text"]
      },
      "capabilities": {
        "vision": false,
        "tool_call": true,
        "reasoning": true
      },
      "pricing": {
        "input": 30.0,
        "output": 60.0,
        "cache_read": 15.0,
        "cache_write": 30.0,
        "unit": "per_1m_tokens",
        "currency": "USD"
      },
      "icon": "https://example.com/icon.png",
      "type": "chat"
    }
  ]
}
```

**Extended Fields:**
- `name` - Human-readable model name
- `description` - Detailed model description
- `context_length` - Maximum tokens in context window
- `max_output_tokens` - Maximum tokens in response
- `modalities` - Model supported input/output modality types:
  - `input` - Array of supported input types (e.g. `text`, `image`, `audio`, `video`, `pdf`)
  - `output` - Array of supported output types (e.g. `text`)
- `capabilities` - Object with boolean flags:
  - `vision` - Supports image inputs
  - `tool_call` - Supports function calling
  - `reasoning` - Supports advanced reasoning
- `pricing` - Object with pricing details:
  - `input` - Input token price per 1M tokens
  - `output` - Output token price per 1M tokens
  - `cache_read` - Cache read price per 1M tokens
  - `cache_write` - Cache write price per 1M tokens
  - `unit` - Always "per_1m_tokens"
  - `currency` - Always "USD"
- `icon` - URL to model icon image
- `type` - Model category (chat, embedding, image, rerank, moderation, tts, stt)

### Examples

**Basic Request (Default):**
```bash
curl -s http://localhost:8090/v1/models \
  -H "Authorization: Bearer your-api-key" | jq
```

**Include All Extended Fields:**
```bash
curl -s "http://localhost:8090/v1/models?include=all" \
  -H "Authorization: Bearer your-api-key" | jq
```

**Selective Fields Only:**
```bash
curl -s "http://localhost:8090/v1/models?include=name,pricing" \
  -H "Authorization: Bearer your-api-key" | jq
```

**OpenAI SDK (Python):**
```python
import openai

client = openai.OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# Get models with extended metadata
models = client.models.list()
for model in models.data:
    print(f"Model: {model.id}")
    # Access extended fields if available
    if hasattr(model, 'name'):
        print(f"  Name: {model.name}")
    if hasattr(model, 'pricing'):
        print(f"  Input price: ${model.pricing.input}/1M tokens")
```

### Error Responses

**401 Unauthorized - Invalid API Key:**
```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

**500 Internal Server Error:**
```json
{
  "error": {
    "message": "Internal server error",
    "type": "internal_error",
    "code": "internal_error"
  }
}
```

### Field Availability Note

> **Note:** Extended fields are only populated if the model has ModelCard data configured in the database. Models without ModelCard data will return `null` for extended fields.

## Authentication

The OpenAI API format uses Bearer token authentication:

- **Header**: `Authorization: Bearer <your-api-key>`

The API keys are managed through AxonHub's API Key management system.

## Streaming Support

OpenAI API format supports streaming responses:

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

## Error Handling

OpenAI format error responses:

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

## Tool Support

AxonHub supports **function tools** (custom function calling) through the OpenAI API format. However, provider-specific tools are **not supported**:

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
