# AxonHub 三协议请求字段命运总表

> 并集 59 字段 | 来源 openrouter-openapi.yaml | 对照 canonical llm.Request 与各 transformer 源码实测

图例: ✅直转同名 · 🔄改名重组进canonical已有槽 · 🧩合并进别层(system→messages[0]) · 📤TransformerMetadata透传上游 · ❌默认丢弃 · ⚠️待核 · · 该协议无此字段

| # | 字段 | chat_completions | anthropic_messages | openai_responses | 关键证据 |
|---|---|---|---|---|---|
| 1 | `background` | ❌丢弃 | ❌丢弃 | 📤透传 | inbound.go TransformerMetadata |
| 2 | `cache_control` | ⚠️待核 | 📤透传 | ⚠️待核 |  |
| 3 | `context_management` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 4 | `debug` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 5 | `fallbacks` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 6 | `frequency_penalty` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 7 | `image_config` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 8 | `include` | ❌丢弃 | ❌丢弃 | 📤透传 | inbound.go TransformerMetadata |
| 9 | `input` | · | 🧩合并 | 🔄改名 | inbound.go:260 convertInputToMessages |
| 10 | `instructions` | · | · | 🧩合并 | inbound.go:250 注入messages[0] |
| 11 | `logit_bias` | ✅直转 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 12 | `logprobs` | ✅直转 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 13 | `max_completion_tokens` | ✅直转 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 14 | `max_output_tokens` | · | · | 🔄改名 | inbound.go:179 MaxOutputTokens→MaxCompletionTokens |
| 15 | `max_tokens` | ✅直转 | ✅直转 | · |  |
| 16 | `max_tool_calls` | ❌丢弃 | ❌丢弃 | 📤透传 | inbound.go TransformerMetadata |
| 17 | `messages` | ✅直转 | ✅直转 | 🔄改名 |  |
| 18 | `metadata` | ✅直转 | 🔄改名 | ✅直转 |  |
| 19 | `min_p` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 20 | `modalities` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 21 | `model` | ✅直转 | ✅直转 | ✅直转 |  |
| 22 | `models` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 23 | `output_config` | ⚠️待核 | 📤透传 | ⚠️待核 |  |
| 24 | `parallel_tool_calls` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 25 | `plugins` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 26 | `presence_penalty` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 27 | `previous_response_id` | · | · | ✅直转 |  |
| 28 | `prompt` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 29 | `prompt_cache_key` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 30 | `provider` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 31 | `reasoning` | 🔄改名 | · | 🔄改名 |  |
| 32 | `reasoning_effort` | ✅直转 | · | · |  |
| 33 | `repetition_penalty` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 34 | `response_format` | ✅直转 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 35 | `route` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 36 | `safety_identifier` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 37 | `seed` | ✅直转 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 38 | `service_tier` | ✅直转 | ✅直转 | ✅直转 |  |
| 39 | `session_id` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 40 | `speed` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 41 | `stop` | ✅直转 | 🔄改名 | ❌丢弃 | canon无槽且入站未映射 |
| 42 | `stop_sequences` | · | 🔄改名 | · | StopSequences→Stop;Thinking.BudgetTokens→ReasoningEffort+Budget |
| 43 | `stop_server_tools_when` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 44 | `store` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 45 | `stream` | ✅直转 | ✅直转 | ✅直转 |  |
| 46 | `stream_options` | ✅直转 | ❌丢弃 | 📤透传 | canon无槽且入站未映射 |
| 47 | `system` | · | 🧩合并 | · | inbound_convert.go:95 System.Prompt→messages[0] |
| 48 | `temperature` | ✅直转 | ✅直转 | ✅直转 |  |
| 49 | `text` | ❌丢弃 | ❌丢弃 | ❌丢弃 | ⚠TextExtendedConfig零引用=DROP |
| 50 | `thinking` | · | 🔄改名 | · | StopSequences→Stop;Thinking.BudgetTokens→ReasoningEffort+Budget |
| 51 | `tool_choice` | ✅直转 | ✅直转 | ✅直转 |  |
| 52 | `tools` | ✅直转 | ✅直转 | ✅直转 |  |
| 53 | `top_a` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 54 | `top_k` | ❌丢弃 | 📤透传 | ❌丢弃 | ★仅anthropic存TransformerMetadata;chat/resp不收 |
| 55 | `top_logprobs` | ✅直转 | ❌丢弃 | ✅直转 | canon无槽且入站未映射 |
| 56 | `top_p` | ✅直转 | ✅直转 | ✅直转 |  |
| 57 | `trace` | ❌丢弃 | ❌丢弃 | ❌丢弃 | canon无槽且入站未映射 |
| 58 | `truncation` | ❌丢弃 | ❌丢弃 | 📤透传 | inbound.go TransformerMetadata |
| 59 | `user` | ✅直转 | 🔄改名 | ✅直转 |  |