# Module 2 commit record — Responses response / stream fidelity

时间：2026-07-07

## 提交

- Worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
- Branch: `codex/responses-top-level-preservation-clean`
- Commit: `1e056535 fix: preserve responses response stream fields`
- Base checkpoint before this module: `fe716145 fix: preserve responses request native fields`

## 本模块完成范围

Module 2 当前只提交已切片、已测试、已审查通过的 Responses response / stream 保真修复，不声称全部 Responses stream family 已完成。

已完成切片：

1. Response top-level native fields
   - `completed_at`
   - `output_text`
   - 通过 `TransformerMetadata` 在 provider Responses body -> `llm.Response` -> client Responses body 间保真。

2. `response.output_text.annotation.added`
   - 解析 `annotation_index` 和官方 flat annotation payload。
   - 输出到 `llm.Message.Annotations`。

3. `response.reasoning_text.delta/done`
   - `delta` 复用 reasoning content 路径。
   - `done` 作为终止/确认事件跳过，内容由 delta 输出。

4. `response.refusal.delta/done`
   - `delta` 输出到 `llm.Message.Refusal`。
   - `done` 跳过，内容由 delta 输出。

## 验证

提交前已执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

结果：均通过。

## 审查

Module 2 经过初审、按 P2 建议修复、focused re-review。

最终 focused re-review 结论：

- P0：无
- P1：无
- P2：无新增
- 上轮 P2 均已解决
- Module 2 focused re-review 通过

## 已知未覆盖，后续必须独立切片

仍未覆盖的官方 Responses stream event families：

```text
response.audio.delta
response.audio.done
response.audio.transcript.delta
response.audio.transcript.done

response.code_interpreter_call.completed
response.code_interpreter_call.in_progress
response.code_interpreter_call.interpreting
response.code_interpreter_call_code.delta
response.code_interpreter_call_code.done

response.file_search_call.completed
response.file_search_call.in_progress
response.file_search_call.searching

response.mcp_call.completed
response.mcp_call.failed
response.mcp_call.in_progress
response.mcp_call_arguments.delta
response.mcp_call_arguments.done

response.mcp_list_tools.completed
response.mcp_list_tools.failed
response.mcp_list_tools.in_progress

response.web_search_call.completed
response.web_search_call.in_progress
response.web_search_call.searching
```

建议下一批优先顺序：

1. MCP stream events；
2. web_search / file_search lifecycle；
3. code_interpreter；
4. audio。
