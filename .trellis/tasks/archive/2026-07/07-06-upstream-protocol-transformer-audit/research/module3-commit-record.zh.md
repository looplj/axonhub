# Module 3 commit record — Responses MCP stream events

时间：2026-07-07

## 提交

- Worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
- Branch: `codex/responses-top-level-preservation-clean`
- Commit: `66001c46 fix: preserve responses mcp stream events`
- Previous checkpoint: `1e056535 fix: preserve responses response stream fields`

## 完成范围

Module 3 覆盖 OpenAI Responses MCP stream 保真：

1. MCP call lifecycle events
   - `response.mcp_call.in_progress`
   - `response.mcp_call.completed`
   - `response.mcp_call.failed`
2. MCP call arguments events
   - `response.mcp_call_arguments.delta`
   - `response.mcp_call_arguments.done`
3. MCP list_tools lifecycle events
   - `response.mcp_list_tools.in_progress`
   - `response.mcp_list_tools.completed`
   - `response.mcp_list_tools.failed`
4. MCP output item payload
   - `response.output_item.added/done` with `item.type == "mcp_call"`
   - `response.output_item.added/done` with `item.type == "mcp_list_tools"`

## 关键设计

- 不新增 common `llm.Response` / `llm.ToolCall` 字段。
- 使用 Responses-native private metadata key：`openai_responses_mcp_stream_events`。
- `stream_extensions.go` 只允许白名单 MCP stream events 和 MCP output item 进入 metadata，不是 raw event bucket。
- `sequence_number` 保真：
  - outbound copy 保存原始 `SequenceNumber`；
  - inbound replay 使用 `enqueuePreservedEvent`，不覆盖原始序号，只推进后续本地序号。

## 审查

已完成：

- Slice 自审 3.1 / 3.2 / 3.3 / 3.4。
- Standards review。
- Spec review。
- Architecture / improve-codebase-architecture focused review。
- Focused re-review。

最终复审结论：

- P0：无。
- P1：无。
- P2：无阻塞。
- `sequence_number` P1 已修复。
- `stream_extensions.go` 已纳入 staged diff，自包含。
- `stream_extensions.go` 是可接受的 Responses-native preservation module，不是 raw bucket。
- 允许提交 Module 3。

## 验证

提交前执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --cached --check
```

结果：均通过。

## 后续未完成

OpenAI Responses stream 仍未覆盖的官方 event families：

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

response.web_search_call.completed
response.web_search_call.in_progress
response.web_search_call.searching
```

建议下一模块：web_search / file_search lifecycle，因为它与内置工具状态保真相关，且比 audio/code_interpreter 更接近当前 stream metadata 模式。
