# PRD — Chat `n` Preservation

## Goal

修复 OpenAI Chat Completions request `n` 在 AxonHub 中静默丢失的问题，同时不把多候选生成伪装成已支持的跨协议能力。

## Required Behavior

1. Chat -> AxonHub -> Chat 的请求必须保留原始 `n` 值。
2. `n` 不进入 `llm.Request` common abstraction，除非后续存在完整多候选 response 语义设计与测试。
3. Chat -> Responses / Anthropic 不得假装生成等价的多候选响应；无等价路径时必须记录 explicit lossy/unsupported 行为。
4. 不允许 silent drop。

## Acceptance Criteria

- 有 red fixture 证明当前 Chat same-protocol 会丢 `n`。
- Chat same-protocol replay 保留 `n` 的 raw JSON 值。
- `n=1` 与 `n>1` 都有 request-level fixture。
- modern Chat response path 不被错误扩展为 multi-choice implementation。
- targeted tests、`git diff --check`、baseline self-review 通过。

