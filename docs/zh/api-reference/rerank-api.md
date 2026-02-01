# 重排序 API 参考

## 概述

AxonHub 通过 Jina AI 重排序 API 支持文档重排序，允许您根据与查询的相关性重新排列文档。这对于改善搜索结果、RAG（检索增强生成）管道以及其他需要按相关性对文档进行排序的应用程序非常有用。

## 核心优势

- **提升搜索质量**：重新排序搜索结果，使最相关的文档排在前面
- **增强 RAG**：优化检索增强生成的文档选择
- **灵活集成**：兼容 Jina AI 重排序格式

## 支持的端点

**端点：**
- `POST /v1/rerank` - Jina 兼容重排序 API（便捷端点）
- `POST /jina/v1/rerank` - Jina AI 特定重排序 API

> **注意**：OpenAI 不提供原生重排序 API。两个端点都使用 Jina 的重排序格式。

## 请求格式

```json
{
  "model": "jina-reranker-v1-base-en",
  "query": "什么是机器学习？",
  "documents": [
    "机器学习是人工智能的一个子集...",
    "深度学习使用神经网络...",
    "统计学涉及数据收集和分析..."
  ],
  "top_n": 2,
  "return_documents": true
}
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `model` | string | ✅ | 用于重排序的模型（例如 `jina-reranker-v1-base-en`）。 |
| `query` | string | ✅ | 用于比较文档的搜索查询。 |
| `documents` | string[] | ✅ | 要重排序的文档列表。最少 1 个文档。 |
| `top_n` | integer | ❌ | 返回最相关文档的数量。如果未指定，返回所有文档。 |
| `return_documents` | boolean | ❌ | 是否在响应中返回原始文档。默认：false。 |

## 响应格式

```json
{
  "model": "jina-reranker-v1-base-en",
  "object": "list",
  "results": [
    {
      "index": 0,
      "relevance_score": 0.95,
      "document": {
        "text": "机器学习是人工智能的一个子集..."
      }
    },
    {
      "index": 1,
      "relevance_score": 0.87,
      "document": {
        "text": "深度学习使用神经网络..."
      }
    }
  ],
  "usage": {
    "prompt_tokens": 45,
    "total_tokens": 45
  }
}
```

## 认证

重排序 API 使用 Bearer 令牌认证：

- **请求头**：`Authorization: Bearer <your-api-key>`

## 示例

### Python 示例

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
        "query": "什么是机器学习？",
        "documents": [
            "机器学习是人工智能的一个子集，使计算机能够在没有明确编程的情况下学习。",
            "深度学习使用具有许多层的神经网络。",
            "统计学是数据收集和分析的研究。"
        ],
        "top_n": 2
    }
)

result = response.json()
for item in result["results"]:
    print(f"分数: {item['relevance_score']:.3f} - {item['document']['text'][:50]}...")
```

### Jina 端点 (Python)

```python
import requests

# Jina 特定的重排序请求
response = requests.post(
    "http://localhost:8090/jina/v1/rerank",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "model": "jina-reranker-v1-base-en",
        "query": "可再生能源的好处是什么？",
        "documents": [
            "太阳能从阳光中产生电力。",
            "煤矿开采提供就业但损害环境。",
            "风力涡轮机将风能转化为电力。",
            "化石燃料是不可再生的并导致气候变化。"
        ],
        "top_n": 3,
        "return_documents": True
    }
)

result = response.json()
print("重排序文档:")
for i, item in enumerate(result["results"]):
    print(f"{i+1}. 分数: {item['relevance_score']:.3f}")
    print(f"   文本: {item['document']['text']}")
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
        Query: "什么是人工智能？",
        Documents: []string{
            "人工智能指的是机器执行通常需要人类智能的任务。",
            "机器学习是人工智能的一个子集。",
            "深度学习使用神经网络。",
        },
        TopN: &[]int{2}[0], // 指向 2 的指针
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
        fmt.Printf("分数: %.3f, 文本: %s\n",
            item.RelevanceScore,
            item.Document.Text[:50]+"...")
    }
}
```

## 最佳实践

1. **使用追踪头**：添加 `AH-Trace-Id` 和 `AH-Thread-Id` 头以获得更好的可观测性
2. **限制结果数量**：使用 `top_n` 限制结果数量以提高性能
3. **返回文档**：仅在需要响应中包含文档文本时设置 `return_documents: true`
4. **模型选择**：根据您的用例和语言选择合适的重排序模型
