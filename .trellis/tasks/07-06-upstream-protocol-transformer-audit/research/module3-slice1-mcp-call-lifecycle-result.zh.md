# Module 3 Slice 1 — Responses MCP call lifecycle stream events

时间：2026-07-07

## 目标

修复 OpenAI Responses stream 中 MCP call lifecycle 事件被静默吞掉的问题，先覆盖最小事件族：

- `response.mcp_call.in_progress`
- `response.mcp_call.completed`
- `response.mcp_call.failed`

这些事件没有自然的 Chat-compatible `llm.Response` 字段，因此不能新增 common 字段伪装成通用能力；应作为 OpenAI Responses-native stream metadata 保真，并只由 OpenAI Responses inbound stream 回放。

## 实现

修改 worktree：

`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`

涉及文件：

- `llm/transformer/openai/responses/stream_event.go`
  - 增加 MCP call lifecycle event constants。
- `llm/transformer/openai/responses/stream_extensions.go`
  - 新增 `responsesMCPStreamEventsTransformerMetadataKey`。
  - 新增 metadata append/get/copy helpers。
  - 只允许 MCP call lifecycle 事件进入该 metadata 队列。
- `llm/transformer/openai/responses/outbound_stream.go`
  - provider Responses SSE -> `llm.Response.TransformerMetadata`。
- `llm/transformer/openai/responses/inbound_stream.go`
  - `llm.Response.TransformerMetadata` -> client Responses SSE 即时回放。
- `llm/transformer/openai/responses/outbound_stream_test.go`
  - 覆盖 outbound 保存 lifecycle metadata。
- `llm/transformer/openai/responses/inbound_stream_test.go`
  - 覆盖 inbound 回放 lifecycle events。

## TDD 证据

先写红测后，初次运行失败，缺口为：

- `responsesMCPStreamEventsTransformerMetadataKey` 未定义；
- `StreamEventTypeMCPCallInProgress` / `Completed` 未定义；
- `getResponsesMCPStreamEventsFromMetadata` 未定义。

实现后验证：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesMCPCallLifecycleEvents|InboundTransformer_TransformStream_ReplaysMCPCallLifecycleEventsFromMetadata)$' -count=1
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

结果：均通过。

## 自审

- 没有新增 `llm.Request` / `llm.Response` common 字段。
- MCP lifecycle 作为 Responses-native metadata 处理，不跨协议解释。
- inbound 只回放当前 chunk metadata 里的事件，避免把同一事件累计后重复回放。
- `enqueueEvent` 会重新分配 `sequence_number`，符合当前 inbound stream 的统一序号生成策略。
- helper 过滤 metadata 中非 MCP call lifecycle 事件，避免变成无限制 raw event bucket。

## 未覆盖，后续切片继续

本 slice 未覆盖：

- `response.mcp_call_arguments.delta`
- `response.mcp_call_arguments.done`
- `response.mcp_list_tools.in_progress`
- `response.mcp_list_tools.completed`
- `response.mcp_list_tools.failed`
- `output_item.added/done` 中 `mcp_call` / `mcp_list_tools` item 的完整字段保真

下一 slice 建议：MCP call arguments delta/done。
