# 协议转换修复基准(核心对照)

## A. 最大公约数 — 三方同名共有字段 (18)

| 字段 | chat | messages | responses | 备注 |
|---|---|---|---|---|
| cache_control | object | object | object | 同名直传 |
| metadata | object | object | object | 同名直传 |
| model | string | string | string | 同名直传 |
| models | array | []string | []string | 同名直传 |
| plugins | []? | []? | []? | 同名直传 |
| provider | object | object | object | 同名直传 |
| route | string | string | string | 同名直传 |
| service_tier | string(auto/default/flex…) | string | string(auto/default/flex…) | 同名直传 |
| session_id | string | string | string | 同名直传 |
| stop_server_tools_when | array | array | array | 同名直传 |
| stream | boolean | boolean | boolean | 同名直传 |
| temperature | number | number | number | 同名直传 |
| tool_choice | ChatToolChoice |  | OpenAIResponsesToolChoice | 同名直传 |
| tools | []ChatFunctionTool | []? | []? | 同名直传 |
| top_k | integer | integer | integer | 同名直传 |
| top_p | number | number | number | 同名直传 |
| trace | object | object | object | 同名直传 |
| user | string | string | string | 同名直传 |

## B. 功能等价命名不同 — 跨协议改名映射高危点

| 概念 | chat | messages | responses | 风险说明 |
|---|---|---|---|---|
| 输出长度上限 | max_completion_tokens / max_tokens | max_tokens | max_output_tokens | canonical 双槽 MaxTokens+MaxCompletionTokens |
| 停止条件 | stop | stop_sequences | (无) | responses 无顶层对应;chat<->messages 改名 |
| 推理控制 | reasoning_effort (+reasoning obj) | thinking | reasoning(obj) | 结构差异大需展开 effort/summary/budget |
| 系统提示词 | messages[role=system] | system | instructions | 位置完全不同极易丢 |
| 输入内容载体 | messages | messages | input | responses 叫 input 且类型多态 |
| 日志概率 | logprobs/top_logprobs | (不支持) | top_logprobs | messages 不支持 |

## C. 仅单一协议特有字段(跨协议默认丢失或塞 ExtraBody/metadata)

- 仅 chat (11): `logit_bias` `logprobs` `max_completion_tokens` `min_p` `reasoning_effort` `repetition_penalty` `response_format` `seed` `stop` `stream_options` `top_a`
- 仅 messages (7): `context_management` `fallbacks` `output_config` `speed` `stop_sequences` `system` `thinking`
- 仅 responses (13): `background` `include` `input` `instructions` `max_output_tokens` `max_tool_calls` `previous_response_id` `prompt` `prompt_cache_key` `safety_identifier` `store` `text` `truncation`