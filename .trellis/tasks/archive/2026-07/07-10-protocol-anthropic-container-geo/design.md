# Design — Anthropic Container / Inference Geo

## Classification

两者均为 Anthropic native top-level object：

```text
Anthropic -> Anthropic: native/raw-preserve
Anthropic -> OpenAI: lossy/unsupported
OpenAI -> Anthropic: absent source; no synthesis
```

## Seam

`MessageRequest` native raw fields + Anthropic inbound/outbound raw top-level sidecar。字段已有 typed raw slots 时优先复用，避免进入 `llm.Request` 或 `TransformerMetadata`。

## Tests

1. inbound preserves arbitrary object bytes/semantic JSON.
2. outbound restores both fields on same Anthropic protocol family.
3. unknown nested key survives round trip.
4. no OpenAI alias mapping.

