# OpenAI Responses Protocol Baseline

- Regenerated: 2026-07-06
- Scope: OpenAI Responses wire protocol baseline for AxonHub conversion/audit work.
- Status: regenerated from canonical sources under `docs/specs/vendor/protocol-canonical-2026-07-06/`.
- Important: previous generated protocol docs are not used as source-of-truth for this file.

## 0. Canonical sources used

| Local file | Role |
|---|---|
| `docs/specs/vendor/protocol-canonical-2026-07-06/openai-api-definition.fetch.yaml` | OpenAI official machine-readable API definition fetched through `smart-search fetch`; verified text markers include `/responses` and `operationId: createResponse`. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/openai-responses-reference.exa.md` | Exa-retrieved official Developers reference extract for Responses; verified markers include `post /responses`, `### Body Parameters`, `context_management`. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/openai-responses-create.platform-snapshot.md` | Larger official platform create snapshot retained for detailed schema/event lookup. |
| `docs/specs/vendor/protocol-canonical-2026-07-06/SOURCES.md` | Source selection and failed-candidate notes. |

## 1. Protocol identity

| Property | Value |
|---|---|
| Endpoint | `POST /responses` |
| Primary input field | `input` |
| Input style | string or array of typed input/message/tool items |
| Output style | typed `response` object with `output[]` items |
| Streaming style | Responses semantic SSE events, not Chat delta chunks |
| State support | `previous_response_id`, `conversation`, `store`, prompt cache fields |
| Tool model | first-class typed tools and tool-call/tool-output items |

Implementation rule: `llm.Request` may be a cross-protocol abstraction, but it is not the complete Responses wire schema. Same-protocol Responses round trip needs a native sidecar/preservation layer.

## 2. `POST /responses` request fields

The current canonical Responses reference extract lists these create-body fields:

| Field | Canonical meaning for conversion |
|---|---|
| `background` | Run response in background. Preserve on same-protocol replay. |
| `context_management` | Request-level context management, including compaction configuration. Preserve; do not map to Chat blindly. |
| `conversation` | Conversation attachment/state. Not equivalent to Chat `messages`. |
| `include` | Requests additional output data, e.g. search sources, file search results, image URLs, logprobs, encrypted reasoning. |
| `input` | String or typed item list. Preserve raw order and item types. |
| `instructions` | Top-level instruction string. Can map to Chat developer/system only with explicit bridge policy. |
| `max_output_tokens` | Responses output-token upper bound, including visible output and reasoning tokens. Not the same name as Chat `max_completion_tokens`. |
| `max_tool_calls` | Upper bound for tool calls. Protocol-specific; preserve/diagnose if target lacks equivalent. |
| `metadata` | Request metadata map. |
| `model` | Model identifier. |
| `parallel_tool_calls` | Whether tools may be called in parallel. |
| `previous_response_id` | Server-side state continuation. Not representable as plain Chat without reconstructing context. |
| `prompt` | Prompt object/reference. Preserve as native field. |
| `prompt_cache_key` | Prompt-cache routing key. |
| `prompt_cache_retention` | Prompt-cache retention policy such as `in_memory` or `24h`. |
| `reasoning` | Reasoning configuration/state. Preserve raw object unless target protocol has a verified equivalent. |
| `safety_identifier` | Stable safety/user identifier. |
| `service_tier` | Service tier selection. |
| `store` | Whether to store the response. |
| `stream` | Whether to stream Responses semantic events. |
| `stream_options` | Responses stream options. |
| `temperature` | Sampling temperature. |
| `text` | Text output configuration. |
| `tool_choice` | Tool-choice configuration; may have object forms beyond function-only. Preserve raw if not structurally represented. |
| `tools` | Array of native Responses tools. Do not coerce to Chat function-only tools. |
| `top_logprobs` | Output logprob count/config. |
| `top_p` | Nucleus sampling. |
| `truncation` | Deprecated truncation mode. Preserve if supplied; prefer explicit compatibility handling. |
| `user` | Deprecated user identifier. Preserve/diagnose according to compatibility policy. |

Not included as a public baseline field here: `client_metadata`. It is not confirmed by these public canonical OpenAI sources; keep it in a Codex/provider compatibility sidecar if needed.

## 3. Input item protocol

Responses `input` is not just Chat `messages` under another name. The canonical reference extract contains typed item families including:

| Family | Examples / conversion note |
|---|---|
| Message-like input | input text, input image, input file, role/status/type-bearing message objects. |
| Assistant/model output replay | prior response output/message/function items can appear as input items. |
| Function call/output | `function_call`, `function_call_output`; preserve `call_id`, `id`, `status`, `namespace` when present. |
| Tool Search items | `tool_search_call`, `tool_search_output`; preserve order and tool definitions. |
| Tool outputs with multimodal content | function output can include text/image/file content, not only JSON string. |

Implementation rule: never flatten `input[]` into plain text before deciding whether the target protocol can carry each item.

## 4. Tools and lazy loading

Canonical sources confirm Responses has first-class tool families and lazy-loading features:

| Feature | Source-backed status | Implementation rule |
|---|---|---|
| `function` tools | Confirmed | Preserve schema, strict flag, description, and `defer_loading` when present. |
| hosted tools | Confirmed by reference/guides: web/file/computer/code/image-related families appear in Responses docs. | Keep as typed native tools; do not collapse to function unless explicitly bridged. |
| `tool_search` | Confirmed in canonical/reference/guides. | Bridge or preserve; Chat does not natively have this Responses item protocol. |
| `defer_loading` | Confirmed on deferred tool definitions. | Must not be silently expanded/dropped without bridge diagnostics. |
| `tool_search_call` / `tool_search_output` | Confirmed typed item families. | Preserve item order and loaded tool definitions. |
| `additional_tools` | Confirmed in detailed platform snapshot / Tool Search flow; use as native Responses state. | Preserve in native sidecar when same-protocol replay is possible. |
| `namespace` | Confirmed on function-call-related item fields. | Do not infer namespace from string splitting; preserve explicit field/mapping. |
| `mcp` | Confirmed in OpenAI tool connector docs as Responses tool family. | Treat as native Responses tool shape; not equivalent to Anthropic `mcp_servers`. |
| `apply_patch` / local shell families | Confirmed as Responses/Codex-adjacent tool families in OpenAI docs/snapshots. | Keep in native/Codex-compatible layer if present; do not force into generic function without reversible metadata. |

## 5. Response object and streaming

Responses returns a typed response object, not a Chat completion object. Conversion code should preserve:

- response id/status/error/incomplete details where available;
- `output[]` item types and order;
- usage details including reasoning/output-token-related fields;
- semantic SSE events in streaming mode.

Responses streaming events are semantic event objects such as response lifecycle, output item, content delta, tool-call-argument delta, and completion/error events. They are not equivalent to Chat `choices[].delta` chunks.

## 6. Cross-protocol implications

### Responses → Responses

Must preserve:

- raw top-level fields not represented in `llm.Request`;
- raw `input[]` typed items and order;
- raw `tools[]`, `tool_choice`, and lazy-loading state;
- `previous_response_id`, `conversation`, `include`, prompt cache fields;
- reasoning and encrypted reasoning include state;
- stream event types and item IDs/statuses.

### Responses → Chat

Potentially common fields: `model`, some sampling fields, metadata, stream flag, plain text messages, basic function tools.

Lossy or bridge-required fields: `input` typed items, `tool_search`, `defer_loading`, `tool_search_call`, `tool_search_output`, `additional_tools`, `namespace`, `mcp`, `include`, `conversation`, `previous_response_id`, Responses semantic streaming, hosted tool shapes.

### Responses → Claude Messages

Bridge-required fields: OpenAI typed input items, OpenAI hosted tools, OpenAI MCP shape, Responses stream events, `tool_search` item protocol, state/cache semantics. Claude content blocks and tool protocols are different and should not be treated as renamed Responses fields.

## 7. AxonHub implementation guardrails

1. Treat Responses as a native protocol with a sidecar/preservation layer.
2. Do not use an opaque whole-body passthrough as the only architecture; it blocks partial conversion and diagnostics.
3. Same-protocol replay should prefer native preservation over lossy canonical abstraction.
4. Cross-protocol conversion must either bridge explicitly or emit lossy diagnostics.
5. Do not silently drop unknown Responses typed items/tools.
6. Do not classify Codex-only `client_metadata` as public Responses baseline without a canonical source.
