# OpenAI Chat protocol-gap inventory — read-only evidence round 1

Date: 2026-07-13. Scope: every primary strict-matrix ID under `CHAT.TOP.*`, `CHAT.MSG.*`, `CHAT.TOOL.*`, `CHAT.STREAM.*`, `CHAT.RESP.*`, plus baseline response usage (which lacks an ID). No production code, fixture, strict matrix, or existing spec was modified.

## Method and evidence rules

Compared `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`, `.trellis/spec/backend/protocol-transformer-guidelines.md`, `docs/specs/protocols/openai-chat-completions-protocol.md`, and local canonical snapshot `docs/specs/vendor/protocol-canonical-2026-07-06/openai-chat-completions-create.developers-snapshot.md`. Code was located/read with `/Users/asuan/.local/bin/codebase-memory-mcp cli` against `Users-asuan-AI-axonhub-llm`; `rg` was used only on Markdown.

Classifications: **IMPLEMENTATION** only for source/baseline-proved observable loss; **TEST_ONLY** for implemented handling without a field-focused public fixture; **DIAGNOSTIC** for a proved incompatible bridge requiring visible loss handling; **DOC_ONLY** for ledger/spec-only defects; **EXPLICIT_NO_IMPLEMENT** for already-proved preservation or intentional no-synthesis. This does not claim six-direction closure.

## Reused source evidence

| Ref | Source symbol + path | Assignment/return evidence |
|---|---|---|
| S1 | `(*Request).ToLLMRequest`, `llm/transformer/openai/inbound_convert.go:40-180` | Copies typed request fields; maps messages, stop, `stream_options.include_usage`, tools, function-shaped tool choice, response format. |
| S2 | `RequestFromLLM`, `llm/transformer/openai/outbound_convert.go:12-132` | Copies canonical fields back; **filters tools to `llm.ToolTypeFunction`** and emits only string/function named choice. |
| S3 | `openAIChatRawPreserveFields`, `chat_n.go:12-21`; `marshalOpenAIChatRequest`, `:27-58` | Chat-source-only raw replay of `n`, retention, audio, prediction, moderation, web-search options, deprecated functions/function_call. |
| S4 | Chat `Tool`, `ToolChoice`, `NamedToolChoice`, `ToolCall`, `MessageContentPart`, `llm/transformer/openai/model.go` | Tool declaration/choice/call structs are function-shaped; content part has text/image/video/input-audio but no file/refusal-part payload or raw sidecar. |
| S5 | `(*Message).ToLLMMessage`, `inbound_convert.go`; `MessageFromLLMWithConfig`, `outbound_convert.go`; `chat_deprecated_functions.go` | Maps roles/content/audio/tool calls; legacy `message.function_call` uses origin metadata for same-family replay. |
| S6 | `(*Response).ToLLMResponse`, `outbound_convert.go:335-377`; `ResponseFromLLM`, `inbound_convert.go:323-363`; `Choice` converters | Maps all choices, index, finish reason, message/delta, logprobs, usage and response metadata both ways. |
| S7 | `(*Usage).ToLLMUsage`, `usage.go:34-71`; `UsageFromLLM`, `:74-103` | Maps total and detailed audio/cache/reasoning/prediction token usage. |
| S8 | `OutboundTransformer.TransformStreamChunk`, `outbound.go:321-342`; `InboundTransformer.TransformStreamChunk`, `inbound.go:112-146`; `AggregateStreamChunks`, `aggregator.go` | Parses/serializes Chat chunks, `[DONE]`, choices, tool deltas and usage. |

## Field inventory

| Field ID | Owner / handling | Source symbol + path | Current tests | Classification | Exact observable gap | RED fixture candidate | Blocked evidence |
|---|---|---|---|---|---|---|---|
| `CHAT.TOP.messages` | common typed / Chat message adapter | S1, S2, S5 | broad inbound/outbound message tests; legacy tests | TEST_ONLY | No general source loss proved; no single public mixed-role/multimodal/tool-history round trip. | Ordered system/developer/user/assistant/tool mixed-history fixture. | Matrix itself says total typed content/tool/result coverage unconfirmed. |
| `CHAT.TOP.model` | `llm.Request.Model` typed | S1, S2 | broad transformer tests | TEST_ONLY | Symmetric assignment; exact unknown model public fixture absent. | Unknown model string, assert exact replay. | Routing alias behavior is outside serialization. |
| `CHAT.TOP.audio` | Chat raw preserve | S3 | `TestOpenAIChatRequestOutputControlsRawRoundTrip`, no-synthesis and Anthropic diagnostic tests | EXPLICIT_NO_IMPLEMENT | Exact Chat replay already proved. | None. | Cross-family audio shapes are not equivalent. |
| `CHAT.TOP.frequency_penalty` | common typed | S1, S2 | broad conversion tests | TEST_ONLY | No focused negative/zero/positive/omitted public proof. | Table fixture for bounds and omission. | No source-proved loss. |
| `CHAT.TOP.logit_bias` | common typed map | S1, S2 | `TestRequestFromLLM_LogitBiasPreservesFloat` | TEST_ONLY | Helper proves float, not public same-family JSON round trip. | Mixed integer/fractional/negative values. | No source-proved loss. |
| `CHAT.TOP.logprobs` | common typed | S1, S2 | broad tests | TEST_ONLY | True/false/omitted request presence unproven publicly. | Three-state request fixture. | Response logprobs are separate. |
| `CHAT.TOP.max_completion_tokens` | common typed | S1, S2 | broad tests | TEST_ONLY | No fixture distinguishes it from `max_tokens`. | Both fields with distinct values and independent omission. | Cross-protocol equivalence unverified. |
| `CHAT.TOP.metadata` | common typed map | S1, S2 | broad tests | TEST_ONLY | Nested values/omission lack field-focused public proof. | Nested scalar map + omitted. | Official limits are not a conversion defect. |
| `CHAT.TOP.modalities` | common typed | S1, S2 | `...ModalitiesRoundTripChat`, `...ModalitiesOmittedChat` | EXPLICIT_NO_IMPLEMENT | Supplied and omitted public cases already pass. | None. | No cross-family synthesis claim. |
| `CHAT.TOP.moderation` | Chat raw preserve | S3 | output-control round-trip/no-synthesis tests | EXPLICIT_NO_IMPLEMENT | Exact Chat replay proved. | None. | No equivalent bridge established. |
| `CHAT.TOP.n` | Chat raw preserve | S3 | `TestOpenAIChatRequestNRawRoundTrip`, no-synthesis test | EXPLICIT_NO_IMPLEMENT | Request preservation proved. | None. | Response plurality is `CHAT.RESP.choices`. |
| `CHAT.TOP.parallel_tool_calls` | common typed; cleared if no emitted tools | S1, S2 | broad tests | TEST_ONLY | Function-tool true/false/omitted not focused; custom loss is downstream of `CHAT.TOP.tools`. | Function-tool three-state fixture. | Do not double-count custom tool root defect. |
| `CHAT.TOP.prediction` | Chat raw preserve | S3 | output-control tests | EXPLICIT_NO_IMPLEMENT | Exact Chat replay proved. | None. | No cross-family equivalent proved. |
| `CHAT.TOP.presence_penalty` | common typed | S1, S2 | broad tests | TEST_ONLY | No focused bounds/zero/omitted proof. | Table fixture. | No source-proved loss. |
| `CHAT.TOP.prompt_cache_key` | common typed | S1, S2 | cache tests elsewhere | TEST_ONLY | Exact unknown value/omission not proved at Chat public seam. | Unknown key + omitted. | Not Anthropic cache-control. |
| `CHAT.TOP.prompt_cache_retention` | Chat raw preserve | S3 | retention round-trip/no-synthesis/diagnostic tests | EXPLICIT_NO_IMPLEMENT | Exact Chat replay proved. | None. | Explicitly not Anthropic cache-control. |
| `CHAT.TOP.reasoning_effort` | open-string common typed | S1, S2 | reasoning tests; Responses unknown-string test is not Chat proof | TEST_ONLY | Chat unknown-string same-family fixture absent. | `future-tier` exact replay. | Codex `ultra→max` is client policy. |
| `CHAT.TOP.response_format` | typed type + raw JSON schema | S1, S2 | model/helper tests | TEST_ONLY | Known shape represented; no public text/json_object/json_schema round trip. | Three official forms + omission. | Unsupported future variants need canonical evidence before implementation. |
| `CHAT.TOP.safety_identifier` | common typed | S1, S2 | broad tests | TEST_ONLY | Exact/omitted public proof absent. | Supplied and omitted with `user`. | Must not rewrite `user`. |
| `CHAT.TOP.seed` | common typed pointer | S1, S2 | broad tests | TEST_ONLY | Zero/negative/omitted presence unproven. | Table fixture. | No source-proved loss. |
| `CHAT.TOP.service_tier` | common typed request/response | S1, S2, S6 | broad tests | TEST_ONLY | Unknown string/omission not focused. | Unknown tier request and response. | No enum clamp found. |
| `CHAT.TOP.stop` | common typed string/array union | S1, S2 | broad tests | TEST_ONLY | Scalar/array/empty-array presence not publicly proved. | Four-form fixture. | No source-proved loss. |
| `CHAT.TOP.store` | common typed pointer | S1, S2 | broad tests | TEST_ONLY | False versus omitted not publicly proved. | true/false/omitted. | No source-proved loss. |
| `CHAT.TOP.stream` | common typed + stream owner | S1, S2, S8 | stream tests | TEST_ONLY | No request fixture ties exact body flag to stream options. | `stream:true` + `include_usage`. | Chunk fidelity is separate. |
| `CHAT.TOP.stream_options` | typed `include_usage` only | S1, S2 | pipeline `TestEnsureUsage_*` | TEST_ONLY | include_usage true/false/omitted public Chat proof absent. No canonical evidence found for another required nested key, so not implementation. | Three-state include_usage fixture. | Additional keys blocked on canonical schema evidence. |
| `CHAT.TOP.temperature` | common typed | S1, S2 | broad tests | TEST_ONLY | Zero/nonzero/omitted not focused. | Three-state fixture. | No source-proved loss. |
| `CHAT.TOP.tool_choice` | function-only typed choice | S1, S2, S4 | string/named-function unit tests; deprecated precedence test | IMPLEMENTATION | Canonical snapshot proves named custom `{type:"custom",custom:{name}}` and `{type:"allowed_tools",allowed_tools:{mode,tools}}`; S4 only carries `{type,function}`, so both valid objects lose shape/data. | Public table fixture for named custom and allowed_tools; function/string controls. | Copy exact Chat forms from canonical snapshot ~2680-2774; never infer from Responses. |
| `CHAT.TOP.tools` | function-only typed declaration/emission | S1, S2, S4 | function-tool tests; no Chat custom fixture | IMPLEMENTATION | Canonical snapshot ~2778-2912 proves `{type:"custom",custom:{name,description,format}}`; S4 cannot store it and S2 filters non-function canonical tools. | Function + exact Chat custom declaration, assert order and full custom format. | Do not use Responses custom schema. |
| `CHAT.TOP.top_logprobs` | common typed | S1, S2 | broad/logprob tests | TEST_ONLY | Request value/omission not focused. | Paired with logprobs, zero/positive/omitted. | No source-proved loss. |
| `CHAT.TOP.top_p` | common typed | S1, S2 | broad tests | TEST_ONLY | 0/1/fraction/omission not focused. | Table fixture. | No source-proved loss. |
| `CHAT.TOP.user` | common typed | S1, S2 | broad tests | TEST_ONLY | Deprecated value/omission not field-focused. | User + safety identifier coexistence. | No silent substitution. |
| `CHAT.TOP.verbosity` | open-string common typed | S1, S2 | broad tests | TEST_ONLY | Unknown string compatibility unproved publicly. | Unknown value + omitted. | No enum clamp found. |
| `CHAT.TOP.web_search_options` | Chat raw preserve | S3 | round-trip/no-synthesis/Anthropic diagnostic tests | EXPLICIT_NO_IMPLEMENT | Exact Chat replay and no fake Responses tool proved. | None. | Not Responses hosted web search. |
| `CHAT.TOP.functions` | deprecated Chat raw preserve | S3 | deprecated raw/precedence/no-synthesis tests | EXPLICIT_NO_IMPLEMENT | Legacy shape and coexistence proved. | None. | Deprecation does not permit rewrite. |
| `CHAT.TOP.function_call` | deprecated request raw preserve | S3 | deprecated call round-trip/precedence tests | EXPLICIT_NO_IMPLEMENT | Exact request legacy shape proved. | None. | Distinct from message response field. |
| `CHAT.TOP.max_tokens` | common typed | S1, S2 | broad tests | TEST_ONLY | Coexistence with max_completion_tokens unproved. | Distinct simultaneous values. | Cross-provider precedence out of scope. |
| `CHAT.MSG.roles` | direct role string | S5 | system/user/assistant broad tests | TEST_ONLY | No public valid-role table including developer/tool/function. | Role-specific valid messages. | Canonical role restrictions must drive fixture. |
| `CHAT.MSG.content_parts` | typed part union lacking file/refusal-part carriers | S4, S5 | string/text/image/audio helper tests | IMPLEMENTATION | Baseline lists file and refusal content parts. `MessageContentPart` has no payload member/raw sidecar, so decode retains at most `type`; outbound cannot reconstruct payload. Message-level refusal is not equivalent. | Public mixed part fixture with exact canonical file/refusal payloads plus represented controls. | Role restrictions and exact payloads must come from canonical snapshot. |
| `CHAT.MSG.function_call` | legacy origin-aware bridge | S5, `chat_deprecated_functions.go` | response/history/stream legacy tests and modern isolation tests | EXPLICIT_NO_IMPLEMENT | Existing tests prove legacy replay only with legacy origin. | None. | Missing peer row in §5 is documentary, not code loss. |
| `CHAT.TOOL.tool_calls` | function-only Chat call model | S4, S5, S6, S8 | function stream/aggregator tests; no custom call fixture | IMPLEMENTATION | Canonical snapshot ~1618 and ~3344 proves `{id,type:"custom",custom:{name,input}}`; S4 has only `Function`, so custom response/history/chunk calls cannot round trip. | Exact custom call in non-stream response, request history, and canonical stream deltas; fragmented function control. | Use Chat custom-call shape, not Responses item shape. |
| `CHAT.STREAM.chunks` | stream fidelity module | S8 | inbound stream, aggregate, nonzero-index, no-usage, legacy stream tests | TEST_ONLY | No single end-to-end fixture combines usage-only chunk, multiple choices, tool deltas, finish reasons, `[DONE]`. Custom-call loss is owned by `CHAT.TOOL.tool_calls`. | Ordered SSE fixture with those elements. | Audio/refusal delta forms require canonical evidence. |
| `CHAT.RESP.choices` | common typed response choices | S6 | `TestResponse_ToLLMResponse`, nonzero-index aggregator tests | TEST_ONLY | Mapping exists; public non-stream n>1 with per-choice finish reason/logprobs absent. | Two-choice response exact replay. | Request n is not response coverage. |

## Baseline field with no strict-matrix ID

| Field ID | Owner / handling | Source symbol + path | Current tests | Classification | Exact observable gap | RED fixture candidate | Blocked evidence |
|---|---|---|---|---|---|---|---|
| **No assigned ID** (candidate only: `CHAT.RESP.usage`) | common typed usage + stream usage chunk | S6, S7, S8 | `TestUsage_ToLLMUsage`, `TestUsageFromLLM`, `TestUsage_RoundTrip`, aggregator/pipeline usage tests | DOC_ONLY | Chat baseline §5 defines response usage, and handling/tests exist, but strict matrix has no Chat response usage row. | After ID assignment, non-stream + usage-only stream detail fixture. | Matrix edits forbidden and Field IDs cannot be invented here. |

## Confirmed results

- **IMPLEMENTATION (4):** `CHAT.TOP.tools`, `CHAT.TOP.tool_choice`, `CHAT.MSG.content_parts`, `CHAT.TOOL.tool_calls`.
- **DOC_ONLY (1):** baseline response usage has no strict-matrix Field ID.
- **EXPLICIT_NO_IMPLEMENT (10):** already-proved raw/typed preservation rows listed above.
- Remaining 28 Chat Field-ID rows are **TEST_ONLY**. No standalone **DIAGNOSTIC** defect was established; uncertain cross-protocol behavior was not guessed.
