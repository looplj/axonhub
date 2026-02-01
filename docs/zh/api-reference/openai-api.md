# OpenAI API 参考

## 概述

AxonHub 完全支持 OpenAI API 规范，允许您使用任何 OpenAI 兼容的客户端 SDK 访问多个提供商的模型。

## 核心优势

- **API 互操作性**：使用 OpenAI Chat Completions API 调用 Anthropic、Gemini 和其他支持的模型
- **零代码变更**：继续使用现有的 OpenAI 客户端 SDK，无需修改
- **自动转换**：AxonHub 在需要时自动在 API 格式之间进行转换
- **提供商灵活性**：使用 OpenAI API 格式访问任何支持的 AI 提供商

## 支持的端点

### OpenAI Chat Completions API

**端点：**
- `POST /v1/chat/completions` - 文本生成
- `GET /v1/models` - 列出可用模型

**示例请求：**
```go
import (
    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)

// 使用 AxonHub 配置创建 OpenAI 客户端
client := openai.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
    
)

// 使用 OpenAI API 格式调用 Anthropic 模型
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("Hello, Claude!"),
    },
    Model: openai.ChatModel("claude-3-5-sonnet"),
},
    option.WithHeader("AH-Trace-Id", "trace-example-123"),
    option.WithHeader("AH-Thread-Id", "thread-example-abc"))
if err != nil {
    // 适当处理错误
    panic(err)
}

// 访问响应内容
responseText := completion.Choices[0].Message.Content
fmt.Println(responseText)
```

### OpenAI Responses API

AxonHub 提供对 OpenAI Responses API 的部分支持。该 API 为单轮交互提供了简化的接口。

**端点：**
- `POST /v1/responses` - 生成响应

**限制：**
- ❌ **不支持** `previous_response_id` - 对话历史需要在客户端管理
- ✅ 基本响应生成完全可用
- ✅ 支持流式响应

**示例请求：**
```go
import (
    "context"
    "fmt"

    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
    "github.com/openai/openai-go/v3/responses"
    "github.com/openai/openai-go/v3/shared"
)

// 使用 AxonHub 配置创建 OpenAI 客户端
client := openai.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
)

ctx := context.Background()

// 生成响应（不支持 previous_response_id）
params := responses.ResponseNewParams{
    Model: shared.ResponsesModel("gpt-4o"),
    Input: responses.ResponseNewParamsInputUnion{
        OfString: openai.String("你好，最近怎么样？"),
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

**示例：流式响应**
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
        OfString: openai.String("给我讲一个关于机器人的短故事。"),
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
        fmt.Print(event.Delta) // 边传输边打印
    }
}

if err := stream.Err(); err != nil {
    panic(err)
}

fmt.Println("\n完整响应:", fullContent.String())
```

## API 转换能力

AxonHub 自动在 API 格式之间进行转换，实现以下强大场景：

### 使用 OpenAI SDK 调用 Anthropic 模型
```go
// OpenAI SDK 调用 Anthropic 模型
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("请解释什么是机器学习"),
    },
    Model: openai.ChatModel("claude-3-5-sonnet"),  // Anthropic 模型
})

// 访问响应
responseText := completion.Choices[0].Message.Content
fmt.Println(responseText)
// AxonHub 自动转换 OpenAI 格式 → Anthropic 格式
```

### 使用 OpenAI SDK 调用 Gemini 模型
```go
// OpenAI SDK 调用 Gemini 模型
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("解释神经网络"),
    },
    Model: openai.ChatModel("gemini-2.5"),  // Gemini 模型
})

// 访问响应
responseText := completion.Choices[0].Message.Content
fmt.Println(responseText)
// AxonHub 自动转换 OpenAI 格式 → Gemini 格式
```

## 嵌入 API

AxonHub 通过 OpenAI 兼容 API 提供全面的文本和多模态嵌入生成支持。

**端点：**
- `POST /v1/embeddings` - OpenAI 兼容嵌入 API

**支持的输入类型：**
- 单个文本字符串
- 文本字符串数组
- 令牌数组（整数）
- 多个令牌数组

**支持的编码格式：**
- `float` - 默认，返回嵌入向量为浮点数组
- `base64` - 返回嵌入为 base64 编码字符串

### 请求格式

```json
{
  "input": "要嵌入的文本",
  "model": "text-embedding-3-small",
  "encoding_format": "float",
  "dimensions": 1536,
  "user": "user-id"
}
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `input` | string \| string[] \| number[] \| number[][] | ✅ | 要嵌入的文本。可以是单个字符串、字符串数组、令牌数组或多个令牌数组。 |
| `model` | string | ✅ | 用于嵌入生成的模型。 |
| `encoding_format` | string | ❌ | 返回嵌入的格式。可以是 `float` 或 `base64`。默认：`float`。 |
| `dimensions` | integer | ❌ | 输出嵌入的维度数。 |
| `user` | string | ❌ | 最终用户的唯一标识符。 |

### 响应格式

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

### 示例

**OpenAI SDK (Python)：**
```python
import openai

client = openai.OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

response = client.embeddings.create(
    input="你好，世界！",
    model="text-embedding-3-small"
)

print(response.data[0].embedding[:5])  # 前 5 个维度
```

**OpenAI SDK (Go)：**
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
        Input: openai.Union[string](openai.String("你好，世界！")),
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

**多个文本：**
```python
response = client.embeddings.create(
    input=["你好，世界！", "你好吗？"],
    model="text-embedding-3-small"
)

for i, data in enumerate(response.data):
    print(f"文本 {i}: {data.embedding[:3]}...")
```

## 认证

OpenAI API 格式使用 Bearer 令牌认证：

- **头部**：`Authorization: Bearer <your-api-key>`

API 密钥通过 AxonHub 的 API 密钥管理系统进行管理。

## 流式支持

OpenAI API 格式支持流式响应：

```go
// OpenAI SDK 流式传输
completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
    Messages: []openai.ChatCompletionMessageParamUnion{
        openai.UserMessage("写一篇关于人工智能的短篇故事"),
    },
    Model:  openai.ChatModel("claude-3-5-sonnet"),
    Stream: openai.Bool(true),
})
if err != nil {
    panic(err)
}

// 遍历流式数据块
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

## 错误处理

OpenAI 格式错误响应：

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

## 工具支持

AxonHub 通过 OpenAI API 格式支持**函数工具**（自定义函数调用）。但是，**不支持**各提供商特有的工具：

| 工具类型 | 支持状态 | 说明 |
| -------- | -------- | ---- |
| **函数工具（Function Tools）** | ✅ 支持 | 自定义函数定义可跨所有提供商使用 |
| **网页搜索（Web Search）** | ❌ 不支持 | 提供商特有功能（OpenAI、Anthropic 等） |
| **代码解释器（Code Interpreter）** | ❌ 不支持 | 提供商特有功能（OpenAI、Anthropic 等） |
| **文件搜索（File Search）** | ❌ 不支持 | 提供商特有功能 |
| **计算机使用（Computer Use）** | ❌ 不支持 | Anthropic 特有功能 |

> **注意**：仅支持可跨提供商转换的通用函数工具。网页搜索、代码解释器、计算机使用等提供商特有工具需要直接访问提供商的基础设施，无法通过 AxonHub 代理。

## 最佳实践

1. **使用追踪头部**：包含 `AH-Trace-Id` 和 `AH-Thread-Id` 头部以获得更好的可观测性
2. **模型选择**：在请求中明确指定目标模型
3. **错误处理**：为 API 响应实现适当的错误处理
4. **流式处理**：对于长响应使用流式处理以获得更好的用户体验
5. **使用函数工具**：进行工具调用时，请使用通用函数工具而非提供商特有工具

## 迁移指南

### 从 OpenAI 迁移到 AxonHub
```go
// 之前：直接 OpenAI
client := openai.NewClient(
    option.WithAPIKey("openai-key"),
)

// 之后：使用 OpenAI API 的 AxonHub
client := openai.NewClient(
    option.WithAPIKey("axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/v1"),
)
// 您的现有代码继续工作！
```
