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
