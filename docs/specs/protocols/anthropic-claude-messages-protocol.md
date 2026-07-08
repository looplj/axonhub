# Anthropic Claude Messages Protocol Baseline

- Regenerated: 2026-07-06
- Scope: Anthropic Claude Messages wire protocol baseline for AxonHub conversion/audit work.
- Status: regenerated from canonical sources under `docs/specs/vendor/protocol-canonical-2026-07-06/`.
- Important: previous generated protocol docs are not used as source-of-truth for this file.

## 0. Canonical sources used

| Local file | Role |
|---|---|
| `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-messages-api.official-raw.md` | Official raw Markdown for `POST /v1/messages`; verified markers include `**post** /v1/messages` and `### Body Parameters`. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-messages-streaming.official-raw.md` | Official raw Markdown for Messages streaming behavior. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-mcp-connector.official-raw.md` | Official raw Markdown for MCP connector, `mcp_servers`, and `mcp_toolset`. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/SOURCES.md` | Source selection and failed-candidate notes. |

This search did not identify a better official OpenAPI/YAML schema for Anthropic Messages. The raw Markdown files above are the canonical local source set for this baseline.

## 1. Protocol identity

| Property | Value |
|---|---|
| Endpoint | `POST /v1/messages` |
| Primary input field | `messages` |
| Input style | ordered `user` / `assistant` messages with string shorthand or typed content blocks |
| System instructions | top-level `system`; no `system` role in `messages[]` |
| Output style | assistant `message` object with `content[]` blocks |
| Streaming style | named SSE events: `message_start`, `content_block_*`, `message_delta`, `message_stop` |
| Tool model | model emits `tool_use`; client replies with `tool_result` in a later user message |
| MCP connector | remote MCP through `mcp_servers` plus `tools[].type = mcp_toolset` |

Implementation rule: Claude Messages is a content-block protocol. It is not OpenAI Chat with renamed fields and not OpenAI Responses with renamed fields.

## 2. `POST /v1/messages` request fields

The canonical raw Messages API source exposes these request-body fields:

| Field | Canonical meaning for conversion |
|---|---|
| `max_tokens` | Required output token maximum; models may stop before this. |
| `messages` | Ordered prior conversation turns. Consecutive same-role turns may be combined by the API. |
| `model` | Claude model identifier. |
| `container` | Container/context feature for supported workflows. Preserve if present. |
| `inference_geo` | Inference geography selection/observation field. |
| `metadata` | Metadata object, including user-identification related fields where supported. |
| `output_config` | Output configuration object. |
| `service_tier` | Standard/priority/batch tier selection. |
| `stop_sequences` | Custom stop sequences. |
| `stream` | Whether to stream SSE events. |
| `system` | Top-level system prompt string or content blocks. There is no `system` role in messages. |
| `temperature` | Sampling temperature. |
| `thinking` | Extended/adaptive thinking configuration. |
| `tool_choice` | Tool choice configuration. |
| `tools` | Tool definitions, including client tools and server/toolset entries. |
| `top_k` | Top-k sampling. |
| `top_p` | Nucleus sampling. |

Content-block-level fields such as `cache_control`, citations, document/image/file source fields, and tool-result fields appear inside nested content/tool schemas; do not treat them as plain top-level request parameters unless source schema says so.

`mcp_servers` is not in the base Messages raw API field list extracted from `messages-api.official-raw.md`; it is confirmed by the official MCP connector companion doc. Treat it as a protocol extension/companion parameter that must be preserved when present.

## 3. Message and content block protocol

`messages[]` contains only `user` and `assistant` roles. `content` can be a string shorthand or an array of typed content blocks.

Canonical content-block families include, at minimum:

| Family | Conversion note |
|---|---|
| `text` | Text content block; may carry citations/cache control. |
| `image` | Image input block with source/media details. |
| `document` / file-related blocks | Document/file source semantics differ from OpenAI input_file. |
| `search_result` and citation-related blocks | Preserve citation metadata if present. |
| `thinking` / `redacted_thinking` | Extended thinking blocks; preserve exactly in multi-turn/tool flows. |
| `tool_use` | Assistant-requested tool call. |
| `tool_result` | User-supplied tool result in later message. |
| server tool result blocks | Web/code/bash/text-editor/server-tool outputs may appear with specific block shapes. |

Implementation rule: unknown content block types should be preserved raw on same-protocol replay rather than dropped.

## 4. Tool protocol

Claude client-side tool use is content-block based:

1. Request includes `tools[]` and optional `tool_choice`.
2. Model emits `tool_use` content block.
3. Client executes the tool.
4. Client sends a later `user` message containing `tool_result` that references the tool use id.

Important differences from OpenAI:

- Claude tool calls are content blocks, not Chat `tool_calls[]` and not Responses `function_call` items.
- Tool result is a later user content block, not a Chat `tool` role message.
- Thinking blocks around tool use have preservation requirements.
- Some tools/server tools include additional pricing/usage fields in response usage.

The canonical raw Messages source also includes tool fields such as `input_schema`, `allowed_callers`, `defer_loading`, `description`, `input_examples`, and `strict` in tool-related schemas. Preserve raw tool fields that AxonHub does not understand.

## 5. MCP connector

The official MCP connector doc confirms a separate Messages extension shape:

| Field / shape | Meaning |
|---|---|
| `mcp_servers` | Array of remote MCP server definitions: connection details, URL/name/auth. |
| `tools[].type = "mcp_toolset"` | Enables tools from a named MCP server. |
| `mcp_server_name` | Must match a server defined in `mcp_servers`. |
| per-tool config | Allowlist/denylist and tool-level configuration are documented in the connector guide. |

This is not equivalent to OpenAI Responses `mcp` tool shape. Cross-protocol conversion requires a named bridge, not simple field renaming.

## 6. Streaming protocol

Claude Messages streaming uses named SSE events. Canonical streaming source confirms event families including:

| Event / delta | Meaning |
|---|---|
| `message_start` | Starts assistant message. |
| `content_block_start` | Starts a content block. |
| `content_block_delta` | Carries deltas for text, JSON input, thinking, signatures, etc. |
| `content_block_stop` | Ends a content block. |
| `message_delta` | Updates message-level fields such as stop reason/usage. |
| `message_stop` | Ends message stream. |
| `ping` / `error` | Keepalive/error events. |
| `text_delta` | Text content delta. |
| `input_json_delta` | Incremental tool input JSON. |
| `thinking_delta` | Incremental thinking text. |
| `signature_delta` | Thinking signature delta. |

These events are neither OpenAI Chat chunks nor OpenAI Responses events.

## 7. Response object

Claude returns an assistant `message` object. Preserve:

- `id`, `type`, `role`, `model`;
- `content[]` typed blocks and order;
- `stop_reason`, `stop_sequence`, `stop_details` where present;
- `usage`, including cache, thinking, and server-tool usage details;
- `service_tier` and inference/metadata details where present.

Stop reasons include values such as `end_turn`, `max_tokens`, `stop_sequence`, `tool_use`, `pause_turn`, and `refusal` in the canonical raw source. Preserve unknown future stop reasons.

## 8. Cross-protocol implications

### Claude → Claude

Preserve raw content blocks, thinking/redacted thinking, tool_use/tool_result IDs, cache/citation fields, server-tool results, `mcp_servers`, `mcp_toolset`, stop details, and usage details.

### Claude → OpenAI Chat

Requires bridge:

- Claude content blocks → Chat messages/content parts;
- `tool_use` / `tool_result` blocks → Chat `tool_calls` / tool role messages;
- Claude MCP connector → no direct Chat equivalent;
- Claude streaming events → Chat chunks.

### Claude → OpenAI Responses

Requires bridge:

- Claude content blocks → Responses typed input/output items;
- Claude tools/server tools/MCP connector → OpenAI tool families only where a verified shape exists;
- thinking blocks → not the same as OpenAI `reasoning` items;
- Claude streaming events → Responses semantic events.

## 9. AxonHub implementation guardrails

1. Do not model Claude Messages as OpenAI Chat internally without preserving raw blocks.
2. Preserve `thinking` and `redacted_thinking` blocks exactly in multi-turn/tool flows.
3. Treat `mcp_servers` + `mcp_toolset` as Claude-specific; do not rename to OpenAI `mcp`.
4. Preserve unknown content block and tool fields on same-protocol replay.
5. Emit lossy diagnostics for cross-protocol downgrades that cannot represent Claude blocks/tools/state.
