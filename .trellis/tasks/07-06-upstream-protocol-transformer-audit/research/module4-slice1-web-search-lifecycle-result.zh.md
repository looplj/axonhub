# Module 4 Slice 4.1 — Responses web_search lifecycle stream fidelity

## 范围

本 slice 只处理 OpenAI Responses 原生流事件：

- `response.web_search_call.in_progress`
- `response.web_search_call.searching`
- `response.web_search_call.completed`

字段所有者：`llm/transformer/openai/responses` stream fidelity module。

不处理：

- file_search 生命周期；
- output item payload 扩展；
- Chat / Anthropic；
- cross-protocol 映射；
- 新增公共 `llm.Request` / `llm.Response` 字段。

## Red test

新增测试：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/outbound_stream_test.go`
  - `TestOutboundTransformer_TransformStream_PreservesWebSearchLifecycleEvents`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/inbound_stream_test.go`
  - `TestInboundTransformer_TransformStream_ReplaysWebSearchLifecycleEventsFromMetadata`

初始失败类型：

- 缺少 `StreamEventTypeWebSearchCallInProgress` / `Searching` / `Completed` 常量；
- 缺少 search stream metadata helper；
- outbound/inbound stream 未保存/回放该事件族。

## Green implementation

新增文件：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/search_stream_extensions.go`

实现策略：

- 新增私有 metadata key：`openai_responses_search_stream_events`；
- metadata 只保存 Responses search lifecycle replay 所需字段：`type`、`sequence_number`、`output_index`、`item_id`；
- 用白名单保存事件，当前只允许 web_search lifecycle 三种事件；
- outbound 遇到 web_search lifecycle 时写入 transformer metadata；
- inbound 从 transformer metadata 用 `enqueuePreservedEvent` 回放，保留原始 `sequence_number`。

## 验证命令

在 `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm` 执行：

```bash
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesWebSearchLifecycleEvents|InboundTransformer_TransformStream_ReplaysWebSearchLifecycleEventsFromMetadata)$' -count=1
go test ./transformer/openai/responses -count=1
git diff --check
```

结果：

```text
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.984s
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.491s
```

`git diff --check` 无输出，表示 whitespace check 通过。

## 自审

- 字段归属：通过独立 Responses search stream metadata seam 处理，未扩大公共 `llm.Request` / `llm.Response`。
- 协议边界：只做 Responses -> llm -> Responses 的同协议保真，不做 Chat/Anthropic 映射。
- 架构边界：未混入 MCP metadata key；search 事件族和 MCP 事件族分离，避免 TransformerMetadata 变成混合垃圾桶。
- 保真字段：测试覆盖 `type`、`sequence_number`、`output_index`、`item_id`。
- 顺序保真：inbound 回放使用 `enqueuePreservedEvent`，不会重新生成 sequence number。
- 范围控制：当前 helper 白名单只接受 web_search lifecycle 三种事件；file_search 留给 Slice 4.2。

## 结论

Slice 4.1 通过，可以进入 Slice 4.2（file_search lifecycle）。
