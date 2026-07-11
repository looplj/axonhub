# PRD — Fix Anthropic Opus adaptive thinking conversion

## Goal

修复 8091 修复版 Hub 中 OpenAI Responses `reasoning.effort` 转 Anthropic Messages 时对 Claude Opus 4.8 发送旧 manual thinking 的问题。

## Problem evidence

- `#21807`: `anthropic/messages` 原生 `claude-opus-4-8` 成功。
- `#23429/#23430`: `openai/responses` 入站 `claude-opus-4-8` 失败。
- Hub 出站到 Anthropic 时生成了：
  - `max_tokens: 8192`
  - `thinking: {type:"enabled", budget_tokens:30000}`
- 上游报错：`thinking.budget_tokens must be less than max_tokens`。
- 本地 Anthropic 官方文档说明：
  - `budget_tokens` 是 manual thinking 旧模式；
  - Claude Opus 4.8 / 4.7 只支持 adaptive thinking；
  - 应使用 `thinking:{type:"adaptive"}` + `output_config.effort` 控制强度。

## Requirements

1. Claude Opus 4.8 / Opus 4.7 / Sonnet 5 等 adaptive-only 或 effort-first 模型，不得从 `reasoning.effort` 转出 `thinking.enabled + budget_tokens`。
2. 对这些模型，`reasoning.effort` 应转为 `thinking:{type:"adaptive"}` + `output_config.effort`。
3. 用户要求映射：
   - `xhigh -> max`
   - `high -> max`
4. `none` 仍表示关闭 thinking，保持 `thinking:{type:"disabled"}`。
5. 老模型 manual thinking 逻辑保留，但必须保证 `budget_tokens < max_tokens`。
6. 添加测试覆盖 `claude-opus-4-8` 的 Responses/LLM -> Anthropic 出站转换。
7. 通过 `llm` 定向测试、静态 diff 检查和三轴代码审查；不启动或请求任何运行服务。

## Acceptance criteria

- AC1: `claude-opus-4-8` + `ReasoningEffort=high` 出站 Anthropic 请求包含 `thinking.type=adaptive`。
- AC2: 同请求包含 `output_config.effort=max`。
- AC3: 同请求不包含 `thinking.budget_tokens`。
- AC4: `ReasoningEffort=xhigh` 同样映射为 `output_config.effort=max`。
- AC5: manual thinking 老路径的 `budget_tokens` 不会大于等于 `max_tokens`。
- AC6: `cd llm && go test ./transformer/anthropic -count=1` 通过。
- AC7: 不进行运行态实测；代码级验收由定向单测、`git diff --check` 和独立的 bug / 协议 / 架构审查共同完成。

## Out of scope

- 不改 8090。
- 不改 OpenAI Responses 字段保真架构。
- 不推 GitHub。
