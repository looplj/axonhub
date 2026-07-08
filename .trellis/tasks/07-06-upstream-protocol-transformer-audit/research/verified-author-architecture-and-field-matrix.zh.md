# 已核实作者架构问题与协议字段矩阵（重新生成）

日期：2026-07-07

## 0. 证据范围

本文件只基于以下可核实来源，不再沿用 `/tmp` 临时源码结论：

- 作者 upstream worktree：`/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable`
- upstream commit：`97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)`
- codebase-memory project：`Users-asuan-AI-axonhub-worktrees-upstream-unstable`，状态 ready，37149 nodes / 205094 edges
- 本地官方协议抽取：`research/protocol-field-extraction/openai-fields.json`、`anthropic-fields.json`、`code-field-coverage.json`

当前结论的边界：这是“基于本地已下载官方协议 + 作者 upstream 代码”的核实；如果要保证 2026-07-07 官网实时最新，需要单独刷新官方文档后重跑本矩阵。

## 1. 作者架构真实问题

1. **不是没有保真 seam，而是 seam 覆盖不完整**：作者已有 `ProviderExtensions.OpenAIResponses.Request`，但只保 `tools/tool_choice/input` 内 raw 片段；top-level raw 字段没有同等机制。
2. **`llm.Request` 是公共语义层，不是 Responses 完整 AST**：协议专属字段全塞进 `llm.Request` 会把公共层变成垃圾桶；正确位置是协议 struct 或 ProviderExtensions sidecar。
3. **Responses -> Responses 不是原样透传**：入站 `json.Unmarshal` 到 `responses.Request`，出站重新 marshal；没有被 struct 或 sidecar 接住的字段会丢。
4. **pipeline 不会恢复 body 字段**：`RawRequest` 被保存，但 `MergeInboundRequest` 只合并 headers/query，不合并 body。
5. **`TransformerMetadata` 有扩散风险**：现有少数字段经 metadata 恢复可以保留；新增协议字段不应继续塞 magic key。
6. **跨协议和同协议处理混在同一抽象中容易误判**：同协议应默认保真；跨协议没有等价物时应明确 drop/diagnostic，不应偷透传。
7. **Chat/OpenAI-compatible provider 共享 builder 有污染风险**：官方 Chat 字段不能无条件发给所有 OpenAI-compatible provider，需要 provider policy seam。
8. **stream/event 是另一套问题**：Responses 大量 stream event 不能通过 request struct 修复，需单独审计 stream model/aggregator。

## 2. 架构改动原则

| 决策问题 | 归属 | 说明 |
|---|---|---|
| 多协议稳定等价字段 | `llm.Request` / `llm.Response` | 例如 model/messages/tools 中可公共表达的部分 |
| OpenAI Responses 官方字段 | `llm/transformer/openai/responses` native struct | 例如 `prompt`、`conversation` 这类 TODO 字段 |
| OpenAI Responses 同协议 unknown/profile top-level | `ProviderExtensions.OpenAIResponses.Request` raw top-level fallback | 例如 `context_management`、`additional_tools`、`defer_loading` |
| 当前已支持的 raw tools/input/tool_choice | 保留现有 `request_extensions.go` 机制 | 不重复造 Codex/MCP 转换层 |
| Chat 官方但 provider 兼容性不稳定字段 | Chat native + provider policy seam | 例如 `web_search_options`、`prediction`、top-level `audio` |
| Anthropic native/MCP connector 字段 | Anthropic native/provider extension | 不自动映射到 OpenAI Responses MCP |
| stream event | stream model/aggregator | 不放 request/response 字段矩阵里修 |

## 3. 字段全量矩阵

### 3.1 OpenAI Responses request

代码目标：`llm/transformer/openai/responses/model.go` struct `Request`

| 字段 | 必填 | 类型 | 字段作用/含义 | 作者 upstream 是否顶层支持 | 作者处理现状 | 建议归属/动作 |
|---|---:|---|---|---:|---|---|
| `metadata` | 否 | `Metadata` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_logprobs` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `temperature` | 否 | `anyOf(number \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_p` | 否 | `anyOf(number \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `user` | 否 | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifi… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `safety_identifier` | 否 | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies. The IDs should be a string that uniquely ident… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt_cache_key` | 否 | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](/docs/guides/prompt-caching). | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `service_tier` | 否 | `ServiceTier` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt_cache_retention` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已顶层建模；当前部分经 TransformerMetadata 恢复，可保留但不扩散 |
| `previous_response_id` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `model` | 否 | `ModelIdsResponses` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, a… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `reasoning` | 否 | `anyOf(Reasoning \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `background` | 否 | `anyOf(boolean \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `max_tool_calls` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已顶层建模；当前部分经 TransformerMetadata 恢复，可保留但不扩散 |
| `text` | 否 | `ResponseTextParam` | Configuration options for a text response from the model. Can be plain text or structured JSON data. Learn more: - [Text inputs and outputs](/docs/guides/text)… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tools` | 否 | `ToolsArray` | An array of tools the model may call while generating a response. You can specify which tool to use by setting the `tool_choice` parameter. We support the foll… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tool_choice` | 否 | `ToolChoiceParam` | How the model should select which tool (or tools) to use when generating a response. See the `tools` parameter to see how to specify which tools the model can … | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt` | 否 | `Prompt` |  | 否 | nested/response/helper struct only | Responses typed TODO；优先恢复到 responses.Request |
| `truncation` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已顶层建模；当前部分经 TransformerMetadata 恢复，可保留但不扩散 |
| `input` | 否 | `InputParam` | Text, image, or file inputs to the model, used to generate a response. Learn more: - [Text inputs and outputs](/docs/guides/text) - [Image inputs](/docs/guides… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `include` | 否 | `anyOf(array[IncludeEnum] \| null)` |  | 是 | native top-level struct | 已顶层建模；当前部分经 TransformerMetadata 恢复，可保留但不扩散 |
| `parallel_tool_calls` | 否 | `anyOf(boolean \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `store` | 否 | `anyOf(boolean \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `instructions` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stream` | 否 | `anyOf(boolean \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stream_options` | 否 | `ResponseStreamOptions` |  | 是 | native top-level struct | 已顶层建模；当前部分经 TransformerMetadata 恢复，可保留但不扩散 |
| `conversation` | 否 | `anyOf(ConversationParam \| null)` |  | 否 | nested/response/helper struct only | Responses typed TODO；优先恢复到 responses.Request |
| `context_management` | 否 | `anyOf(array[ContextManagementParam] \| null)` |  | 否 | missing in upstream request; should be native/opaque request field | Responses top-level raw/native；不进 llm.Request |
| `max_output_tokens` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |

### 3.2 OpenAI Responses response

代码目标：`llm/transformer/openai/responses/model.go` struct `Response`

| 字段 | 必填 | 类型 | 字段作用/含义 | 作者 upstream 是否顶层支持 | 作者处理现状 | 建议归属/动作 |
|---|---:|---|---|---:|---|---|
| `metadata` | 是 | `Metadata` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_logprobs` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `temperature` | 是 | `anyOf(number \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_p` | 是 | `anyOf(number \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `user` | 否 | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifi… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `safety_identifier` | 否 | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies. The IDs should be a string that uniquely ident… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt_cache_key` | 否 | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](/docs/guides/prompt-caching). | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `service_tier` | 否 | `ServiceTier` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt_cache_retention` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `previous_response_id` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `model` | 是 | `ModelIdsResponses` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, a… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `reasoning` | 否 | `anyOf(Reasoning \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `background` | 否 | `anyOf(boolean \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `max_tool_calls` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `text` | 否 | `ResponseTextParam` | Configuration options for a text response from the model. Can be plain text or structured JSON data. Learn more: - [Text inputs and outputs](/docs/guides/text)… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tools` | 是 | `ToolsArray` | An array of tools the model may call while generating a response. You can specify which tool to use by setting the `tool_choice` parameter. We support the foll… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tool_choice` | 是 | `ToolChoiceParam` | How the model should select which tool (or tools) to use when generating a response. See the `tools` parameter to see how to specify which tools the model can … | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt` | 否 | `Prompt` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `truncation` | 否 | `anyOf(string \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `id` | 是 | `string` | Unique identifier for this Response. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `object` | 是 | `string` | The object type of this resource - always set to `response`. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `status` | 否 | `string` | The status of the response generation. One of `completed`, `failed`, `in_progress`, `cancelled`, `queued`, or `incomplete`. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `created_at` | 是 | `number` | Unix timestamp (in seconds) of when this Response was created. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `completed_at` | 否 | `anyOf(number \| null)` |  | 否 | missing/not modeled | 按协议层判定 |
| `error` | 是 | `ResponseError` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `incomplete_details` | 是 | `anyOf(object \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `output` | 是 | `array[OutputItem]` | An array of content items generated by the model. - The length and order of items in the `output` array is dependent on the model's response. - Rather than acc… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `instructions` | 是 | `anyOf(oneOf(string \| array[InputItem]) \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `output_text` | 否 | `anyOf(string \| null)` |  | 否 | missing/not modeled | 按协议层判定 |
| `usage` | 否 | `ResponseUsage` | Represents token usage details including input tokens, output tokens, a breakdown of output tokens, and the total tokens used. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `parallel_tool_calls` | 是 | `boolean` | Whether to allow the model to run tool calls in parallel. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `conversation` | 否 | `anyOf(Conversation-2 \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `max_output_tokens` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |

### 3.3 OpenAI Chat request

代码目标：`llm/transformer/openai/model.go` struct `Request`

| 字段 | 必填 | 类型 | 字段作用/含义 | 作者 upstream 是否顶层支持 | 作者处理现状 | 建议归属/动作 |
|---|---:|---|---|---:|---|---|
| `metadata` | 否 | `Metadata` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_logprobs` | 否 | `anyOf(integer \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `temperature` | 否 | `anyOf(number \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_p` | 否 | `anyOf(number \| null)` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `user` | 否 | `string` | This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use `prompt_cache_key` instead to maintain caching optimizations. A stable identifi… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `safety_identifier` | 否 | `string` | A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies. The IDs should be a string that uniquely ident… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt_cache_key` | 否 | `string` | Used by OpenAI to cache responses for similar requests to optimize your cache hit rates. Replaces the `user` field. [Learn more](/docs/guides/prompt-caching). | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `service_tier` | 否 | `ServiceTier` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `prompt_cache_retention` | 否 | `anyOf(string \| null)` |  | 否 | nested/response/helper struct only | Chat native 或显式不支持 |
| `messages` | 是 | `array[ChatCompletionRequestMessage]` | A list of messages comprising the conversation so far. Depending on the [model](/docs/models) you use, different message types (modalities) are supported, like… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `model` | 是 | `ModelIdsShared` | Model ID used to generate the response, like `gpt-4o` or `o3`. OpenAI offers a wide range of models with different capabilities, performance characteristics, a… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `modalities` | 否 | `ResponseModalities` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `verbosity` | 否 | `Verbosity` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `reasoning_effort` | 否 | `ReasoningEffort` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `max_completion_tokens` | 否 | `integer` | An upper bound for the number of tokens that can be generated for a completion, including visible output tokens and [reasoning tokens](/docs/guides/reasoning). | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `frequency_penalty` | 否 | `number` | Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to rep… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `presence_penalty` | 否 | `number` | Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk abo… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `web_search_options` | 否 | `object` | This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](/docs/guides/tools-web-search?api-mode=chat). | 否 | missing in upstream request; modern Chat native field candidate | Chat native/policy-gated emission；不要污染所有 OpenAI-compatible provider |
| `response_format` | 否 | `oneOf(ResponseFormatText \| ResponseFormatJsonSchema \| ResponseFormatJsonObject)` | An object specifying the format that the model must output. Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensur… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `audio` | 否 | `object` | Parameters for audio output. Required when audio output is requested with `modalities: ["audio"]`. [Learn more](/docs/guides/audio). | 否 | nested/response/helper struct only | Chat native/policy-gated emission；不要污染所有 OpenAI-compatible provider |
| `store` | 否 | `boolean` | Whether or not to store the output of this chat completion request for use in our [model distillation](/docs/guides/distillation) or [evals](/docs/guides/evals… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stream` | 否 | `boolean` | If set to true, the model response data will be streamed to the client as it is generated using [server-sent events](https://developer.mozilla.org/en-US/docs/W… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stop` | 否 | `StopConfiguration` | Not supported with latest reasoning models `o3` and `o4-mini`. Up to 4 sequences where the API will stop generating further tokens. The returned text will not … | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `logit_bias` | 否 | `object` | Modify the likelihood of specified tokens appearing in the completion. Accepts a JSON object that maps tokens (specified by their token ID in the tokenizer) to… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `logprobs` | 否 | `boolean` | Whether to return log probabilities of the output tokens or not. If true, returns the log probabilities of each output token returned in the `content` of `mess… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `max_tokens` | 否 | `integer` | The maximum number of [tokens](/tokenizer) that can be generated in the chat completion. This value can be used to control [costs](https://openai.com/api/prici… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `n` | 否 | `integer` | How many chat completion choices to generate for each input message. Note that you will be charged based on the number of generated tokens across all of the ch… | 否 | missing/not modeled | 缺失/旧字段；需单独设计或明确不支持 |
| `prediction` | 否 | `oneOf(PredictionContent)` | Configuration for a [Predicted Output](/docs/guides/predicted-outputs), which can greatly improve response times when large parts of the model response are kno… | 否 | missing in upstream request; modern Chat native field candidate | Chat native/policy-gated emission；不要污染所有 OpenAI-compatible provider |
| `seed` | 否 | `integer` | This feature is in Beta. If specified, our system will make a best effort to sample deterministically, such that repeated requests with the same `seed` and par… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stream_options` | 否 | `ChatCompletionStreamOptions` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tools` | 否 | `array[oneOf(ChatCompletionTool \| CustomToolChatCompletions)]` | A list of tools the model may call. You can provide either [custom tools](/docs/guides/function-calling#custom-tools) or [function tools](/docs/guides/function… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tool_choice` | 否 | `ChatCompletionToolChoiceOption` | Controls which (if any) tool is called by the model. `none` means the model will not call any tool and instead generates a message. `auto` means the model can … | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `parallel_tool_calls` | 否 | `ParallelToolCalls` | Whether to enable [parallel function calling](/docs/guides/function-calling#configuring-parallel-function-calling) during tool use. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `function_call` | 否 | `oneOf(string \| ChatCompletionFunctionCallOption)` | Deprecated in favor of `tool_choice`. Controls which (if any) function is called by the model. `none` means the model will not call a function and instead gene… | 否 | missing/not modeled | 缺失/旧字段；需单独设计或明确不支持 |
| `functions` | 否 | `array[ChatCompletionFunctions]` | Deprecated in favor of `tools`. A list of functions the model may generate JSON inputs for. | 否 | missing/not modeled | 缺失/旧字段；需单独设计或明确不支持 |

### 3.4 OpenAI Chat response

代码目标：`llm/transformer/openai/model.go` struct `Response`

| 字段 | 必填 | 类型 | 字段作用/含义 | 作者 upstream 是否顶层支持 | 作者处理现状 | 建议归属/动作 |
|---|---:|---|---|---:|---|---|
| `id` | 是 | `string` | A unique identifier for the chat completion. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `choices` | 是 | `array[object]` | A list of chat completion choices. Can be more than one if `n` is greater than 1. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `created` | 是 | `integer` | The Unix timestamp (in seconds) of when the chat completion was created. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `model` | 是 | `string` | The model used for the chat completion. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `service_tier` | 否 | `ServiceTier` |  | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `system_fingerprint` | 否 | `string` | This fingerprint represents the backend configuration that the model runs with. Can be used in conjunction with the `seed` request parameter to understand when… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `object` | 是 | `string` | The object type, which is always `chat.completion`. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `usage` | 否 | `CompletionUsage` | Usage statistics for the completion request. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |

### 3.5 Anthropic Messages request

代码目标：`llm/transformer/anthropic/model.go` struct `MessageRequest`

| 字段 | 必填 | 类型 | 字段作用/含义 | 作者 upstream 是否顶层支持 | 作者处理现状 | 建议归属/动作 |
|---|---:|---|---|---:|---|---|
| `max_tokens` | 是 | `number` | `max_tokens: number` The maximum number of tokens to generate before stopping. Note that our models may stop _before_ reaching this maximum. This parameter onl… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `messages` | 是 | `array[MessageParam]` | `messages: array of MessageParam` Input messages. Our models are trained to operate on alternating `user` and `assistant` conversational turns. When creating a… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `model` | 是 | `Model` | `model: Model` The model that will complete your prompt. See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options. -… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `container` | 否 | `string` | `container: optional string` Container identifier for reuse across requests. - `inference_geo: optional string` Specifies the geographic region for inference p… | 否 | missing or companion-native field candidate | Anthropic native/provider extension；不自动跨协议映射 |
| `inference_geo` | 否 | `string` | `inference_geo: optional string` Specifies the geographic region for inference processing. If not specified, the workspace's `default_inference_geo` is used. | 否 | missing or companion-native field candidate | Anthropic native/provider extension；不自动跨协议映射 |
| `metadata` | 否 | `Metadata` | `metadata: optional Metadata` An object describing metadata about the request. - `user_id: optional string` An external identifier for the user who is associat… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `output_config` | 否 | `OutputConfig` | `output_config: optional OutputConfig` Configuration options for the model's output, such as the output format. | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `service_tier` | 否 | `auto \| standard_only` | `service_tier: optional "auto" or "standard_only"` Determines whether to use priority capacity (if available) or standard capacity for this request. Anthropic … | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stop_sequences` | 否 | `array[string]` | `stop_sequences: optional array of string` Custom text sequences that will cause the model to stop generating. Our models will normally stop when they have nat… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stream` | 否 | `boolean` | `stream: optional boolean` Whether to incrementally stream the response using server-sent events. See [streaming](https://platform.claude.com/docs/en/build-wit… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `system` | 否 | `string \| array[TextBlockParam]` | `system: optional string or array of TextBlockParam` System prompt. A system prompt is a way of providing context and instructions to Claude, such as specifyin… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `temperature` | 否 | `number` | `temperature: optional number` Amount of randomness injected into the response. Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `thinking` | 否 | `ThinkingConfigParam` | `thinking: string` - `type: "thinking"` - `"thinking"` - `RedactedThinkingBlockParam object { data, type }` | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tool_choice` | 否 | `ToolChoice` | `tool_choice: optional ToolChoice` How the model should use the provided tools. The model can use a specific tool, any available tool, decide by itself, or not… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `tools` | 否 | `array[ToolUnion]` | `tools: optional array of ToolUnion` Definitions of tools that the model may use. If you include `tools` in your API request, the model may return `tool_use` c… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_k` | 否 | `number` | `top_k: optional number` Only sample from the top K options for each subsequent token. Used to remove "long tail" low probability responses. [Learn more techni… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `top_p` | 否 | `number` | `top_p: optional number` Use nucleus sampling. In nucleus sampling, we compute the cumulative distribution over all the options for each subsequent token in de… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |

### 3.6 Anthropic Message response

代码目标：`llm/transformer/anthropic/model.go` struct `Message`

| 字段 | 必填 | 类型 | 字段作用/含义 | 作者 upstream 是否顶层支持 | 作者处理现状 | 建议归属/动作 |
|---|---:|---|---|---:|---|---|
| `id` | 是 | `string` | `id: string` Unique object identifier. The format and length of IDs may change over time. - `container: Container` Information about the container used in the … | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `container` | 否 | `Container` | `container: Container` Information about the container used in the request (for the code execution tool) | 否 | missing or companion-native field candidate | Anthropic native/provider extension；不自动跨协议映射 |
| `content` | 是 | `array[ContentBlock]` | `content: array of ContentBlock` Content generated by the model. This is an array of content blocks, each of which has a `type` that determines its shape. Exam… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `model` | 是 | `Model` | `model: Model` The model that will complete your prompt. See [models](https://docs.anthropic.com/en/docs/models-overview) for additional details and options. -… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `role` | 是 | `assistant` | `role: "assistant"` Conversational role of the generated message. This will always be `"assistant"`. - `"assistant"` - `stop_details: RefusalStopDetails` Struc… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stop_details` | 否 | `StopDetails` | `stop_details: RefusalStopDetails` Structured information about a refusal. - `category: "cyber" or "bio" or "frontier_llm" or "reasoning_extraction"` The polic… | 否 | missing/not modeled | Anthropic native/provider extension；不自动跨协议映射 |
| `stop_reason` | 是 | `string` | `stop_reason: StopReason` The reason that we stopped. This may be one the following values: * `"end_turn"`: the model reached a natural stopping point * `"max_… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `stop_sequence` | 否 | `string\|null` | `stop_sequence: string` Which custom stop sequence was generated, if any. This value will be a non-null string if one of your custom stop sequences was generat… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `type` | 是 | `message` | `type: "char_location"` - `"char_location"` - `CitationPageLocation object { cited_text, document_index, document_title, 4 more }` | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |
| `usage` | 是 | `Usage` | `usage: Usage` Billing and rate-limit usage. Anthropic's API bills and rate-limits by token counts, as tokens represent the underlying cost to our systems. Und… | 是 | native top-level struct | 已由目标协议 struct 顶层支持；保持现状/补测试即可 |

## 4. 非 OpenAI Responses 官方 schema 的 profile/companion 字段

| 字段/形态 | 来源/出现位置 | 含义 | 作者 upstream 状态 | 建议 |
|---|---|---|---|---|
| `tools[]` 内 `type=tool_search` | Codex/Responses-shaped payload；upstream tests | 工具搜索/lazy tool discovery 入口 | 已有 raw tool fallback 测试覆盖 | 保留现有机制，不重复实现 |
| `input[]` 内 `type=tool_search_call` | Codex/Responses-shaped payload；upstream tests | 工具搜索调用结果/调用记录项 | 已有 raw input fallback 测试覆盖 | 保留现有机制，不重复实现 |
| `namespace` | tool/input item 字段；upstream model/test | 工具命名空间/路由身份 | tools/input 内已可 raw 保真，部分 item struct 有字段 | 不要放入 llm.Request 公共字段 |
| `additional_tools` | Codex/profile top-level 候选；不在当前 OpenAI Responses official request JSON 抽取中 | 额外工具集合/懒加载候选 | upstream 未出现 | 如果确有入站 payload，走 top-level raw fallback |
| `defer_loading` | Anthropic tool docs里存在；Codex/profile top-level 或 tool 字段候选 | 延迟加载工具，初始 prompt 不展开 | Responses upstream 未出现 | 按出现位置处理：tool 内 raw 已可保，top-level 走 raw fallback |
| Anthropic `mcp_servers` | Anthropic MCP connector companion docs | 远程 MCP server 定义 | Anthropic upstream 未完整建模 | Anthropic native/provider extension；不自动转 OpenAI |

## 5. Stream / event 字段概览

stream 字段不进入 P1 请求保真修复，但必须单独列出，避免把断流/事件丢失误判成 request 字段问题。

| 类别 | 数量 | 处理建议 |
|---|---:|---|
| OpenAI Responses stream schemas/events | 57 | 单独审计 `inbound_stream/outbound_stream/aggregator` |
| OpenAI Chat stream schemas | 4 | 单独审计 Chat stream 兼容 |
| Anthropic stream events | 8 | 单独审计 Anthropic stream transformer |
| Anthropic stream delta types | 4 | 单独审计 delta 映射 |

## 6. 当前优先修复项

| 优先级 | 问题 | 为什么先做 | 修复入口 |
|---|---|---|---|
| P1a | OpenAI Responses top-level raw fallback | 这是当前同协议丢字段的核心缺口 | `provider_extensions.go` + `responses/request_extensions.go` |
| P1b | `prompt` / `conversation` typed TODO | 作者已有 struct/TODO，适合恢复 typed support | `responses/model.go` + inbound/outbound tests |
| P1c | 保护现有 raw tools/input/tool_choice | upstream 已支持，不能重复造或破坏 | 保持现有 tests 通过 |
| P2 | Chat native policy seam | 避免官方 Chat 字段污染所有 provider | `openai` shared builder + provider policy |
| P3 | Anthropic native/MCP connector | 与 OpenAI MCP 不同，不应自动互转 | `anthropic` native/provider extension |
| P4 | Stream/event fidelity | 解释断流/事件丢失必须靠 stream 层 | stream transformers/aggregators |
