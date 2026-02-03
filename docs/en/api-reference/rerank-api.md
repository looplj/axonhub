# Rerank API Reference

## Overview

AxonHub supports document reranking through Jina AI rerank API, allowing you to reorder documents based on relevance to a query. This is useful for improving search results, RAG (Retrieval-Augmented Generation) pipelines, and other applications that need to rank documents by relevance.

## Key Benefits

- **Improved Search Quality**: Rerank search results to surface the most relevant documents
- **RAG Enhancement**: Optimize document selection for retrieval-augmented generation
- **Flexible Integration**: Compatible with Jina AI rerank format

## Supported Endpoints

**Endpoints:**
- `POST /v1/rerank` - Jina-compatible rerank API (convenience endpoint)
- `POST /jina/v1/rerank` - Jina AI-specific rerank API

> **Note**: OpenAI does not provide a native rerank API. Both endpoints use Jina's rerank format.

## Request Format

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

## Response Format

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

## Authentication

The Rerank API uses Bearer token authentication:

- **Header**: `Authorization: Bearer <your-api-key>`

## Examples

### Python Example

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

### Jina Endpoint (Python)

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

### Go Example

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

## Best Practices

1. **Use Tracing Headers**: Include `AH-Trace-Id` and `AH-Thread-Id` headers for better observability
2. **Limit Results**: Use `top_n` to limit results and improve performance
3. **Return Documents**: Set `return_documents: true` only when you need the document text in the response
4. **Model Selection**: Choose the appropriate reranker model for your use case and language
