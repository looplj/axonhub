# OpenAI Chat Completions Protocol Baseline

- Regenerated: 2026-07-06
- Scope: OpenAI Chat Completions wire protocol baseline for AxonHub conversion/audit work.
- Status: regenerated from canonical sources under `docs/specs/vendor/protocol-canonical-2026-07-06/`.
- Important: previous generated protocol docs are not used as source-of-truth for this file.

## 0. Canonical sources used

| Local file | Role |
|---|---|
| `docs/specs/vendor/protocol-canonical-2026-07-06/openai-api-definition.fetch.yaml` | OpenAI official machine-readable API definition fetched through `smart-search fetch`; verified markers include `/chat/completions` and `operationId: createChatCompletion`. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/openai-chat-completions-create.developers-snapshot.md` | Full official Developers create page snapshot. Verified markers include `POST/chat/completions`, `# Create chat completion`, `### Body Parameters`, `web_search_options`. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/openai-chat-completions-create.developers-snapshot.html` | HTML capture of the same Developers page. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/SOURCES.md` | Source selection and failed-candidate notes. |

## 1. Protocol identity

| Property | Value |
|---|---|
| Endpoint | `POST /chat/completions` |
| Primary input field | `messages` |
| Input style | ordered role-message array |
| Output style | `chat.completion` with `choices[]` |
| Streaming style | chat completion chunks with `choices[].delta` |
| Tool model | Chat tools/tool calls, primarily `function` and current `custom` tool forms |

Implementation rule: Chat Completions is not Responses with different field names. It has different message, tool, output, and streaming shapes.

## 2. `POST /chat/completions` request fields

The current canonical Chat create page and API definition expose these body fields:

| Field | Canonical meaning for conversion |
|---|---|
| `messages` | Ordered conversation messages. Required for Chat-style requests. |
| `model` | Model identifier. |
| `audio` | Audio output/input-related parameters, used with audio modalities. |
| `frequency_penalty` | Frequency penalty. |
| `logit_bias` | Token logit bias map. |
| `logprobs` | Whether to return output-token log probabilities. |
| `max_completion_tokens` | Current maximum generated-token field for Chat completions. |
| `metadata` | Metadata map. |
| `modalities` | Output modalities, e.g. text/audio combinations. |
| `moderation` | Moderation configuration object. |
| `n` | Number of choices. |
| `parallel_tool_calls` | Whether tools may be called in parallel. |
| `prediction` | Predicted output configuration. |
| `presence_penalty` | Presence penalty. |
| `prompt_cache_key` | Prompt-cache routing key. |
| `prompt_cache_retention` | Prompt-cache retention policy. |
| `reasoning_effort` | Reasoning effort field for models that support it. |
| `response_format` | Text/JSON schema/JSON object response format. |
| `safety_identifier` | Stable safety/user identifier. |
| `service_tier` | Service tier selection. |
| `stop` | Stop sequence(s). |
| `store` | Whether to store completion output. |
| `stream` | Whether to stream chat chunks. |
| `stream_options` | Chat stream options. |
| `temperature` | Sampling temperature. |
| `tool_choice` | Chat tool-choice configuration. |
| `tools` | Chat tool definitions. |
| `top_logprobs` | Number of top logprobs per token; depends on `logprobs`. |
| `top_p` | Nucleus sampling. |
| `verbosity` | Verbosity setting where supported. |
| `web_search_options` | Chat web-search options object. This is not the same shape as a Responses `web_search` tool. |

Deprecated but still present in the canonical create page/API definition:

| Deprecated field | Replacement / note |
|---|---|
| `function_call` | Deprecated in favor of `tool_choice`. |
| `functions` | Deprecated in favor of `tools`. |
| `max_tokens` | Deprecated in favor of `max_completion_tokens`; not compatible with some reasoning models. |
| `seed` | Deprecated/compatibility field. |
| `user` | Deprecated; use `safety_identifier` where applicable. |

## 3. `messages[]` protocol

Canonical Chat messages are role-tagged objects. Current source confirms:

| Role | Notes |
|---|---|
| `developer` | Developer instructions; for o1 and newer, developer messages replace previous system-message pattern. |
| `system` | Legacy/system instructions still represented in Chat message union. |
| `user` | End-user messages; may contain text and supported multimodal content parts. |
| `assistant` | Model responses; can include content, refusals, audio/tool-call fields depending on schema. |
| `tool` | Tool result messages. |
| legacy `function` | Compatibility role for old function-calling flow. |

Content part families confirmed in the canonical create page include text, image URL, input audio, file, and refusal-related parts. Do not reduce multimodal content to plain text unless target protocol requires a lossy downgrade and diagnostics are emitted.

## 4. Tools protocol

Current canonical source confirms Chat supports current `tools` plus deprecated `functions`.

| Tool area | Conversion rule |
|---|---|
| `function` tools | Must be supported; preserve `name`, `description`, `parameters`, `strict` where present. |
| `custom` tools | Canonical Chat create page/API definition includes custom tool forms; do not treat Chat as function-only. |
| `tool_choice` | Preserve string and object forms; do not collapse to only function name. |
| deprecated `function_call` / `functions` | Preserve for compatibility if source request used them, but normalize internally only with explicit policy. |
| `web_search_options` | Chat-specific web search configuration, not equivalent to Responses native web-search tool item. |

## 5. Response and streaming protocol

Chat non-streaming response is a chat completion object with `choices[]`, each containing a message-like object and `finish_reason`. Usage details may include prompt/completion/total token fields and modality/reasoning-related details depending on model.

Chat streaming uses chat completion chunk objects. The incremental content is carried in `choices[].delta`; this is structurally different from Responses semantic events.

## 6. Cross-protocol implications

### Chat → Chat

Preserve raw request fields including deprecated compatibility fields, multimodal message parts, current tool forms, custom tools, `web_search_options`, prompt cache fields, reasoning effort, and stream options.

### Chat → Responses

Potentially common fields: `model`, some sampling fields, metadata, basic tool/function schemas, plain user/developer/system text.

Requires deliberate mapping:

- `messages[]` → Responses `input[]` typed items;
- Chat `tools`/`tool_choice` → Responses tools/tool choice;
- Chat `web_search_options` → not automatically a Responses web-search tool;
- Chat chunk streaming → Responses semantic event stream.

### Responses → Chat

Chat cannot natively carry Responses-only structures such as `tool_search`, `tool_search_call`, `tool_search_output`, `additional_tools`, `namespace`, `mcp`, `previous_response_id`, `conversation`, and `include`. Use an explicit bridge or emit lossy diagnostics.

## 7. AxonHub implementation guardrails

1. Do not implement Chat outbound as function-tools-only; canonical Chat supports more than deprecated function tools.
2. Do not silently drop deprecated fields if a caller supplied them.
3. Do not confuse Chat `web_search_options` with Responses hosted web-search tool definitions.
4. Keep Chat streaming conversion explicit; chunk deltas are not Responses events.
5. Preserve message content parts, tool-call IDs, and tool result messages whenever possible.
