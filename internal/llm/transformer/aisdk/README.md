# Vercel AI SDK Inbound Transformer

这个包实现了 Vercel AI SDK 的 inbound transformer，用于将 AI SDK 格式的请求转换为内部 LLM 格式，并将响应转换回 AI SDK 格式。

## 功能特性

### 请求转换 (TransformRequest)
- 解析 AI SDK JSON 请求格式
- 支持基本文本消息和多模态内容（文本 + 图片）
- 支持工具定义和参数
- 转换为内部 `llm.ChatCompletionRequest` 格式

### 响应转换 (TransformResponse)
- 将内部 `llm.ChatCompletionResponse` 转换为 AI SDK 格式
- 保持响应结构的兼容性
- 设置正确的 Content-Type 头

### 流式响应转换 (TransformStreamChunk)
- 实现 Vercel AI SDK Data Stream Protocol
- 支持以下流式数据格式：
  - `0:"text"\n` - 文本内容
  - `b:{"toolCallId":"id","toolName":"name"}\n` - 工具调用开始
  - `c:{"toolCallId":"id","argsTextDelta":"delta"}\n` - 工具调用参数增量
  - `9:{"toolCallId":"id","toolName":"name","args":{}}\n` - 完整工具调用
  - `e:{"finishReason":"stop","usage":{}}\n` - 完成原因和使用统计

## 数据流协议

根据 Vercel AI SDK 官方规范，流式响应使用以下格式：

```
Content-Type: text/plain; charset=utf-8
x-vercel-ai-data-stream: v1

TYPE_ID:CONTENT_JSON\n
```

其中 `TYPE_ID` 标识数据类型，`CONTENT_JSON` 是 JSON 格式的内容。

## 使用示例

```go
package main

import (
    "context"
    "github.com/looplj/axonhub/internal/llm/transformer/aisdk"
)

func main() {
    transformer := aisdk.NewInboundTransformer()
    
    // 转换请求
    llmReq, err := transformer.TransformRequest(ctx, httpReq)
    if err != nil {
        // 处理错误
    }
    
    // 转换响应
    httpResp, err := transformer.TransformResponse(ctx, llmResp)
    if err != nil {
        // 处理错误
    }
    
    // 转换流式块
    streamEvent, err := transformer.TransformStreamChunk(ctx, chunk)
    if err != nil {
        // 处理错误
    }
}
```

## 支持的消息格式

### 基本文本消息
```json
{
  "messages": [
    {
      "role": "user",
      "content": "Hello, world!"
    }
  ]
}
```

### 多模态消息
```json
{
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "What's in this image?"
        },
        {
          "type": "image_url",
          "image_url": {
            "url": "data:image/jpeg;base64,..."
          }
        }
      ]
    }
  ]
}
```

### 工具定义
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather information",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string"
            }
          }
        }
      }
    }
  ]
}
```

## 测试

运行测试：

```bash
go test -v
```

所有测试都验证了：
- 请求格式转换的正确性
- 响应格式转换的正确性
- 流式数据协议的实现
- 错误处理

## 兼容性

这个实现完全兼容 Vercel AI SDK 的官方规范，特别是：
- Data Stream Protocol v1
- 消息格式
- 工具调用格式
- 流式响应格式