# Protocol Transformer Guidelines

These rules apply to AxonHub protocol transformer changes under `llm/transformer/**` and related shared `llm` models.

## Core Principle

Preserve the author's transformer framework. Do not replace it with a universal native AST and do not widen `llm.Request` for protocol-specific fields. Deepen the existing native preservation seams instead.

## Field Ownership

Every new or repaired protocol field must have one primary owner before code is written:

| Field kind | Owner |
|---|---|
| Stable cross-protocol semantics | `llm.Request` / `llm.Response` |
| OpenAI Responses official fields | `llm/transformer/openai/responses` native preservation |
| Codex Responses usage-profile fields | OpenAI Responses native preservation |
| OpenAI Chat official fields | OpenAI Chat native model / Chat emission policy |
| Anthropic official or companion fields | Anthropic native model / adapter |
| Provider-specific controls | Provider extension or provider adapter |
| Same-protocol unknown fields | Same-protocol raw fallback |
| Cross-protocol incompatible fields | `LossyDowngrade` diagnostic |
| Stream events | Stream event fidelity module |

## Required Rules

- Fix same-protocol round-trip before designing cross-protocol mapping.
- Do not add protocol-specific fields to `llm.Request` unless they are proven stable cross-protocol semantics.
- Treat Codex Responses behavior as a usage profile inside OpenAI Responses native preservation, not as a separate private protocol.
- Raw fallback may re-emit only to the same protocol family unless an adapter explicitly documents and tests a mapping.
- `TransformerMetadata` may hold bridge hints, but must not become a hidden protocol model or field garbage bucket.
- `ProviderExtensions` must have a named owner and must not be expanded without a field-routing reason.
- Cross-protocol field loss must be visible through `LossyDowngrade` diagnostics or documented deliberate unsupported behavior.
- Stream events must be handled by stream fidelity code, not request or response request-body models.
- Each implementation slice must include targeted preserve-or-diagnose tests for the fields it touches.



## Responses Body Field Storage

OpenAI Responses-native request and response body fields must not be stored in `TransformerMetadata`.

Rules:

- Responses request body fields (`background`, `include`, `max_tool_calls`, `prompt_cache_retention`, `truncation`, `stream_options` and nested extensions) are stored on `ProviderExtensions.OpenAIResponses.Request` as typed/raw fields.
- Responses response body fields (`completed_at`, `output_text`) and non-stream search call output items are stored on `ProviderExtensions.OpenAIResponses.Response` (`RawTopLevelFields`, `RawOutputItems`).
- `TransformerMetadata` is reserved for bridge hints and stream/image staging only (e.g. `prompt_cache_key`, image generation action/options, Responses stream-event lifecycle state). It must not carry protocol-native body fields.
- `openai.Request` must not implement `MarshalJSON`, because downstream providers embed it and a promoted method breaks their wrapper marshalling. OpenAI Chat raw top-level replay is done by an explicit outbound helper (`marshalOpenAIChatRequest`) called only by the OpenAI Chat outbound.
- Cross-protocol conversion of these fields still follows LossyDowngrade diagnostics; same-protocol replay reads from the sidecar, not from metadata.
- Complex native objects with both typed fields and raw extension space (for example Responses `stream_options`) must merge typed emission with raw nested fields on same-protocol replay. Do not use an all-or-nothing top-level merge that skips raw replay once the typed struct populated the key; add a regression test with a typed known field plus an unknown nested field.

## LossyDowngrade Diagnostics

Use `LossyDowngrade` only for cross-protocol loss that has no tested equivalent semantics in the target protocol.

Rules:

- Store downgrade diagnostics under `ProviderExtensions.Diagnostics.LossyDowngrades`; do not add diagnostics to common `llm.Request` / `llm.Response`.
- Keep diagnostics `json:"-"`; they are sidecar evidence for routing/debugging/review, not provider payload.
- The target protocol outbound adapter owns downgrade decisions because it knows what the target can express.
- Do not reuse protocol-native extension storage for diagnostics.
- Do not diagnose fields that are explicitly bridged and tested, such as OpenAI Chat `<->` OpenAI Responses `prompt_cache_retention`.
- Do diagnose same-protocol raw fallback fields when they are converted to a different protocol family and no explicit mapping exists.
- Do not fake-map incompatible tool ecosystems: OpenAI Responses `mcp` / `file_search` / `code_interpreter` and Anthropic `mcp_servers` / `mcp_toolset` need explicit tested semantics before any bridge.
- Same-protocol native/raw preservation, cross-protocol bridge, and cross-protocol diagnostic must remain separate code paths.
- Shared JSON helpers under `llm/internal/pkg/xjson` may clone/capture raw JSON bytes only. They must not contain protocol field names, protocol ownership decisions, target protocol capability checks, or downgrade reason/severity policy.
- LossyDowngrade presence decisions belong in the target outbound adapter. Shared helpers may centralize default diagnostic writing (`present=false` guard, default reason/severity, de-dup delegation) but must not centralize field matrices until a separate reviewed design proves the target-policy boundary stays explicit.

## First Slice Constraint

The first implementation slice is frozen to OpenAI Responses -> OpenAI Responses native preservation. It may cover official Responses fields, Codex Responses MCP/lazy-loading identity, raw-only Responses input/tool variants, and same-protocol unknown fallback. It must not bundle Chat emission policy, Anthropic native preservation, or stream event fidelity.

## Review Checklist

Before accepting a transformer change, verify:

- The source protocol, target protocol, and same-protocol/cross-protocol mode are explicit.
- Each touched field has a documented owner.
- Same-protocol behavior is tested before cross-protocol downgrade behavior.
- Unsupported cross-protocol fields are diagnosed rather than silently dropped.
- Shared builders such as OpenAI-compatible Chat emission are not widened without provider blast-radius review.

## Parent Task Vertical Slice Workflow

Large protocol-transformer work must be handled as a parent task with independently verifiable vertical slices.

```text
Parent Task
  -> split into vertical slices
  -> Slice 1
      -> TDD
      -> Trellis check
      -> Matt code-review
      -> pass?
          no  -> return to TDD / diagnosing-bugs / planning for the same slice
          yes -> Slice 2
  -> repeat until Slice N passes
  -> final parent review
  -> overall architecture review
  -> update-spec
  -> ADR / CONTEXT updates if needed
  -> finish-work
```

Rules:

- Each slice must be independently testable and reviewable.
- Do not enter the next slice until the current slice passes TDD, Trellis check, and code-review.
- If review fails, return to the appropriate earlier phase for the same slice: TDD for missing behavior, diagnosing-bugs for broken behavior, or planning for a wrong slice boundary.
- After all slices pass, run a parent-level review for cross-slice consistency, architecture drift, useless code, dead code, over-broad abstractions, and violations of ADR/spec/CONTEXT.
- After the parent review, run an overall architecture review before moving to the next module.
- Capture durable lessons through update-spec; create ADR or update CONTEXT only when the decision/term meets the documented criteria.
- In inline mode, Trellis check and code-review are performed by the main session. In sub-agent mode, the same gates may be delegated to check/review agents.

## Baseline Reference Routing

Protocol-transformer work must route through the protocol baseline documents before implementation.

Use this section when a task touches any of:

- `llm/model.go`
- `llm/provider_extensions.go`
- `llm/openai_responses_classification.go`
- `llm/transformer/openai/**`
- `llm/transformer/openai/responses/**`
- `llm/transformer/anthropic/**`
- request/response/stream conversion tests under `llm/transformer/**`

Default references:

| Purpose | File |
|---|---|
| Main strict audit matrix | `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` |
| OpenAI Responses protocol baseline | `docs/specs/protocols/openai-responses-protocol.md` |
| OpenAI Chat Completions protocol baseline | `docs/specs/protocols/openai-chat-completions-protocol.md` |
| Anthropic Messages protocol baseline | `docs/specs/protocols/anthropic-claude-messages-protocol.md` |
| Common/simple fields draft evidence | `docs/specs/protocols/drafts/batch-common-fields.md` |
| Limits/state/cache draft evidence | `docs/specs/protocols/drafts/batch-limits-state-cache.md` |
| Output/message/content draft evidence | `docs/specs/protocols/drafts/batch-output-message-content.md` |
| Reasoning/thinking/stream draft evidence | `docs/specs/protocols/drafts/batch-reasoning-stream.md` |
| Tools/MCP/tool-calls draft evidence | `docs/specs/protocols/drafts/batch-tools-mcp.md` |
| Round 3 architecture synthesis | `.agent/summary/2026-07-09-protocol-round3-architecture-summary.md` |
| Round 4 readiness synthesis | `.agent/summary/2026-07-09-protocol-round4-readiness-summary.md` |
| Round 5 baseline hardening synthesis | `.agent/summary/2026-07-09-protocol-round5-baseline-hardening-summary.md` |

Routing rules:

- Read the main strict audit matrix first.
- Then read only the batch draft that matches the touched field family.
- Use summary files as navigation and task history, not as final protocol truth.
- Treat the three protocol baseline files and official source extracts cited by the drafts as higher-trust than summaries.
- Do not paste draft rows wholesale into implementation. Convert a row into a small TDD slice first.
- If a field appears in multiple drafts, pick the row family by runtime role:
  - request control;
  - response item;
  - message/content block;
  - usage statistic;
  - stream option;
  - stream event;
  - tool declaration;
  - tool-call lifecycle;
  - raw-preserve sidecar.

## Baseline Row Granularity Rules

Before changing a transformer field, classify the row shape:

| Shape | Required handling |
|---|---|
| Scalar request field | May be one row if value shape/default/null semantics are simple. |
| Object | Keep overview row for navigation only; split important child fields. |
| Array | Split container semantics from element semantics when element constraints matter. |
| Union / typed union | Split by variant before claiming final support. |
| Deprecated field | Keep separate from current replacement field. |
| Request vs response | Never merge into the same row. |
| Stream option vs stream event | Never merge into the same row. |
| Usage statistic | Keep separate from request controls and response content. |
| Same-protocol raw preserve | Separate from cross-protocol semantic conversion. |
| Codex usage-profile behavior | Mark as P1/Codex-specific unless P0 OpenAI public evidence exists. |

## Baseline Field Classification Rules

Use these classes consistently in specs, tests, and review notes:

| Class | Meaning | Implementation direction |
|---|---|---|
| `common-abstraction` | Stable semantic overlap across protocols. | May live in `llm.Request`, `llm.Response`, `llm.Message`, `llm.Tool`, etc. |
| `native-field` | Native field of one protocol. | Keep in that adapter's request/response model. |
| `adapter-specific` | Provider-specific feature with no stable common semantics. | Keep in provider adapter or provider extension. |
| `raw-preserve` | Same-protocol replay must preserve a field even when not fully modeled. | Store in named raw sidecar; only re-emit to the same protocol family. |
| `lossy-conversion` | Target protocol cannot express the same semantics. | Emit `LossyDowngrade` or document deliberate unsupported behavior. |
| `unsupported/absent` | No target field or no current storage path. | Do not fake-map. Add implementation slice only if product wants support. |
| `deprecated-compat` | Old protocol field kept for compatibility. | Keep separate from modern field and test precedence. |

## Baseline Decisions From Round 5

The following decisions are durable until the protocol docs or code prove otherwise:

- Preserve the author's transformer architecture; do not rewrite into a universal native AST.
- Same-protocol round-trip safety comes before cross-protocol mapping.
- Do not widen `llm.Request` for protocol-specific fields unless the field has stable cross-protocol semantics.
- `TransformerMetadata` is for bridge hints, not a hidden protocol model or arbitrary field dump.
- `ProviderExtensions` and raw sidecars must have named owners.
- OpenAI Responses `namespace` / Codex sub-agent / `codex_app` behavior is a Codex P1 usage profile unless public OpenAI P0 evidence says otherwise.
- OpenAI Responses MCP tools and Anthropic MCP connector fields are not equivalent by name alone.
- `reasoning.effort` / Chat `reasoning_effort` are not Anthropic `thinking.budget_tokens`.
- `stream_options` are request controls; stream events are runtime event shapes. Do not store or test them as the same thing.
- Token-limit fields must remain split by protocol and current/deprecated name: Responses `max_output_tokens`, Chat `max_completion_tokens`, Chat deprecated `max_tokens`, Anthropic `max_tokens`.
- Anthropic outbound default token insertion, currently observed as `8192`, must have explicit tests before final matrix completion claims.
- Chat top-level `audio`, `prediction`, `moderation`, Chat `web_search_options`, Anthropic `container`, `inference_geo`, and Anthropic MCP connector fields are implementation candidates, not completed baseline support.

## Baseline Implementation Slice Order

When converting the Round 5 baseline into code, prefer this slice order unless the active task explicitly says otherwise:

1. Chat `n` storage/preservation decision.
2. OpenAI `prompt_cache_retention` / Chat coverage.
3. Anthropic `container` and `inference_geo`.
4. Chat top-level `audio`, `prediction`, `moderation`.
5. Chat `web_search_options`.
6. Deprecated Chat `functions` / request `function_call` / response `message.function_call`.
7. Anthropic `mcp_servers` / `tools[].type="mcp_toolset"`.
8. Responses reasoning object and stream events, split into request object, output item, and stream-event sub-slices.

Each slice must include targeted tests for one of:

- same-protocol typed preservation;
- same-protocol raw preservation;
- cross-protocol explicit bridge;
- cross-protocol lossy/unsupported diagnostic.

## Anthropic Thinking Mode Routing

Anthropic outbound conversion must route reasoning controls by model capability, not by a single global `ReasoningEffort -> budget_tokens` rule.

Rules:

- Keep model-family recognition inside one Anthropic capability resolver. Request conversion consumes only a capability result (`adaptive_only`, `adaptive_preferred`, `manual_supported`, or `unknown`); do not scatter model-name checks through request builders.
- An Anthropic-compatible channel may override the resolved capability through its adapter configuration. This is channel capability data, not a cross-protocol field mapping.
- Adaptive / effort-first Claude models must not receive manual `thinking.type="enabled"` with `budget_tokens` when the source only provides OpenAI/LLM `ReasoningEffort`.
- For adaptive / effort-first models, emit:
  - `thinking: {"type":"adaptive"}`
  - `output_config.effort`
- Local required effort mapping:
  - `minimal -> low`
  - `xhigh -> max`
  - `high -> max`
  - `medium -> medium`
  - `low -> low`
  - `max -> max`
  - unknown -> explicit unsupported/lossy diagnostic; never guess `max` or a manual budget.
- `ReasoningEffort="none"` means disabled thinking and must emit `thinking: {"type":"disabled"}` without `output_config.effort`.
- Old manual-thinking models may still emit `thinking.type="enabled"` with `budget_tokens`, but generated or explicit `budget_tokens` must be at least `1024` and strictly less than `max_tokens`; reject an impossible manual request instead of serializing invalid JSON.
- Do not confuse reasoning effort with thinking budget:
  - effort is an enum-style effort control;
  - `budget_tokens` is manual thinking token budget.
- DeepSeek Anthropic-format keeps its separate platform policy: it may use `output_config.effort` without Anthropic `thinking.type="adaptive"`; it must not share Claude adaptive-effort normalization and must not re-emit adaptive-thinking metadata as an Anthropic adaptive wire shape.
- Explicit native Anthropic thinking metadata takes precedence for a compatible Anthropic target. It does not override an incompatible platform policy such as DeepSeek.

Required tests for this area:

- adaptive/effort-first model with `ReasoningEffort=high` emits `thinking.type="adaptive"` and `output_config.effort="max"`.
- adaptive/effort-first model with `ReasoningEffort=xhigh` emits `output_config.effort="max"`.
- adaptive/effort-first model with `ReasoningEffort=none` emits disabled thinking and no output_config effort.
- manual-thinking model rejects illegal explicit/configured budgets and emits only `1024 <= budget_tokens < max_tokens`.
- DeepSeek with adaptive-thinking metadata never serializes `thinking.type="adaptive"`.
- unknown target capability records a lossy/unsupported diagnostic rather than guessing manual or adaptive thinking.
- outbound HTTP/request transformer tests must assert the serialized request body, not only helper structs.


## Field Evidence Index (2026-07-12)

Implementation modules G1–G7 closed the following high-priority seams with targeted tests. Full matrix status language is in `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` §9.

| Seam | Owner package | Primary tests |
|---|---|---|
| Chat top-level raw preserve (`n`, `prompt_cache_retention`, `audio`, `prediction`, `moderation`, `web_search_options`, `functions`, `function_call`) | `llm/transformer/openai` | `chat_n_test.go`, `chat_deprecated_functions_test.go` |
| Chat deprecated `message.function_call` | `llm/transformer/openai` | `chat_deprecated_functions_test.go` |
| Anthropic `container` / `inference_geo` | `llm/transformer/anthropic` | `container_inference_geo_test.go` |
| Anthropic MCP connector | `llm/transformer/anthropic` | `mcp_connector_test.go` |
| Responses reasoning object/stream | `llm/transformer/openai/responses` | `reasoning_context_test.go`, `reasoning_g7_test.go` |

Rules unchanged: same-protocol first; no fake MCP bridges; LossyDowngrade for documented cross-protocol loss; stream fidelity stays in stream code.

