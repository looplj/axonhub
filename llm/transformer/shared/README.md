# Shared Transformer Helpers

This folder contains small, provider-agnostic helpers used by multiple transformers.
The most important concept here is the **signature filtering** scheme used to make
provider-specific private protocols survive **same-session channel/model switching**.

## Problem: same-session switching breaks provider private protocols

In AxonHub a single user session can route consecutive turns through different
channels/providers/models (load-balancing, failover, or a user switching channels).

Some providers emit extra "private protocol" fields that other providers don't
understand, for example:

- Anthropic extended thinking signature
- Gemini thought signature
- OpenAI Responses `reasoning.encrypted_content`

If these values are forwarded naively, they can be dropped, mis-parsed, or paired
with incompatible fields when the session switches providers, and then switching
back loses context and may degrade model behavior.

Worse, forwarding an incompatible signature to a downstream model causes hard
failures — e.g. `invalid_request_body` with "encrypted content could not be
verified" — because the downstream model attempts to decrypt/verify a blob that
was produced by a different provider.

## Terminology: inbound vs outbound transformers

In this repository, **inbound/outbound** are named from AxonHub's point of view:

- **Outbound transformer**: converts unified structs to an upstream provider request, and converts the upstream provider response back into unified structs.
  - request direction: `llm.Request` -> provider request
  - response direction: provider response -> `llm.Response`
- **Inbound transformer**: converts a client request (in some external API format) into unified structs, and converts unified structs back into that external API response format.
  - request direction: client request -> `llm.Request`
  - response direction: `llm.Response` -> client response

For streaming, apply the same naming convention to stream events/items in each direction.

## Design: heuristic-based signature filtering

We store these provider-specific values in the unified message field
`llm.Message.ReasoningSignature` as an **internal transport field**.

At each outbound provider boundary, `Decode...` helpers use
`GuessSignatureProvider` — a heuristic that inspects the raw blob — to decide
whether the signature is safe to forward to the target provider. Only signatures
that are **positively identified** as belonging to the target provider are kept;
everything else (including `unknown`) is dropped.

### Heuristics (`GuessSignatureProvider`)

| Pattern | Provider |
|---------|----------|
| `gAAAA*` / `gAAA*` prefix | OpenAI |
| `EqQ*` / `Eqo*` / `Eqr*` prefix | Anthropic |
| standard base64 with protobuf-like decoded bytes | Gemini |
| anything else | `unknown` (filtered by all `Decode...` helpers) |

### Behavioral contract (how it survives switching)

1. A provider response that contains a signature-like field is stored in
   `llm.Message.ReasoningSignature` (Encode helpers are passthrough).
2. Inbound conversions return the value back to the client unchanged.
3. On the next request, the client echoes the value unchanged.
4. When routing switches, outbound transformers call `Decode...` which only
   forwards the signature if `GuessSignatureProvider` identifies it as belonging
   to the target provider; otherwise the signature is **dropped** (do not
   forward private protocol fields across providers).

Practical invariants:

- **Strict filtering**: all three `Decode...` helpers (OpenAI, Anthropic, Gemini) only keep signatures positively identified as their own provider. `unknown` signatures are filtered to prevent `invalid_request_body` errors from downstream models.
- **At provider edges**: a transformer decodes **only when required by that provider API**, and only when the heuristic matches that provider (otherwise drop on mismatch).
- **Anthropic-specific exception**: decode is only required for Anthropic official platforms (`direct`, `claudecode`, `vertex`, `bedrock`). For other Anthropic-compatible outbound platforms, AxonHub forwards the value unchanged.

### Mermaid: end-to-end encode/decode flow

```mermaid
flowchart TD
    C[Client]
    IT[Inbound transformer<br/>external API ↔ llm]
    LLM[AxonHub unified structs<br/>llm.Request / llm.Response]
    OT[Outbound transformer<br/>llm ↔ provider API]
    P[Upstream provider API]

    C -->|1. request| IT
    IT -->|2. convert to llm.Request<br/>ReasoningSignature is passed through as-is| LLM
    LLM -->|3. llm.Request| OT
    OT -->|4. provider request<br/>native provenance first; heuristic only for legacy cross-protocol signatures| P

    P -->|5. provider response<br/>may contain private signature field| OT
    OT -->|6. convert to llm.Response<br/>store signature into ReasoningSignature| LLM
    LLM -->|7. llm.Response| IT
    IT -->|8. client response<br/>pass ReasoningSignature through unchanged| C
    
    style IT fill:#e1f5fe
    style OT fill:#fff3e0
    style LLM fill:#e8f5e8
    style P fill:#ffebee
```

## OpenAI Responses API note (why inbound must not decode)

OpenAI Responses has a `reasoning` output item with `encrypted_content`.
The ciphertext is opaque and must remain paired with its native reasoning
item ID. Its bytes do not define a stable provider-prefix contract.

Therefore:

- **Responses provider response -> client response** preserves the complete
  native reasoning output item (`id`, `encrypted_content`, shape, and position)
  on the Responses response sidecar.
- **Responses client request -> provider request** uses
  `ResponseReasoningItemID` as request-input provenance and forwards the paired
  `ReasoningSignature` exactly, without guessing from ciphertext bytes.
- **Legacy cross-protocol signatures** that have no Responses request-input
  provenance still use `DecodeOpenAIEncryptedContent` / provider heuristics so
  Anthropic or Gemini signatures are not invented as Responses state.
- **Responses inbound-stream (llm stream -> OpenAI Responses SSE)** passes
  through the signature as `encrypted_content` (do not decode).

Normal same-source continuation keeps the original ID/ciphertext pair. A channel
switch crosses an issuer boundary, so AxonHub removes opaque reasoning identity,
ciphertext, and compaction state while retaining visible summaries and tool
lifecycle data. An explicit provider rejection gets the same cleanup once.

## Evolution note

An earlier version of this design used a **footprint-aware internal marker**
scheme: Encode/Decode helpers accepted a `footprint` parameter and wrapped raw
values with a stable marker prefix so the signature could be matched against the
expected transport scope.

The current version uses **native request/response provenance** for Responses
round trips. The Encode/Decode helpers no longer take a `footprint` parameter;
`GuessSignatureProvider` remains only as a legacy cross-protocol safeguard when
no protocol-native provenance carrier exists.

## Practical guidance

- When adding a new provider-specific signature-like field, prefer:
  1) add detection patterns to `GuessSignatureProvider`,
  2) add `Encode/Decode` helpers around it,
  3) store it in `llm.Message.ReasoningSignature`,
  4) forward/decode only at the target provider boundary (drop on mismatch).
