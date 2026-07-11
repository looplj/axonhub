# PRD — Responses Reasoning Object and Stream Events

## Goal

补齐 OpenAI Responses reasoning request object、reasoning output items、reasoning stream events 与未知 nested reasoning variants 的 protocol-native preservation；不把 request reasoning、response reasoning、usage 与 stream event 混为一类。

## Mandatory Micro-Slices

1. `reasoning.context`
2. deprecated `reasoning.generate_summary`
3. reasoning output item `content[]` / `reasoning_text`
4. reasoning stream events
5. unknown nested reasoning future variants

## Required Behavior

1. 每个 micro-slice 必须先有 independent red fixture。
2. Responses -> Responses same-protocol preservation 优先。
3. request `reasoning` object 不得存入 `TransformerMetadata` 作为 protocol body field。
4. stream event fidelity 只能在 stream path 处理。
5. cross-protocol target 无等价语义时 explicit lossy/unsupported；不得把 effort 当 Anthropic budget。

## Acceptance Criteria

- 五个 micro-slice 各有 fixture/test 和自审报告。
- typed + unknown nested raw field 同时存在时不丢未知 nested key。
- deprecated `generate_summary` 保留其 compatibility identity，不和 `summary` 混写。
- output item 与 stream event 断言分别验证。
- unknown future nested reasoning variants 不 silent drop。
- scoped Responses transformer tests 通过，模块多审通过。

