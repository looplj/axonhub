# Shared Transformer Helpers

This folder contains small, provider-agnostic helpers used by multiple transformers.
The most important concept here is the **signature prefixing** scheme used to make
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

## Design: carry provider signatures via a stable internal prefix

We store these provider-specific values in the unified message field
`llm.Message.ReasoningSignature` as an **internal transport field**.

To preserve the original provider identity across conversions, we wrap the raw
value with a stable base64 prefix:

- `AnthropicSignaturePrefix` (`AXN101` base64)
- `GeminiThoughtSignaturePrefix` (`AXN102` base64)
- `OpenAIEncryptedContentPrefix` (`AXN103` base64)

Helpers live in:

- `anthropic.go`: `EncodeAnthropicSignature`, `DecodeAnthropicSignature`, `IsAnthropicSignature`
- `gemini.go`: `EncodeGeminiThoughtSignature`, `DecodeGeminiThoughtSignature`, `IsGeminiThoughtSignature`
- `openai.go`: `EncodeOpenAIEncryptedContent`, `DecodeOpenAIEncryptedContent`, `IsOpenAIEncryptedContent`

The invariant is:

- **Inside AxonHub** (unified `llm.*` structs): `ReasoningSignature` is kept in the **internal encoded form** (with prefix).
- **At provider edges**: a transformer may decode **only when required by that provider API**.

## OpenAI Responses API note (why inbound must not decode)

OpenAI Responses has a `reasoning` output item with `encrypted_content`.
If AxonHub decodes/removes the internal prefix on inbound conversion, the client
will send the next request without the prefix, and AxonHub can no longer identify
which provider protocol the signature belongs to; it may then be dropped by other
transformers for safety.

Therefore:

- **Responses outbound (llm -> OpenAI Responses request)** should decode the prefix
  into the raw `encrypted_content` field when calling OpenAI.
- **Responses inbound (OpenAI Responses response -> llm)** should encode/prefix
  `encrypted_content` and store it in `ReasoningSignature`.
- **Responses inbound-stream (llm stream -> OpenAI Responses SSE)** should pass
  through the internal encoded signature as `encrypted_content` (do not decode).

This keeps the session round-trip stable even if the client only "speaks" OpenAI
Responses and AxonHub switches the actual upstream provider behind the scenes.

## Practical guidance

- When adding a new provider-specific signature-like field, prefer:
  1) define a new prefix constant in `constants.go`,
  2) add `Is/Encode/Decode` helpers,
  3) store it in `llm.Message.ReasoningSignature`,
  4) decode only at the target provider boundary.

