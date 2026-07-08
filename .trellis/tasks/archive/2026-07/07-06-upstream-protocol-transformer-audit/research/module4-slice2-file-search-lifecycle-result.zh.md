# Module 4 Slice 4.2 — Responses file_search lifecycle stream fidelity

## 范围

本 slice 只处理 OpenAI Responses 原生流事件：

- `response.file_search_call.in_progress`
- `response.file_search_call.searching`
- `response.file_search_call.completed`

字段所有者：`llm/transformer/openai/responses` stream fidelity module。

不处理：

- file_search output item payload 的 `queries/results` 等内容字段；
- web_search output item payload 扩展；
- Chat / Anthropic；
- cross-protocol 映射；
- 公共 `llm.Request` / `llm.Response` 字段扩展。

## Red test

新增测试：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/outbound_stream_test.go`
  - `TestOutboundTransformer_TransformStream_PreservesFileSearchLifecycleEvents`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/inbound_stream_test.go`
  - `TestInboundTransformer_TransformStream_ReplaysFileSearchLifecycleEventsFromMetadata`

红测试失败证据：

```text
undefined: StreamEventTypeFileSearchCallInProgress
undefined: StreamEventTypeFileSearchCallSearching
undefined: StreamEventTypeFileSearchCallCompleted
FAIL github.com/looplj/axonhub/llm/transformer/openai/responses [build failed]
```

## Green implementation

修改：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/stream_event.go`
  - 新增 file_search lifecycle 事件常量；
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/outbound_stream.go`
  - outbound stream switch 将 file_search lifecycle 送入 search preservation seam；
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/search_stream_extensions.go`
  - 白名单扩展到 file_search lifecycle 三种事件。

实现策略：

- 复用 Slice 4.1 建立的 `openai_responses_search_stream_events` seam，因为 web_search/file_search 同属 Responses built-in search lifecycle event family；
- 仍只保存 replay 所需字段：`type`、`sequence_number`、`output_index`、`item_id`；
- 不把 file_search lifecycle 误建成公共 `llm.Response` 字段；
- 不碰 output item payload，避免 slice 扩大。

## 验证命令

在 `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm` 执行：

```bash
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesFileSearchLifecycleEvents|InboundTransformer_TransformStream_ReplaysFileSearchLifecycleEventsFromMetadata)$' -count=1
go test ./transformer/openai/responses -count=1
git diff --check
```

结果：

```text
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	1.282s
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.549s
```

`git diff --check` 无输出。

## 自审

- 字段归属：file_search lifecycle 属于 Responses stream fidelity，不属于公共 llm 模型。
- 协议边界：只做同协议 stream event 保真，不做跨协议转换。
- 架构边界：复用 search lifecycle seam 合理；没有创建另一个 file_search-only metadata key，也没有混进 MCP key。
- 保真字段：测试覆盖 `type`、`sequence_number`、`output_index`、`item_id`。
- 顺序保真：inbound 回放继续使用 `enqueuePreservedEvent`，保留原始 sequence number。
- 范围控制：未处理 file_search `output_item.added/done` payload，留给 Slice 4.3 判断和测试。

## 结论

Slice 4.2 通过。下一步进入 Slice 4.3：审计 search output item payload 是否仍丢字段，重点检查 `file_search_call` 的 `queries/results` 和 `web_search_call` 的 `action/status` 是否已被 Item 模型覆盖或仍需 metadata 保真。
