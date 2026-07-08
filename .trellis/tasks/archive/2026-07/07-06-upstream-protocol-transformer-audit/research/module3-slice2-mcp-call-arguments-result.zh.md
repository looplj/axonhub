# Module 3 Slice 2 — Responses MCP call arguments stream events

时间：2026-07-07

## 目标

补齐 MCP call arguments stream events：

- `response.mcp_call_arguments.delta`
- `response.mcp_call_arguments.done`

这些事件是 OpenAI Responses-native MCP stream event，不应伪装成普通 function call arguments，也不应落入 common `llm.ToolCall` 字段造成跨协议误读。

## 实现

沿用 Slice 1 的 `responsesMCPStreamEventsTransformerMetadataKey`：

- `stream_event.go` 增加 arguments delta/done event constants。
- `outbound_stream.go` 将 arguments delta/done 封装到 `llm.Response.TransformerMetadata`。
- `stream_extensions.go` 的 MCP metadata copy helper 保留：
  - `type`
  - `output_index`
  - `item_id`
  - `delta`
  - `arguments`
- `inbound_stream.go` 已通过同一 replay helper 回放 metadata events。

## 验证

红测先失败，缺失 event constants。实现后执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesMCPCallArgumentEvents|InboundTransformer_TransformStream_ReplaysMCPCallArgumentEventsFromMetadata)$' -count=1
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

结果：均通过。

## 自审

- 没有把 MCP arguments 合并到普通 function_call/tool_call 参数里，避免语义混淆。
- 复用同一个 MCP stream metadata 队列，没有为每种 MCP event 新造一套机制。
- inbound replay 与 Slice 1 共用逻辑，保持 locality。

## 未覆盖

- `response.mcp_list_tools.*`
- MCP `output_item.added/done` 里的 `mcp_call` / `mcp_list_tools` item 完整字段。
