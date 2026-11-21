# Embedding API Reference

## Overview

AxonHub provides embeddings through OpenAI, Jina AI, and Cohere Embed v2 protocols. Image and mixed text/image inputs for Qwen3-VL-Embedding use the native Cohere endpoint.

## Key Benefits

- **OpenAI Compatibility**: Use existing OpenAI SDKs without modification
- **Jina AI Support**: Native Jina embedding format support for specialized use cases
- **Multiple Input Types**: Support for single text, text arrays, token arrays, and multiple token arrays
- **Flexible Output Formats**: Choose between float arrays or base64-encoded embeddings

## Supported Endpoints

**Endpoints:**
- `POST /v1/embeddings` - OpenAI-compatible embedding API
- `POST /jina/v1/embeddings` - Jina AI-specific embedding API
- `POST /v2/embed` - Native Cohere Embed v2 text, image, and mixed embeddings

## Cohere Embed v2 and Qwen Image Embeddings

Manually add a `cohere/embed` custom endpoint to an existing OpenAI channel, with the real service Base URL, path, and API key. The usual path is `/v2/embed`; IG1 uses `/cohere/v2/embed`. An explicit path is appended directly to the endpoint Base URL, so use the service root to avoid a duplicated `/v1`. These upstream paths do not change the AxonHub client route `/v2/embed`.

For a self-hosted vLLM service without authentication, the channel API key can be `EMPTY`, following the [official vLLM client example](https://docs.vllm.ai/en/latest/examples/pooling/embed/#openai-embedding-client). This is a client placeholder: AxonHub still sends `Authorization: Bearer EMPTY`. Use the service's real API key when authentication is enabled. Clients calling AxonHub still need an AxonHub API key.

Add the model name manually and pin it to `cohere/embed` in the channel's Model Protocols. Model entities use the existing `embedding` type. Channel tests send short text and omit the deployment-specific `input_type` by default. For a hosted Cohere service that requires this field, set it to `search_document` through the existing channel body overrides. Verify image support separately with a real image.

The following example applies to deployments that support `input_type: image`. Cohere's hosted service requires `input_type` for v3 and newer models; vLLM Qwen3-VL-Embedding deployments without task instructions can omit it. Remove `input_type` from the example when calling such a deployment, according to its actual API contract.

```bash
curl -X POST "http://localhost:8090/v2/embed" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-vl-embedding-8b",
    "inputs": [{
      "content": [{
        "type": "image_url",
        "image_url": {"url": "data:image/png;base64,<BASE64_ENCODED_IMAGE_DATA>"}
      }]
    }],
    "input_type": "image",
    "output_dimension": 2048,
    "embedding_types": ["float"]
  }'
```

Replace the image placeholder with real PNG Base64 data. For mixed input, add a `{"type":"text","text":"Product description"}` block to the same `content` array. Each `inputs` item produces one vector, preserving batch and content order. The endpoint also accepts the upstream's `texts` or `images` input forms. `images` entries must be image data URIs; send remote URLs through `inputs[].content[].image_url.url`. Remote image URL support depends on the target service; the gateway does not download or preprocess images.

| Field | Description |
| --- | --- |
| `model` | Required gateway model or alias, replaced by the mapped upstream model |
| `texts` / `images` / `inputs` | Provide exactly one nonempty input array; combining input forms is rejected by the gateway |
| `input_type` | Supported nonempty values: `search_document`, `search_query`, `classification`, `clustering`, or `image`. Omission, `null`, and an empty string are preserved without a default; whether the field is required and which values are accepted depend on the target deployment |
| `output_dimension` | Output dimensions supported by the model and service |
| `embedding_types` | Native vector types, such as `["float"]` |
| `truncate` | Preserved according to the upstream truncation rules |

Responses preserve the native `embeddings` object, `id`, `texts`, `images`, `meta`, `usage`, and extension fields. Float vectors are under `embeddings.float`; other vector types are returned only when supplied by the upstream. The gateway does not change vector encoding or quantization. Vector counts must match the input batch and each vector must be nonempty.

Error responses follow the system's upstream error policy: passthrough mode preserves the upstream HTTP status and error body; hidden and custom modes use the configured message.

Usage uses token fields from `usage.billed_units` first, selecting `meta.billed_units` only when both `input_tokens` and `image_tokens` are absent there. The gateway records `input_tokens + image_tokens` from that one source as prompt and total tokens, without combining sources or counting duplicate values. A missing component counts as zero; when both are absent, usage remains unknown. Values must be nonnegative integers and their sum must not overflow.

[vLLM's Cohere response implementation](https://github.com/vllm-project/vllm/blob/main/vllm/entrypoints/pooling/embed/serving.py) partitions total input tokens into these two components. For example, `input_tokens: 0, image_tokens: 44` records 44 tokens, while `input_tokens: 26, image_tokens: 0` records 26 tokens. `billed_units.images` counts images and is not included in the token sum. Raw responses and logs preserve all usage fields; existing cost calculation uses token usage.

This JSON protocol is non-streaming and rejects `stream:true`. Native requests select only `cohere/embed` endpoints; there is no conversion to OpenAI, Jina, or Gemini embedding protocols. `/v1/embeddings` retains the text and token inputs described below.

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
