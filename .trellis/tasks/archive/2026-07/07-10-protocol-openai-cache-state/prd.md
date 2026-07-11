# PRD — OpenAI `prompt_cache_retention` Chat Coverage

## Goal

补齐 OpenAI Chat `prompt_cache_retention` 的 same-protocol preservation；Responses 已有处理不得回归。

## Required Behavior

1. Chat request 的 `prompt_cache_retention` 必须 capture/replay。
2. Chat -> Chat 保留原始合法值和 unknown future value，不把 value vocabulary 重写为本地 enum。
3. Responses 与 Chat 的 native paths 分开；不得把字段塞进 `llm.Request`。
4. Chat -> Anthropic 以及无法表达的方向必须按 target adapter 的 lossy/unsupported 规则处理。

## Acceptance Criteria

- fixture 证明旧路径丢失或未 re-emit 该字段。
- Chat -> Chat request body 保留该字段。
- Responses existing `prompt_cache_retention` tests 仍通过。
- unknown future string value same-protocol 不被过滤。
- targeted tests 和 diff check 通过。

