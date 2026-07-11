# Design — Chat `n`

## Field Classification

`n` 是 OpenAI Chat native request control，不是已证明稳定的三协议 common semantic。

| Direction | Handling |
|---|---|
| Chat -> Chat | `raw-preserve` on OpenAI Chat native request extension path. |
| Chat -> Responses | `lossy-conversion` / unsupported; Responses does not gain synthetic multi-response behavior. |
| Chat -> Anthropic | `lossy-conversion` / unsupported. |
| Responses/Anthropic -> Chat | absent source; do not synthesize `n`. |

## Seam

Use the existing OpenAI Chat raw top-level capture/replay seam. Do not add `N` to `llm.Request`; do not alter common response choice semantics.

## Tests

1. inbound Chat JSON with `n: 1` captures raw field.
2. inbound Chat JSON with `n: 3` survives Chat outbound replay.
3. cross-protocol outbound receives loss diagnostic or documented unsupported outcome, according to existing adapter policy.

