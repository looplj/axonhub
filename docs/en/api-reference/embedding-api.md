# Embedding API Reference

## Overview

AxonHub provides comprehensive support for text and multimodal embedding generation through OpenAI-compatible and Jina AI-specific APIs.

## Key Benefits

- **OpenAI Compatibility**: Use existing OpenAI SDKs without modification
- **Jina AI Support**: Native Jina embedding format support for specialized use cases
- **Multiple Input Types**: Support for single text, text arrays, token arrays, and multiple token arrays
- **Flexible Output Formats**: Choose between float arrays or base64-encoded embeddings

## Supported Endpoints

**Endpoints:**
- `POST /v1/embeddings` - OpenAI-compatible embedding API
- `POST /jina/v1/embeddings` - Jina AI-specific embedding API

## Request Format

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

## Response Format

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

## Examples

### OpenAI SDK (Python)

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

### OpenAI SDK (Go)

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

### Multiple Texts

```python
response = client.embeddings.create(
    input=["Hello, world!", "How are you?"],
    model="text-embedding-3-small"
)

for i, data in enumerate(response.data):
    print(f"Text {i}: {data.embedding[:3]}...")
```

### Jina-Specific Task

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

## Authentication

The Embedding API uses Bearer token authentication:

- **Header**: `Authorization: Bearer <your-api-key>`

The API keys are managed through AxonHub's API Key management system.

## Best Practices

1. **Use Tracing Headers**: Include `AH-Trace-Id` and `AH-Thread-Id` headers for better observability
2. **Batch Requests**: When embedding multiple texts, send them in a single request for better performance
3. **Choose Appropriate Dimensions**: Use the `dimensions` parameter to reduce embedding size if full dimensionality isn't needed
4. **Select Proper Encoding**: Use `base64` encoding if you need to transmit embeddings over the network to reduce payload size
5. **Jina Task Types**: When using Jina embeddings, select the appropriate `task` type for your use case to optimize retrieval quality

## Related Resources

- [OpenAI API](openai-api.md)
- [Rerank API](rerank-api.md)
