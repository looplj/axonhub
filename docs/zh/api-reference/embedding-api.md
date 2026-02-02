# Embedding API 参考文档

## 概述

AxonHub 通过 OpenAI 兼容和 Jina AI 专用 API 提供全面的文本和多模态嵌入生成支持。

## 主要优势

- **OpenAI 兼容性**：无需修改即可使用现有的 OpenAI SDK
- **Jina AI 支持**：原生支持 Jina 嵌入格式，适用于特殊用例
- **多种输入类型**：支持单文本、文本数组、Token 数组和多个 Token 数组
- **灵活的输出格式**：可选择浮点数组或 base64 编码的嵌入向量

## 支持的端点

**端点：**
- `POST /v1/embeddings` - OpenAI 兼容的嵌入 API
- `POST /jina/v1/embeddings` - Jina AI 专用的嵌入 API

## 请求格式

```json
{
  "input": "要嵌入的文本",
  "model": "text-embedding-3-small",
  "encoding_format": "float",
  "dimensions": 1536,
  "user": "用户ID"
}
```

**参数：**

| 参数 | 类型 | 必填 | 描述 |
|-----------|------|----------|-------------|
| `input` | string \| string[] \| number[] \| number[][] | ✅ | 要嵌入的文本。可以是单个字符串、字符串数组、Token 数组或多个 Token 数组。 |
| `model` | string | ✅ | 用于生成嵌入的模型。 |
| `encoding_format` | string | ❌ | 返回嵌入的格式。可选 `float` 或 `base64`。默认：`float`。 |
| `dimensions` | integer | ❌ | 输出嵌入的维度数。 |
| `user` | string | ❌ | 终端用户的唯一标识符。 |

**Jina 专用参数：**

| 参数 | 类型 | 必填 | 描述 |
|-----------|------|----------|-------------|
| `task` | string | ❌ | Jina 嵌入的任务类型。选项：`text-matching`、`retrieval.query`、`retrieval.passage`、`separation`、`classification`、`none`。 |

## 响应格式

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

## 示例

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

print(response.data[0].embedding[:5])  # 前 5 个维度
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

    fmt.Printf("嵌入维度: %d\n", len(embedding.Data[0].Embedding))
    fmt.Printf("前 5 个值: %v\n", embedding.Data[0].Embedding[:5])
}
```

### 多个文本

```python
response = client.embeddings.create(
    input=["Hello, world!", "How are you?"],
    model="text-embedding-3-small"
)

for i, data in enumerate(response.data):
    print(f"文本 {i}: {data.embedding[:3]}...")
```

### Jina 专用任务

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

## 认证

Embedding API 使用 Bearer Token 认证：

- **请求头**: `Authorization: Bearer <your-api-key>`

API 密钥通过 AxonHub 的 API 密钥管理系统进行管理。

## 最佳实践

1. **使用追踪请求头**：包含 `AH-Trace-Id` 和 `AH-Thread-Id` 请求头以获得更好的可观测性
2. **批量请求**：嵌入多个文本时，在单个请求中发送以提高性能
3. **选择合适的维度**：如果不需要完整的维度，使用 `dimensions` 参数减少嵌入大小
4. **选择合适的编码**：如果需要在网络上传输嵌入，使用 `base64` 编码以减少负载大小
5. **Jina 任务类型**：使用 Jina 嵌入时，根据用例选择合适的 `task` 类型以优化检索质量

## 相关资源

- [OpenAI API](openai-api.md)
- [Rerank API](rerank-api.md)
