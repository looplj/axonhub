# PRD — Deprecated Chat `functions` / `function_call`

## Goal

实现 OpenAI Chat deprecated request `functions`、request `function_call` 和 response `message.function_call` 的兼容处理，且不破坏现代 `tools`、`tool_choice`、`tool_calls` 路径。

## Required Behavior

1. deprecated fields可 same-protocol capture/replay。
2. deprecated 与 modern fields 并存时，行为和 precedence 必须显式测试，不得靠 accidental struct order。
3. request `function_call`、response `message.function_call`、modern tool-call lifecycle 必须分开。
4. cross-protocol仅转换已证明等价的 function semantic；否则 explicit lossy/unsupported。

## Acceptance Criteria

- 3 个 deprecated field family 各有 fixture。
- modern tools/tool_choice/tool_calls regression tests 通过。
- 同时出现 legacy+modern field 时 precedence 被明确断言。
- 不向 Anthropic/Responses 合成未证明的旧字段语义。
- targeted tests 和 diff check 通过。

