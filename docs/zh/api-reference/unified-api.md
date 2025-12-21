# 统一 API 参考

## 概述

AxonHub 提供统一的 API 网关，通过单一接口同时支持 OpenAI Chat Completions 和 Anthropic Messages API。这使您可以在使用现有 OpenAI 或 Anthropic 客户端 SDK 的同时，无缝访问多个提供商的模型。平台自动处理 API 格式转换，让您可以使用一种 API 格式访问任何支持的提供商。

## 核心优势

- **API 互操作性**：使用 OpenAI Chat Completions API 调用 Anthropic 模型，或使用 Anthropic Messages API 调用 OpenAI 模型
- **零代码变更**：继续使用现有的 OpenAI 或 Anthropic 客户端 SDK，无需修改
- **自动转换**：AxonHub 在需要时自动在 API 格式之间进行转换
- **提供商灵活性**：无论您偏好哪种 API 格式，都可以访问任何支持的 AI 提供商

## 支持的 API 格式

### OpenAI Chat Completions API

AxonHub 完全支持 OpenAI Chat Completions API 规范，允许您使用任何 OpenAI 兼容的客户端 SDK。

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
})
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

response, err := client.Responses.New(ctx, params)
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

stream := client.Responses.NewStreaming(ctx, params)

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

### Anthropic Messages API

AxonHub 还支持原生 Anthropic Messages API，适用于偏好 Anthropic 特定功能和响应格式的应用程序。

**端点：**
- `POST /anthropic/v1/messages` - 文本生成
- `GET /anthropic/v1/models` - 列出可用模型

**示例请求：**
```go
import (
    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

// 使用 AxonHub 配置创建 Anthropic 客户端
client := anthropic.NewClient(
    option.WithAPIKey("your-axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/anthropic"),
    
)

// 使用 Anthropic API 格式调用 OpenAI 模型
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, GPT!")),
}

response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gpt-4o"),
    Messages:  messages,
    MaxTokens: 1024,
})
if err != nil {
    // 适当处理错误
    panic(err)
}

// 从响应中提取文本内容
responseText := ""
for _, block := range response.Content {
    if textBlock := block.AsText(); textBlock != nil {
        responseText += textBlock.Text
    }
}
fmt.Println(responseText)
```

### Gemini API

AxonHub 原生支持 Gemini API，可访问 Gemini 强大的多模态功能。

**端点：**
- `POST /gemini/v1beta/models/{model}:generateContent` - 文本和多模态内容生成
- `GET /gemini/v1beta/models` - 列出可用模型

**示例请求：**
```go
import (
    "context"
    "google.golang.org/genai"
)

// 使用 AxonHub 配置创建 Gemini 客户端
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey:  "your-axonhub-api-key",
    Backend: genai.Backend(genai.APIBackendUnspecified), // 使用默认后端
})
if err != nil {
    // 适当处理错误
    panic(err)
}

// 使用 Gemini API 格式调用 OpenAI 模型
modelName := "gpt-4o"  // 通过 Gemini API 格式访问 OpenAI 模型
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("Hello, GPT!")},
    },
}

// 可选：配置生成参数
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.7)),
    MaxOutputTokens: genai.Ptr(int32(1024)),
}

response, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{content}, config)
if err != nil {
    // 适当处理错误
    panic(err)
}

// 从响应中提取文本
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    responseText := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*responseText)
}
```

**示例：多轮对话**
```go
// 创建带有对话历史的聊天会话
modelName := "claude-3-5-sonnet"
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.5)),
}

chat, err := client.Chats.Create(ctx, modelName, config, nil)
if err != nil {
    panic(err)
}

// 第一条消息
response1, err := chat.SendMessage(ctx, genai.Part{Text: genai.Ptr("My name is Alice")})
if err != nil {
    panic(err)
}

// 后续消息（模型记住上下文）
response2, err := chat.SendMessage(ctx, genai.Part{Text: genai.Ptr("What is my name?")})
if err != nil {
    panic(err)
}

// 提取响应
if len(response2.Candidates) > 0 {
    text := response2.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)  // 应该包含 "Alice"
}
```

### 嵌入 API

AxonHub 通过 OpenAI 兼容和 Jina AI 特定的 API 提供全面的文本和多模态嵌入生成支持。

**端点：**
- `POST /v1/embeddings` - OpenAI 兼容嵌入 API
- `POST /jina/v1/embeddings` - Jina AI 特定嵌入 API

**支持的输入类型：**
- 单个文本字符串
- 文本字符串数组
- 令牌数组（整数）
- 多个令牌数组

**支持的编码格式：**
- `float` - 默认，返回嵌入向量为浮点数组
- `base64` - 返回嵌入为 base64 编码字符串

#### 请求格式

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

**Jina 特定参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `task` | string | ❌ | Jina 嵌入的任务类型。选项：`text-matching`、`retrieval.query`、`retrieval.passage`、`separation`、`classification`、`none`。 |

#### 响应格式

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

#### 示例

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

**Jina 特定任务：**
```python
import requests

response = requests.post(
    "http://localhost:8090/jina/v1/embeddings",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
        "input": "什么是机器学习？",
        "model": "jina-embeddings-v2-base-en",
        "task": "retrieval.query"
    }
)

result = response.json()
print(result["data"][0]["embedding"][:5])
```

### 重排序 API

AxonHub 通过 OpenAI 兼容和 Jina AI 特定的 API 支持文档重排序，允许您根据与查询的相关性重新排列文档。

**端点：**
- `POST /v1/rerank` - OpenAI 兼容重排序 API
- `POST /jina/v1/rerank` - Jina AI 特定重排序 API

#### 请求格式

```json
{
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
| `query` | string | ✅ | 用于比较文档的搜索查询。 |
| `documents` | string[] | ✅ | 要重排序的文档列表。最少 1 个文档。 |
| `top_n` | integer | ❌ | 返回最相关文档的数量。如果未指定，返回所有文档。 |
| `return_documents` | boolean | ❌ | 是否在响应中返回原始文档。默认：false。 |

#### 响应格式

```json
{
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

#### 示例

**OpenAI SDK (Python)：**
```python
import requests

response = requests.post(
    "http://localhost:8090/v1/rerank",
    headers={
        "Authorization": "Bearer your-axonhub-api-key",
        "Content-Type": "application/json"
    },
    json={
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

**Jina SDK (Python)：**
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

**Go 示例：**
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
    Query     string   `json:"query"`
    Documents []string `json:"documents"`
    TopN      *int     `json:"top_n,omitempty"`
}

type RerankResponse struct {
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

### 使用 Anthropic SDK 调用 OpenAI 模型
```go
// Anthropic SDK 调用 OpenAI 模型
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("你好，世界！")),
}

response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gpt-4o"),  // OpenAI 模型
    Messages:  messages,
    MaxTokens: 1024,
})

// 访问响应
for _, block := range response.Content {
    if textBlock := block.AsText(); textBlock != nil {
        fmt.Println(textBlock.Text)
    }
}
// AxonHub 自动转换 Anthropic 格式 → OpenAI 格式
```

### 使用 Gemini SDK 调用 OpenAI 模型
```go
// Gemini SDK 调用 OpenAI 模型
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("什么是人工智能？")},
    },
}

response, err := client.Models.GenerateContent(
    ctx,
    "gpt-4o",  // OpenAI 模型
    []*genai.Content{content},
    nil,
)

// 访问响应
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    text := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)
}
// AxonHub 自动转换 Gemini 格式 → OpenAI 格式
```

## 支持的提供商

| 提供商                   | 状态       | 支持模型示例                 | 兼容 API |
| ------------------------ | ---------- | ---------------------------- | --------------- |
| **OpenAI**               | ✅ 已完成  | GPT-4、GPT-4o、GPT-5 等      | OpenAI, Anthropic |
| **Anthropic**            | ✅ 已完成  | Claude 3.5、Claude 3.0 等    | OpenAI, Anthropic |
| **智谱 AI（Zhipu）**     | ✅ 已完成  | GLM-4.5、GLM-4.5-air 等      | OpenAI, Anthropic |
| **月之暗面（Moonshot）** | ✅ 已完成  | kimi-k2 等                   | OpenAI, Anthropic |
| **DeepSeek**             | ✅ 已完成  | DeepSeek-V3.1 等             | OpenAI, Anthropic |
| **字节跳动豆包**         | ✅ 已完成  | doubao-1.6 等                | OpenAI, Anthropic |
| **Gemini**               | ✅ 已完成  | Gemini 2.5 等                | OpenAI, Anthropic |
| **AWS Bedrock**          | 🔄 测试中  | Claude on AWS                | OpenAI, Anthropic |
| **Google Cloud**         | 🔄 测试中  | Claude on GCP                | OpenAI, Anthropic |

## 认证

两种 API 格式使用相同的认证系统：

- **OpenAI API**：使用 `Authorization: Bearer <your-api-key>` 头部
- **Anthropic API**：使用 `X-API-Key: <your-api-key>` 头部
- **Gemini API**：使用 `X-Goog-API-Key: <your-api-key>` 头部

API 密钥通过 AxonHub 的 API 密钥管理系统进行管理，无论使用哪种 API 格式，都提供相同的权限。

## 流式支持

两种 API 格式都支持流式响应：

### OpenAI 流式
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

### Anthropic 流式
```go
// Anthropic SDK 流式传输
messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("从一数到五")),
}

stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
    Model:     anthropic.Model("gpt-4o"),
    Messages:  messages,
    MaxTokens: 1024,
})

// 收集流式内容
var content string
for stream.Next() {
    event := stream.Current()
    switch event := event.(type) {
    case anthropic.ContentBlockDeltaEvent:
        if event.Type == "content_block_delta" {
            content += event.Delta.Text
            fmt.Print(event.Delta.Text) // 边传输边打印
        }
    }
}

if err := stream.Err(); err != nil {
    panic(err)
}

fmt.Println("\n完整响应:", content)
```

## 错误处理

两种 API 格式都返回标准化的错误响应：

### OpenAI 格式错误
```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

### Anthropic 格式错误
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key"
  }
}
```

## 工具支持

AxonHub 支持所有 API 格式的 **函数工具**（自定义函数调用）。但是，**不支持** 各提供商特有的工具：

| 工具类型 | 支持状态 | 说明 |
| -------- | -------- | ---- |
| **函数工具（Function Tools）** | ✅ 支持 | 自定义函数定义可跨所有提供商使用 |
| **网页搜索（Web Search）** | ❌ 不支持 | 提供商特有功能（OpenAI、Anthropic 等） |
| **代码解释器（Code Interpreter）** | ❌ 不支持 | 提供商特有功能（OpenAI、Anthropic 等） |
| **文件搜索（File Search）** | ❌ 不支持 | 提供商特有功能 |
| **计算机使用（Computer Use）** | ❌ 不支持 | Anthropic 特有功能 |

> **注意**：仅支持可跨提供商转换的通用函数工具。网页搜索、代码解释器、计算机使用等提供商特有工具需要直接访问提供商的基础设施，无法通过 AxonHub 代理。

## 最佳实践

1. **选择偏好的 API**：使用最适合应用程序需求和现有代码库的 API 格式
2. **一致的认证**：在两种 API 格式中使用相同的 API 密钥
3. **模型选择**：在请求中明确指定目标模型
4. **错误处理**：为两种 API 格式实现适当的错误处理
5. **流式处理**：对于长响应使用流式处理以获得更好的用户体验
6. **使用函数工具**：进行工具调用时，请使用通用函数工具而非提供商特有工具

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

### 从 Anthropic 迁移到 AxonHub
```go
// 之前：直接 Anthropic
client := anthropic.NewClient(
    option.WithAPIKey("anthropic-key"),
)

// 之后：使用 Anthropic API 的 AxonHub
client := anthropic.NewClient(
    option.WithAPIKey("axonhub-api-key"),
    option.WithBaseURL("http://localhost:8090/anthropic"),
)
// 您的现有代码继续工作！
```