# Protocol field ownership and first preservation slice

**Status:** Accepted (ownership rules durable). **Slice history refreshed:** 2026-07-22.

## Decision (durable) — FieldOwnership

AxonHub preserves the author’s transformer framework. Every protocol field has **one primary owner** before code is written:

| Field kind | Owner |
|---|---|
| Stable cross-protocol semantics | `llm.Request` / `llm.Response` (CrossProtocolCanonical) |
| OpenAI Responses official / Codex usage-profile fields | `llm/transformer/openai/responses` + `ProviderExtensions.OpenAIResponses` |
| OpenAI Chat official / Chat-only raw fields | Chat transformer + Chat-native PE or same-protocol raw owner (not common `llm.Request` widen) |
| Anthropic official / companion fields | Anthropic transformer + `ProviderExtensions.Anthropic` (migrate off metadata body dumps) |
| Provider-specific controls | Named provider extension / adapter |
| Same-protocol unknowns | Same-protocol raw fallback (re-emit only to that family) |
| Cross-protocol incompatible | `ProviderExtensions.Diagnostics.LossyDowngrades` |
| Stream events | Stream fidelity path (not request-body models) |

**Rejected options** (still rejected):

- Put every missing field into `llm.Request` → universal AST, blast radius.
- Use `PassThroughBody` as the fix → bytes without ownership/diagnostics.
- Patch each provider adapter independently → scatters field-loss policy.
- **Accepted:** deepen native preservation seams; one owner per field; preserve-or-diagnose tests at the public transformer seam.

## Historical first slice (do not re-freeze)

The **first** implementation slice was frozen to **OpenAI Responses → OpenAI Responses** so Codex/MCP/lazy-loading and official Responses request fields could land without bundling Chat emission policy, Anthropic native work, and stream fidelity into one unreviewable change.

That freeze was **ordering**, not a permanent prohibition.

## Implementation progress (2026-07-22)

FieldOwnership rules now apply to **all** protocols. Later slices already landed evidence for:

- Chat same-protocol raw preserve (`n`, cache retention, audio, prediction, moderation, `web_search_options`, deprecated functions / function_call) and custom-tool carriers (residuals remain on cross-protocol completeness).
- Anthropic same-protocol metadata/raw for `container`, `inference_geo`, `mcp_servers`, `mcp_toolset` (cross-protocol still no-synth / Lossy).
- Responses response/stream carriers (`RawOutputItems`, `RawStreamEvents`) and encrypted-reasoning recovery policy in orchestrator (strip shape still dual-path residual).
- Shared LossyDowngrade helpers for Chat / Responses / Anthropic targets.

**Remaining work is residual cutover and dual-path deletion**, not “wait until Responses request-only is finished before touching Chat.”

## Consequences

- New work must name the owner **before** editing code.
- Same-protocol tests before cross-protocol bridges.
- Do not reintroduce “first slice freeze” as a reason to reject Chat/Anthropic/stream fixes that already have ownership rules and tests.
- Completion claims still require the strict verification matrix `CONFIRMED` gate—not ADR progress tables alone.
