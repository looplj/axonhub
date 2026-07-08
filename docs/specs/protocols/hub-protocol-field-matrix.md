# Hub Protocol Field Matrix

- Generated: 2026-07-06
- Scope: request-side field matrix for AxonHub protocol conversion across:
  - OpenAI Responses
  - OpenAI Chat Completions
  - Anthropic Claude Messages
- Source-of-truth for protocol fields: `docs/specs/vendor/protocol-canonical-2026-07-06/`
- Source-of-truth for Hub handling: current `llm/` code as inspected by codebase-memory graph and targeted source reads.
- Important: this matrix is a factual audit table, not a refactor design.

## Legend

| Mark | Meaning |
|---|---|
| `Native struct` | Hub has a field in that protocol's native request struct. |
| `llm.Request` | Hub has a common canonical field. |
| `TransformerMetadata` | Hub carries it through metadata side channel. |
| `ProviderExtensions` | Hub carries it through provider-native sidecar. |
| `Raw fallback` | Hub keeps raw JSON fragment/unknown field for later merge. |
| `Dropped / not modeled` | No confirmed current carrying path. |
| `Bridge required` | Target protocol lacks equivalent; conversion must be explicit and diagnosable. |

## Current Hub carrier modules

| Carrier | File | What it currently carries |
|---|---|---|
| `llm.Request` | `llm/model.go` | Cross-protocol common fields: messages, model, sampling, tool basics, stream flags, response format, reasoning shorthands, metadata, provider extensions. |
| `OpenAI Responses native request struct` | `llm/transformer/openai/responses/model.go` | Many Responses request fields: input, tools, tool_choice, include, previous_response_id, prompt cache, reasoning, etc. |
| `OpenAI Chat request struct` | `llm/transformer/openai/model.go` | Chat request fields: messages, tools, tool_choice, response_format, web-ish/sampling/reasoning fields except some current canonical fields are missing or external. |
| `Anthropic MessageRequest` | `llm/transformer/anthropic/model.go` | Claude Messages request fields: max_tokens, messages, system, thinking, output_config, tools, tool_choice, cache_control, context_management, etc. |
| `ProviderExtensions.OpenAIResponses.Request` | `llm/provider_extensions.go` | Responses private sidecar: client_metadata, raw top-level fields, native tools, additional_tools, raw tools, raw tool_choice, raw input items, prepend count. |

---

# 1. OpenAI Responses request fields

Canonical source files:

- `openai-api-definition.fetch.yaml`
- `openai-responses-reference.exa.md`
- `openai-responses-create.platform-snapshot.md`

| Responses field / family | Meaning | Hub has field? | Same protocol: Responses → Responses | To Chat | To Claude Messages | Notes |
|---|---|---|---|---|---|---|
| `model` | Model id. | `llm.Request.Model`; Responses native `Request.Model`. | Preserved. | Maps to Chat `model`. | Maps to Claude `model`. | Common field. |
| `input` | String or typed input item list. | Responses native `Request.Input`; common conversion to `llm.Messages`; raw-only items in `ProviderExtensions.RawInputItems`; `additional_tools` split out. | Partly native + raw preserved. | Converts mostly to Chat `messages`; Responses-only typed items require downgrade/bridge. | Converts mostly to Claude `messages`; typed items require bridge. | Important source of loss if forced through `llm.Messages`. |
| `instructions` | Top-level developer/system instruction. | Responses native `Request.Instructions`; converts from/to messages. | Preserved as field when emitting Responses. | Usually maps to developer/system message. | Maps to top-level `system` only with bridge policy. | Not identical to Chat/Claude roles. |
| `tools` | Native Responses tool list. | Responses native `Request.Tools`; `ProviderExtensions.NativeTools`, `RawTools`, `ToolSignatures`. | Native/raw tools merged back by `request_extensions.go`. | Current Chat path mostly uses `llm.Tools`; Responses-specific tools need downgrade diagnostics. | Current Claude path converts function and some web search; others skipped or need bridge. | Should become native AST field. |
| `tool_choice` | Tool selection policy. | Responses native `Request.ToolChoice`; `ProviderExtensions.RawToolChoice`. | Raw tool_choice can be restored if matching current tools. | Maps only where Chat shape supports it; object forms may need diagnostics. | Maps only where Claude `tool_choice` supports it. | Raw object preservation exists for Responses. |
| `parallel_tool_calls` | Allows parallel tool calls. | `llm.Request.ParallelToolCalls`; protocol structs. | Preserved. | Preserved. | Converted inversely into Claude `tool_choice.disable_parallel_tool_use` when tools exist. | Claude shape differs. |
| `tool_search` tool | Lazy tool discovery tool. | Known/preserved through Responses native/raw tools. | Preserved through native tools/raw merge. | No native Chat equivalent. | Claude has conceptually related `tool_search`, but different shape. | Bridge required. |
| `defer_loading` on tools | Mark expensive tools for lazy loading. | Preserved in Responses native/raw tools. | Preserved for Responses tools. | No native Chat equivalent. | Claude tools also have `defer_loading`, but shape differs. | Must not silently drop. |
| `tool_search_call` input item | Model/client tool-search state item. | Raw/known input fragment preservation. | Preserved as raw item if not structurally represented. | No native Chat item. | Different Claude protocol. | Bridge or diagnostics. |
| `tool_search_output` input item | Loaded tool definitions returned after tool search. | Raw/known input fragment preservation. | Preserved as raw item. | No native Chat item. | Different Claude protocol. | Bridge or diagnostics. |
| `additional_tools` input item | Lazy-loaded extra tool definitions. | `ProviderExtensions.AdditionalTools`. | Replayed separately from `RawInputItems`. | No native Chat item. | No direct Claude equivalent. | Existing sidecar exists. |
| `namespace` on function/tool call identity | Tool grouping / DispatchRegistry identity. | Some Responses model fields/tests; namespace map behavior in Responses tests; raw fragments. | Preserved/restored in some custom/function call paths. | Chat lacks namespace; current bridge may flatten to CompositeName. | Claude lacks same OpenAI namespace field. | BridgeAsymmetry hotspot. |
| `mcp` tool | OpenAI Responses MCP connector/tool family. | Not a first-class common field; can be preserved as native/raw tool. | Preserved only if native/raw tool merge keeps it. | No native Chat equivalent. | Not same as Claude `mcp_servers`. | Bridge required. |
| `apply_patch` / shell-like tool families | Responses/Codex-adjacent tool families. | Not a common field; raw/native tool preservation path. | Preserve as raw/native tool where present. | No native Chat equivalent except custom/function emulation. | No direct Claude equivalent. | Needs Codex/Responses native handling. |
| `include` | Request additional output data. | Responses native `Request.Include`; `llm.TransformerMetadata[include]`. | Preserved on Responses emit. | No direct Chat equivalent. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `previous_response_id` | Continue from prior Responses state. | `llm.Request.PreviousResponseID`; Responses native. | Preserved. | No direct Chat equivalent. | No direct Claude equivalent. | Requires context reconstruction if downgrading. |
| `conversation` | Attach response to server conversation. | Responses native has commented-out field; raw top-level fallback may preserve. | Raw fallback if inbound unknown/top-level preservation captures it. | No Chat equivalent. | No Claude equivalent. | Needs first-class native field if standard. |
| `background` | Run response in background. | Responses native `Request.Background`; metadata key used in outbound. | Preserved. | No direct Chat equivalent. | No direct Claude equivalent. | Downgrade diagnostics. |
| `context_management` | Context compaction/management config. | Not in Responses native struct seen; raw top-level fallback may preserve. | Raw fallback only if captured. | No direct Chat equivalent. | Claude has different `context_management`. | Should be first-class in Responses native. |
| `max_output_tokens` | Responses output token cap. | Responses native `MaxOutputTokens`; maps from `llm.MaxCompletionTokens/MaxTokens`. | Preserved. | Maps to Chat `max_completion_tokens` only by policy. | Maps to Claude `max_tokens`. | Names/semantics differ. |
| `max_tool_calls` | Max total tool calls. | Responses native `MaxToolCalls`; metadata key. | Preserved. | No direct Chat equivalent. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `metadata` | Metadata map. | `llm.Request.Metadata`; protocol structs. | Preserved. | Preserved. | Converted to Claude metadata where supported. | Mostly common. |
| `client_metadata` | Codex/client metadata, not public canonical baseline here. | Responses native `ClientMetadata`; `ProviderExtensions.ClientMetadata`. | Preserved if sidecar present. | No Chat equivalent. | No Claude equivalent. | Keep as compatibility field, not public Responses baseline. |
| `prompt` | Prompt template/reference. | Responses native `Prompt`; metadata key used outbound. | Preserved. | No direct Chat equivalent. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `prompt_cache_key` | Prompt cache key. | `llm.Request.PromptCacheKey`; protocol structs. | Preserved. | Preserved in Chat. | No direct Claude top-level equivalent; Claude uses cache_control. | Cross-protocol requires policy. |
| `prompt_cache_retention` | Prompt cache retention. | Responses native `PromptCacheRetention`; metadata key. | Preserved. | Chat also has canonical field; Hub Chat struct lacks direct field currently. | No direct Claude equivalent. | Chat gap. |
| `reasoning` | Responses reasoning config/state. | Responses native `Reasoning`; `llm` has reasoning shorthands. | Preserved as Responses object where set. | Chat has `reasoning_effort`/other reasoning forms, not same object. | Claude `thinking` differs. | Bridge required. |
| `safety_identifier` | Stable safety/user id. | `llm.Request.SafetyIdentifier`; protocol structs. | Preserved. | Preserved. | No exact Claude field; may map to metadata/user_id if policy says. | Needs policy. |
| `service_tier` | Service tier. | `llm.Request.ServiceTier`; protocol structs. | Preserved. | Preserved. | Claude has service_tier but enum differs. | Validate per target. |
| `store` | Store output. | `llm.Request.Store`; protocol structs. | Preserved. | Preserved. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `stream` | Enable stream. | `llm.Request.Stream`; protocol structs. | Preserved. | Preserved. | Preserved. | Stream event shapes differ. |
| `stream_options` | Responses stream options. | `llm.Request.StreamOptions`; Responses native. | Preserved where modeled. | Chat has stream_options but shape differs. | Claude streaming has no same object. | Bridge/diagnostics. |
| `temperature` | Sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `text` | Responses text output config. | Responses native `Text`. | Preserved for Responses. | Chat uses `response_format`/message output, not same. | Claude no direct equivalent. | Bridge required. |
| `top_logprobs` | Logprob count. | `llm.Request.TopLogprobs`; protocol structs. | Preserved. | Preserved. | No direct Claude equivalent. | Diagnostics on Claude downgrade. |
| `top_p` | Nucleus sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common-ish. |
| `truncation` | Responses truncation mode. | Responses native `Truncation`; metadata key. | Preserved. | No direct Chat equivalent. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `user` | Deprecated user id. | `llm.Request.User`; protocol structs. | Preserved. | Preserved as deprecated Chat field. | No direct Claude equivalent. | Prefer target-specific safety/metadata. |

---

# 2. OpenAI Chat Completions request fields

Canonical source files:

- `openai-api-definition.fetch.yaml`
- `openai-chat-completions-create.developers-snapshot.md`

| Chat field / family | Meaning | Hub has field? | Same protocol: Chat → Chat | To Responses | To Claude Messages | Notes |
|---|---|---|---|---|---|---|
| `messages` | Ordered role messages. | `llm.Request.Messages`; Chat native `Request.Messages`. | Preserved. | Converts to Responses `input`/`instructions`. | Converts to Claude `messages` + top-level `system`. | Role semantics differ. |
| `model` | Model id. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `audio` | Audio output params. | Chat native `Request` has no direct `Audio` field in inspected struct; `llm` may carry modalities only. | Likely not fully modeled. | No direct Responses unless modalities/audio bridge. | No direct Claude equivalent. | Gap. |
| `frequency_penalty` | Sampling penalty. | Common/protocol structs. | Preserved. | Responses native has field though not in canonical public list used here; current code emits it. | No Claude equivalent. | Potential provider extension / downgrade. |
| `logit_bias` | Token bias map. | `llm.Request.LogitBias`; Chat native. | Preserved. | No direct Responses field confirmed in regenerated baseline. | No Claude equivalent. | Diagnostics on downgrade. |
| `logprobs` | Return logprobs. | `llm.Request.Logprobs`; Chat native. | Preserved. | Responses uses `top_logprobs`/include logprobs. | No Claude equivalent. | Needs policy. |
| `max_completion_tokens` | Chat output cap. | `llm.Request.MaxCompletionTokens`; Chat native. | Preserved. | Maps to Responses `max_output_tokens`. | Maps to Claude `max_tokens`. | Semantics differ around reasoning tokens. |
| `max_tokens` deprecated | Deprecated Chat output cap. | `llm.Request.MaxTokens`; Chat native. | Preserved. | Used as fallback for Responses `max_output_tokens`. | Used by Claude max token resolver. | Compatibility field. |
| `metadata` | Metadata map. | Common/protocol structs. | Preserved. | Preserved. | Converted to Claude metadata where supported. | Mostly common. |
| `modalities` | Output modalities. | `llm.Request.Modalities`; Chat native; Responses native. | Preserved. | Preserved to Responses native field. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `moderation` | Moderation config. | Not seen in Chat native `Request` / `llm.Request`. | Dropped / not modeled. | Dropped / not modeled. | Dropped / not modeled. | Gap against canonical Chat. |
| `n` | Number of choices. | Not supported in `llm.Request` comment says always 1. | Not modeled. | Not modeled. | Not modeled. | Intentional unsupported. |
| `parallel_tool_calls` | Parallel tools. | Common/protocol structs. | Preserved. | Preserved. | Inverted into Claude `disable_parallel_tool_use` if tools exist. | Target shape differs. |
| `prediction` | Predicted output. | TODO in `llm.Request`; not modeled. | Dropped / not modeled. | No direct equivalent. | No direct equivalent. | Gap. |
| `presence_penalty` | Sampling penalty. | Common/protocol structs. | Preserved. | Current Responses emits it although regenerated baseline did not list it. | No Claude equivalent. | Needs policy. |
| `prompt_cache_key` | Prompt cache key. | `llm.Request.PromptCacheKey`; Chat native. | Preserved. | Preserved. | No direct Claude equivalent. | Claude uses cache_control. |
| `prompt_cache_retention` | Prompt cache retention. | `llm.Request` lacks direct field; Responses metadata path exists; Chat native struct lacks direct field. | Likely dropped in Chat path. | Responses can preserve via metadata if inbound captured. | No direct Claude equivalent. | Gap against canonical Chat. |
| `reasoning_effort` | Reasoning effort. | `llm.Request.ReasoningEffort`; Chat native. | Preserved. | Converts to Responses reasoning-ish only where implemented. | Converts to Claude thinking via thinking helpers. | Semantics differ. |
| `response_format` | Text/JSON schema/object. | `llm.Request.ResponseFormat`; Chat native. | Preserved. | Responses `text` config may represent some forms. | Claude JSON/output config differs. | Bridge required. |
| `safety_identifier` | Safety/user id. | Common/protocol structs. | Preserved. | Preserved. | No direct Claude field except metadata policy. | Needs policy. |
| `seed` deprecated | Deterministic sampling hint. | `llm.Request.Seed`; Chat native. | Preserved. | No direct Responses equivalent. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `service_tier` | Service tier. | Common/protocol structs. | Preserved. | Preserved. | Claude has different enum. | Validate per target. |
| `stop` | Stop sequence(s). | `llm.Request.Stop`; Chat native. | Preserved. | No direct Responses field in regenerated baseline. | Converts to `stop_sequences`. | Target semantics differ. |
| `store` | Store output. | Common/protocol structs. | Preserved. | Preserved. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `stream` | Enable stream. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Event shape differs. |
| `stream_options` | Chat stream options. | Common/protocol structs. | Preserved. | Responses stream options differ. | No direct Claude equivalent. | Bridge/diagnostics. |
| `temperature` | Sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `tool_choice` | Tool selection. | `llm.Request.ToolChoice`; Chat native. | Preserved for supported forms. | Converts to Responses tool_choice. | Converts to Claude tool_choice. | Object forms differ. |
| `tools` function | Function tools. | `llm.Tools`; Chat native `Tools`. | Preserved. | Converts to Responses function tool. | Converts to Claude client tool. | Common-ish. |
| `tools` custom | Custom tool forms. | Canonical Chat supports it; current Chat outbound filters `llm.Tools` to function only. | Currently at risk / not fully supported. | Responses has custom tools. | Claude custom client tools differ. | Clear gap. |
| deprecated `function_call` | Old tool choice. | Chat native has field in canonical; current struct not seen with direct field; conversion focuses `ToolChoice`. | Likely normalized/dropped unless inbound handles elsewhere. | Should map to tool_choice/function. | Should map to Claude tool_choice. | Need audit. |
| deprecated `functions` | Old tool list. | Current `llm.Request` uses `Tools`; direct deprecated field not in inspected common struct. | Likely normalized on inbound if implemented; not direct. | Should map to tools. | Should map to tools. | Need audit. |
| `top_logprobs` | Logprob count. | Common/protocol structs. | Preserved. | Responses has top_logprobs. | No Claude equivalent. | Diagnostics on Claude downgrade. |
| `top_p` | Nucleus sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `user` deprecated | User id. | Common/protocol structs. | Preserved. | Preserved as Responses deprecated field. | No direct Claude equivalent. | Prefer safety/metadata. |
| `verbosity` | Verbosity setting. | `llm.Request.Verbosity`; Chat native. | Preserved. | No direct Responses equivalent confirmed. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `web_search_options` | Chat web search options object. | Canonical Chat supports it; inspected Chat native `Request` lacks direct field. | Likely not modeled in current Chat path. | Not same as Responses web_search tool. | Not same as Claude web search tool. | Gap and bridge required. |

---

# 3. Anthropic Claude Messages request fields

Canonical source files:

- `anthropic-messages-api.official-raw.md`
- `anthropic-messages-streaming.official-raw.md`
- `anthropic-mcp-connector.official-raw.md`

| Claude field / family | Meaning | Hub has field? | Same protocol: Claude → Claude | To Responses | To Chat | Notes |
|---|---|---|---|---|---|---|
| `max_tokens` | Required output cap. | `MessageRequest.MaxTokens`; maps from `llm.MaxTokens/MaxCompletionTokens`. | Preserved. | Maps to Responses `max_output_tokens`. | Maps to Chat `max_completion_tokens`/`max_tokens`. | Semantics differ. |
| `messages` | Ordered user/assistant turns. | `MessageRequest.Messages`; `llm.Messages`. | Preserved through Claude native conversion. | Converts to Responses `input`/messages. | Converts to Chat `messages`. | Claude has no `system` role. |
| `model` | Claude model id. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `system` | Top-level system prompt/content. | `MessageRequest.System`; conversion helpers. | Preserved. | Maps to Responses instructions/input by policy. | Maps to Chat developer/system messages. | Role semantics differ. |
| `temperature` | Sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `top_k` | Top-k sampling. | `MessageRequest.TopK`; common carries via `TransformerMetadata`. | Preserved via metadata restoration. | Responses has extension `top_k` in current struct. | Chat has OpenRouter extension `top_k` in native struct. | Extension, not universal. |
| `top_p` | Nucleus sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `metadata` | Request metadata, user id. | `MessageRequest.Metadata`; `llm.Metadata` partial. | Preserved. | Maps to metadata/safety policy. | Maps to metadata/user policy. | Shapes differ. |
| `service_tier` | Claude service tier. | `MessageRequest.ServiceTier`; common service tier. | Preserved. | OpenAI service tier enum differs. | OpenAI service tier enum differs. | Validate per target. |
| `stop_sequences` | Stop sequences. | `MessageRequest.StopSequences`; `llm.Stop`. | Preserved. | No direct Responses field in regenerated baseline. | Maps to Chat `stop`. | Target differences. |
| `stream` | Enable stream. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Event shape differs. |
| `thinking` | Extended/adaptive thinking config. | `MessageRequest.Thinking`; llm reasoning helpers. | Preserved. | Not same as Responses `reasoning`. | Not same as Chat `reasoning_effort`. | Bridge required. |
| `redacted_thinking` block | Preserved hidden thinking content block. | Content block support/tests exist. | Preserved in Claude paths. | No direct Responses equivalent. | No direct Chat equivalent. | Must preserve raw if same-protocol. |
| `output_config` | Claude output config. | `MessageRequest.OutputConfig`. | Preserved. | No direct Responses equivalent. | Maybe response_format bridge, not same. | Bridge/diagnostics. |
| `tools` client tools | Claude client tool definitions. | `MessageRequest.Tools`; `llm.Tools` conversion. | Preserved for supported forms. | Converts to Responses tools only by bridge. | Converts to Chat tools only by bridge. | Tool schemas differ. |
| `tool_choice` | Claude tool choice. | `MessageRequest.ToolChoice`; conversion helpers. | Preserved. | Different Responses shape. | Different Chat shape. | Bridge required. |
| `tool_use` content block | Assistant tool call. | Claude content block model/tests. | Preserved. | Converts to Responses function/tool call item only with bridge. | Converts to Chat `tool_calls` only with bridge. | Not same structure. |
| `tool_result` content block | User tool result. | Claude content block model/tests. | Preserved. | Converts to Responses function_call_output only with bridge. | Converts to Chat `tool` role message only with bridge. | Not same structure. |
| `server_tool_use` / server tool results | Claude server-side tool blocks/results. | Some content block support visible in tests/model. | Preserve if modeled/raw. | OpenAI tools differ. | Chat tools differ. | Needs native coverage audit. |
| `cache_control` | Prompt caching at block/top-level. | `MessageRequest.CacheControl`; content block cache_control. | Preserved. | OpenAI prompt_cache_key/retention differ. | OpenAI prompt_cache_key/retention differ. | Bridge policy required. |
| `container` | Container/context field. | Canonical field; not seen in `MessageRequest` inspected struct. | Likely dropped / not modeled. | No direct equivalent. | No direct equivalent. | Gap. |
| `inference_geo` | Inference geography. | Canonical field; not seen in `MessageRequest` inspected struct. | Likely dropped / not modeled. | No direct equivalent. | No direct equivalent. | Gap. |
| `context_management` | Claude context edits/management. | `MessageRequest.ContextManagement`; carried via metadata comment. | Preserved. | Responses has different `context_management`. | No direct Chat equivalent. | Same name does not mean same shape. |
| `mcp_servers` | Remote MCP server definitions. | Canonical MCP connector field; not seen in `MessageRequest` inspected struct. | Likely dropped / not modeled unless raw path elsewhere. | Not same as OpenAI Responses `mcp`. | No Chat equivalent. | Major gap. |
| `mcp_toolset` tool | Enables tools from MCP server. | Could be represented as `Tool` if model supports type; current conversion needs audit. | Not confirmed full support. | Bridge to OpenAI `mcp` required. | No direct Chat equivalent. | Major gap. |
| `anthropic_version` | Anthropic version header/field for platforms. | `MessageRequest.AnthropicVersion`. | Preserved where needed. | No OpenAI equivalent. | No OpenAI equivalent. | Provider-specific. |
| `anthropic_beta` | Anthropic beta flags. | `MessageRequest.AnthropicBeta`. | Preserved where needed. | No OpenAI equivalent. | No OpenAI equivalent. | Provider-specific. |

---

# 4. Cross-protocol summary table

| Source → Target | Current broad behavior | Main risks |
|---|---|---|
| Responses → Responses | Uses `llm.Request` plus `ProviderExtensions.OpenAIResponses.Request` and raw merge to preserve some native fields. | Native protocol is split across common fields, metadata, provider extensions, raw fragments; `context_management`/`conversation` need first-class audit. |
| Responses → Chat | Converts through common messages/tools; lossy diagnostics exist for some Responses-native fields. | `tool_search`, `additional_tools`, `namespace`, `mcp`, Responses typed items cannot be represented natively in Chat. |
| Responses → Claude | Converts through common messages/tools. | OpenAI Responses `mcp`/tool_search/items/reasoning differ from Claude content blocks/MCP connector/thinking. |
| Chat → Chat | Uses Chat native `RequestFromLLM`; preserves many fields. | Current tool conversion is function-only; `custom tools`, `web_search_options`, `prompt_cache_retention`, `moderation`, `prediction`, `audio` need audit. |
| Chat → Responses | Converts messages/tools into Responses request. | Chat `web_search_options` is not Responses web_search tool; custom tools need explicit support. |
| Chat → Claude | Converts messages/tools into Claude request. | Chat custom tools / web_search_options / response_format semantics may be dropped or approximated. |
| Claude → Claude | Uses Claude native `MessageRequest` for many fields. | `mcp_servers`, `container`, `inference_geo` not seen in current struct; stream aggregator is complex hotspot. |
| Claude → Responses | Converts through common abstraction. | Claude thinking/redacted_thinking/tool_use/tool_result/MCP connector do not map directly to Responses. |
| Claude → Chat | Converts through common abstraction. | Claude content blocks and thinking/server tool results do not map directly to Chat messages. |

---

# 5. Immediate gaps confirmed by this matrix

| Gap | Why it matters | Where seen |
|---|---|---|
| Chat `custom` tools are canonical but current Chat outbound filters to function tools. | Tool calls can silently disappear or become impossible in Chat same-protocol/cross-protocol paths. | `transformer/openai/outbound_convert.go RequestFromLLM`. |
| Responses native protocol is split across native struct + `llm.Request` + metadata + provider extensions. | Same-protocol fidelity is hard to reason about; adding fields is scattered. | `responses/outbound.go`, `responses/request_extensions.go`, `provider_extensions.go`. |
| Responses `context_management` and `conversation` are canonical but not first-class in inspected Responses native struct. | These can only survive if raw top-level fallback catches them; code cannot reason about them cleanly. | `responses/model.go`; canonical docs. |
| Chat `web_search_options` is canonical but not seen in inspected Chat native struct. | Chat same-protocol and Chat→Responses web search behavior may be wrong. | `openai/model.go`; canonical docs. |
| Anthropic `mcp_servers`, `container`, `inference_geo` are canonical/companion fields but not seen in inspected `MessageRequest`. | Claude same-protocol MCP connector/context fields may be dropped. | `anthropic/model.go`; canonical docs. |
| Claude stream aggregation is a high-complexity hotspot. | Even if request fields are fixed, response/stream native fidelity is risky. | `anthropic/aggregator.go AggregateStreamChunks`, cognitive 254. |

## Recommended next audit slice

Before implementing architecture changes, verify the gaps above with targeted tests or request round-trip fixtures:

1. OpenAI Chat same-protocol: `custom` tool and `web_search_options` round-trip.
2. OpenAI Responses same-protocol: `context_management`, `conversation`, `tool_search`, `additional_tools`, `namespace`, `mcp` round-trip.
3. Anthropic same-protocol: `mcp_servers`, `mcp_toolset`, `container`, `inference_geo`, `thinking/redacted_thinking`, `tool_use/tool_result` round-trip.
