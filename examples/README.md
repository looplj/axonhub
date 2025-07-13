# Chat Completion Transformer 架构示例

本示例展示了如何使用 AxonHub 的 Transformer 架构来处理聊天完成请求。

## 架构流程

```
HTTP Request → Inbound Transformer → [Decorators] → Outbound Transformer → Provider API
                                                                              ↓
HTTP Response ← Inbound Transformer ← [Decorators] ← Outbound Transformer ← Provider Response
```

### 流程说明

1. **Inbound Transformation**: 将 HTTP 请求转换为标准的 `ChatCompletionRequest`
2. **Decorators** (待实现): 应用中间件如认证、限流、日志等
3. **Outbound Transformation**: 将标准请求转换为特定提供商的格式
4. **Provider API**: 调用实际的 AI 提供商 API
5. **Response Processing**: 反向处理响应数据

## 组件说明

### InboundTransformer
- 负责解析 HTTP 请求
- 将请求转换为内部标准格式
- 处理响应的最终格式化

### OutboundTransformer
- 将标准请求转换为提供商特定格式
- 处理提供商响应的转换
- 支持流式和非流式响应

### HTTPClient
- 统一的 HTTP 客户端接口
- 支持重试、超时、认证等功能
- 处理流式和非流式请求

## 运行示例

```bash
go run examples/chat_completion_example.go
```

服务器将在 `:8080` 端口启动，提供 `/v1/chat/completions` 端点。

## 测试请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "Hello, how are you?"
      }
    ]
  }'
```

## 扩展性

- **添加新的提供商**: 实现新的 `OutboundTransformer`
- **添加新的输入格式**: 实现新的 `InboundTransformer`
- **添加中间件**: 在 Decorator 链中添加新的处理器
- **自定义 HTTP 客户端**: 实现 `HttpClient` 接口