# PRD — Protocol Transformer Field Fixes

## Goal

依据已建立的协议基准文档，在保留 AxonHub 作者现有 transformer 架构的前提下，系统修复 OpenAI Responses、OpenAI Chat Completions 与 Anthropic Messages 之间所有已识别且**应实现**的协议转换缺口。

## Source of Truth

1. `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`
2. 三份协议基线：Responses、Chat Completions、Anthropic Messages。
3. `docs/specs/protocols/drafts/*.md` 的 Round 1–5 证据、字段拆分、fixture backlog 与 implementation candidates。
4. `.trellis/spec/backend/protocol-transformer-guidelines.md`。

## Parent Acceptance Criteria

1. Slice 0（Anthropic Opus adaptive thinking）隔离验证、审查、提交与 8091 验证完成。
2. 每个下列模块都有独立 PRD/design/implement/context，并遵循 slice-quality-loop：
   - Chat `n`
   - OpenAI `prompt_cache_retention` / Chat coverage
   - Anthropic `container` / `inference_geo`
   - Chat top-level `audio` / `prediction` / `moderation`
   - Chat `web_search_options`
   - deprecated Chat `functions` / `function_call` / `message.function_call`
   - Anthropic `mcp_servers` / `mcp_toolset`
   - Responses reasoning object / stream events
   - high-priority fixture-only gaps 与矩阵/规范同步
3. 每个小切片先有失败测试或 fixture 证明当前缺口，再做最小实现。
4. 每个切片自审通过；每个模块的多子代理审查通过。失败审查必须回到 TDD、diagnosis 或 architecture planning，不得进入下一模块。
5. 同协议保真优先于跨协议映射；跨协议无等价字段必须有 `LossyDowngrade` 或明确 unsupported 说明。
6. 所有高优先 fixture/test 补齐，或在主矩阵/模块报告中有明确、可复核的不适用理由。
7. 最终父级架构审查、协议一致性审查、规范同步与本地提交均完成；不存在已知 blocker 或未处理 review finding。

## Explicit Non-Goals

- 不把 intentional lossy / unsupported 字段伪装成等价转换。
- 不将 provider-specific 字段随意塞进 `llm.Request`。
- 不重写作者架构为 universal native AST。
- 不把 Codex P1 usage profile 误写为 OpenAI P0 协议。
- 不推送 GitHub。

