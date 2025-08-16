# OpenAI FakeTransformer

## 概述

`FakeTransformer` 是一个用于测试目的的 OpenAI transformer 实现，它实现了 `pipeline.ChannelCustomizedExecutor` 接口，能够返回固定的响应数据而不需要实际调用 OpenAI API。

## 特性

- **固定响应**: 返回来自 `testdata/openai-stop.response.json` 的预定义 JSON 响应
- **固定流式响应**: 返回来自 `testdata/openai-stop.stream.jsonl` 的预定义事件流
- **无需 API 密钥**: 不需要真实的 OpenAI API 密钥或网络连接
- **快速测试**: 适用于单元测试和集成测试场景

## 使用方法

### 创建 FakeTransformer

```go
fake := openai.NewFakeTransformer()
```

### 在 Pipeline 中使用

```go
// 创建 fake transformer
fake := openai.NewFakeTransformer()

// 在 pipeline 中使用
pipeline := pipeline.New()
pipeline.SetChannelCustomizedExecutor(fake)
```

## 返回的数据格式

### 非流式响应

返回标准的 OpenAI Chat Completion 响应格式：

```json
{
  "id": "gen-1754577344-bfGaoVZhBY3iT78Psu02",
  "model": "gpt-4o-mini",
  "object": "chat.completion",
  "created": 1754577344,
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Sure! Here's the output from 1 to 20, with 5 numbers on each line:\n\n```\n1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20\n```"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 19,
    "completion_tokens": 65,
    "total_tokens": 84
  }
}
```

### 流式响应

返回一系列 OpenAI Chat Completion Chunk 事件，每个事件包含：

```json
{
  "id": "gen-1754577344-bfGaoVZhBY3iT78Psu02",
  "model": "gpt-4o-mini",
  "object": "chat.completion.chunk",
  "created": 1754577344,
  "choices": [
    {
      "index": 0,
      "delta": {
        "role": "assistant",
        "content": "Sure"
      },
      "finish_reason": null
    }
  ]
}
```

## 测试

运行 FakeTransformer 的测试：

```bash
go test -v ./internal/llm/transformer/openai -run TestFake
```

运行完整的 OpenAI transformer 测试套件：

```bash
go test ./internal/llm/transformer/openai
```

## 实现细节

- `FakeTransformer` 实现了 `pipeline.ChannelCustomizedExecutor` 接口
- `fakeExecutor` 实现了 `pipeline.Executor` 接口的 `Do` 和 `DoStream` 方法
- 所有响应数据都从 `testdata` 目录中的文件读取
- 流式响应通过解析 JSONL 格式的测试数据生成

## 注意事项

1. 这个实现仅用于测试目的，不应在生产环境中使用
2. 返回的响应是固定的，不会根据输入请求的内容变化
3. 确保 `testdata` 目录中的测试文件存在且格式正确
4. 流式响应会立即返回所有事件，没有实际的延迟模拟