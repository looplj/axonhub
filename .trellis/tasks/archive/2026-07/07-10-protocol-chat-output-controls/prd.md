# PRD — Chat Top-Level `audio`, `prediction`, `moderation`

## Goal

实现 OpenAI Chat Completions top-level request `audio`、`prediction`、`moderation` 的 same-protocol preservation；禁止把它们和 assistant audio、Responses image-generation moderation 或 Anthropic output config 混为同一语义。

## Required Behavior

1. Chat -> Chat 保留 top-level `audio`、`prediction`、`moderation`。
2. 每个字段及其 object/union child variants 独立处理。
3. Chat assistant message `audio` 不受此任务影响。
4. Responses/Anthropic 无经证明等价字段时明确 unsupported/lossy，不 synthetic-map。

## Acceptance Criteria

- 三个字段各有 request fixture。
- object/union 子字段不因 typed struct 部分填充而丢 unknown nested fields。
- Chat same-protocol outbound body 保留输入语义。
- assistant message audio 回放测试不回归。
- targeted Chat/OpenAI transformer tests 和 diff check 通过。

