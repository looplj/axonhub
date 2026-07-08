# 三协议全字段列表与字段归属决策树

本文件基于已抓取/解析的官方协议字段文档生成，作为实现前的字段地图。它不是代码实现，不代表所有字段都要第一轮实现。

## 结论先行

- 当前架构方向是对的：保留作者 transformer 框架，不新增全局万能 converter。
- 当前还不能写业务代码：必须先用本列表确定字段归属，再进入 P1a TDD。
- 作者缺字段时，不是统一“开新类”：先按协议族、字段语义、same-protocol/cross-protocol 行为决策。
- 第一轮只做 OpenAI Responses -> OpenAI Responses native preservation；Chat、Anthropic、stream 暂不实现。

## 协议字段决策树

```text
遇到一个作者没有/处理不清的字段
  ↓
1. 这个字段是不是多个协议稳定等价的语义？
   ├─ 是：进入 CrossProtocolCanonical（llm.Request / llm.Response）或 common 映射；必须证明多个协议等价。
   └─ 否：继续
  ↓
2. 它是不是某个协议官方字段？
   ├─ OpenAI Responses：放 OpenAI Responses native preservation。
   ├─ OpenAI Chat：放 OpenAI Chat native model / emission policy。
   ├─ Anthropic：放 Anthropic native model / adapter。
   └─ 否：继续
  ↓
3. 它是不是某个客户端/供应商使用画像字段？
   ├─ Codex Responses profile：作为 OpenAIResponsesNative 内的 profile 保真。
   ├─ Provider 私有字段：放 provider adapter / ProviderExtensions。
   └─ 否：继续
  ↓
4. 它是不是未知 future field？
   ├─ 同协议：raw fallback 保留。
   └─ 跨协议：禁止静默透传，走 LossyDowngrade diagnostic。
  ↓
5. 它是不是 stream event？
   ├─ 是：放 stream event fidelity module，不能塞 request/response body model。
   └─ 否：记录 deliberate unsupported / diagnostic。
```

## 什么时候开新 module / 类

| 判断 | 结论 |
|---|---|
| 只有一个字段缺失 | 不开新 module，优先补该协议 native struct / preservation。 |
| 多个字段共享同一套 same-protocol 保真规则 | 可在协议 package 内加深 native preservation seam。 |
| 多个 provider 复用同一出站 builder，但能力不同 | 可新增小型 emission policy，不要扩大共享 builder。 |
| 跨协议字段无法表达 | 不开转换类假装能转；记录 LossyDowngrade diagnostic。 |
| stream event 很多且解析/聚合复杂 | 单独 stream fidelity module，独立于 request 字段。 |
| 只是为了“看起来架构化” | 不开，避免 wrapper/adapter 屎山。 |

## OpenAI Responses request 顶层字段

| 字段 | 必填 | 类型 | 含义摘要 | 作者 upstream | 当前分支 | 建议归属 | 作者缺失/不清时处理 |
|---|---:|---|---|---|---|---|---|
| `metadata` | 否 | `Metadata` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_logprobs` | 否 | `anyOf(integer | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `temperature` | 否 | `anyOf(number | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_p` | 否 | `anyOf(number | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `user` | 否 | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` i… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `safety_identifier` | 否 | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usa… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt_cache_key` | 否 | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces th… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `service_tier` | 否 | `ServiceTier` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt_cache_retention` | 否 | `anyOf(string | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `previous_response_id` | 否 | `anyOf(string | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `model` | 否 | `ModelIdsResponses` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models … | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `reasoning` | 否 | `anyOf(Reasoning | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `background` | 否 | `anyOf(boolean | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `max_tool_calls` | 否 | `anyOf(integer | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `text` | 否 | `ResponseTextParam` | Configuration options for a text response from the model. Can be plain text or structured JSON data.… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `tools` | 否 | `ToolsArray` | An array of tools the model may call while generating a response. You can specify which tool to use … | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `tool_choice` | 否 | `ToolChoiceParam` | How the model should select which tool (or tools) to use when generating a response. See the `tools`… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt` | 否 | `Prompt` |  | 缺 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `truncation` | 否 | `anyOf(string | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `input` | 否 | `InputParam` | Text, image, or file inputs to the model, used to generate a response. Learn more: - [Text inputs an… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `include` | 否 | `anyOf(array[IncludeEnum] | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `parallel_tool_calls` | 否 | `anyOf(boolean | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `store` | 否 | `anyOf(boolean | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `instructions` | 否 | `anyOf(string | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `stream` | 否 | `anyOf(boolean | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `stream_options` | 否 | `ResponseStreamOptions` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `conversation` | 否 | `anyOf(ConversationParam | null)` |  | 缺 | 缺 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `context_management` | 否 | `anyOf(array[ContextManagementParam] | null)` |  | 缺 | 缺 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `max_output_tokens` | 否 | `anyOf(integer | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |

## OpenAI Responses response 顶层字段

| 字段 | 必填 | 类型 | 含义摘要 | 作者 upstream | 当前分支 | 建议归属 | 作者缺失/不清时处理 |
|---|---:|---|---|---|---|---|---|
| `metadata` | 是 | `Metadata` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_logprobs` | 否 | `anyOf(integer | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `temperature` | 是 | `anyOf(number | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_p` | 是 | `anyOf(number | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `user` | 否 | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` i… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `safety_identifier` | 否 | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usa… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt_cache_key` | 否 | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces th… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `service_tier` | 否 | `ServiceTier` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt_cache_retention` | 否 | `anyOf(string | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `previous_response_id` | 否 | `anyOf(string | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `model` | 是 | `ModelIdsResponses` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models … | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `reasoning` | 否 | `anyOf(Reasoning | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `background` | 否 | `anyOf(boolean | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `max_tool_calls` | 否 | `anyOf(integer | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `text` | 否 | `ResponseTextParam` | Configuration options for a text response from the model. Can be plain text or structured JSON data.… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `tools` | 是 | `ToolsArray` | An array of tools the model may call while generating a response. You can specify which tool to use … | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `tool_choice` | 是 | `ToolChoiceParam` | How the model should select which tool (or tools) to use when generating a response. See the `tools`… | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt` | 否 | `Prompt` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `truncation` | 否 | `anyOf(string | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `id` | 是 | `string` | Unique identifier for this Response. | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `object` | 是 | `string` | The object type of this resource - always set to `response`. | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `status` | 否 | `string` | The status of the response generation. One of `completed`, `failed`, `in_progress`, `cancelled`, `qu… | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `created_at` | 是 | `number` | Unix timestamp (in seconds) of when this Response was created. | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `completed_at` | 否 | `anyOf(number | null)` |  | 缺 | 缺 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `error` | 是 | `ResponseError` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `incomplete_details` | 是 | `anyOf(object | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `output` | 是 | `array[OutputItem]` | An array of content items generated by the model. - The length and order of items in the `output` ar… | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `instructions` | 是 | `anyOf(oneOf(string | array[InputItem]) | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `output_text` | 否 | `anyOf(string | null)` |  | 缺 | 缺 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `usage` | 否 | `ResponseUsage` | Represents token usage details including input tokens, output tokens, a breakdown of output tokens, … | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `parallel_tool_calls` | 是 | `boolean` | Whether to allow the model to run tool calls in parallel. | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `conversation` | 否 | `anyOf(Conversation-2 | null)` |  | 有 | 有 | OpenAI Responses native preservation | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `max_output_tokens` | 否 | `anyOf(integer | null)` |  | 有 | 有 | CrossProtocolCanonical + Responses native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |

## OpenAI Responses stream/event 字段

| Schema | Event type | 作者 upstream | 当前分支 | 建议归属 | 处理 |
|---|---|---|---|---|---|
| `ResponseAudioDeltaEvent` | `response.audio.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseAudioDoneEvent` | `response.audio.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseAudioTranscriptDeltaEvent` | `response.audio.transcript.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseAudioTranscriptDoneEvent` | `response.audio.transcript.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCodeInterpreterCallCodeDeltaEvent` | `response.code_interpreter_call_code.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCodeInterpreterCallCodeDoneEvent` | `response.code_interpreter_call_code.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCodeInterpreterCallCompletedEvent` | `response.code_interpreter_call.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCodeInterpreterCallInProgressEvent` | `response.code_interpreter_call.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCodeInterpreterCallInterpretingEvent` | `response.code_interpreter_call.interpreting` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCompletedEvent` | `response.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseContentPartAddedEvent` | `response.content_part.added` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseContentPartDoneEvent` | `response.content_part.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCreatedEvent` | `response.created` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCustomToolCallInputDeltaEvent` | `response.custom_tool_call_input.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseCustomToolCallInputDoneEvent` | `response.custom_tool_call_input.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseErrorEvent` | `error` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseFailedEvent` | `response.failed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseFileSearchCallCompletedEvent` | `response.file_search_call.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseFileSearchCallInProgressEvent` | `response.file_search_call.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseFileSearchCallSearchingEvent` | `response.file_search_call.searching` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseFunctionCallArgumentsDeltaEvent` | `response.function_call_arguments.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseFunctionCallArgumentsDoneEvent` | `response.function_call_arguments.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseImageGenCallCompletedEvent` | `response.image_generation_call.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseImageGenCallGeneratingEvent` | `response.image_generation_call.generating` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseImageGenCallInProgressEvent` | `response.image_generation_call.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseImageGenCallPartialImageEvent` | `response.image_generation_call.partial_image` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseInProgressEvent` | `response.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseIncompleteEvent` | `response.incomplete` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPCallArgumentsDeltaEvent` | `response.mcp_call_arguments.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPCallArgumentsDoneEvent` | `response.mcp_call_arguments.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPCallCompletedEvent` | `response.mcp_call.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPCallFailedEvent` | `response.mcp_call.failed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPCallInProgressEvent` | `response.mcp_call.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPListToolsCompletedEvent` | `response.mcp_list_tools.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPListToolsFailedEvent` | `response.mcp_list_tools.failed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseMCPListToolsInProgressEvent` | `response.mcp_list_tools.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseOutputItemAddedEvent` | `response.output_item.added` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseOutputItemDoneEvent` | `response.output_item.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseOutputTextAnnotationAddedEvent` | `response.output_text.annotation.added` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseQueuedEvent` | `response.queued` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseReasoningSummaryPartAddedEvent` | `response.reasoning_summary_part.added` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseReasoningSummaryPartDoneEvent` | `response.reasoning_summary_part.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseReasoningSummaryTextDeltaEvent` | `response.reasoning_summary_text.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseReasoningSummaryTextDoneEvent` | `response.reasoning_summary_text.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseReasoningTextDeltaEvent` | `response.reasoning_text.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseReasoningTextDoneEvent` | `response.reasoning_text.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseRefusalDeltaEvent` | `response.refusal.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseRefusalDoneEvent` | `response.refusal.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseStreamEvent` | `response.custom_tool_call_input.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseTextDeltaEvent` | `response.output_text.delta` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseTextDoneEvent` | `response.output_text.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseWebSearchCallCompletedEvent` | `response.web_search_call.completed` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseWebSearchCallInProgressEvent` | `response.web_search_call.in_progress` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponseWebSearchCallSearchingEvent` | `response.web_search_call.searching` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponsesClientEvent` | `response.create` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponsesClientEventResponseCreate` | `response.create` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |
| `ResponsesServerEvent` | `response.custom_tool_call_input.done` | 缺 | 缺 | Responses stream event fidelity module | stream 层解析/聚合/重发；不塞 request model。 |

## OpenAI Chat request 顶层字段

| 字段 | 必填 | 类型 | 含义摘要 | 作者 upstream | 当前分支 | 建议归属 | 作者缺失/不清时处理 |
|---|---:|---|---|---|---|---|---|
| `metadata` | 否 | `Metadata` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_logprobs` | 否 | `anyOf(integer | null)` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `temperature` | 否 | `anyOf(number | null)` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_p` | 否 | `anyOf(number | null)` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `user` | 否 | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` i… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `safety_identifier` | 否 | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usa… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt_cache_key` | 否 | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces th… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `service_tier` | 否 | `ServiceTier` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `prompt_cache_retention` | 否 | `anyOf(string | null)` |  | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `messages` | 是 | `array[ChatCompletionRequestMessage]` | A list of messages comprising the conversation so far. Depending on the [model](/docs/models) you us… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `model` | 是 | `ModelIdsShared` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models … | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `modalities` | 否 | `ResponseModalities` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `verbosity` | 否 | `Verbosity` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `reasoning_effort` | 否 | `ReasoningEffort` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `max_completion_tokens` | 否 | `integer` | An upper bound for the number of tokens that can be generated for a completion, including visible ou… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `frequency_penalty` | 否 | `number` | Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency i… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `presence_penalty` | 否 | `number` | Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `web_search_options` | 否 | `object` | This tool searches the web for relevant results to use in a response. Learn more about the [web sear… | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `response_format` | 否 | `oneOf(ResponseFormatText | ResponseFormatJsonSchema | ResponseFormatJsonObject)` | An object specifying the format that the model must output. Setting to `{ "type": "json_schema", "js… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `audio` | 否 | `object` | Parameters for audio output. Required when audio output is requested with `modalities: ["audio"]`. [… | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `store` | 否 | `boolean` | Whether or not to store the output of this chat completion request for use in our [model distillatio… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `stream` | 否 | `boolean` | If set to true, the model response data will be streamed to the client as it is generated using [ser… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `stop` | 否 | `StopConfiguration` | Not supported with latest reasoning models `o3` and `o4-mini`. Up to 4 sequences where the API will … | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `logit_bias` | 否 | `object` | Modify the likelihood of specified tokens appearing in the completion. Accepts a JSON object that ma… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `logprobs` | 否 | `boolean` | Whether to return log probabilities of the output tokens or not. If true, returns the log probabilit… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `max_tokens` | 否 | `integer` | The maximum number of [tokens](/tokenizer) that can be generated in the chat completion. This value … | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `n` | 否 | `integer` | How many chat completion choices to generate for each input message. Note that you will be charged b… | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `prediction` | 否 | `oneOf(PredictionContent)` | Configuration for a [Predicted Output](/docs/guides/predicted-outputs), which can greatly improve re… | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `seed` | 否 | `integer` | This feature is in Beta. If specified, our system will make a best effort to sample deterministicall… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `stream_options` | 否 | `ChatCompletionStreamOptions` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `tools` | 否 | `array[oneOf(ChatCompletionTool | CustomToolChatCompletions)]` | A list of tools the model may call. You can provide either [custom tools](/docs/guides/function-call… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `tool_choice` | 否 | `ChatCompletionToolChoiceOption` | Controls which (if any) tool is called by the model. `none` means the model will not call any tool a… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `parallel_tool_calls` | 否 | `ParallelToolCalls` | Whether to enable [parallel function calling](/docs/guides/function-calling#configuring-parallel-fun… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `function_call` | 否 | `oneOf(string | ChatCompletionFunctionCallOption)` | Deprecated in favor of `tool_choice`. Controls which (if any) function is called by the model. `none… | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `functions` | 否 | `array[ChatCompletionFunctions]` | Deprecated in favor of `tools`. A list of functions the model may generate JSON inputs for. | 缺 | 缺 | OpenAI Chat native model / emission policy | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |

## OpenAI Chat response 顶层字段

| 字段 | 必填 | 类型 | 含义摘要 | 作者 upstream | 当前分支 | 建议归属 | 作者缺失/不清时处理 |
|---|---:|---|---|---|---|---|---|
| `id` | 是 | `string` | A unique identifier for the chat completion. | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `choices` | 是 | `array[object]` | A list of chat completion choices. Can be more than one if `n` is greater than 1. | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `created` | 是 | `integer` | The Unix timestamp (in seconds) of when the chat completion was created. | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `model` | 是 | `string` | The model used for the chat completion. | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `service_tier` | 否 | `ServiceTier` |  | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `system_fingerprint` | 否 | `string` | This fingerprint represents the backend configuration that the model runs with. Can be used in conju… | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `object` | 是 | `string` | The object type, which is always `chat.completion`. | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `usage` | 否 | `CompletionUsage` | Usage statistics for the completion request. | 有 | 有 | CrossProtocolCanonical + Chat native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |

## OpenAI Chat stream schema

| Schema | 作者 upstream | 当前分支 | 建议归属 | 处理 |
|---|---|---|---|---|
| `ChatCompletionMessageToolCallChunk` | 缺 | 缺 | Chat stream fidelity module | stream 层处理；不要污染 Chat request model。 |
| `ChatCompletionStreamOptions` | 缺 | 缺 | Chat stream fidelity module | stream 层处理；不要污染 Chat request model。 |
| `ChatCompletionStreamResponseDelta` | 缺 | 缺 | Chat stream fidelity module | stream 层处理；不要污染 Chat request model。 |
| `CreateChatCompletionStreamResponse` | 缺 | 缺 | Chat stream fidelity module | stream 层处理；不要污染 Chat request model。 |

## Anthropic Messages request 顶层字段

| 字段 | 必填 | 类型 | 含义摘要 | 作者 upstream | 当前分支 | 建议归属 | 作者缺失/不清时处理 |
|---|---:|---|---|---|---|---|---|
| `max_tokens` | 是 | `number` | `max_tokens: number` The maximum number of tokens to generate before stopping. Note that our models … | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `messages` | 是 | `array[MessageParam]` | `messages: array of MessageParam` Input messages. Our models are trained to operate on alternating `… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `model` | 是 | `Model` | `model: Model` The model that will complete your prompt. See [models](https://docs.anthropic.com/en/… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `container` | 否 | `string` | `container: optional string` Container identifier for reuse across requests. - `inference_geo: optio… | 缺 | 缺 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `inference_geo` | 否 | `string` | `inference_geo: optional string` Specifies the geographic region for inference processing. If not sp… | 缺 | 缺 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `metadata` | 否 | `Metadata` | `metadata: optional Metadata` An object describing metadata about the request. - `user_id: optional … | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `output_config` | 否 | `OutputConfig` | `output_config: optional OutputConfig` Configuration options for the model's output, such as the out… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `service_tier` | 否 | `auto | standard_only` | `service_tier: optional "auto" or "standard_only"` Determines whether to use priority capacity (if a… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `stop_sequences` | 否 | `array[string]` | `stop_sequences: optional array of string` Custom text sequences that will cause the model to stop g… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `stream` | 否 | `boolean` | `stream: optional boolean` Whether to incrementally stream the response using server-sent events. Se… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `system` | 否 | `string | array[TextBlockParam]` | `system: optional string or array of TextBlockParam` System prompt. A system prompt is a way of prov… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `temperature` | 否 | `number` | `temperature: optional number` Amount of randomness injected into the response. Defaults to `1.0`. R… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `thinking` | 否 | `ThinkingConfigParam` | `thinking: string` - `type: "thinking"` - `"thinking"` - `RedactedThinkingBlockParam object { data, … | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `tool_choice` | 否 | `ToolChoice` | `tool_choice: optional ToolChoice` How the model should use the provided tools. The model can use a … | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `tools` | 否 | `array[ToolUnion]` | `tools: optional array of ToolUnion` Definitions of tools that the model may use. If you include `to… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `top_k` | 否 | `number` | `top_k: optional number` Only sample from the top K options for each subsequent token. Used to remov… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `top_p` | 否 | `number` | `top_p: optional number` Use nucleus sampling. In nucleus sampling, we compute the cumulative distri… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |

## Anthropic Message response 顶层字段

| 字段 | 必填 | 类型 | 含义摘要 | 作者 upstream | 当前分支 | 建议归属 | 作者缺失/不清时处理 |
|---|---:|---|---|---|---|---|---|
| `id` | 是 | `string` | `id: string` Unique object identifier. The format and length of IDs may change over time. - `contain… | 有 | 有 | Anthropic native model | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `container` | 否 | `Container` | `container: Container` Information about the container used in the request (for the code execution t… | 缺 | 缺 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `content` | 是 | `array[ContentBlock]` | `content: array of ContentBlock` Content generated by the model. This is an array of content blocks,… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `model` | 是 | `Model` | `model: Model` The model that will complete your prompt. See [models](https://docs.anthropic.com/en/… | 有 | 有 | CrossProtocolCanonical + Anthropic native emission | 若作者已有 common/native 映射则保持；缺失时先验证是否真跨协议等价，再考虑 common 字段或协议 emitter。 |
| `role` | 是 | `assistant` | `role: "assistant"` Conversational role of the generated message. This will always be `"assistant"`.… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `stop_details` | 否 | `StopDetails` | `stop_details: RefusalStopDetails` Structured information about a refusal. - `category: "cyber" or "… | 缺 | 缺 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `stop_reason` | 是 | `string` | `stop_reason: StopReason` The reason that we stopped. This may be one the following values: * `"end_… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `stop_sequence` | 否 | `string|null` | `stop_sequence: string` Which custom stop sequence was generated, if any. This value will be a non-n… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `type` | 是 | `message` | `type: "char_location"` - `"char_location"` - `CitationPageLocation object { cited_text, document_in… | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |
| `usage` | 是 | `Usage` | `usage: Usage` Billing and rate-limit usage. Anthropic's API bills and rate-limits by token counts, … | 有 | 有 | Anthropic native model / adapter | 作者缺失时优先补协议 native struct / native preservation；不要先塞 llm.Request；同协议保真，跨协议无等价则 diagnostic。 |

## Anthropic stream events / delta types

| 字段/事件 | 类别 | 作者 upstream | 当前分支 | 建议归属 | 处理 |
|---|---|---|---|---|---|
| `message_start` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `content_block_start` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `content_block_delta` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `content_block_stop` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `message_delta` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `message_stop` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `ping` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `error` | stream event | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `text_delta` | stream delta | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `input_json_delta` | stream delta | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `thinking_delta` | stream delta | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |
| `signature_delta` | stream delta | 有 | 有 | Anthropic stream fidelity module | stream 层处理；不塞 request/response body。 |

## Anthropic MCP connector companion 字段

| 字段 | 类型 | 含义摘要 | 建议归属 | 处理 |
|---|---|---|---|---|
| `mcp_servers` | `array[MCPServer]` | Remote MCP server definitions used with Messages API MCP connector companion feature. | Anthropic MCP connector companion/native | Anthropic native/companion；不要自动映射到 OpenAI Responses MCP。 |
| `tools[].type=mcp_toolset` | `ToolUnion variant` | Toolset entry referencing an MCP server by name. | Anthropic MCP connector companion/native | Anthropic native/companion；不要自动映射到 OpenAI Responses MCP。 |
| `mcp_servers[].name` | `string` | MCP server definition field. | Anthropic MCP connector companion/native | Anthropic native/companion；不要自动映射到 OpenAI Responses MCP。 |
| `mcp_servers[].url` | `string` | MCP server definition field. | Anthropic MCP connector companion/native | Anthropic native/companion；不要自动映射到 OpenAI Responses MCP。 |
| `mcp_servers[].authorization_token` | `string` | MCP server definition field. | Anthropic MCP connector companion/native | Anthropic native/companion；不要自动映射到 OpenAI Responses MCP。 |
| `mcp_servers[].tool_configuration` | `object` | MCP server definition field. | Anthropic MCP connector companion/native | Anthropic native/companion；不要自动映射到 OpenAI Responses MCP。 |

## OpenAI related nested schemas（嵌套结构索引）

这些不是全部都要第一轮实现；它们用于识别 `tools`、`tool_choice`、content part、stream chunk 等内部结构。

| Schema | Fields | 初步归属判断 |
|---|---|---|
| `ChatCompletionAllowedTools` | `mode`, `tools` | OpenAI Chat/Shared nested native |
| `ChatCompletionAllowedToolsChoice` | `type`, `allowed_tools` | OpenAI Chat/Shared nested native |
| `ChatCompletionDeleted` | `object`, `id`, `deleted` | OpenAI Chat/Shared nested native |
| `ChatCompletionFunctionCallOption` | `name` | OpenAI Chat/Shared nested native |
| `ChatCompletionFunctions` | `description`, `name`, `parameters` | OpenAI Chat/Shared nested native |
| `ChatCompletionList` | `object`, `data`, `first_id`, `last_id`, `has_more` | OpenAI Chat/Shared nested native |
| `ChatCompletionMessageCustomToolCall` | `id`, `type`, `custom` | OpenAI Chat/Shared nested native |
| `ChatCompletionMessageList` | `object`, `data`, `first_id`, `last_id`, `has_more` | OpenAI Chat/Shared nested native |
| `ChatCompletionMessageToolCall` | `id`, `type`, `function` | OpenAI Chat/Shared nested native |
| `ChatCompletionMessageToolCallChunk` | `index`, `id`, `type`, `function` | OpenAI Chat/Shared nested native |
| `ChatCompletionNamedToolChoice` | `type`, `function` | OpenAI Chat/Shared nested native |
| `ChatCompletionNamedToolChoiceCustom` | `type`, `custom` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestAssistantMessage` | `content`, `refusal`, `role`, `name`, `audio`, `tool_calls`, `function_call` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestAssistantMessageContentPart` | `type`, `text`, `refusal` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestDeveloperMessage` | `content`, `role`, `name` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestFunctionMessage` | `role`, `content`, `name` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestMessage` | `content`, `role`, `name`, `refusal`, `audio`, `tool_calls`, `function_call`, `tool_call_id` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestMessageContentPartAudio` | `type`, `input_audio` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestMessageContentPartFile` | `type`, `file` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestMessageContentPartImage` | `type`, `image_url` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestMessageContentPartRefusal` | `type`, `refusal` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestMessageContentPartText` | `type`, `text` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestSystemMessage` | `content`, `role`, `name` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestSystemMessageContentPart` | `type`, `text` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestToolMessage` | `role`, `content`, `tool_call_id` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestToolMessageContentPart` | `type`, `text` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestUserMessage` | `content`, `role`, `name` | OpenAI Chat/Shared nested native |
| `ChatCompletionRequestUserMessageContentPart` | `type`, `text`, `image_url`, `input_audio`, `file` | OpenAI Chat/Shared nested native |
| `ChatCompletionResponseMessage` | `content`, `refusal`, `tool_calls`, `annotations`, `role`, `function_call`, `audio` | OpenAI Responses nested/native |
| `ChatCompletionStreamOptions` | `include_usage`, `include_obfuscation` | OpenAI Chat/Shared nested native |
| `ChatCompletionStreamResponseDelta` | `content`, `function_call`, `tool_calls`, `role`, `refusal` | OpenAI Responses nested/native |
| `ChatCompletionTokenLogprob` | `token`, `logprob`, `bytes`, `top_logprobs` | OpenAI Chat/Shared nested native |
| `ChatCompletionTool` | `type`, `function` | OpenAI Chat/Shared nested native |
| `ChatCompletionToolChoiceOption` | `type`, `allowed_tools`, `function`, `custom` | OpenAI Chat/Shared nested native |
| `ChatSessionAutomaticThreadTitling` | `enabled` | OpenAI Chat/Shared nested native |
| `ChatSessionChatkitConfiguration` | `automatic_thread_titling`, `file_upload`, `history` | OpenAI Chat/Shared nested native |
| `ChatSessionFileUpload` | `enabled`, `max_file_size`, `max_files` | OpenAI Chat/Shared nested native |
| `ChatSessionHistory` | `enabled`, `recent_threads` | OpenAI Chat/Shared nested native |
| `ChatSessionRateLimits` | `max_requests_per_1_minute` | OpenAI Chat/Shared nested native |
| `ChatSessionResource` | `id`, `object`, `expires_at`, `client_secret`, `workflow`, `user`, `rate_limits`, `max_requests_per_1_minute`, `status`, `chatkit_configuration` | OpenAI Chat/Shared nested native |
| `ChatkitConfigurationParam` | `automatic_thread_titling`, `file_upload`, `history` | OpenAI Chat/Shared nested native |
| `ChatkitWorkflow` | `id`, `version`, `state_variables`, `tracing` | OpenAI Chat/Shared nested native |
| `ChatkitWorkflowTracing` | `enabled` | OpenAI Chat/Shared nested native |
| `CreateChatCompletionStreamResponse` | `id`, `choices`, `created`, `model`, `service_tier`, `system_fingerprint`, `object`, `usage` | OpenAI Responses nested/native |
| `ResponseAudioDeltaEvent` | `type`, `sequence_number`, `delta` | OpenAI Responses nested/native |
| `ResponseAudioDoneEvent` | `type`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseAudioTranscriptDeltaEvent` | `type`, `delta`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseAudioTranscriptDoneEvent` | `type`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCodeInterpreterCallCodeDeltaEvent` | `type`, `output_index`, `item_id`, `delta`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCodeInterpreterCallCodeDoneEvent` | `type`, `output_index`, `item_id`, `code`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCodeInterpreterCallCompletedEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCodeInterpreterCallInProgressEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCodeInterpreterCallInterpretingEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCompletedEvent` | `type`, `response`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseContentPartAddedEvent` | `type`, `item_id`, `output_index`, `content_index`, `part`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseContentPartDoneEvent` | `type`, `item_id`, `output_index`, `content_index`, `sequence_number`, `part` | OpenAI Responses nested/native |
| `ResponseCreatedEvent` | `type`, `response`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseCustomToolCallInputDeltaEvent` | `type`, `sequence_number`, `output_index`, `item_id`, `delta` | OpenAI Responses nested/native |
| `ResponseCustomToolCallInputDoneEvent` | `type`, `sequence_number`, `output_index`, `item_id`, `input` | OpenAI Responses nested/native |
| `ResponseError` | `code`, `message` | OpenAI Responses nested/native |
| `ResponseErrorEvent` | `type`, `code`, `message`, `param`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseFailedEvent` | `type`, `sequence_number`, `response` | OpenAI Responses nested/native |
| `ResponseFileSearchCallCompletedEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseFileSearchCallInProgressEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseFileSearchCallSearchingEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseFormatJsonObject` | `type` | OpenAI Responses nested/native |
| `ResponseFormatJsonSchema` | `type`, `json_schema` | OpenAI Responses nested/native |
| `ResponseFormatText` | `type` | OpenAI Responses nested/native |
| `ResponseFormatTextGrammar` | `type`, `grammar` | OpenAI Responses nested/native |
| `ResponseFormatTextPython` | `type` | OpenAI Responses nested/native |
| `ResponseFunctionCallArgumentsDeltaEvent` | `type`, `item_id`, `output_index`, `sequence_number`, `delta` | OpenAI Responses nested/native |
| `ResponseFunctionCallArgumentsDoneEvent` | `type`, `item_id`, `name`, `output_index`, `sequence_number`, `arguments` | OpenAI Responses nested/native |
| `ResponseImageGenCallCompletedEvent` | `type`, `output_index`, `sequence_number`, `item_id` | OpenAI Responses nested/native |
| `ResponseImageGenCallGeneratingEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseImageGenCallInProgressEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseImageGenCallPartialImageEvent` | `type`, `output_index`, `item_id`, `sequence_number`, `partial_image_index`, `partial_image_b64` | OpenAI Responses nested/native |
| `ResponseInProgressEvent` | `type`, `response`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseIncompleteEvent` | `type`, `response`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseItemList` | `object`, `data`, `has_more`, `first_id`, `last_id` | OpenAI Responses nested/native |
| `ResponseLogProb` | `token`, `logprob`, `top_logprobs` | OpenAI Responses nested/native |
| `ResponseMCPCallArgumentsDeltaEvent` | `type`, `output_index`, `item_id`, `delta`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseMCPCallArgumentsDoneEvent` | `type`, `output_index`, `item_id`, `arguments`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseMCPCallCompletedEvent` | `type`, `item_id`, `output_index`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseMCPCallFailedEvent` | `type`, `item_id`, `output_index`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseMCPCallInProgressEvent` | `type`, `sequence_number`, `output_index`, `item_id` | OpenAI Responses nested/native |
| `ResponseMCPListToolsCompletedEvent` | `type`, `item_id`, `output_index`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseMCPListToolsFailedEvent` | `type`, `item_id`, `output_index`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseMCPListToolsInProgressEvent` | `type`, `item_id`, `output_index`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseOutputItemAddedEvent` | `type`, `output_index`, `sequence_number`, `item` | OpenAI Responses nested/native |
| `ResponseOutputItemDoneEvent` | `type`, `output_index`, `sequence_number`, `item` | OpenAI Responses nested/native |
| `ResponseOutputText` | `type`, `text`, `annotations` | OpenAI Responses nested/native |
| `ResponseOutputTextAnnotationAddedEvent` | `type`, `item_id`, `output_index`, `content_index`, `annotation_index`, `sequence_number`, `annotation` | OpenAI Responses nested/native |
| `ResponseProperties` | `previous_response_id`, `model`, `reasoning`, `background`, `max_tool_calls`, `text`, `tools`, `tool_choice`, `prompt`, `truncation` | OpenAI Responses nested/native |
| `ResponseQueuedEvent` | `type`, `response`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseReasoningSummaryPartAddedEvent` | `type`, `item_id`, `output_index`, `summary_index`, `sequence_number`, `part` | OpenAI Responses nested/native |
| `ResponseReasoningSummaryPartDoneEvent` | `type`, `item_id`, `output_index`, `summary_index`, `sequence_number`, `part` | OpenAI Responses nested/native |
| `ResponseReasoningSummaryTextDeltaEvent` | `type`, `item_id`, `output_index`, `summary_index`, `delta`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseReasoningSummaryTextDoneEvent` | `type`, `item_id`, `output_index`, `summary_index`, `text`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseReasoningTextDeltaEvent` | `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseReasoningTextDoneEvent` | `type`, `item_id`, `output_index`, `content_index`, `text`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseRefusalDeltaEvent` | `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseRefusalDoneEvent` | `type`, `item_id`, `output_index`, `content_index`, `refusal`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseStreamEvent` | `type`, `sequence_number`, `delta`, `output_index`, `item_id`, `code`, `response`, `content_index`, `part`, `message`, `param`, `name`, `arguments`, `item`, `summary_index`, `text`, `refusal`, `logprobs`, `partial_image_index`, `partial_image_b64`, `annotation_index`, `annotation`, `input` | OpenAI Responses nested/native |
| `ResponseStreamOptions` | `include_obfuscation` | OpenAI Responses nested/native |
| `ResponseTextDeltaEvent` | `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number`, `logprobs` | OpenAI Responses nested/native |
| `ResponseTextDoneEvent` | `type`, `item_id`, `output_index`, `content_index`, `text`, `sequence_number`, `logprobs` | OpenAI Responses nested/native |
| `ResponseTextParam` | `format`, `verbosity` | OpenAI Responses nested/native |
| `ResponseUsage` | `input_tokens`, `input_tokens_details`, `output_tokens`, `output_tokens_details`, `total_tokens` | OpenAI Responses nested/native |
| `ResponseWebSearchCallCompletedEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseWebSearchCallInProgressEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponseWebSearchCallSearchingEvent` | `type`, `output_index`, `item_id`, `sequence_number` | OpenAI Responses nested/native |
| `ResponsesClientEvent` | `type`, `metadata`, `top_logprobs`, `temperature`, `top_p`, `user`, `safety_identifier`, `prompt_cache_key`, `service_tier`, `prompt_cache_retention`, `previous_response_id`, `model`, `reasoning`, `background`, `max_tool_calls`, `text`, `tools`, `tool_choice`, `prompt`, `truncation`, `input`, `include`, `parallel_tool_calls`, `store`, `instructions`, `stream`, `stream_options`, `conversation`, `context_management`, `max_output_tokens` | OpenAI Responses nested/native |
| `ResponsesClientEventResponseCreate` | `type`, `metadata`, `top_logprobs`, `temperature`, `top_p`, `user`, `safety_identifier`, `prompt_cache_key`, `service_tier`, `prompt_cache_retention`, `previous_response_id`, `model`, `reasoning`, `background`, `max_tool_calls`, `text`, `tools`, `tool_choice`, `prompt`, `truncation`, `input`, `include`, `parallel_tool_calls`, `store`, `instructions`, `stream`, `stream_options`, `conversation`, `context_management`, `max_output_tokens` | OpenAI Responses nested/native |
| `ResponsesServerEvent` | `type`, `sequence_number`, `delta`, `output_index`, `item_id`, `code`, `response`, `content_index`, `part`, `message`, `param`, `name`, `arguments`, `item`, `summary_index`, `text`, `refusal`, `logprobs`, `partial_image_index`, `partial_image_b64`, `annotation_index`, `annotation`, `input` | OpenAI Responses nested/native |

## 第一轮实现范围标记

| 切片 | 字段/事件 | 状态 |
|---|---|---|
| P1a | OpenAI Responses request: `conversation`, `context_management`, `prompt` | 第一轮做 |
| P1b | OpenAI Responses unknown top-level request fields | 第一轮做，但只 same-protocol raw fallback |
| P1c | OpenAI Responses tool variants / raw tools | 第一轮做，需基于字段矩阵和 raw 证据 |
| P1d | Codex Responses profile: `tool_search`, `defer_loading`, `additional_tools`, `namespace` | 第一轮做，需基于 raw payload 证据 |
| Chat fields | `web_search_options`, `prediction`, `audio`, `n`, deprecated functions | 延后到 Chat emission policy |
| Anthropic fields | `container`, `inference_geo`, `mcp_servers`, `mcp_toolset` | 延后到 Anthropic native preservation |
| Stream events | Responses/Chat/Anthropic stream events | 延后到 stream fidelity module |
