# PRD — Anthropic `container` and `inference_geo`

## Goal

实现 Anthropic Messages 官方顶层 `container` 与 `inference_geo` 的 same-protocol native/raw preservation，避免当前静默丢失。

## Required Behavior

1. Anthropic -> AxonHub -> Anthropic 保留 `container` 与 `inference_geo` 原始 JSON。
2. 不猜测其内部 shape 或 future values；保持 raw JSON。
3. 不映射到 OpenAI Responses/Chat 的相似字段。
4. 跨协议方向必须 explicit lossy/unsupported，而非 silent drop。

## Acceptance Criteria

- 两个字段各有 fixture，含未知 nested field。
- same-protocol outbound body 与 inbound JSON 语义相同。
- raw JSON 不被 stringify、omitempty 或 struct decode 破坏。
- Chat/Responses outbound 有明确 unsupported/lossy 行为测试或明确不适用证据。
- targeted Anthropic tests 通过。

