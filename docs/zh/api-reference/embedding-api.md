# Embedding API 参考文档

## 概述

AxonHub 通过 OpenAI、Jina AI 和 Cohere Embed v2 协议提供嵌入生成。Qwen3-VL-Embedding 的图像与图文混合输入使用 Cohere 原生端点。

## 主要优势

- **OpenAI 兼容性**：无需修改即可使用现有的 OpenAI SDK
- **Jina AI 支持**：原生支持 Jina 嵌入格式，适用于特殊用例
- **多种输入类型**：支持单文本、文本数组、Token 数组和多个 Token 数组
- **灵活的输出格式**：可选择浮点数组或 base64 编码的嵌入向量

## 支持的端点

**端点：**
- `POST /v1/embeddings` - OpenAI 兼容的嵌入 API
- `POST /jina/v1/embeddings` - Jina AI 专用的嵌入 API
- `POST /v2/embed` - Cohere Embed v2 原生文本、图像与混合嵌入 API

## Cohere Embed v2 与 Qwen 图像嵌入

在现有 OpenAI 类型渠道中手动添加 `cohere/embed` 自定义端点，配置真实服务的 Base URL、路径及 API Key。路径通常为 `/v2/embed`；IG1 服务使用 `/cohere/v2/embed`。路径非空时直接追加到端点 Base URL，因此使用服务根地址，避免重复添加 `/v1`。这些上游路径不改变 AxonHub 的客户端入口 `/v2/embed`。

对于未启用鉴权的自托管 vLLM 服务，渠道 API Key 可按 [vLLM 官方客户端示例](https://docs.vllm.ai/en/latest/examples/pooling/embed/#openai-embedding-client)填写 `EMPTY`。这是客户端占位值，AxonHub 仍会发送 `Authorization: Bearer EMPTY`；启用鉴权的服务应填写其真实 API Key。客户端调用 AxonHub 时仍需使用 AxonHub API Key。

手动添加模型名称，并在渠道的 Model Protocols 中为该模型固定 `cohere/embed`。模型实体使用已有 `embedding` 类型。渠道测试发送短文本，默认省略与部署相关的 `input_type`；目标 Cohere 托管服务要求该字段时，可通过现有渠道正文覆盖将其设为 `search_document`。图像能力需要使用真实图像另外验证。

以下示例适用于支持 `input_type: image` 的部署。Cohere 托管服务的 v3 及更新模型要求提供 `input_type`；未定义任务指令的 vLLM Qwen3-VL-Embedding 部署可以省略该字段。调用这类部署时删除示例中的 `input_type`，以实际服务契约为准。

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

将示例中的图像内容替换为真实 PNG 的 Base64。图文混合输入可在同一 `content` 数组中添加 `{"type":"text","text":"商品描述"}`；每个 `inputs` 项生成一个向量，数组与内容块的顺序保持不变。也接受上游支持的 `texts` 或 `images` 输入形式。`images` 中的每项必须是图像 data URI；远程 URL 应通过 `inputs[].content[].image_url.url` 传递。远程图像 URL 是否可用由目标服务决定，网关不会下载或预处理图像。

| 字段 | 说明 |
| --- | --- |
| `model` | 必填网关模型名或别名，出站替换为渠道映射后的模型名 |
| `texts` / `images` / `inputs` | 必须且只能提供其中一个非空输入数组；同时提供多种输入形式会被网关拒绝 |
| `input_type` | 非空值支持 `search_document`、`search_query`、`classification`、`clustering`、`image`；网关原样保留省略、`null` 和空字符串，不补默认值。是否必填及接受哪些值由目标部署决定 |
| `output_dimension` | 输出维度；支持的维度由模型及服务决定 |
| `embedding_types` | 请求的原生向量类型，如 `["float"]` |
| `truncate` | 按上游规则传递截断选项 |

响应保留原生 `embeddings` 对象、`id`、`texts`、`images`、`meta`、`usage` 及扩展字段。浮点向量位于 `embeddings.float`；其他向量类型仅在上游实际提供时返回。网关不转换向量编码或量化格式。向量数量须与输入批次一致，每个向量不能为空。

错误响应遵循系统的上游错误策略：原样模式保留上游 HTTP 状态及错误正文，隐藏和自定义模式使用配置文案。

用量优先使用 `usage.billed_units` 中的 token 字段；其中同时缺少 `input_tokens` 和 `image_tokens` 时使用 `meta.billed_units`。网关将同一来源的 `input_tokens + image_tokens` 计入输入 token 和总 token，用量不会跨来源拼接或重复累计；缺省分量按零处理，两者均缺失时用量保持未知。字段必须是非负整数，合计不得溢出。

[vLLM 的 Cohere 响应实现](https://github.com/vllm-project/vllm/blob/main/vllm/entrypoints/pooling/embed/serving.py)将总输入 token 分成这两个分量。例如 `input_tokens: 0, image_tokens: 44` 计为 44 token，`input_tokens: 26, image_tokens: 0` 计为 26 token。`billed_units.images` 表示图片张数，不参与 token 合计。原始响应和日志保留全部用量字段，现有费用计算继续使用 token 用量。

该协议仅使用 JSON 且不支持流式，`stream:true` 会被拒绝。原生请求只选择 `cohere/embed` 端点；与 `/v1/embeddings`、Jina 和 Gemini 嵌入格式之间不进行协议转换。`/v1/embeddings` 继续接受下述文本与 token 输入。

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
