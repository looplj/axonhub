# 最终字段归属矩阵

本矩阵把三协议全字段列表转成实现地图：每个字段属于哪里、作者缺失时怎么补、是否需要新 module、属于哪个实现阶段。

## 总规则

- 不新增全局万能 converter。
- 单个字段缺失：优先补对应协议 native struct / preservation。
- 多字段共享同一套保真规则：在协议 package 内加深 seam。
- 多 provider 共享 builder 且能力不同：后续引入小型 emission policy。
- 未知字段：同协议 raw fallback；跨协议 LossyDowngrade diagnostic。
- stream event：stream fidelity module；不塞 request/response body model。

## 第一轮实施边界

| 切片 | 范围 | 不做 |
|---|---|---|
| P1a | OpenAI Responses request 顶层官方字段：`conversation`, `context_management`, `prompt` | 不碰 Chat / Anthropic / stream / tools 复杂变体 |
| P1b | OpenAI Responses unknown top-level same-protocol raw fallback | 不跨协议透传 |
| P1c | OpenAI Responses tool variants / raw tools：从 `tools`, `tool_choice` 入手 | 不改共享 Chat `RequestFromLLM` |
| P1d | Codex Responses profile：`tool_search`, `defer_loading`, `additional_tools`, `namespace` 等 | 不建独立 Codex 私有协议 |

## OpenAI Responses request 字段归属

| 字段 | 类别 | 作者 upstream | 当前分支 | 作者处理摘要 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|---|---|
| `metadata` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `top_logprobs` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `temperature` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `top_p` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `user` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `safety_identifier` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `prompt_cache_key` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `service_tier` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `prompt_cache_retention` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `previous_response_id` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `model` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `reasoning` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `background` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `max_tool_calls` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `text` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `tools` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 写 targeted round-trip 测试；最小实现；不进 llm.Request；不碰 Chat/Anthropic/Stream。 | 不新建全局类；在 Responses native tool preservation 内收口 | P1c 第一轮 |
| `tool_choice` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 写 targeted round-trip 测试；最小实现；不进 llm.Request；不碰 Chat/Anthropic/Stream。 | 不新建全局类；在 Responses native tool preservation 内收口 | P1c 第一轮 |
| `prompt` | 跨协议通用 + Responses native emission | 缺 | 有 | nested/response/helper struct only | llm.Request/Response common + responses emitter | 写 targeted round-trip 测试；最小实现；不进 llm.Request；不碰 Chat/Anthropic/Stream。 | 否；补 Responses native 字段/保真路径 | P1a 第一轮 |
| `truncation` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `input` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `include` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `parallel_tool_calls` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `store` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `instructions` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `stream` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `stream_options` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `conversation` | OpenAI Responses 官方 native 字段 | 缺 | 缺 | nested/response/helper struct only | llm/transformer/openai/responses native preservation | 写 targeted round-trip 测试；最小实现；不进 llm.Request；不碰 Chat/Anthropic/Stream。 | 否；补 Responses native 字段/保真路径 | P1a 第一轮 |
| `context_management` | OpenAI Responses 官方 native 字段 | 缺 | 缺 | missing in upstream request; should be native/opaque request… | llm/transformer/openai/responses native preservation | 写 targeted round-trip 测试；最小实现；不进 llm.Request；不碰 Chat/Anthropic/Stream。 | 否；补 Responses native 字段/保真路径 | P1a 第一轮 |
| `max_output_tokens` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |

## OpenAI Responses response 字段归属

| 字段 | 类别 | 作者 upstream | 当前分支 | 作者处理摘要 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|---|---|
| `metadata` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `top_logprobs` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `temperature` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `top_p` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `user` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `safety_identifier` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `prompt_cache_key` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `service_tier` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `prompt_cache_retention` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `previous_response_id` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `model` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `reasoning` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `background` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `max_tool_calls` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `text` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `tools` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `tool_choice` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `prompt` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `truncation` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 后续 Responses native 扩展 |
| `id` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `object` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `status` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `created_at` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `completed_at` | OpenAI Responses 官方 native 字段 | 缺 | 缺 | missing/not modeled | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `error` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `incomplete_details` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `output` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `instructions` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `output_text` | OpenAI Responses 官方 native 字段 | 缺 | 缺 | missing/not modeled | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `usage` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `parallel_tool_calls` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `conversation` | OpenAI Responses 官方 native 字段 | 有 | 有 | native top-level struct | llm/transformer/openai/responses native preservation | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |
| `max_output_tokens` | 跨协议通用 + Responses native emission | 有 | 有 | native top-level struct | llm.Request/Response common + responses emitter | 同协议保真优先；缺失时补 native/raw；跨协议无等价则 diagnostic。 | 否，除非删除测试证明复杂度扩散 | 保持/按需验证 |

## OpenAI Responses unknown / Codex profile 归属

| 字段/族 | 类别 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|
| unknown top-level request fields | OpenAI Responses future/native raw | Responses same-protocol raw fallback | 只同协议重放；跨协议 diagnostic | 否；收口 raw merge 规则 | P1b 第一轮 |
| `tool_search` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |
| `defer_loading` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |
| `additional_tools` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |
| `namespace` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |
| `tool_search_call` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |
| `tool_search_output` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |
| `function_call.namespace` | CodexResponsesProfile inside OpenAIResponsesNative | Responses native preservation / Codex profile | 基于 raw payload 证据保真；不进 llm.Request | 否；不建私有协议 | P1d 第一轮 |

## OpenAI Responses stream/event 归属

| Schema | Event type | 作者 upstream | 当前分支 | 正确归属 | 实现阶段 |
|---|---|---|---|---|---|
| `ResponseAudioDeltaEvent` | `response.audio.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseAudioDoneEvent` | `response.audio.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseAudioTranscriptDeltaEvent` | `response.audio.transcript.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseAudioTranscriptDoneEvent` | `response.audio.transcript.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCodeInterpreterCallCodeDeltaEvent` | `response.code_interpreter_call_code.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCodeInterpreterCallCodeDoneEvent` | `response.code_interpreter_call_code.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCodeInterpreterCallCompletedEvent` | `response.code_interpreter_call.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCodeInterpreterCallInProgressEvent` | `response.code_interpreter_call.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCodeInterpreterCallInterpretingEvent` | `response.code_interpreter_call.interpreting` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCompletedEvent` | `response.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseContentPartAddedEvent` | `response.content_part.added` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseContentPartDoneEvent` | `response.content_part.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCreatedEvent` | `response.created` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCustomToolCallInputDeltaEvent` | `response.custom_tool_call_input.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseCustomToolCallInputDoneEvent` | `response.custom_tool_call_input.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseErrorEvent` | `error` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseFailedEvent` | `response.failed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseFileSearchCallCompletedEvent` | `response.file_search_call.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseFileSearchCallInProgressEvent` | `response.file_search_call.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseFileSearchCallSearchingEvent` | `response.file_search_call.searching` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseFunctionCallArgumentsDeltaEvent` | `response.function_call_arguments.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseFunctionCallArgumentsDoneEvent` | `response.function_call_arguments.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseImageGenCallCompletedEvent` | `response.image_generation_call.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseImageGenCallGeneratingEvent` | `response.image_generation_call.generating` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseImageGenCallInProgressEvent` | `response.image_generation_call.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseImageGenCallPartialImageEvent` | `response.image_generation_call.partial_image` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseInProgressEvent` | `response.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseIncompleteEvent` | `response.incomplete` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPCallArgumentsDeltaEvent` | `response.mcp_call_arguments.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPCallArgumentsDoneEvent` | `response.mcp_call_arguments.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPCallCompletedEvent` | `response.mcp_call.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPCallFailedEvent` | `response.mcp_call.failed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPCallInProgressEvent` | `response.mcp_call.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPListToolsCompletedEvent` | `response.mcp_list_tools.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPListToolsFailedEvent` | `response.mcp_list_tools.failed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseMCPListToolsInProgressEvent` | `response.mcp_list_tools.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseOutputItemAddedEvent` | `response.output_item.added` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseOutputItemDoneEvent` | `response.output_item.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseOutputTextAnnotationAddedEvent` | `response.output_text.annotation.added` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseQueuedEvent` | `response.queued` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseReasoningSummaryPartAddedEvent` | `response.reasoning_summary_part.added` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseReasoningSummaryPartDoneEvent` | `response.reasoning_summary_part.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseReasoningSummaryTextDeltaEvent` | `response.reasoning_summary_text.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseReasoningSummaryTextDoneEvent` | `response.reasoning_summary_text.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseReasoningTextDeltaEvent` | `response.reasoning_text.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseReasoningTextDoneEvent` | `response.reasoning_text.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseRefusalDeltaEvent` | `response.refusal.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseRefusalDoneEvent` | `response.refusal.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseStreamEvent` | `response.custom_tool_call_input.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseTextDeltaEvent` | `response.output_text.delta` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseTextDoneEvent` | `response.output_text.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseWebSearchCallCompletedEvent` | `response.web_search_call.completed` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseWebSearchCallInProgressEvent` | `response.web_search_call.in_progress` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponseWebSearchCallSearchingEvent` | `response.web_search_call.searching` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponsesClientEvent` | `response.create` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponsesClientEventResponseCreate` | `response.create` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |
| `ResponsesServerEvent` | `response.custom_tool_call_input.done` | 缺 | 缺 | Responses stream fidelity module | 后续 Stream 模块 |

## OpenAI Chat request 字段归属

| 字段 | 类别 | 作者 upstream | 当前分支 | 作者处理摘要 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|---|---|
| `metadata` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `top_logprobs` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `temperature` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `top_p` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `user` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `safety_identifier` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `prompt_cache_key` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `service_tier` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `prompt_cache_retention` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | nested/response/helper struct only | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `messages` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `model` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `modalities` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `verbosity` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `reasoning_effort` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `max_completion_tokens` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `frequency_penalty` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `presence_penalty` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `web_search_options` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | missing in upstream request; modern Chat native field candid… | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `response_format` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `audio` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | nested/response/helper struct only | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `store` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `stream` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `stop` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `logit_bias` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `logprobs` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `max_tokens` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `n` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | missing/not modeled | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `prediction` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | missing in upstream request; modern Chat native field candid… | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `seed` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `stream_options` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `tools` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `tool_choice` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `parallel_tool_calls` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `function_call` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | missing/not modeled | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `functions` | OpenAI Chat 官方 native 字段 | 缺 | 缺 | missing/not modeled | OpenAI Chat native model / Chat emission policy | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |

## OpenAI Chat response 字段归属

| 字段 | 类别 | 作者 upstream | 当前分支 | 作者处理摘要 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|---|---|
| `id` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `choices` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `created` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `model` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `service_tier` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `system_fingerprint` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `object` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |
| `usage` | 跨协议通用 + Chat native emission | 有 | 有 | native top-level struct | llm common + OpenAI Chat native model | 延后；先列入 Chat native/emission policy，避免污染所有 OpenAI-compatible provider。 | 可能；后续小型 Chat emission policy，不是万能 converter | 后续 Chat 模块 |

## OpenAI Chat stream schema 归属

| Schema | 作者 upstream | 当前分支 | 正确归属 | 实现阶段 |
|---|---|---|---|---|
| `ChatCompletionMessageToolCallChunk` | 缺 | 缺 | Chat stream fidelity module | 后续 Stream/Chat 模块 |
| `ChatCompletionStreamOptions` | 缺 | 缺 | Chat stream fidelity module | 后续 Stream/Chat 模块 |
| `ChatCompletionStreamResponseDelta` | 缺 | 缺 | Chat stream fidelity module | 后续 Stream/Chat 模块 |
| `CreateChatCompletionStreamResponse` | 缺 | 缺 | Chat stream fidelity module | 后续 Stream/Chat 模块 |

## Anthropic Messages request 字段归属

| 字段 | 类别 | 作者 upstream | 当前分支 | 作者处理摘要 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|---|---|
| `max_tokens` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `messages` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `model` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `container` | Anthropic 官方 native 字段 | 缺 | 缺 | missing or companion-native field candidate | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `inference_geo` | Anthropic 官方 native 字段 | 缺 | 缺 | missing or companion-native field candidate | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `metadata` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `output_config` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `service_tier` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `stop_sequences` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `stream` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `system` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `temperature` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `thinking` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `tool_choice` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `tools` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `top_k` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `top_p` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |

## Anthropic Message response 字段归属

| 字段 | 类别 | 作者 upstream | 当前分支 | 作者处理摘要 | 正确归属 | 作者缺失时补法 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|---|---|
| `id` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `container` | Anthropic 官方 native 字段 | 缺 | 缺 | missing or companion-native field candidate | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `content` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `model` | 跨协议通用 + Anthropic native emission | 有 | 有 | native top-level struct | llm common + Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `role` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `stop_details` | Anthropic 官方 native 字段 | 缺 | 缺 | missing/not modeled | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `stop_reason` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `stop_sequence` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `type` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |
| `usage` | Anthropic 官方 native 字段 | 有 | 有 | native top-level struct | Anthropic native model / Anthropic adapter | 延后；保留在 Anthropic native/companion，不和 OpenAI MCP 混。 | 否；优先 Anthropic native adapter | 后续 Anthropic 模块 |

## Anthropic stream / MCP companion 归属

| 字段/事件 | 类别 | 作者 upstream | 当前分支 | 正确归属 | 是否新 module | 实现阶段 |
|---|---|---|---|---|---|---|
| `message_start` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `content_block_start` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `content_block_delta` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `content_block_stop` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `message_delta` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `message_stop` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `ping` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `error` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `text_delta` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `input_json_delta` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `thinking_delta` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `signature_delta` | Anthropic stream | 有 | 有 | Anthropic stream fidelity module | 可能，stream 层 | 后续 Stream 模块 |
| `mcp_servers` | Anthropic MCP connector companion | 未比对 | 未比对 | Anthropic native/companion adapter | 否；先放 Anthropic native | 后续 Anthropic 模块 |
| `tools[].type=mcp_toolset` | Anthropic MCP connector companion | 未比对 | 未比对 | Anthropic native/companion adapter | 否；先放 Anthropic native | 后续 Anthropic 模块 |
| `mcp_servers[].name` | Anthropic MCP connector companion | 未比对 | 未比对 | Anthropic native/companion adapter | 否；先放 Anthropic native | 后续 Anthropic 模块 |
| `mcp_servers[].url` | Anthropic MCP connector companion | 未比对 | 未比对 | Anthropic native/companion adapter | 否；先放 Anthropic native | 后续 Anthropic 模块 |
| `mcp_servers[].authorization_token` | Anthropic MCP connector companion | 未比对 | 未比对 | Anthropic native/companion adapter | 否；先放 Anthropic native | 后续 Anthropic 模块 |
| `mcp_servers[].tool_configuration` | Anthropic MCP connector companion | 未比对 | 未比对 | Anthropic native/companion adapter | 否；先放 Anthropic native | 后续 Anthropic 模块 |

## 后续实现优先级总结

1. **P1a**：只做 `conversation` / `context_management` / `prompt`。
2. **P1b**：只做 OpenAI Responses same-protocol unknown top-level raw fallback。
3. **P1c**：只做 OpenAI Responses tools/tool_choice 的 native/raw 保真，不碰 Chat shared builder。
4. **P1d**：只做 CodexResponsesProfile inside Responses native preservation。
5. **P2 Chat**：再处理 Chat native 和 emission policy。
6. **P3 Anthropic**：再处理 Anthropic native / MCP companion。
7. **P4 Diagnostics**：集中 LossyDowngrade。
8. **P5 Stream**：单独 stream fidelity。

## 实现前检查

- 如果一个字段在本矩阵中标为后续模块，当前切片不得实现。
- 如果一个字段标为 P1a/P1b/P1c/P1d，必须先写 targeted failing test。
- 如果实现需要新增 module，必须先通过删除测试和 locality/leverage 判断。
- 如果字段没有明确归属，回到 planning，不进入 TDD。