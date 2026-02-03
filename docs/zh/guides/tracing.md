# 追踪指南

---

### 概览
AxonHub 可以在不引入额外 SDK 的情况下，为每一次请求构建线程感知的追踪。只要客户端已经兼容 OpenAI 协议，您就可以通过传递追踪与线程请求头，或直接让 AxonHub 自动生成，实现低侵入的可观测能力。

使用追踪的主要优势包括：
- **可观测性**：清晰地查看每一条用户消息及其触发的所有 agent 请求。
- **性能优化**：AxonHub 会将同一个 Trace 的请求优先转发到同一个上游渠道，从而大幅提高提供商端的缓存命中率（例如 Anthropic 的 Prompt Caching），降低响应延迟并减少成本。
- **调试便捷**：结合线程 ID 还原完整的会话上下文，快速定位多轮对话中的问题。

### 关键概念
- **Thread ID（`AH-Thread-Id`）** – 代表用户的一个完整对话会话，将多条追踪关联起来，帮助重现完整的用户旅程。
- **Trace ID（`AH-Trace-Id`）** – 代表用户发出的一条消息以及该消息触发的所有 agent 请求。需要在需要串联多次调用时显式提供；未携带该请求头时，AxonHub 会为单次调用生成 ID 但无法自动关联其他请求。
- **Request（请求）** – 单次 API 调用的最小单元，包含完整的请求/响应数据、耗时、Token 使用量等信息。
- **额外追踪请求头** – 可配置备用请求头（如 `Sentry-Trace`），以复用已有的可观测工具链。

### Thread、Trace 与 Request 的关系

```
Thread (完整用户对话会话)
  └── Trace 1 (用户消息 1 + 所有 agent 请求)
        ├── Request 1 (agent 调用 1)
        ├── Request 2 (agent 调用 2)
        └── Request 3 (agent 调用 3)
  └── Trace 2 (用户消息 2 + 所有 agent 请求)
        ├── Request 4 (agent 调用 4)
        └── Request 5 (agent 调用 5)
```

- **Thread**：代表用户的一个完整对话会话，包含多条用户消息（每条消息对应一个 Trace）
- **Trace**：代表用户发出的一条消息以及该消息在处理过程中触发的所有 agent 请求
- **Request**：代表对 LLM 或其他服务的单次 API 调用，包含请求体、响应体、Token 使用量等详细信息

**层级关系**：
- 1 个 Thread 可以包含多个 Trace（每条用户消息一个 Trace）
- 1 个 Trace 可以包含多个 Request（该消息触发的所有 agent 调用）
- 1 个 Request 只能属于 1 个 Trace
- 1 个 Trace 只能属于 1 个 Thread（可选关联）

**实际应用场景**：
- **单条消息带 agent**：1 Thread → 1 Trace → N Request（用户发送一条消息，agent 发起多次 API 调用）
- **多轮对话**：1 Thread → 多 Trace（每条用户消息一个 Trace）→ 每个 Trace 包含 N Request
- **独立请求**：无 Thread → 1 Trace → 1 Request（无对话上下文的单次 API 调用）

### 配置
```yaml
# config.yml
trace:
  thread_header: "AH-Thread-Id"
  trace_header: "AH-Trace-Id"
  extra_trace_headers:
    - "Sentry-Trace"
```

- 通过 `extra_trace_headers` 复用已有的埋点请求头。
- 如不配置，将采用上述默认值。

### 在 OpenAI 兼容客户端中使用追踪
```bash
curl https://your-axonhub-instance/v1/chat/completions \
  -H "Authorization: Bearer ${AXONHUB_API_KEY}" \
  -H "Content-Type: application/json" \
  -H "AH-Trace-Id: at-demo-123" \
  -H "AH-Thread-Id: thread-abc" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      { "role": "user", "content": "Diagnose latency in my pipeline" }
    ]
  }'
```

- 当需要让多次请求落在同一追踪中时，请显式提供 `AH-Trace-Id`；若缺失该请求头，AxonHub 会分别记录这些调用，即便会为单次请求生成 ID。
- 任何 OpenAI 兼容 SDK 均可直接使用，只需根据需要添加请求头即可。

### SDK 示例
如需完整可运行的样例，可参考 `integration_test/openai/trace_multiple_requests/trace_test.go` 与 `integration_test/anthropic/trace_multiple_requests/trace_test.go`。以下片段展示了在生产代码中最核心的部分。

#### OpenAI Go SDK
```go
package traces

import (
    "context"

    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
)

func sendTracedChat(ctx context.Context, apiKey string) (*openai.ChatCompletion, error) {
    client := openai.NewClient(
        option.WithAPIKey(apiKey),
        option.WithBaseURL("https://your-axonhub-instance/v1"),
    )

    params := openai.ChatCompletionNewParams{
        Model: openai.ChatModel("gpt-4o"),
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.UserMessage("请帮我诊断一下管道的延迟问题"),
        },
    }

    // 在请求级别传递追踪和线程请求头
    return client.Chat.Completions.New(ctx, params,
        option.WithHeader("AH-Trace-Id", "trace-example-123"),
        option.WithHeader("AH-Thread-Id", "thread-example-abc"),
    )
}
```

#### Anthropic Go SDK
```go
package traces

import (
    "context"

    anthropic "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

func sendTracedMessage(ctx context.Context, apiKey string) (*anthropic.Message, error) {
    client := anthropic.NewClient(
        option.WithAPIKey(apiKey),
        option.WithBaseURL("https://your-axonhub-instance/anthropic"),
    )

    params := anthropic.MessageNewParams{
        Model: anthropic.Model("claude-3-5-sonnet"),
        Messages: []anthropic.MessageParam{
            anthropic.NewUserMessage(
                anthropic.NewTextBlock("请帮我诊断一下管道的延迟问题"),
            ),
        },
    }

    // 在请求级别传递追踪和线程请求头
    return client.Messages.New(ctx, params,
        option.WithHeader("AH-Trace-Id", "trace-example-123"),
        option.WithHeader("AH-Thread-Id", "thread-example-abc"),
    )
}
```

### 追踪数据存储
- 可在系统策略中决定是否保存完整请求／响应体：若仅需指标，可关闭以减少敏感数据留存。
- 在管理后台配置默认数据存储，若该存储不可用会自动回退到主存储，保障访问稳定。
- 大体量内容可放在外部存储（本地磁盘、S3、GCS），追踪页面仍能快速加载。

### Claude Code 追踪支持
- 将 `server.trace.claude_code_trace_enabled` 设为 `true`，AxonHub 会自动读取 Claude Code 产生的追踪 ID。
- `/anthropic/v1/messages` (及 `/v1/messages`) 的 `metadata.user_id` 会作为追踪 ID 使用，同时不会影响请求体给后续逻辑的读取。
- 如果请求已经带有追踪请求头，系统会优先使用该值，与自动提取机制兼容。

### Codex 追踪支持
- 将 `server.trace.codex_trace_enabled` 设为 `true`，AxonHub 会将 `Session_id` header 作为追踪 ID 使用。
- 如果请求已经带有追踪请求头，系统会优先使用该值，与自动提取机制兼容。

### 在控制台中探索追踪
1. 在 AxonHub 管理后台进入 **Traces** 页面。
2. 按项目、模型或时间范围筛选目标追踪。
3. 展开追踪查看 span、提示/回复内容、耗时及渠道元数据。
4. 跳转关联的线程，结合追踪细节还原完整会话。

<table>
  <tr align="center">
    <td align="center">
      <a href="../../screenshots/axonhub-trace.png">
        <img src="../../screenshots/axonhub-trace.png" alt="Trace Details" width="600"/>
      </a>
      <br/>
      Trace 详情页面展示了请求的时间线、Token 使用量及缓存命中情况
    </td>
  </tr>
</table>

### 故障排查
- **未生成追踪** – 确认请求已通过认证且项目 ID 正确解析（API Key 必须隶属于某个项目）。
- **缺少线程关联** – 在请求中提供 `AH-Thread-Id`，或先通过 API 创建线程。
- **追踪 ID 异常** – 检查上游代理是否覆盖了相关请求头。
