# 作者 upstream 协议转换架构重读（作废 `/tmp` 结论）

日期：2026-07-06

源码基线：

```text
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
HEAD: 97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)
codebase-memory project: Users-asuan-AI-axonhub-worktrees-upstream-unstable
```

本文件用于替代早期基于 `/tmp/axonhub-upstream-20260706-175405` 和零散 `git show` 的临时判断。

## 1. 作者真实主架构

### 1.1 pipeline 主链路

证据：`llm/pipeline/pipeline.go:256-438`

真实链路是：

```text
client HTTP request
  -> Inbound.TransformRequest(ctx, httpclient.Request)
  -> llm.Request
  -> before request middlewares
  -> Outbound.TransformRequest(ctx, llm.Request)
  -> provider HTTP request
  -> executor
  -> provider response/stream
  -> outbound response/stream to llm.Response
  -> inbound response/stream to client format
```

关键点：

- `pipeline.Process` 第一步调用 `p.Inbound.TransformRequest` 得到 `llm.Request`。
- `pipeline.processRequest` 再调用 `p.Outbound.TransformRequest` 得到发往上游的 HTTP 请求。
- `llmRequest.RawRequest = request` 会保存原始 HTTP 请求，但后续 `httpclient.MergeInboundRequest` 只合并 headers/query，不合并 JSON body。

因此：

```text
原始 JSON body 字段如果在 inbound transformer 阶段丢了，pipeline 不会自动把它补回来。
```

### 1.2 `llm.Request` 的定位

证据：`llm/model.go:40-287`

`llm.Request` 是 AxonHub 的公共请求模型。注释明确说它基于 OpenAI Chat Completion 扩展，用来兼容主要 app/framework。

它包含：

- 公共 chat 字段：`messages`、`model`、`tools`、`tool_choice`、`stream`、`metadata`、`reasoning_*`、`response_format` 等；
- request type 子结构：`Embedding`、`Rerank`、`Image`、`Video`、`Compact`、`Completion`；
- 转换辅助字段：`RequestType`、`APIFormat`、`TransformOptions`、`TransformerMetadata`、`RawRequest`；
- provider/API-format 私有 sidecar：`ProviderExtensions`。

重要边界：

```text
llm.Request 不是 OpenAI Responses 的完整 AST。
```

不能把 Responses 的每个原生字段都塞进 `llm.Request`，否则公共模型会变成万能垃圾桶。

### 1.3 `ProviderExtensions` 的定位

证据：`llm/provider_extensions.go:5-20`

作者已经定义：

```go
// ProviderExtensions carries provider/API-format private data that should not
// be serialized through the common llm request/response JSON model.
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

这不是我们新造的概念，而是作者已经有的设计：

```text
公共语义 -> llm.Request
协议私有且不能稳定公共化的字段 -> ProviderExtensions
```

## 2. OpenAI Responses 入站/出站真实行为

### 2.1 入站 Responses -> llm.Request

证据：`llm/transformer/openai/responses/inbound.go:37-64`、`169-303`

入站流程：

1. `json.Unmarshal(httpReq.Body, &req)` 把 body 解到 `responses.Request`。
2. `convertToLLMRequest(&req, httpReq.Body)` 把 `responses.Request` 转成 `llm.Request`。
3. `attachOpenAIResponsesRequestExtensions(chatReq, req, rawBody)` 从原始 body 里补捞 raw-only 片段。

当前结构化映射：

```text
model                  -> llm.Request.Model
input/instructions     -> llm.Request.Messages
max_output_tokens      -> llm.Request.MaxCompletionTokens
stream/store/user/etc. -> llm.Request 公共字段
tools                  -> llm.Request.Tools 中可表示的部分
tool_choice            -> llm.Request.ToolChoice 中可表示的部分
reasoning              -> llm.Request.ReasoningEffort/Budget/Summary
```

当前 `TransformerMetadata` 保存：

```text
include
max_tool_calls
prompt_cache_retention
truncation
include_obfuscation
```

### 2.2 出站 llm.Request -> Responses

证据：`llm/transformer/openai/responses/outbound.go:191-318`

出站流程：

1. 从 `llm.Request` 重建 `responses.Request` payload。
2. 从 `TransformerMetadata` 恢复少量 Responses 字段。
3. 调用 `marshalRequestPayload(payload, llmReq)`。
4. `marshalRequestPayload` 再把 `ProviderExtensions.OpenAIResponses.Request` 里的 raw-only 片段合并回 JSON。

这个设计意味着：

```text
Responses -> Responses 不是原始 body 透传。
它是“结构化重建 + 局部 raw 回放”。
```

这也是为什么没有被 struct 或 raw sidecar 接住的字段会丢。

## 3. 作者已经做了哪些保真

### 3.1 raw tools / tool_choice / input items 已有保真

证据：`llm/transformer/openai/responses/request_extensions.go:9-365`

当前 sidecar 捕获：

```text
RawTools
RawToolChoice
RawInputItems
ToolSignatures
```

当前 raw 回放策略不是无脑透传，而是带保护：

- 只回放结构体无法表示的 tool；
- 如果结构化 tools 已经变了，不回放旧 raw tool；
- raw input item 按原始 index 插回；
- raw tool_choice 仅在当前 tool_choice 仍匹配时回放。

### 3.2 Codex `tool_search` / `tool_search_call` 不是完全缺失

证据：

- `llm/transformer/openai/responses/outbound_test.go:227-281`
- `llm/transformer/openai/responses/outbound_test.go:283-332`
- `llm/transformer/openai/responses/outbound_test.go:334-374`

作者最新版 upstream 已有测试证明：

```text
tool_search tool with namespace
raw tool_choice type=tool_search
tool_search_call input item
```

可以通过 `ProviderExtensions.OpenAIResponses.Request` 保真回放。

所以早期结论里“Codex MCP/lazy-loading 字段都缺失”是错误的。准确说法是：

```text
Codex/MCP lazy-loading 的一部分字段已经通过 raw tools/input/tool_choice 保真；
但 top-level 或其他未被 raw parser 捕获的字段仍会丢。
```

## 4. 当前真实缺口

### 4.1 Request struct 缺口

证据：`llm/transformer/openai/responses/model.go:89-156`

`responses.Request` 当前有：

```text
model
instructions
temperature
input
tools
parallel_tool_calls
background
stream
store
service_tier
safety_identifier
user
metadata
max_output_tokens
max_tool_calls
text
include
previous_response_id
prompt_cache_key
prompt_cache_retention
reasoning
stream_options
tool_choice
truncation
top_logprobs
top_p
```

当前注释/缺失：

```text
prompt              // 有 Prompt struct，但 Request.Prompt 被注释 TODO
conversation        // 有 Conversation struct，但 Request.Conversation 被注释 TODO
context_management  // 未出现
```

### 4.2 top-level raw fallback 缺口

证据：`request_extensions.go:33-63`

当前 `parseRawRequestFragments` 只解析：

```text
tools
tool_choice
input
```

不会捕获：

```text
prompt
conversation
context_management
additional_tools
defer_loading
任何未来 top-level unknown field
```

如果这些字段没有被 `responses.Request` struct 接住，就会在 `json.Unmarshal` 后丢失。

### 4.3 pipeline 不会救回 body

证据：`llm/httpclient/utils.go:267-279`

`MergeInboundRequest` 只合并 headers/query：

```text
Headers: merge
Query: merge unless SkipInboundQueryMerge
Body: 不合并
```

所以不能指望 `RawRequest` 或 pipeline 自动恢复 JSON body 字段。

## 5. 早期结论修正

### 保留的结论

```text
作者主架构是 inbound -> llm.Request -> outbound。
llm.Request 不应变成协议原生字段垃圾桶。
ProviderExtensions 是作者已有的协议私有 sidecar seam。
Responses -> Responses 需要 same-protocol preservation，否则会丢字段。
```

### 作废/修正的结论

旧说法：

```text
Codex MCP/lazy-loading 字段基本都没保真。
```

修正为：

```text
tool_search、tool_search_call、namespace 这类位于 tools/input/tool_choice 内的字段，作者最新版 upstream 已有 raw 保真测试。
真正缺的是 top-level raw fallback，以及 prompt/conversation/context_management/additional_tools/defer_loading 这类不在当前 raw parser 范围里的字段。
```

旧说法：

```text
P1d 需要整体补 Codex Responses MCP / lazy-loading identity。
```

修正为：

```text
P1d 应缩小为：补当前 raw parser 未覆盖的 top-level Codex/Responses 字段；不要重复实现已经存在的 raw tool/input 回放。
```

## 6. 应该怎么改

### 6.1 不应该改哪里

不要改：

```text
pipeline 主流程
llm.Request 公共字段大结构
响应/stream 聚合主流程
Chat/Anthropic 跨协议转换
```

原因：当前 bug 是 Responses 请求字段在入站解析/出站重建之间丢失，不是 pipeline 路由问题。

### 6.2 应该沿用作者 seam 小改

改动入口应集中在：

```text
llm/provider_extensions.go
llm/transformer/openai/responses/model.go
llm/transformer/openai/responses/request_extensions.go
llm/transformer/openai/responses/inbound.go
llm/transformer/openai/responses/outbound.go
```

最小设计：

```text
1. 对官方且已知的 Responses top-level 字段：
   - 能清楚建模的，加到 responses.Request。
   - 例如 prompt/conversation 可用已有 Prompt/Conversation struct 恢复。

2. 对 schema 不稳定或 Codex/profile 私有 top-level 字段：
   - 不塞 llm.Request。
   - 放进 ProviderExtensions.OpenAIResponses.Request 的 raw top-level fallback。

3. 出站 marshal 时：
   - 先 marshal 结构化 payload。
   - 再 merge raw top-level fallback。
   - 不能覆盖已经结构化重建的字段，例如 model/input/tools/tool_choice。
```

### 6.3 建议新增的 sidecar 字段

在 `OpenAIResponsesRequestExtensions` 增加类似：

```go
RawTopLevel map[string]json.RawMessage `json:"-"`
```

或者为了顺序和可测试性，用 slice：

```go
type OpenAIResponsesRawField struct {
    Key string          `json:"-"`
    Raw json.RawMessage `json:"-"`
}

RawTopLevelFields []OpenAIResponsesRawField `json:"-"`
```

推荐 slice，因为：

- clone 逻辑更直观；
- 测试顺序稳定；
- 和已有 `OpenAIResponsesRawFragment` 风格一致。

但 JSON object 字段顺序本身无语义，因此 map 也可接受。

### 6.4 raw top-level 捕获规则

入站从原始 body 解析 `map[string]json.RawMessage`，只捕获“不由当前结构化 payload 管理”的 key：

优先捕获：

```text
prompt
conversation
context_management
additional_tools
defer_loading
其他 unknown top-level fields
```

不捕获/不回放已结构化字段：

```text
model
input
instructions
tools
tool_choice
stream
store
metadata
max_output_tokens
reasoning
text
include
max_tool_calls
truncation
prompt_cache_key
prompt_cache_retention
previous_response_id
parallel_tool_calls
service_tier
safety_identifier
user
temperature
top_p
top_logprobs
background
stream_options
```

防止旧 raw 字段覆盖 outbound 阶段已经正确改写的字段，例如模型映射后的 `model`。

### 6.5 typed 字段还是 raw 字段？

建议：

```text
prompt / conversation：优先启用 typed field，因为作者已经写了 Prompt/Conversation struct 和 TODO。
context_management：先用 raw fallback，除非官方 schema 已确认并写成 struct。
additional_tools / defer_loading：先 raw fallback，因为更像 Codex/profile 扩展，不应进 llm.Request。
unknown future top-level：raw fallback，仅 same-protocol Responses -> Responses 使用。
```

## 7. 新的最小切片计划

### Slice A：补 top-level raw fallback，先不改 typed struct

目标：证明 `prompt/conversation/context_management/additional_tools/defer_loading/unknown_x` 经过 Responses inbound -> Responses outbound 不丢。

验收：

- 模型映射后 `model` 仍使用新模型，不被 raw 覆盖；
- `tools/input/tool_choice` 原有测试继续通过；
- raw top-level 只在 OpenAIResponses ProviderExtensions 内，不进 `llm.Request` 公共字段。

### Slice B：启用官方 typed 字段

目标：把作者已 TODO 的：

```text
Prompt *Prompt
Conversation *Conversation
```

恢复到 `responses.Request`，并在 outbound payload 中结构化输出。

验收：

- typed 字段和 raw fallback 不重复冲突；
- 如果 outbound 结构化已经有 key，raw fallback 不覆盖。

### Slice C：整理 `TransformerMetadata` 边界

目标：不扩大 `TransformerMetadata` 用途，只保留当前兼容路径。新增字段不塞进去。

验收：

- 新增字段归属清楚：公共语义、Responses typed、Responses raw sidecar 三选一；
- 没有 magic key 垃圾桶扩散。

## 8. 当前结论

这次不是要推翻作者架构，而是要修正我们之前的错误理解：

```text
作者已经做了一部分 same-protocol raw preservation；
现在应该补的是 top-level preservation 和官方 TODO 字段，
不是重写整个 Codex/MCP 工具转换。
```
