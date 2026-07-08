# Module 3 Slice 4 — Responses MCP output_item payload preservation

时间：2026-07-07

## 目标

补齐 MCP 相关 `response.output_item.added/done` item payload 保真：

- `item.type == "mcp_call"`
- `item.type == "mcp_list_tools"`

这些 item 是 OpenAI Responses-native 输出项，不应被转换成普通 function/custom tool call，也不应静默丢弃。

## 实现

- `model.go`
  - `Item` 增加 MCP item 字段：
    - `server_label`
    - `approval_request_id`
    - `tools`，用 `json.RawMessage` 保留 MCP tool schema 原始 JSON；
    - `error`，用 `json.RawMessage` 保留 provider-native error payload。
- `stream_extensions.go`
  - MCP metadata helper 允许 `response.output_item.added/done` 中的 `mcp_call` / `mcp_list_tools` item。
  - copy helper 只复制 MCP item，避免把 metadata key 变成任意 raw event bucket。
- `outbound_stream.go`
  - `output_item.added` 遇到 MCP item 时写入 Responses-native MCP stream metadata。
  - `output_item.done` 遇到 MCP item 时写入 Responses-native MCP stream metadata。
- `inbound_stream.go`
  - 继续复用 `replayMCPStreamEvents` 回放。

## 验证

红测先失败，证明确实缺字段：

- `Item.ServerLabel` 不存在；
- `Item.Tools` 不存在；
- MCP output_item 不会进入 metadata 队列。

实现后执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesMCPOutputItemEvents|InboundTransformer_TransformStream_ReplaysMCPOutputItemEventsFromMetadata)$' -count=1
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

结果：均通过。

## 自审

- 没有新增 common `llm.Response` / `llm.ToolCall` 字段。
- MCP tool schema / error 使用 `json.RawMessage`，不窄化第三方 connector 或 MCP schema。
- metadata helper 只接受：
  - MCP event types；
  - `output_item.added/done` 且 item type 是 `mcp_call` / `mcp_list_tools`。
- `output_item.added/done` 的 MCP item 不被误转为普通 function tool call。

## Module 3 当前覆盖

已覆盖 MCP 相关 Responses stream events：

- `response.mcp_call.in_progress`
- `response.mcp_call.completed`
- `response.mcp_call.failed`
- `response.mcp_call_arguments.delta`
- `response.mcp_call_arguments.done`
- `response.mcp_list_tools.in_progress`
- `response.mcp_list_tools.completed`
- `response.mcp_list_tools.failed`
- `response.output_item.added/done` 中的 `mcp_call` / `mcp_list_tools` item payload

下一步：模块级 code-review + architecture review + 多 agent 交叉审查。
