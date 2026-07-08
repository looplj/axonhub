# OpenAI Responses 原生保真入口分析（基于本地 upstream worktree + codebase-memory CLI）

日期：2026-07-06

源码来源：

```text
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
HEAD: 97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)
MCP/CLI project: Users-asuan-AI-axonhub-worktrees-upstream-unstable
```

## 1. 作者当前架构

作者的主干链路是：

```text
OpenAI Responses HTTP request
  -> llm/transformer/openai/responses/inbound.go
  -> llm.Request 跨协议公共结构
  -> llm/transformer/openai/responses/outbound.go
  -> OpenAI Responses HTTP request
```

这个框架不需要推翻。问题在于 Responses 原生字段经过 `llm.Request` 时，有些字段没有稳定归属，会丢失或被错误塞进 `TransformerMetadata`。

## 2. 已存在的保真 seam

作者已经有一个专门用于 OpenAI Responses 原生字段旁路保真的结构：

```go
// llm/provider_extensions.go

type ProviderExtensions struct {
    OpenAIResponses *OpenAIResponsesProviderExtensions `json:"-"`
}

type OpenAIResponsesProviderExtensions struct {
    Request *OpenAIResponsesRequestExtensions `json:"-"`
}

type OpenAIResponsesRequestExtensions struct {
    RawTools       []OpenAIResponsesRawFragment `json:"-"`
    ToolSignatures []string                     `json:"-"`
    RawToolChoice  json.RawMessage              `json:"-"`
    RawInputItems  []OpenAIResponsesRawFragment `json:"-"`
}
```

含义：

- `llm.Request` 仍负责跨协议公共语义；
- OpenAI Responses 专属且不能安全降级的字段，应该放进 `ProviderExtensions.OpenAIResponses.Request`；
- 这个结构是作者已有 seam，不是新架构。

## 3. inbound 入口

文件：

```text
llm/transformer/openai/responses/inbound.go
```

关键函数：

```text
InboundTransformer.TransformRequest
convertToLLMRequest
```

当前行为：

1. `TransformRequest` 把 HTTP body 解析成 `responses.Request`。
2. `convertToLLMRequest` 把 `responses.Request` 降到 `llm.Request`。
3. 部分 Responses 字段进入 `llm.Request` 公共字段。
4. 部分字段进入 `TransformerMetadata`：
   - `include`
   - `max_tool_calls`
   - `prompt_cache_retention`
   - `truncation`
   - `include_obfuscation`
5. `attachOpenAIResponsesRequestExtensions(chatReq, req, rawBody)` 捕获 raw-only 片段。

当前 raw-only 捕获范围：

```text
tools
tool_choice
input array items
```

## 4. outbound 入口

文件：

```text
llm/transformer/openai/responses/outbound.go
```

关键函数：

```text
OutboundTransformer.TransformRequest
marshalRequestPayload
```

当前行为：

1. 从 `llm.Request` 重建 `responses.Request` payload。
2. 从 `TransformerMetadata` 取回：
   - `include`
   - `max_tool_calls`
   - `prompt_cache_retention`
   - `truncation`
3. `marshalRequestPayload(payload, llmReq)` 负责把 `ProviderExtensions.OpenAIResponses.Request` 里的 raw-only 片段合并回 JSON。

当前合并范围：

```text
raw-only tools
raw tool_choice
raw-only input items
```

## 5. 当前 Request struct 明确缺口

文件：

```text
llm/transformer/openai/responses/model.go
```

`Request` 当前已有但未完全保真的字段：

```text
background
include
max_tool_calls
prompt_cache_retention
truncation
previous_response_id
prompt_cache_key
stream_options.include_obfuscation
```

`Request` 当前注释/缺失字段：

```text
prompt              // 注释 TODO
conversation        // 注释 TODO
context_management  // 不存在
```

这些字段中：

- 如果是 OpenAI Responses 官方 top-level 字段，应该优先加到 Responses native struct；
- 如果能降成跨协议公共语义，才进入 `llm.Request`；
- 如果不能降级但 same-protocol 要保真，进入 `ProviderExtensions.OpenAIResponses.Request` 或 raw top-level fallback；
- 不应该继续无边界地塞进 `TransformerMetadata`。

## 6. P1 切片落点

### P1a：官方 top-level Responses 字段

推荐落点：

```text
llm/transformer/openai/responses/model.go
llm/transformer/openai/responses/inbound.go
llm/transformer/openai/responses/outbound.go
llm/provider_extensions.go（必要时）
```

字段：

```text
conversation
context_management
prompt
```

判断：这些是 Responses 原生请求字段。先保证 same-protocol Responses -> Responses 不丢；不要强行映射到 Chat/Anthropic。

### P1b：unknown top-level raw fallback

推荐落点：

```text
llm/provider_extensions.go
llm/transformer/openai/responses/request_extensions.go
```

判断：作者已有 raw-only fragment 机制，但目前只覆盖 tools/input/tool_choice。可以沿着同一 seam 扩展为 same-protocol top-level raw fallback，但必须限制为 OpenAI Responses -> OpenAI Responses，不做跨协议乱透传。

### P1c：Responses tool variants / raw tools

推荐落点：

```text
llm/transformer/openai/responses/model.go
llm/transformer/openai/responses/request_extensions.go
```

判断：作者已有 tool raw fallback。新工具类型如果能结构化就加到 `responses.Tool`；不能安全结构化就继续 raw fallback。

### P1d：Codex Responses MCP / lazy-loading identity

推荐落点：

```text
llm/transformer/openai/responses/model.go
llm/transformer/openai/responses/request_extensions.go
llm/provider_extensions.go
```

字段族：

```text
tool_search
defer_loading
additional_tools
namespace
tool_search_call
tool_search_output
function_call.namespace
```

判断：这些不是跨协议公共字段。先按 Responses native / Codex profile 同协议保真处理；不要塞到 `llm.Request` 公共层。

## 7. 明确不能这样改

不要：

1. 新增全局万能 converter；
2. 把所有未知字段塞进 `TransformerMetadata`；
3. 把 OpenAI Responses 原生字段强行塞进 `llm.Request`；
4. same-protocol raw fallback 跨协议透传；
5. 为了“看起来完整”一次性重写 Chat/Anthropic 转换。

## 8. 当前最小下一步

在进入实现前，还需要完成：

1. 当前 dirty tree 的 git 存档/隔离方案；
2. 将旧实现污染和规划文档分开；
3. 确认 P1a 的测试输入/输出；
4. `task.py start` 后才进入 TDD 实现。
