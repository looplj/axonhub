# Hub Protocol Field Matrix

- Generated: 2026-07-06
- Implementation notes refreshed: 2026-07-22
- Scope: request-side field matrix for AxonHub protocol conversion across:
  - OpenAI Responses
  - OpenAI Chat Completions
  - Anthropic Claude Messages
- Source-of-truth for protocol fields: `docs/specs/vendor/protocol-canonical-2026-07-06/`
- Source-of-truth for Hub handling: current `llm/` code as inspected by codebase-memory graph and targeted source reads.
- Important: this matrix is a factual audit table, not a refactor design.
- **Authority:** for wire facts prefer the three `*-protocol.md` baselines; for completion status prefer `protocol-conversion-strict-verification-matrix.md` Field IDs; for ownership rules prefer ADR 0001/0002 and `.trellis/spec/backend/protocol-transformer-guidelines.md`. This file is navigation + gap index and can lag code.

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
| `OpenAI Chat request + raw replay` | `llm/transformer/openai/model.go`, `llm/transformer/openai/chat_n.go` | Typed Chat fields plus same-protocol raw replay for `n`, `prompt_cache_retention`, `audio`, `prediction`, `moderation`, `web_search_options`, deprecated `functions`, and deprecated request `function_call`. |
| `Anthropic MessageRequest + metadata/raw fragments` | `llm/transformer/anthropic/model.go`, `inbound_convert.go`, `outbound_convert.go` | Native Claude fields plus opaque metadata roundtrip for `container`, `inference_geo`, `mcp_servers`; adapter-only `mcp_toolset` entries are captured from `anthropic.Tool.Raw` as indexed raw fragments and merged back in order. |
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
| request `input[]` item `id` | Same-protocol identity for message / tool / reasoning input items. | `Message.ID`, `ToolCall.ResponseItemID`, `Message.ResponseReasoningItemID` (`*string` presence carrier). | G15a/b/c: preserve supplied non-empty ids; omit when source omitted; identities stay independent under multi-item merge; no synthesis/fallback. | Not a Chat field; do not invent Responses item ids from Chat history. | Not a Claude field; do not invent Responses item ids from Claude blocks. | Same-protocol fidelity only; cross-protocol no-synth. |
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
| `prompt_cache_retention` | Prompt cache retention. | Responses native `PromptCacheRetention`; metadata key. | Preserved in Responses. | Chat has a separate same-protocol `RawRequest` replay seam; no cross-protocol synthesis from Responses metadata. | No direct Claude equivalent. | Responses and Chat each preserve their native wire field, without claiming semantic bridgeability. |
| `reasoning` | Responses reasoning config/state and reasoning output/stream fidelity. | Native `Reasoning` plus metadata/raw object sidecars; item/stream/aggregator support for reasoning text. | G7 preserves `context`, keeps deprecated `generate_summary` identity distinct, restores unknown nested keys, and handles reasoning `content[]`/`reasoning_text` plus `.delta`/`.done`. G13/G14 same-protocol preserve supplied request `reasoning`/`include`/`summary`/`stream_options` without Codex default injection or model-capability gates. | Chat has `reasoning_effort`/other reasoning forms, not the same object. | Claude `thinking` differs. | Same-protocol support does not establish cross-protocol equivalence. |
| `safety_identifier` | Stable safety/user id. | `llm.Request.SafetyIdentifier`; protocol structs. | Preserved. | Preserved. | No exact Claude field; may map to metadata/user_id if policy says. | Needs policy. |
| `service_tier` | Service tier. | `llm.Request.ServiceTier`; protocol structs. | Preserved. | Preserved. | Claude has service_tier but enum differs. | Validate per target. |
| `store` | Store output. | `llm.Request.Store`; protocol structs. | Preserved. | Preserved. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `stream` | Enable stream. | `llm.Request.Stream`; protocol structs. | Preserved. | Preserved. | Preserved. | Stream event shapes differ. |
| `stream_options` | Responses stream options. | Typed `StreamOptions` plus dedicated `ProviderExtensions.OpenAIResponses.Request.RawStreamOptions` deep-clone sidecar. | G9/G14 same-protocol: typed + raw nested merge; known fields (incl. summary delivery) do not pollute raw top-level or false unknown diagnostics. | Chat has stream_options but shape differs. | Claude streaming has no same object. | Same-protocol preserve only; no cross-protocol synthesis. |
| `temperature` | Sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `text` | Responses text output config. | Responses native `Text`. | Preserved for Responses. | Chat uses `response_format`/message output, not same. | Claude no direct equivalent. | Bridge required. |
| `top_logprobs` | Logprob count. | `llm.Request.TopLogprobs`; protocol structs. | Preserved. | Preserved. | No direct Claude equivalent. | Diagnostics on Claude downgrade. |
| `top_p` | Nucleus sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common-ish. |
| `truncation` | Responses truncation mode. | Responses native `Truncation`; metadata key. | Preserved. | No direct Chat equivalent. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `user` | Deprecated user id. | `llm.Request.User`; protocol structs. | Preserved. | Preserved as deprecated Chat field. | No direct Claude equivalent. | Prefer target-specific safety/metadata. |

---

# 2. OpenAI Chat Completions request and deprecated compatibility fields

Canonical source files:

- `openai-api-definition.fetch.yaml`
- `openai-chat-completions-create.developers-snapshot.md`

| Chat field / family | Meaning | Hub has field? | Same protocol: Chat → Chat | To Responses | To Claude Messages | Notes |
|---|---|---|---|---|---|---|
| `messages` | Ordered role messages. | `llm.Request.Messages`; Chat native `Request.Messages`. | Preserved. | Converts to Responses `input`/`instructions`. | Converts to Claude `messages` + top-level `system`. | Role semantics differ. |
| `model` | Model id. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `audio` | Audio output params. | Chat-native raw replay in `chat_n.go`; no public common typed carrier. | Preserved byte-for-byte for Chat→Chat from original `RawRequest`. | No direct Responses synthesis. | No direct Claude equivalent. | Same-protocol raw support; cross-protocol lossy/unsupported. |
| `frequency_penalty` | Sampling penalty. | Common/protocol structs. | Preserved. | Responses native has field though not in canonical public list used here; current code emits it. | No Claude equivalent. | Potential provider extension / downgrade. |
| `logit_bias` | Token bias map. | `llm.Request.LogitBias`; Chat native. | Preserved. | No direct Responses field confirmed in regenerated baseline. | No Claude equivalent. | Diagnostics on downgrade. |
| `logprobs` | Return logprobs. | `llm.Request.Logprobs`; Chat native. | Preserved. | Responses uses `top_logprobs`/include logprobs. | No Claude equivalent. | Needs policy. |
| `max_completion_tokens` | Chat output cap. | `llm.Request.MaxCompletionTokens`; Chat native. | Preserved. | Maps to Responses `max_output_tokens`. | Maps to Claude `max_tokens`. | Semantics differ around reasoning tokens. |
| `max_tokens` deprecated | Deprecated Chat output cap. | `llm.Request.MaxTokens`; Chat native. | Preserved. | Used as fallback for Responses `max_output_tokens`. | Used by Claude max token resolver. | Compatibility field. |
| `metadata` | Metadata map. | Common/protocol structs. | Preserved. | Preserved. | Converted to Claude metadata where supported. | Mostly common. |
| `modalities` | Output modalities. | `llm.Request.Modalities`; Chat native; Responses native. | Preserved. | Preserved to Responses native field. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `moderation` | Moderation config. | Chat-native raw replay in `chat_n.go`; no public common typed carrier. | Preserved for Chat→Chat from original `RawRequest`. | Not synthesized to Responses. | Not synthesized to Claude. | Same-protocol raw support only. |
| `n` | Number of choices. | Chat-native raw replay in `chat_n.go`; the commented common `llm.Request.N` remains unused. | Preserved for Chat→Chat from original `RawRequest`. | Not synthesized to Responses. | Not synthesized to Claude. | Same-protocol wire preservation, not common multi-choice semantics. |
| `parallel_tool_calls` | Parallel tools. | Common/protocol structs. | Preserved. | Preserved. | Inverted into Claude `disable_parallel_tool_use` if tools exist. | Target shape differs. |
| `prediction` | Predicted output. | Chat-native raw replay in `chat_n.go`; no public common typed carrier. | Preserved for Chat→Chat from original `RawRequest`. | No direct Responses synthesis. | No direct Claude equivalent. | Same-protocol raw support only. |
| `presence_penalty` | Sampling penalty. | Common/protocol structs. | Preserved. | Current Responses emits it although regenerated baseline did not list it. | No Claude equivalent. | Needs policy. |
| `prompt_cache_key` | Prompt cache key. | `llm.Request.PromptCacheKey`; Chat native. | Preserved. | Preserved. | No direct Claude equivalent. | Claude uses cache_control. |
| `prompt_cache_retention` | Prompt cache retention. | Responses has native/metadata support; Chat uses raw replay in `chat_n.go`. | Preserved for Chat→Chat from original `RawRequest`. | Responses retains its own native field, but Chat raw state is not synthesized cross-protocol. | No direct Claude equivalent. | Protocol-native preservation paths remain distinct. |
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
| `tools` custom | Custom tool forms. | Canonical Chat supports it; Hub common path supports Chat `custom` plus Responses-custom bridge carriers (`OpenAIChatCustomTool` / `ResponseCustomTool`). | Same-protocol Chat custom preserve + Responses custom bridge with tests (not function-only). | Responses has native `custom` tools. | Claude custom client tools differ; no automatic full equivalence. | Cross-protocol remains partial; do not treat as fully lossless to Claude. |
| deprecated `function_call` | Old request tool choice. | Chat-native raw replay in `chat_n.go`. | Original deprecated wire field is restored for Chat→Chat. | Not automatically rewritten as Responses tool choice. | Not automatically rewritten as Claude tool choice. | Deprecated identity is preserved without claiming a semantic bridge. |
| deprecated `functions` | Old tool list. | Chat-native raw replay in `chat_n.go`. | Original deprecated wire field is restored for Chat→Chat. | Not automatically rewritten as Responses tools. | Not automatically rewritten as Claude tools. | Kept separate from modern `tools`. |
| deprecated response `message.function_call` | Old assistant function-call response/history/stream shape. | Parsed into the modern common tool-call lifecycle with origin metadata in the OpenAI adapter. | Re-emitted as deprecated `message.function_call`; targeted response, request-history, and stream tests cover it. | No claim of Responses wire-shape equivalence. | No claim of Claude wire-shape equivalence. | Modern `tool_calls` remain intact when deprecated origin metadata is absent. |
| `top_logprobs` | Logprob count. | Common/protocol structs. | Preserved. | Responses has top_logprobs. | No Claude equivalent. | Diagnostics on Claude downgrade. |
| `top_p` | Nucleus sampling. | Common/protocol structs. | Preserved. | Preserved. | Preserved. | Common. |
| `user` deprecated | User id. | Common/protocol structs. | Preserved. | Preserved as Responses deprecated field. | No direct Claude equivalent. | Prefer safety/metadata. |
| `verbosity` | Verbosity setting. | `llm.Request.Verbosity`; Chat native. | Preserved. | No direct Responses equivalent confirmed. | No direct Claude equivalent. | Diagnostics on downgrade. |
| `web_search_options` | Chat web search options object. | Chat-native raw replay in `chat_n.go`; no public common typed carrier. | Preserved for Chat→Chat from original `RawRequest`. | Not the Responses web-search tool and not synthesized. | Not the Claude web-search tool and not synthesized. | Same-protocol raw support; cross-protocol bridge remains unsupported. |

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
| `container` | Container/context field. | `MessageRequest.Container` opaque JSON -> `TransformerMetadata[anthropic_container]` -> Anthropic outbound restore. | Preserved for Claude→Claude, including unknown nested keys. | No direct equivalent; not synthesized. | No direct equivalent; not synthesized. | Same-protocol opaque preservation only. |
| `inference_geo` | Inference geography. | `MessageRequest.InferenceGeo` opaque JSON -> `TransformerMetadata[anthropic_inference_geo]` -> Anthropic outbound restore. | Preserved for Claude→Claude. | No direct equivalent; not synthesized. | No direct equivalent; not synthesized. | Allowed-value/source audit remains separate from wire preservation. |
| `context_management` | Claude context edits/management. | `MessageRequest.ContextManagement`; carried via metadata comment. | Preserved. | Responses has different `context_management`. | No direct Chat equivalent. | Same name does not mean same shape. |
| `mcp_servers` | Remote MCP server definitions. | `MessageRequest.MCPServers` opaque JSON -> `TransformerMetadata[anthropic_mcp_servers]` -> Anthropic outbound restore. | Preserved for Claude→Claude, including auth/config and unknown nested keys. | Not equivalent to OpenAI Responses `mcp`; no automatic bridge. | No Chat equivalent. | Same-protocol opaque preservation only. |
| `mcp_toolset` tool | Enables tools from MCP server. | `anthropic.Tool.Raw -> TransformerMetadata[anthropic_raw_tools] []anthropicRawToolFragment{OriginalIndex,Raw} -> appendAnthropicRawTools` ordered merge. It is not stored in public `llm.Tool.Raw`. | Preserved for Claude→Claude at the original `tools[]` index. | Not equivalent to OpenAI Responses `mcp`; no automatic bridge. | No direct Chat equivalent. | Adapter-native ordered raw preservation only. |
| `anthropic_version` | Anthropic version header/field for platforms. | `MessageRequest.AnthropicVersion`. | Preserved where needed. | No OpenAI equivalent. | No OpenAI equivalent. | Provider-specific. |
| `anthropic_beta` | Anthropic beta flags. | `MessageRequest.AnthropicBeta`. | Preserved where needed. | No OpenAI equivalent. | No OpenAI equivalent. | Provider-specific. |

---

# 4. Cross-protocol summary table

| Source → Target | Current broad behavior | Main risks |
|---|---|---|
| Responses → Responses | Uses `llm.Request` plus `ProviderExtensions.OpenAIResponses.Request` and raw merge to preserve some native fields. | Native protocol is split across common fields, metadata, provider extensions, raw fragments; `context_management`/`conversation` need first-class audit. |
| Responses → Chat | Converts through common messages/tools; lossy diagnostics exist for some Responses-native fields. | `tool_search`, `additional_tools`, `namespace`, `mcp`, Responses typed items cannot be represented natively in Chat. |
| Responses → Claude | Converts through common messages/tools. | OpenAI Responses `mcp`/tool_search/items/reasoning differ from Claude content blocks/MCP connector/thinking. |
| Chat → Chat | Uses typed Chat conversion plus original `RawRequest` replay for the eight G1/G2/G4/G5 fields. Deprecated response `message.function_call` also uses bridge+origin metadata across response/history/stream. Custom tools are no longer function-only on Chat outbound. | Real residuals include typed/common semantics for raw-only fields and full cross-protocol custom/tool ecosystems; raw replay does not create cross-protocol support. |
| Chat → Responses | Converts messages/tools into Responses request; Chat custom can bridge to Responses custom where carriers exist. | Chat `web_search_options` is not Responses web_search tool; hosted/native Responses tools remain lossy outside Responses. |
| Chat → Claude | Converts messages/tools into Claude request. | Chat custom tools / web_search_options / response_format semantics may be dropped or diagnosed; Claude tools are content-block based. |
| Claude → Claude | Uses native `MessageRequest`, opaque metadata restoration for `container`/`inference_geo`/`mcp_servers`, and indexed adapter raw fragments for `mcp_toolset`. | Stream aggregation remains a complex hotspot; MCP connector fields still have no automatic OpenAI/Chat bridge. |
| Claude → Responses | Converts through common abstraction. | Claude thinking/redacted_thinking/tool_use/tool_result/MCP connector do not map directly to Responses. |
| Claude → Chat | Converts through common abstraction. | Claude content blocks and thinking/server tool results do not map directly to Chat messages. |

---

# 5. Immediate gaps confirmed by this matrix

| Gap | Why it matters | Where seen |
|---|---|---|
| Chat custom tools: same-protocol/basic bridge exists; residual is cross-protocol completeness and unsupported native tool families. | Not a "function-only outbound" gap anymore; remaining risk is false-equivalence to Claude/hosted tools. | `transformer/openai/outbound_convert.go RequestFromLLM`; custom fidelity / cross-protocol tests. |
| Responses native protocol is split across native struct + `llm.Request` + metadata + provider extensions. | Same-protocol fidelity is hard to reason about; adding fields is scattered. | `responses/outbound.go`, `responses/request_extensions.go`, `provider_extensions.go`. |
| Responses `context_management` and `conversation` are canonical but not first-class in inspected Responses native request struct. | These can only survive if raw top-level fallback catches them; code cannot reason about them cleanly. | `responses/model.go`; canonical docs. |
| Response `output[]` encrypted reasoning identity is carried primarily by `ProviderExtensions.OpenAIResponses.Response.RawOutputItems` (not only request `ResponseReasoningItemID`). | Session replay bugs reappear if structured path invents `rs_*` for ciphertext. | `provider_extensions.go`, `responses/inbound.go`, encrypted-reasoning tests. |
| Chat raw-only G1/G2/G4/G5 fields lack public typed/common carriers. | Same-protocol replay is covered, but code cannot safely reason about or synthesize their semantics for another protocol. | `openai/chat_n.go`; targeted Chat tests. |
| Anthropic MCP/native metadata fields are same-protocol-only carriers. | `container`/`inference_geo`/`mcp_servers` and indexed `mcp_toolset` now round-trip, but remain non-equivalent to Responses MCP and Chat tools. | `anthropic/model.go`, `inbound_convert.go`, `outbound_convert.go`; targeted MCP/container tests. |
| Claude stream aggregation is a high-complexity hotspot. | Even if request fields are fixed, response/stream native fidelity is risky. | `anthropic/aggregator.go AggregateStreamChunks`. |

G1–G7 and G13–G15 request-side same-protocol fixtures now cover the repaired seams. The next audit slice should target residuals rather than re-open completed same-protocol work:

1. OpenAI Chat: `custom` tool handling and any desired typed/common semantics for the eight raw-only fields; keep cross-protocol loss explicit.
2. OpenAI Responses same-protocol residuals outside G13–G15 request scope: `context_management`, `conversation`, `tool_search`, `additional_tools`, `namespace`, `mcp`, plus response `output[]` identity if needed.
3. Anthropic: stream/content-block hotspot coverage and explicit diagnostics for unsupported cross-protocol MCP/thinking/tool-result conversions; do not reclassify completed native metadata roundtrips as gaps.
