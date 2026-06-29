# AxonHub 协议字段基准总表

> 来源:`openrouter-openapi.yaml`(OpenRouter 唯一官方 OpenAPI spec)
> 瘦身版:`docs/specs/openrouter-chat-messages-responses.min.yaml`(仅保留 `/chat/completions` `/messages` `/responses` 三端点及其 $ref 闭包,1114KB→450KB)
> 本表是后续协议转换修复的 **唯一修改基准**。

## 一、三协议请求体顶层字段全景

### chat (ChatRequest) — 共 39 字段

| 字段 | 必填 | 类型 | 说明 |
|---|---|---|---|
| `cache_control` |  | object |  |
| `debug` |  | object |  |
| `frequency_penalty` |  | number | Frequency penalty (-2.0 to 2.0) |
| `image_config` |  | object |  |
| `logit_bias` |  | object | Token logit bias adjustments |
| `logprobs` |  | boolean | Return log probabilities |
| `max_completion_tokens` |  | integer | Maximum tokens in completion |
| `max_tokens` |  | integer | Maximum tokens (deprecated, use max_completion_tokens). Note: some providers enforce a … |
| `messages` | ✓ | []ChatMessages | List of messages for the conversation |
| `metadata` |  | object | Key-value pairs for additional object information (max 16 pairs, 64 char keys, 512 char… |
| `min_p` |  | number | Minimum probability threshold relative to the most likely token. Tokens with probabilit… |
| `modalities` |  | []string | Output modalities for the response. Supported values are "text", "image", and "audio". |
| `model` |  | string |  |
| `models` |  | array |  |
| `parallel_tool_calls` |  | boolean | Whether to enable parallel function calling during tool use. When true, the model may g… |
| `plugins` |  | []? | Plugins you want to enable for this request, including their settings. |
| `presence_penalty` |  | number | Presence penalty (-2.0 to 2.0) |
| `provider` |  | object |  |
| `reasoning` |  | object | Configuration options for reasoning models |
| `reasoning_effort` |  | string(max/xhigh/high…) | Shorthand for setting reasoning effort. Equivalent to setting reasoning.effort. Cannot … |
| `repetition_penalty` |  | number | Penalizes tokens based on how much they have already appeared in the text. A value of 1… |
| `response_format` |  |  | Response format configuration |
| `route` |  | string |  |
| `seed` |  | integer | Random seed for deterministic outputs |
| `service_tier` |  | string(auto/default/flex…) | The service tier to use for processing this request. |
| `session_id` |  | string | A unique identifier for grouping related requests (e.g., a conversation or agent workfl… |
| `stop` |  |  | Stop sequences (up to 4) |
| `stop_server_tools_when` |  | array |  |
| `stream` |  | boolean | Enable streaming response |
| `stream_options` |  | object |  |
| `temperature` |  | number | Sampling temperature (0-2) |
| `tool_choice` |  | ChatToolChoice |  |
| `tools` |  | []ChatFunctionTool | Available tools for function calling |
| `top_a` |  | number | Consider only tokens with "sufficiently high" probabilities based on the probability of… |
| `top_k` |  | integer | Limits the model to choose from the top K most likely tokens at each step. A value of 1… |
| `top_logprobs` |  | integer | Number of top log probabilities to return (0-20) |
| `top_p` |  | number | Nucleus sampling parameter (0-1) |
| `trace` |  | object |  |
| `user` |  | string | Unique user identifier |

### messages (MessagesRequest) — 共 27 字段

| 字段 | 必填 | 类型 | 说明 |
|---|---|---|---|
| `cache_control` |  | object |  |
| `context_management` |  | object |  |
| `fallbacks` |  | []MessagesFallbackParam | Fallback models to try if the primary model fails or refuses, in order. Handled by Open… |
| `max_tokens` |  | integer |  |
| `messages` | ✓ | []MessagesMessageParam |  |
| `metadata` |  | object |  |
| `model` | ✓ | string |  |
| `models` |  | []string |  |
| `output_config` |  | object |  |
| `plugins` |  | []? | Plugins you want to enable for this request, including their settings. |
| `provider` |  | object |  |
| `route` |  | string |  |
| `service_tier` |  | string |  |
| `session_id` |  | string | A unique identifier for grouping related requests (e.g., a conversation or agent workfl… |
| `speed` |  | AnthropicSpeed |  |
| `stop_sequences` |  | []string |  |
| `stop_server_tools_when` |  | array |  |
| `stream` |  | boolean |  |
| `system` |  |  |  |
| `temperature` |  | number |  |
| `thinking` |  |  |  |
| `tool_choice` |  |  |  |
| `tools` |  | []? |  |
| `top_k` |  | integer |  |
| `top_p` |  | number |  |
| `trace` |  | object |  |
| `user` |  | string | A unique identifier representing your end-user, which helps distinguish between differe… |

### responses (ResponsesRequest) — 共 39 字段

| 字段 | 必填 | 类型 | 说明 |
|---|---|---|---|
| `background` |  | boolean |  |
| `cache_control` |  | object |  |
| `debug` |  | object |  |
| `frequency_penalty` |  | number |  |
| `image_config` |  | object |  |
| `include` |  | []ResponseIncludesEnum |  |
| `input` |  | Inputs |  |
| `instructions` |  | string |  |
| `max_output_tokens` |  | integer |  |
| `max_tool_calls` |  | integer |  |
| `metadata` |  | object |  |
| `modalities` |  | []OutputModalityEnum | Output modalities for the response. Supported values are "text" and "image". |
| `model` |  | string |  |
| `models` |  | []string |  |
| `parallel_tool_calls` |  | boolean |  |
| `plugins` |  | []? | Plugins you want to enable for this request, including their settings. |
| `presence_penalty` |  | number |  |
| `previous_response_id` |  | string |  |
| `prompt` |  | object |  |
| `prompt_cache_key` |  | string |  |
| `provider` |  | object |  |
| `reasoning` |  | ReasoningConfig |  |
| `route` |  | string |  |
| `safety_identifier` |  | string |  |
| `service_tier` |  | string(auto/default/flex…) |  |
| `session_id` |  | string | A unique identifier for grouping related requests (e.g., a conversation or agent workfl… |
| `stop_server_tools_when` |  | array |  |
| `store` |  | boolean |  |
| `stream` |  | boolean |  |
| `temperature` |  | number |  |
| `text` |  | TextExtendedConfig |  |
| `tool_choice` |  | OpenAIResponsesToolChoice |  |
| `tools` |  | []? |  |
| `top_k` |  | integer |  |
| `top_logprobs` |  | integer |  |
| `top_p` |  | number |  |
| `trace` |  | object |  |
| `truncation` |  | string |  |
| `user` |  | string | A unique identifier representing your end-user, which helps distinguish between differe… |
