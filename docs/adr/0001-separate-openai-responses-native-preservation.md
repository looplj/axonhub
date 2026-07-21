# Separate OpenAI Responses native preservation from cross-protocol canonical conversion

**Status:** Accepted (direction durable). **Implementation progress refreshed:** 2026-07-22.

## Decision (durable)

AxonHub treats OpenAI Responses as a **native protocol surface**, not merely as another projection of `llm.Request`.

- `llm.Request` / `llm.Response` remain **CrossProtocolCanonical** for shared model semantics only.
- Responses-only structures (for example `namespace` tools, `tool_search`, `additional_tools`, function-call `namespace`, `tool_search_output`, unknown future fields, and same-protocol raw fragments) live in the Responses transformer package and **`ProviderExtensions.OpenAIResponses`**, for Responses→Responses routing.
- Cross-protocol routes may still perform **LossyDowngrade** (for example flattening a namespace into a function name), but that downgrade must be **explicit and diagnosable**, never the default same-protocol behavior.
- **`PassThroughBody`** remains an operational mitigation and correctness baseline. It is **not** the architectural fix. The fix is structured native/raw ownership so same-protocol routes can inspect, log, account, and re-emit without losing OpenAIResponsesNative / CodexResponsesProfile semantics.
- Target scope is **FullNativeRoundTrip** (request + response + stream) where same-protocol re-emission is possible. P0/P1 labels are ordering only, not a permanent scope cut.
- The native layer is structured AST **plus** raw fallback—not raw alone. Do not introduce a universal multi-protocol AST. Prefer deepening `llm/transformer/openai/responses` over a new framework.
- Do not dump Responses-only fields into `llm.Request`. If a field cannot be owned cleanly, document the architecture gap before adding workarounds.

## Historical staging (do not re-freeze)

Original staging (2026-07) started with **request** native round-trip first, then response body and stream events, to keep the first change reviewable. That was an **ordering choice**, not a permanent ban on later work.

## Implementation progress (2026-07-22) — replaces freeze wording

The following already exist in code and tests and must not be treated as “out of scope / not started”:

| Area | Owner (current) | Notes |
|---|---|---|
| Request top-level natives (`include`, `prompt_cache_retention`, `max_tool_calls`, `truncation`, `background`, …) | `ProviderExtensions.OpenAIResponses.Request` | Not `TransformerMetadata` body dump |
| Request raw tools / input fragments / stream_options raw | Same PE request extensions | Same-protocol re-emit only |
| Response lifecycle + raw top-level / **RawOutputItems** | `ProviderExtensions.OpenAIResponses.Response` | Encrypted reasoning id+ciphertext pairing |
| Response **RawStreamEvents** | Same PE response extensions | Same-protocol stream replay |
| Request input item identities | `Message` / `ToolCall` carriers + PE raw | See guidelines § request input identity |
| Cross-protocol loss | `ProviderExtensions.Diagnostics.LossyDowngrades` | Target outbound owns diagnose decisions |
| Codex usage profile | Responses native path (not a private protocol) | e.g. encrypted include injection in codex outbound |

**Still residual / incomplete cutovers** (do not pretend finished): dual metadata leftovers, Chat body-as-sidecar, custom-tool multi-path, pass-through vs convert contract, Anthropic natives still partly on `TransformerMetadata`. Track in strict verification matrix + architecture slices—not by re-freezing this ADR to “request only.”

## Public baseline note

`client_metadata` may appear in native/raw carrying for same-protocol clients, but **public OpenAI P0 baselines** in `docs/specs/protocols/` currently do **not** treat it as a confirmed public Responses field. Codex/session metadata survival is a product/usage concern; do not claim public-protocol completion from it alone.

## Consequences

- Same-protocol Responses→Responses uses PE + transformer native/raw, not pass-through-as-architecture.
- Reviewers must judge new work against **this progress table** and the strict matrix, not against the historical “request-only first slice” sentence.
- Package boundary remains `llm/transformer/openai/responses` until a separate ADR says otherwise.
