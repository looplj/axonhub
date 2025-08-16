# Fake Transformer for Anthropic

这个文档描述了如何使用 Anthropic transformer 包中的 FakeTransformer。

## 概述

FakeTransformer 是一个用于测试和开发的模拟平台，它返回固定的响应数据而不是实际调用 Anthropic API。这对于以下场景非常有用：

- 单元测试
- 集成测试
- 开发环境中的模拟数据
- API 限制或成本控制

## 特性

- 实现了 `pipeline.ChannelCustomizedExecutor` 接口
- 支持非流式响应（返回固定的 JSON 响应）
- 支持流式响应（返回固定的 SSE 事件流）
- 使用 testdata 目录中的真实测试数据

## 使用方法

### 创建 FakeTransformer

```go
fake := anthropic.NewFakeTransformer()
```

### 在 Pipeline 中使用

```go
// 创建 pipeline factory
factory := pipeline.NewFactory(originalExecutor)

// 创建 inbound transformer
inbound := anthropic.NewInboundTransformer()

// 使用 fake transformer 作为 outbound
fake := anthropic.NewFakeTransformer()

// 创建 pipeline
pipeline := factory.Pipeline(inbound, fake)

// 处理请求
result, err := pipeline.Process(ctx, request)
```

## 返回的数据

### 非流式响应

返回 `testdata/anthropic-stop.response.json` 中的固定响应：

```json
{
  "id": "msg_bdrk_01Fbg5HKuVfmtT6mAMxQoCSn",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20"
    }
  ],
  "model": "claude-3-7-sonnet-20250219",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 21,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "output_tokens": 43
  }
}
```

### 流式响应

返回 `testdata/anthropic-stop.stream.jsonl` 中的固定事件流，包含：

- `message_start` - 消息开始事件
- `content_block_start` - 内容块开始事件
- `content_block_delta` - 内容增量事件（多个）
- `content_block_stop` - 内容块结束事件
- `message_delta` - 消息增量事件
- `message_stop` - 消息结束事件

## 测试

运行测试以验证 FakeTransformer 的功能：

```bash
go test -v ./internal/llm/transformer/anthropic -run TestFake
```

## 注意事项

1. FakeTransformer 忽略所有输入请求参数，始终返回相同的固定响应
2. 返回的数据来自 testdata 目录，确保这些文件存在且格式正确
3. 这个 transformer 仅用于测试和开发，不应在生产环境中使用
4. 如果需要不同的测试数据，可以修改 testdata 目录中的文件