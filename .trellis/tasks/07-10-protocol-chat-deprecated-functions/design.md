# Design — Deprecated Chat Function Compatibility

## Classification

所有本任务字段均为 `deprecated-compat`：

| Field family | Owner |
|---|---|
| request `functions` | Chat native/raw compatibility path |
| request `function_call` | Chat native/raw compatibility path |
| response `message.function_call` | Chat response compatibility path |
| modern `tools/tool_choice/tool_calls` | existing modern path; must remain separate |

## Compatibility Rule

不要在没有测试的情况下自动将 legacy request body 改写为 modern payload。若当前 adapter 已有 explicit bridge，则测试 bridge；否则 same-protocol raw preserve 优先。

## Tests

1. legacy-only request replay。
2. legacy-only response replay。
3. legacy + modern coexistence precedence。
4. modern-only regression。
5. cross-protocol function semantic only where existing common `llm.Tool` path proves equivalence。

