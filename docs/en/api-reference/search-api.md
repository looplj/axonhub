# Search API Reference

## Overview

AxonHub provides a unified Web Search API that aggregates multiple search providers (Tavily, Brave Search, Exa) through a single endpoint. The Search API reuses the existing orchestrator infrastructure — load balancing, retry/failover, circuit breaker, connection tracing, and request persistence — to deliver reliable search results optimized for LLM consumption.

## Key Benefits

- **Unified Interface**: One API format for multiple search providers (Tavily, Brave, Exa)
- **Provider Agnostic**: Switch search providers without changing client code
- **LLM Optimized**: Results are formatted for easy consumption by language models
- **Full Orchestrator Support**: Load balancing, failover, circuit breaker, and request tracing

## Supported Endpoint

**Endpoint:**
- `POST /v1/search` - Unified search API

## Request Format

```json
{
  "query": "latest developments in quantum computing",
  "model": "__search",
  "max_results": 5,
  "allowed_domains": ["arxiv.org", "nature.com"],
  "blocked_domains": ["pinterest.com"],
  "extra_body": {}
}
```

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | ✅ | The search query string. |
| `model` | string | ❌ | Search model for routing. Default: `__search`. See [Model Routing](#model-routing). |
| `max_results` | integer | ❌ | Maximum number of results to return. Default: 5, Max: 20. |
| `allowed_domains` | string[] | ❌ | Only search within these domains (e.g., `["arxiv.org"]`). |
| `blocked_domains` | string[] | ❌ | Exclude these domains from results (e.g., `["pinterest.com"]`). |
| `extra_body` | object | ❌ | Provider-specific parameters passed through directly (e.g., `search_depth`, `topic`, `include_answer` for Tavily). |

## Response Format

```json
{
  "query": "latest developments in quantum computing",
  "results": [
    {
      "title": "Quantum Computing Breakthrough at MIT",
      "url": "https://news.mit.edu/2026/quantum-breakthrough",
      "content": "Researchers at MIT have achieved a significant milestone...",
      "score": 0.95,
      "published_date": "2026-02-15",
      "favicon": "https://news.mit.edu/favicon.ico"
    }
  ],
  "response_time": 1.23,
  "usage": {
    "credits": 1
  }
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `query` | string | The original search query. |
| `results` | array | List of search results. |
| `results[].title` | string | Title of the search result. |
| `results[].url` | string | URL of the search result. |
| `results[].content` | string | Summary/snippet of the result content. |
| `results[].score` | float | Relevance score (0-1). Not all providers return this. |
| `results[].published_date` | string | Publication date of the content. |
| `results[].favicon` | string | Favicon URL of the source website. |
| `response_time` | float | Response time in seconds. |
| `usage.credits` | integer | Number of API credits consumed. |

## Model Routing

The `model` field controls which search provider handles the request:

| Model | Description |
|-------|-------------|
| `__search` | Default — routes to any available search channel |
| `__tavily_search` | Routes specifically to Tavily provider |
| `__brave_search` | Routes specifically to Brave Search provider |
| `__exa_search` | Routes specifically to Exa provider |

## Supported Providers

| Provider | Channel Type | Features | Pricing |
|----------|-------------|----------|---------|
| **Tavily** | `search_tavily` | LLM-optimized, citation-ready, AI-generated answers | ~$0.008/req |
| **Brave Search** | `search_brave` | Independent index, privacy-first, rich local data | ~$0.003/req |
| **Exa** | `search_exa` | Neural semantic search, academic/research-oriented | ~$0.005/req |

## Authentication

The Search API uses Bearer token authentication:

- **Header**: `Authorization: Bearer <your-api-key>`

## Channel Configuration

Search providers are configured as channel types in AxonHub, just like LLM channels:

| Channel Type | Base URL | Auth Header |
|-------------|----------|-------------|
| `search_tavily` | `https://api.tavily.com` | `Authorization: Bearer <key>` |
| `search_brave` | `https://api.search.brave.com` | `X-Subscription-Token: <key>` |
| `search_exa` | `https://api.exa.ai` | `Authorization: Bearer <key>` |

**Key Differences from LLM Channels:**
- **No Model Mapping**: Search uses fixed model naming (`__search`, `__tavily_search`, etc.)
- **No Quota**: Search requests are not subject to API Key quota limits
- **No Prompt Injection**: Search does not involve system prompts

## Examples

### Python Example

```python
import requests

response = requests.post(
    "http://localhost:8090/v1/search",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "query": "latest developments in quantum computing",
        "model": "__search",
        "max_results": 5
    }
)

results = response.json()
for r in results["results"]:
    print(f"{r['title']}: {r['url']}")
    print(f"  {r['content'][:100]}...")
```

### Python with Domain Filtering

```python
import requests

response = requests.post(
    "http://localhost:8090/v1/search",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "query": "transformer architecture",
        "model": "__search",
        "max_results": 10,
        "allowed_domains": ["arxiv.org", "nature.com", "science.org"],
        "blocked_domains": ["pinterest.com", "quora.com"]
    }
)

results = response.json()
print(f"Found {len(results['results'])} results in {results['response_time']:.2f}s")
```

### Python with Provider-Specific Parameters

```python
import requests

# Use Tavily with extra parameters
response = requests.post(
    "http://localhost:8090/v1/search",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "query": "AI regulation 2026",
        "model": "__tavily_search",
        "max_results": 5,
        "extra_body": {
            "search_depth": "advanced",
            "topic": "news",
            "include_answer": True
        }
    }
)
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

type SearchRequest struct {
	Query          string   `json:"query"`
	Model          string   `json:"model,omitempty"`
	MaxResults     *int     `json:"max_results,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
}

type SearchResponse struct {
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	ResponseTime float64        `json:"response_time"`
}

type SearchResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Content       string   `json:"content"`
	Score         *float64 `json:"score,omitempty"`
	PublishedDate string   `json:"published_date,omitempty"`
}

func main() {
	maxResults := 5
	req := SearchRequest{
		Query:      "latest developments in quantum computing",
		Model:      "__search",
		MaxResults: &maxResults,
	}

	jsonData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequestWithContext(
		context.TODO(),
		"POST",
		"http://localhost:8090/v1/search",
		bytes.NewBuffer(jsonData),
	)
	httpReq.Header.Set("Authorization", "Bearer your-axonhub-api-key")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("AH-Trace-Id", "trace-search-123")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result SearchResponse
	json.Unmarshal(body, &result)

	fmt.Printf("Found %d results in %.2fs\n", len(result.Results), result.ResponseTime)
	for _, r := range result.Results {
		fmt.Printf("- %s: %s\n", r.Title, r.URL)
	}
}
```

### cURL Example

```bash
curl -X POST http://localhost:8090/v1/search \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "latest developments in quantum computing",
    "model": "__search",
    "max_results": 5
  }'
```

## Error Handling

Errors follow the standard OpenAI-compatible error format:

```json
{
  "error": {
    "message": "Invalid search query: query is required",
    "type": "invalid_request_error",
    "code": "invalid_query"
  }
}
```

## Best Practices

1. **Use Tracing Headers**: Include `AH-Trace-Id` and `AH-Thread-Id` headers for better observability
2. **Limit Results**: Use `max_results` to control the number of results and improve response time
3. **Domain Filtering**: Use `allowed_domains` and `blocked_domains` to focus results on trusted sources
4. **Provider Selection**: Use `__search` for automatic provider selection, or specify a provider (e.g., `__tavily_search`) when you need provider-specific features via `extra_body`
5. **Failover**: Configure multiple search channels for automatic failover if one provider is unavailable
