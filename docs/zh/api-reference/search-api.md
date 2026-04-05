# 搜索 API 参考

## 概述

AxonHub 提供统一的 Web 搜索 API，通过单一端点聚合多个搜索提供商（Tavily、Brave Search、Exa）。搜索 API 复用现有的编排器基础设施——负载均衡、重试/故障转移、熔断器、连接追踪和请求持久化——为 LLM 提供可靠的搜索结果。

## 核心优势

- **统一接口**：一个 API 格式对接多个搜索提供商（Tavily、Brave、Exa）
- **提供商无关**：切换搜索提供商无需修改客户端代码
- **LLM 优化**：结果格式适合语言模型直接消费
- **完整编排支持**：负载均衡、故障转移、熔断器和请求追踪

## 支持的端点

**端点：**
- `POST /v1/search` - 统一搜索 API

## 请求格式

```json
{
  "query": "量子计算最新进展",
  "model": "__search",
  "max_results": 5,
  "allowed_domains": ["arxiv.org", "nature.com"],
  "blocked_domains": ["pinterest.com"],
  "extra_body": {}
}
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `query` | string | ✅ | 搜索查询字符串。 |
| `model` | string | ❌ | 用于路由的搜索模型。默认：`__search`。参见[模型路由](#模型路由)。 |
| `max_results` | integer | ❌ | 返回的最大结果数。默认：5，最大：20。 |
| `allowed_domains` | string[] | ❌ | 仅在这些域名中搜索（例如 `["arxiv.org"]`）。 |
| `blocked_domains` | string[] | ❌ | 从结果中排除这些域名（例如 `["pinterest.com"]`）。 |
| `extra_body` | object | ❌ | 直接透传到提供商的特定参数（例如 Tavily 的 `search_depth`、`topic`、`include_answer`）。 |

## 响应格式

```json
{
  "query": "量子计算最新进展",
  "results": [
    {
      "title": "MIT 量子计算突破",
      "url": "https://news.mit.edu/2026/quantum-breakthrough",
      "content": "MIT 研究人员取得了重大里程碑...",
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

**响应字段：**

| 字段 | 类型 | 描述 |
|------|------|------|
| `query` | string | 原始搜索查询。 |
| `results` | array | 搜索结果列表。 |
| `results[].title` | string | 搜索结果标题。 |
| `results[].url` | string | 搜索结果 URL。 |
| `results[].content` | string | 结果内容摘要/片段。 |
| `results[].score` | float | 相关性分数（0-1）。并非所有提供商都返回此字段。 |
| `results[].published_date` | string | 内容发布日期。 |
| `results[].favicon` | string | 来源网站的 Favicon URL。 |
| `response_time` | float | 响应时间（秒）。 |
| `usage.credits` | integer | 消耗的 API 积分数。 |

## 模型路由

`model` 字段控制由哪个搜索提供商处理请求：

| 模型 | 描述 |
|------|------|
| `__search` | 默认 — 路由到任意可用的搜索渠道 |
| `__tavily_search` | 指定路由到 Tavily 提供商 |
| `__brave_search` | 指定路由到 Brave Search 提供商 |
| `__exa_search` | 指定路由到 Exa 提供商 |

## 支持的提供商

| 提供商 | 渠道类型 | 特点 | 定价 |
|--------|----------|------|------|
| **Tavily** | `search_tavily` | LLM 优化、引用就绪、AI 生成答案 | ~$0.008/次 |
| **Brave Search** | `search_brave` | 独立索引、隐私优先、丰富本地数据 | ~$0.003/次 |
| **Exa** | `search_exa` | 神经语义搜索、学术/研究导向 | ~$0.005/次 |

## 认证

搜索 API 使用 Bearer 令牌认证：

- **请求头**：`Authorization: Bearer <your-api-key>`

## 渠道配置

搜索提供商作为渠道类型在 AxonHub 中配置，与 LLM 渠道完全一致：

| 渠道类型 | Base URL | 认证头 |
|----------|----------|--------|
| `search_tavily` | `https://api.tavily.com` | `Authorization: Bearer <key>` |
| `search_brave` | `https://api.search.brave.com` | `X-Subscription-Token: <key>` |
| `search_exa` | `https://api.exa.ai` | `Authorization: Bearer <key>` |

**与 LLM 渠道的差异：**
- **无模型映射**：搜索使用固定模型命名（`__search`、`__tavily_search` 等）
- **无配额限制**：搜索请求不受 API Key 配额限制
- **无 Prompt 注入**：搜索不涉及系统提示

## 示例

### Python 示例

```python
import requests

response = requests.post(
    "http://localhost:8090/v1/search",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "query": "量子计算最新进展",
        "model": "__search",
        "max_results": 5
    }
)

results = response.json()
for r in results["results"]:
    print(f"{r['title']}: {r['url']}")
    print(f"  {r['content'][:100]}...")
```

### Python 域名过滤

```python
import requests

response = requests.post(
    "http://localhost:8090/v1/search",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "query": "Transformer 架构",
        "model": "__search",
        "max_results": 10,
        "allowed_domains": ["arxiv.org", "nature.com", "science.org"],
        "blocked_domains": ["pinterest.com", "quora.com"]
    }
)

results = response.json()
print(f"找到 {len(results['results'])} 条结果，耗时 {results['response_time']:.2f} 秒")
```

### Python 透传提供商特定参数

```python
import requests

# 使用 Tavily 并透传额外参数
response = requests.post(
    "http://localhost:8090/v1/search",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "query": "AI 监管 2026",
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

### Go 示例

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
		Query:      "量子计算最新进展",
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

	fmt.Printf("找到 %d 条结果，耗时 %.2f 秒\n", len(result.Results), result.ResponseTime)
	for _, r := range result.Results {
		fmt.Printf("- %s: %s\n", r.Title, r.URL)
	}
}
```

### cURL 示例

```bash
curl -X POST http://localhost:8090/v1/search \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "量子计算最新进展",
    "model": "__search",
    "max_results": 5
  }'
```

## 错误处理

错误遵循标准的 OpenAI 兼容错误格式：

```json
{
  "error": {
    "message": "Invalid search query: query is required",
    "type": "invalid_request_error",
    "code": "invalid_query"
  }
}
```

## 最佳实践

1. **使用追踪头**：添加 `AH-Trace-Id` 和 `AH-Thread-Id` 头以获得更好的可观测性
2. **限制结果数量**：使用 `max_results` 控制结果数量并提高响应速度
3. **域名过滤**：使用 `allowed_domains` 和 `blocked_domains` 将结果聚焦于可信来源
4. **提供商选择**：使用 `__search` 进行自动提供商选择，或指定提供商（如 `__tavily_search`）以通过 `extra_body` 使用提供商特定功能
5. **故障转移**：配置多个搜索渠道以实现提供商不可用时的自动故障转移
