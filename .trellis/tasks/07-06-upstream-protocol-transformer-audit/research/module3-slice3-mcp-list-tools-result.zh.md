# Module 3 Slice 3 — Responses MCP list_tools lifecycle stream events

时间：2026-07-07

## 目标

补齐 MCP list tools lifecycle stream events：

- `response.mcp_list_tools.in_progress`
- `response.mcp_list_tools.completed`
- `response.mcp_list_tools.failed`

这些事件只有 `item_id` / `output_index` / `sequence_number`，没有 Chat-compatible common 字段，应沿用 Responses-native MCP stream metadata 回放机制。

## 实现

- `stream_event.go` 增加 list_tools lifecycle event constants。
- `outbound_stream.go` 将 list_tools lifecycle events 加入同一 MCP metadata 分支。
- `stream_extensions.go` 允许 list_tools lifecycle events 进入 `responsesMCPStreamEventsTransformerMetadataKey`。
- inbound 不需新分支，复用 `replayMCPStreamEvents`。

## 验证

红测先失败，缺失 list_tools event constants。实现后执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesMCPListToolsLifecycleEvents|InboundTransformer_TransformStream_ReplaysMCPListToolsLifecycleEventsFromMetadata)$' -count=1
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

结果：均通过。

## 自审

- 复用 Slice 1/2 的 metadata queue，没有新增重复机制。
- 不修改 common `llm.Response` / `llm.ToolCall`。
- 事件回放仍是当前 chunk 一次性回放，避免累计重复。

## Module 3 剩余 MCP 相关缺口

MCP stream event type 本身已经覆盖：

- `response.mcp_call.in_progress/completed/failed`
- `response.mcp_call_arguments.delta/done`
- `response.mcp_list_tools.in_progress/completed/failed`

但 MCP item payload 仍需下一切片审计：

- `response.output_item.added/done` 中 `item.type == "mcp_call"`
- `response.output_item.added/done` 中 `item.type == "mcp_list_tools"`
- 相关 item 字段：`server_label`、`name`、`arguments`、`output`、`error`、`status`、`approval_request_id`、`tools` 等。
