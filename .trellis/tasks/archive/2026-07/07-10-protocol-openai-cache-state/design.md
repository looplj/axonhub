# Design — OpenAI Cache/State

## Classification

`prompt_cache_retention` 是 OpenAI native request body field：

| Direction | Handling |
|---|---|
| Chat -> Chat | OpenAI Chat native raw top-level preserve. |
| Responses -> Responses | Preserve current typed/raw behavior. |
| Chat <-> Responses | Only explicit tested bridge; do not assume equivalent defaults. |
| OpenAI -> Anthropic | adapter-specific/lossy unless target has explicit equivalent. |

## Seam

Use OpenAI protocol-native sidecar/raw replay helpers. `TransformerMetadata` remains bridge-hint-only and must not become storage for Chat request body fields.

## Tests

- Chat capture/replay known value.
- Chat capture/replay unknown future value.
- Responses existing same-protocol regression.
- cross-protocol diagnostic only where target cannot express semantics.

