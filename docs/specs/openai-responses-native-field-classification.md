# OpenAI Responses native 字段分类草案

- 日期：2026-07-05
- 目标：把 OpenAI Responses / Codex 实际使用字段分成三类，指导 AxonHub 做完整 OpenAI Responses native round-trip 改造。
- 结论：以 OpenAI 官方 Responses 为标准；Codex 请求作为高覆盖真实样本；OpenRouter baseline 只能作为第三方兼容参考，不能覆盖 OpenAI native 语义。最终目标是完整保真，不是只修 Codex 当前触发的少数字段。

## 0. 分类原则

### A. 进入 `llm.Request` 的跨协议通用抽象

满足至少一个条件：

1. Chat Completions / Anthropic Messages / OpenAI Responses 都能表达，或可以稳定降级。
2. 是模型调用的通用语义，例如模型名、消息、采样、普通 function tool。
3. 转换后不会破坏 Responses 原生 replay / tool routing。

### B. 保留在 OpenAI Responses native/raw 层

满足至少一个条件：

1. 是 OpenAI Responses 标准字段，但其他协议没有等价表达。
2. 是 Codex / agent 客户端依赖的 Responses item/tool 结构。
3. 经过 `llm.Request` 会丢字段、压扁结构、改变 tool payload 类型或破坏 namespace。
4. 是未来扩展字段，当前 Hub 无法安全理解。

### C. 跨协议转换时降级/诊断/显式丢弃

满足至少一个条件：

1. 目标协议不支持。
2. Hub 不能执行对应 server/client tool。
3. 只能保留在 Responses→Responses，Responses→Chat 时必须 flatten 或 drop。
4. 需要记录诊断，避免 silent loss。

---

## 1. 官方依据

本地官方文档：

- `docs/specs/vendor/openai/api/function-calling.md`
- `docs/specs/vendor/openai/api/tools-tool-search.md`
- `docs/specs/vendor/openai/api/tools-connectors-mcp.md`
- `docs/specs/vendor/openai/api/streaming-responses.md`
- `docs/specs/vendor/openai/api/conversation-state.md`
- `docs/specs/vendor/openai/api/migrate-to-responses.md`
- `docs/specs/vendor/openai/api/responses-create-reference.md`
- `docs/specs/vendor/openai/codex/codex-manual.md`
- `docs/specs/vendor/openai/codex/mcp.md`

本地官方 Codex 源码快照：

- `docs/specs/vendor/openai/codex-source/README.md`
- `docs/specs/vendor/openai/codex-source/codex-rs/codex-api/src/common.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/core/src/client.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/tools/src/tool_spec.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/tools/src/responses_api.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/tools/src/tool_search.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/tools/src/tool_discovery.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/protocol/src/models.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/core/src/mcp_tool_exposure.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/core/src/tool_search_spec.rs`
- `docs/specs/vendor/openai/codex-source/codex-rs/core/src/tool_search_handler.rs`

---

## 2. Codex 官方实际请求字段集合

Codex 源码 `ResponsesApiRequest` 实际发送字段：

| 字段 | OpenAI Responses 标准 | Codex 实际使用 | 分类 | 处理建议 |
|---|---:|---:|---|---|
| `model` | 是 | 是 | A | 进 `llm.Request.Model` |
| `instructions` | 是 | 是 | A/B | 可转 system/developer message；Responses→Responses 保留原字段 |
| `input` | 是 | 是 | A/B | 普通 message/content 可进 `llm.Messages`；未知 item raw-preserve |
| `tools` | 是 | 是 | A/B/C | function 可进 `llm.Tool`；Responses-native tool 必须 raw-preserve |
| `tool_choice` | 是 | 是 | A/B | 简单 auto/none 可抽象；复杂对象 raw-preserve |
| `parallel_tool_calls` | 是 | 是 | A | 进通用抽象 |
| `reasoning` | 是 | 是 | A/B | effort/summary 可抽象；完整对象 raw-preserve |
| `store` | 是 | 是 | B | Responses 状态/存储语义，同协议保留；跨协议诊断 |
| `stream` | 是 | 是 | A | 通用流式开关 |
| `include` | 是 | 是 | B | 尤其 `reasoning.encrypted_content` 必须 native 保留 |
| `service_tier` | 是 | 是 | A/B | 通用服务层可抽象；未知值保留 |
| `prompt_cache_key` | 是 | 是 | B | OpenAI Responses 缓存语义，native 保留 |
| `text` | 是 | 是 | A/B | verbosity / format 可部分抽象；完整对象 native 保留 |
| `client_metadata` | 是/官方源码使用 | 是 | B | Codex/Responses 客户端元数据，native 保留，不进通用层 |

Codex Responses Lite 额外形态：

| 字段/item | OpenAI Responses 标准 | Codex 实际使用 | 分类 | 处理建议 |
|---|---:|---:|---|---|
| `input[].type="additional_tools"` | Codex 源码模型支持 | 是 | B | native item，不能压进普通 message |

---

## 3. 顶层请求字段分类

### 3.1 应进入 `llm.Request` 的字段

| Responses 字段 | 通用抽象归宿 | 说明 |
|---|---|---|
| `model` | `llm.Request.Model` | 所有协议共有 |
| `input` 中普通 user/assistant/system/developer message | `llm.Request.Messages` | 跨协议核心语义 |
| `instructions` | `llm.Request.Messages` 的 system/developer 语义 | 跨协议可表达；Responses→Responses 仍应保留原始字段 |
| `temperature` | `llm.Request.Temperature` | 通用采样 |
| `top_p` | `llm.Request.TopP` | 通用采样 |
| `frequency_penalty` | `llm.Request.FrequencyPenalty` | Chat/Responses 可映射 |
| `presence_penalty` | `llm.Request.PresencePenalty` | Chat/Responses 可映射 |
| `max_output_tokens` | `llm.Request.MaxCompletionTokens` / 输出 token 上限 | 跨协议改名映射 |
| `metadata` | `llm.Request.Metadata` | 通用附加元数据；Responses→Responses 保留原结构 |
| `user` | `llm.Request.User` | 通用用户标识 |
| `safety_identifier` | `llm.Request.SafetyIdentifier` | 可抽象为安全用户标识 |
| `service_tier` | `llm.Request.ServiceTier` | 多协议/多 provider 可用，但未知值要保留 |
| `stream` | pipeline/request stream flag | 通用 |
| `parallel_tool_calls` | `llm.Request.ParallelToolCalls` | 通用工具并行意图 |
| `reasoning.effort` | `llm.Request.ReasoningEffort` | 多协议可映射 |
| `reasoning.summary` / `reasoning.generate_summary` | `llm.Request.ReasoningSummary` | 多协议可部分映射 |
| `reasoning.max_tokens` | `llm.Request.ReasoningBudget` | 多协议可部分映射 |
| `text.verbosity` | `llm.Request.Verbosity` | OpenAI 特性，但可作为通用扩展抽象 |
| `text.format` | `llm.Request.ResponseFormat` | 结构化输出通用概念 |
| `tools[].type="function"` | `llm.Tool{Type:function}` | 普通 function calling 跨协议核心语义 |
| 普通 `function_call` | `llm.ToolCall` | 跨协议可表达 |
| `function_call_output` | tool result message / `llm` tool output | 跨协议可表达 |

### 3.2 不应硬塞进 `llm.Request`，应进入 Responses native/raw 层

| Responses 字段/结构 | 标准来源 | 为什么不进 `llm.Request` | 处理建议 |
|---|---|---|---|
| `tools[].type="namespace"` | OpenAI function calling docs / Codex 源码 | Chat/Claude 无等价；flatten 会丢 namespace | Responses→Responses 原样保留；Responses→Chat 才 flatten |
| `tools[].tools[]` namespace 子工具 | OpenAI function calling docs | 需要和 namespace 一起保真 | native tools raw-preserve |
| `tools[].type="tool_search"` | OpenAI tool search docs / Codex 源码 | 不是普通 function；payload 类型不同 | native 保留；跨协议需明确降级或禁用 |
| `tools[].type="custom"` | OpenAI function calling docs / Codex 源码 | freeform 输入输出不是 JSON function 等价物 | native 保留；跨协议按能力降级 |
| `tools[].type="mcp"` | OpenAI MCP docs | 远程 MCP server tool，由 OpenAI/API 侧执行 | native 保留；非 Responses 目标通常不可执行 |
| `tools[].type="file_search"` | OpenAI tools | server tool，无通用 function 等价 | native 保留或诊断丢弃 |
| `tools[].type="code_interpreter"` | OpenAI tools | server tool，无通用 function 等价 | native 保留或诊断丢弃 |
| `tools[].type="computer_use_preview"` | OpenAI tools | tool action 协议特殊 | native 保留或专门适配 |
| `tools[].type="image_generation"` | OpenAI tools / Hub 已部分支持 | 可抽象一部分，但完整字段应保留 | `llm` 可有语义槽；raw 保留完整字段 |
| `tools[].type="web_search"` | OpenAI tools / Codex 源码 | 可抽象一部分，但完整字段应保留 | `llm` 可有语义槽；raw 保留完整字段 |
| `tools[].type="local_shell"` | OpenAI tools | 本地执行语义，不是普通 function | native 保留或专门 executor |
| `tools[].type="shell"` | OpenAI tools | shell tool 协议特殊 | native 保留或专门 executor |
| `tools[].type="apply_patch"` | OpenAI tools | patch tool / custom tool 形态特殊 | native 保留或专门 executor |
| `tools[].defer_loading` | OpenAI tool search docs | 和 tool_search 懒加载绑定 | native 保留；不可丢 |
| `client_metadata` | Codex 源码 `ResponsesApiRequest` | 客户端/会话元数据，不是模型语义 | native 顶层 raw-preserve |
| `include` | OpenAI Responses | 控制额外输出字段，尤其 reasoning encrypted | native 保留 |
| `include[]="reasoning.encrypted_content"` | Codex 源码 | Codex replay/状态保留相关 | 必须保真 |
| `prompt_cache_key` | OpenAI Responses / Codex 源码 | OpenAI prompt cache 语义 | native 保留 |
| `prompt_cache_retention` | OpenAI Responses | OpenAI prompt cache 语义 | native 保留 |
| `previous_response_id` | OpenAI conversation state | Responses 状态管理 | native 保留；跨协议无法等价 |
| `conversation` | OpenAI conversation state | Responses 状态管理 | native 保留；跨协议无法等价 |
| `prompt` | OpenAI prompt template | OpenAI server-side prompt 引用 | native 保留；跨协议不可展开则诊断 |
| `background` | OpenAI Responses | OpenAI async/background 处理语义 | native 保留；跨协议诊断 |
| `store` | OpenAI Responses / Codex 源码 | OpenAI response storage 语义 | native 保留；跨协议诊断 |
| `truncation` | OpenAI Responses | Responses 上下文截断策略 | native 保留 |
| `stream_options` | OpenAI Responses | 流协议细节 | native 保留 |
| 未识别顶层字段 | future OpenAI extension | Hub 当前不能理解，丢弃风险高 | raw-preserve + diagnostic |

### 3.3 非 OpenAI 标准、provider/router 扩展字段

这些不属于 OpenAI 官方 Responses 标准，但可能来自 OpenRouter/第三方/客户端扩展。

| 字段 | 分类 | 处理建议 |
|---|---|---|
| `provider` | provider extension | 不进 OpenAI Responses 标准层；可放 provider-specific extension |
| `route` | router extension | Hub 自己路由层处理，不转给 OpenAI native，除非目标支持 |
| `models` | provider/router extension | 候选模型列表，不是 OpenAI Responses 标准 `model` |
| `plugins` | provider extension | 非 OpenAI Responses 标准 |
| `top_k` | provider extension | 非 OpenAI Responses 标准；部分第三方支持 |
| `min_p` / `top_a` / `repetition_penalty` | provider extension | 不进 OpenAI native 标准层；可入 ExtraBody/provider extension |
| `stop_server_tools_when` | provider extension | 非 OpenAI Responses 标准 |
| `trace` / `debug` | provider/router extension | 诊断/路由层处理 |

---

## 4. `input[]` / `output[]` item 分类

### 4.1 可进入 `llm.Request.Messages` 的 item

| Responses item | 分类 | 说明 |
|---|---|---|
| `message` | A | 普通消息，可映射 role/content |
| `input_text` | A | 普通文本内容 |
| `output_text` | A | 普通 assistant 输出文本 |
| `input_image` | A/B | 多模态通用语义；完整字段 raw 保留 |
| `input_audio` | A/B | 多模态通用语义；完整字段 raw 保留 |
| `function_call` 无 namespace | A | 普通 function call |
| `function_call_output` | A | 普通 tool result |

### 4.2 必须 native/raw 保留的 item

| Responses item | 标准/来源 | 为什么 native 保留 |
|---|---|---|
| `additional_tools` | Codex 源码 | Responses Lite / Codex 注入额外工具；不是普通 message |
| `function_call.namespace` | Codex 源码 / Responses namespace 语义 | 普通 `llm.ToolCall` 若无 namespace 会路由错 MCP |
| `custom_tool_call` | OpenAI custom tool / Codex 源码 | freeform tool，不是 JSON function |
| `custom_tool_call.namespace` | Codex 源码 | namespace 路由语义 |
| `custom_tool_call_output` | OpenAI custom tool / Codex 源码 | freeform output 语义特殊 |
| `tool_search_call` | OpenAI tool search / Codex 源码 | payload 必须是 ToolSearch，不是 Function |
| `tool_search_output` | OpenAI tool search / Codex 源码 | `tools[]` 返回 loadable tool specs，必须保留 namespace/defer_loading |
| `reasoning` | OpenAI reasoning / Codex 源码 | summary/content/encrypted_content 影响 replay |
| `reasoning.encrypted_content` | OpenAI Responses / Codex 源码 | Codex 通过 include 请求；不能丢 |
| `web_search_call` | OpenAI server tool | server tool output item |
| `image_generation_call` | OpenAI server tool | server tool output item |
| `local_shell_call` | OpenAI local shell | 特殊 tool action |
| `compaction` / `compaction_summary` | Codex/Responses state | 上下文压缩状态，不是普通消息 |
| 未知 item type | future extension | raw-preserve + diagnostic |

---

## 5. `tools[]` 细分分类

| Tool type | OpenAI 标准 | Codex 使用 | `llm.Request` | Responses native | 跨协议降级 |
|---|---:|---:|---:|---:|---|
| `function` | 是 | 是 | 是 | 同协议也保留原始 JSON | 直接转目标 function |
| `namespace` | 是 | 是 | 否，除非降级 | 是 | flatten 为 `namespace__tool`，并记录诊断 |
| `tool_search` | 是 | 是 | 否 | 是 | 通常禁用/诊断；不可冒充普通 function，除非目标客户端能处理 |
| `custom` | 是 | 是 | 部分可抽象 | 是 | 可转文本工具/函数，但有损 |
| `web_search` | 是 | 是 | 部分可抽象 | 是 | 目标支持则映射，否则诊断 |
| `image_generation` | 是 | 是 | 部分可抽象 | 是 | 目标支持则映射，否则诊断 |
| `mcp` | 是 | 不等同 Codex 本地 MCP | 否 | 是 | 非 Responses 目标通常不能执行 |
| `file_search` | 是 | 可能 | 否 | 是 | 诊断/禁用 |
| `code_interpreter` | 是 | 可能 | 否 | 是 | 诊断/禁用 |
| `computer_use_preview` | 是 | 可能 | 否 | 是 | 需专门适配 |
| `local_shell` | 是 | 可能 | 否 | 是 | 需专门适配 |
| `shell` | 是 | 可能 | 否 | 是 | 需专门适配 |
| `apply_patch` | 是 | 可能 | 否/部分 | 是 | 需专门适配或 custom 降级 |
| `openrouter:*` | 否，OpenRouter 扩展 | 否 | 否 | provider extension | 只在对应 provider 保留 |
| 未知 type | future extension | 未知 | 否 | 是 | raw-preserve + diagnostic |

---

## 6. 流式事件分类

### 6.1 可进入通用流式抽象

| SSE / event 语义 | 分类 | 说明 |
|---|---|---|
| text delta | A | 普通文本增量 |
| message completed | A | 普通消息完成 |
| function call arguments delta | A/B | 无 namespace 时可通用；有 namespace 时 native 保留 |
| usage | A | token usage 通用 |
| error | A | 错误通用 |

### 6.2 Responses native 流式事件

| SSE / event 语义 | 分类 | 说明 |
|---|---|---|
| `response.output_item.added` / `done` 中未知 item | B | raw-preserve |
| `reasoning` delta / done | B | 保留 summary/content/encrypted_content |
| `tool_search_call` | B | 不能转普通 function payload |
| `tool_search_output` | B | 里面的 tools 必须保真 |
| `function_call.namespace` | B | MCP 路由必需 |
| `custom_tool_call` delta | B | freeform 输入 |
| server tool call events | B | web/image/file/code interpreter 等 |
| `response.completed` 的完整 response metadata | B | 状态/replay/diagnostic |

---

## 7. Hub 当前代码映射建议

现有关键结构：

| Hub 文件/结构 | 当前角色 | 建议 |
|---|---|---|
| `llm/model.go` `llm.Request` | 跨协议 canonical | 保持通用，不扩成完整 Responses AST |
| `llm/transformer/openai/responses/model.go` `Request` / `Tool` / `Item` | Responses 解析模型 | 扩成 OpenAI Responses native 解析模型，补 `client_metadata`、`tool_search`、`additional_tools` 等 |
| `llm/provider_extensions.go` | provider-specific extension | 增强 OpenAI Responses raw/native extension |
| `llm/transformer/openai/responses/request_extensions.go` | raw fragments | 从“少数 raw fragments”升级为“native-preserve 关键字段/未知字段” |
| `internal/server/orchestrator/pass_through.go` | 同协议 raw body pass-through | Responses→Responses 默认优先启用或按 model/channel toolMode 启用 |
| `llm/transformer/openai/responses/inbound.go convertToolsToLLM` | Responses tools → `llm.Tool` | 只在跨协议/降级路径 flatten namespace；同协议不能破坏 native tools |

---

## 7.5 pass-through 与字段修复的边界

`passThroughBody` 可以作为短期缓解和对照基线：如果原始 body 透传正常，而 transformer 重建后异常，就说明丢失发生在 Responses→`llm.Request`→Responses 的重建链路。

但 `passThroughBody` 不是本计划的字段修复本体。字段修复要求：

1. Responses 解析模型能表达或携带标准 Responses 字段；
2. 不能进入 `llm.Request` 的字段进入 Responses native/raw extension；
3. Responses→Responses 即使经过 transformer，也能 re-emit 原始语义；
4. Responses→非 Responses 的有损降级必须显式标记和诊断。

## 8. 推荐执行规则

### 规则 1：OpenAI Responses→OpenAI Responses

默认走：

```text
native/raw preserve
```

允许修改：

- `model`
- base URL / path / headers
- Hub 必须注入的路由/认证字段

不允许默认改写：

- `tools[]`
- `input[]` item 类型
- `tool_search_*`
- `function_call.namespace`
- `reasoning.encrypted_content`
- `client_metadata`
- 未知 OpenAI native 字段

### 规则 2：OpenAI Responses→Chat/Claude/其他

走：

```text
Responses native parse -> llm.Request -> target protocol
```

此时允许降级：

- `namespace` -> flat function
- `custom` -> function/text tool，若目标支持
- `tool_search` -> 禁用/诊断，除非目标有等价协议
- server tools -> 目标支持则映射，否则诊断

### 规则 3：Chat/Claude/其他→OpenAI Responses

走：

```text
llm.Request -> Responses request
```

这不是 Codex native 保真场景，不要求生成 Codex 的全部 native item，但生成结果必须是 OpenAI Responses 标准合法结构。

---

## 8.5 完整保真范围

“做全”在本设计中指：

1. 完整覆盖 OpenAI Responses 标准顶层请求字段；
2. 完整覆盖 OpenAI Responses 标准 tool variants；
3. 完整覆盖 OpenAI Responses 标准 input/output item variants；
4. 完整覆盖 OpenAI Responses 标准流式事件中会影响 replay、tool routing、reasoning 和 server tool 状态的字段；
5. 对当前 Hub 未建模或未来新增的字段提供 raw-preserve 兜底；
6. Responses→Responses round-trip 不丢字段、不压扁身份、不把特殊 tool payload 冒充普通 function；
7. Responses→非 Responses 的不可避免降级必须显式诊断。

Codex 是验收样本之一，但不是唯一目标。验收标准应以 OpenAI Responses native 语义为准。

---

## 8.6 Native AST + raw fallback

完整保真不等于只保存原始字节。Responses native 层采用：

```text
完整 AST + raw fallback
```

要求：

1. OpenAI Responses 当前标准字段进入一等结构化 AST；
2. Codex 高频使用字段也进入一等结构化 AST；
3. 未知顶层字段、未知 tool、未知 item、未知 event 字段进入 raw fallback；
4. 同协议重建时执行 known + raw merge；
5. known 字段由结构化值覆盖同名 raw，以便模型名映射、策略调整和安全修正能生效；
6. raw fallback 不得用于隐藏有损降级，跨协议降级仍需显式诊断。

推荐结构方向：

```go
type OpenAIResponsesNativeRequest struct {
    Known Request
    RawTopLevel map[string]json.RawMessage
    RawTools []json.RawMessage
    RawInputItems []json.RawMessage
    RawToolChoice json.RawMessage
}

type OpenAIResponsesNativeStreamEvent struct {
    Type string
    Known any
    Raw json.RawMessage
}
```


---

## 8.7 架构约束

核心规则：沿作者现有架构做小改，补全 OpenAI/Codex Responses 字段，不把转换层写成屎山。

具体约束：

1. 不把 `llm.Request` 扩成完整 Responses AST；`llm.Request` 只保留跨协议通用抽象。
2. Responses 专属字段放在 Responses transformer / native extension / raw fallback 边界内。
3. 优先复用现有 `responses.Request`、`responses.Tool`、`responses.Item`、`ProviderExtensions`、`request_extensions.go`、orchestrator pass-through 位置。
4. 不为单个 Codex bug 写散落特判；Codex 是验收样本，修的是 OpenAI Responses native round-trip。
5. 任何有损降级必须集中在跨协议转换边界，并带诊断，不在同协议 Responses→Responses 路径静默发生。
6. 如果现有架构无法干净承载某类字段，先记录架构缺口和备选方案，不用临时字段、字符串拼接或隐式约定硬塞。
7. 新代码以字段保真和边界清晰为完成标准，避免“顺手重构”无关模块。


---

## 8.8 实施阶段边界

完整目标覆盖 request、response 和 stream，但第一阶段只实现 OpenAI Responses request native round-trip。

第一阶段范围：

1. 顶层 request 字段；
2. `tools[]` 及所有标准 tool variants；
3. `input[]` 及 request 侧 item variants；
4. `tool_choice`；
5. request 内部未知字段 raw fallback；
6. Responses→Responses 出站重建不丢 request 语义；
7. Responses→非 Responses 的 request 降级诊断。

第一阶段暂不处理：

1. response body 完整 native round-trip；
2. SSE stream event 完整 native round-trip；
3. UI 配置；
4. 非必要计费/统计重构；
5. 无关协议重构。

后续阶段再扩展 response item 与 stream event native round-trip。


---

## 8.9 第一阶段代码落点

采用方案 A：沿用现有 `llm/transformer/openai/responses` 包小改，不新建独立 Responses protocol framework。

推荐拆分：

```text
llm/transformer/openai/responses/
  model.go             # 现有协议结构，补字段
  native_request.go    # request native round-trip 和 raw fallback
  native_tool.go       # tools[] known/raw 处理
  native_input.go      # input[] known/raw 处理
  native_merge.go      # known + raw merge
  request_extensions.go# ProviderExtensions bridge
  inbound.go           # Responses -> llm.Request / native extension
  outbound.go          # llm.Request/native -> Responses
```

约束：

1. 不新建 `llm/protocol/openairesponses` 之类大框架；
2. 不把所有代码继续堆进 `model.go`；
3. 不把 Responses-only 字段塞进 `llm.Request`；
4. 如果 response/stream 阶段证明包边界不够，再单独提 ADR 抽出 native protocol 包。


---

## 9. 实施优先级

最终范围是完整 OpenAI Responses native round-trip。下面的 P0/P1 只表示实施顺序：先修会直接破坏 Codex/MCP 的字段，再补齐其余标准字段和未知字段兜底；不能把 P0/P1 理解成最终裁剪范围。

| 优先级 | 字段/结构 | 原因 |
|---:|---|---|
| P0 | `tools[].type="namespace"` 原样保留 | MCP 调用路由核心 |
| P0 | `function_call.namespace` 原样保留 | replay / MCP resolver 需要 |
| P0 | `tools[].type="tool_search"` 原样保留 | MCP 懒加载核心 |
| P0 | `tool_search_call` / `tool_search_output` | tool_search 完整闭环 |
| P0 | `input[].type="additional_tools"` | Codex Responses Lite / loaded tools 注入 |
| P0 | `include=["reasoning.encrypted_content"]` | reasoning/replay 保留 |
| P1 | `client_metadata` | Codex request metadata |
| P1 | `tools[].defer_loading` | tool_search 能否省 token 的关键 |
| P1 | `custom_tool_call(.namespace)` | freeform/apply_patch 类工具 |
| P1 | 未知 tools/item 顶层 raw-preserve | 防未来字段继续坏 |

