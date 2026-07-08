# Module 4 Slice 4.3 — Responses search output item payload fidelity

## 范围

本 slice 修复 OpenAI Responses search output item 的同协议保真：

- `response.output_item.added/done` 中 `item.type == "web_search_call"`；
- `response.output_item.added/done` 中 `item.type == "file_search_call"`；
- 最终 `response.completed.response.output` 中 search call item 的 payload。

字段所有者：`llm/transformer/openai/responses` stream fidelity + Responses native output item metadata。

不处理：Chat / Anthropic / cross-protocol 映射 / LossyDowngrade。

## 官方字段依据

本地官方 OpenAPI YAML：

- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/openai-openapi.github.yaml`

抽取到：

- `WebSearchToolCall` required: `id`, `type`, `status`, `action`；
- `WebSearchActionSearch`: `type`, `query`, `queries`, `sources`；
- `WebSearchActionOpenPage`: `type`, `url`；
- `WebSearchActionFind`: `type`, `url`, `pattern`；
- `FileSearchToolCall` required: `id`, `type`, `status`, `queries`；props: `id`, `type`, `status`, `queries`, `results`；
- `ResponseOutputItemAddedEvent` / `ResponseOutputItemDoneEvent`: `type`, `output_index`, `sequence_number`, `item`。

## Red test

新增测试：

- `TestOutboundTransformer_TransformStream_PreservesSearchOutputItemEvents`
- `TestInboundTransformer_TransformStream_ReplaysSearchOutputItemEventsFromMetadata`
- `TestInboundTransformer_TransformStream_PreservesFileSearchCallsFromChunkMetadata`

红测试失败证据：

```text
unknown field URL in struct literal of type WebSearchAction
unknown field Pattern in struct literal of type WebSearchAction
unknown field Queries in struct literal of type Item
unknown field Results in struct literal of type Item
undefined: responsesSearchCallsTransformerMetadataKey
```

说明当前模型确实无法表达官方 search output item payload，且 file_search final output metadata 没有 owner。

## Green implementation

修改点：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/model.go`
  - 新增 `responsesSearchCallsTransformerMetadataKey = "openai_responses_search_calls"`；
  - 保留旧 `openai_responses_web_search_calls` 作为 legacy read key；
  - `WebSearchAction` 增加 `URL json.RawMessage`、`Pattern string`；
  - `Item` 增加 file_search 字段 `Queries []string`、`Results json.RawMessage`。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/outbound_convert.go`
  - 新增 search call copy helper；
  - `convertOutputToMessage` 支持 `web_search_call` 和 `file_search_call` 写入 search metadata；
  - raw `results` 用 `json.RawMessage` 克隆，保留 nullable/future fields。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/inbound.go`
  - `getResponseSearchCallsFromMetadata` 读取新 key 和 legacy web key；
  - 最终 Responses output 从 search metadata 恢复 web/file search items。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/outbound_stream.go`
  - `output_item.added` 对 web/file search item 生成 search stream metadata；
  - `output_item.done` 对 web/file search item 同时生成 replay metadata，并写入最终 response output metadata。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/search_stream_extensions.go`
  - search stream whitelist 扩展到 `response.output_item.added/done`，但只允许 item 为 `web_search_call` / `file_search_call`。

## 验证命令

在 `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm` 执行：

```bash
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesSearchOutputItemEvents|InboundTransformer_TransformStream_ReplaysSearchOutputItemEventsFromMetadata|InboundTransformer_TransformStream_PreservesFileSearchCallsFromChunkMetadata)$' -count=1
go test ./transformer/openai/responses -count=1
git diff --check
```

结果：

```text
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.659s
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.912s
```

`git diff --check` 无输出。

## 自审

- 字段归属：search output item payload 属于 OpenAI Responses native stream/output item preservation，不进入公共 `llm.Request` / `llm.Response`。
- 协议边界：只做 Responses 同协议 replay/final output 保真，不映射到 Chat/Anthropic。
- 架构边界：search output metadata 使用 `openai_responses_search_calls`，旧 `openai_responses_web_search_calls` 仅作为读取兼容；没有继续扩大 web-only 命名。
- Raw 保真：`file_search_call.results` 用 `json.RawMessage`，避免提前建半截类型导致未来字段再次丢失。
- Stream 保真：search `output_item.added/done` 通过 search stream metadata replay，且最终 `response.completed.output` 仍保留 completed search item。
- 白名单：`response.output_item.added/done` 只有 item 为 `web_search_call` / `file_search_call` 时进入 search seam，避免把所有 output item 混进 search metadata。
- 死代码检查：旧 `appendResponseWebSearchCallMetadata` / `getResponseWebSearchCallsFromMetadata` 已移除，无引用。

## 结论

Slice 4.3 通过。Module 4 三个切片已完成本地 TDD、自审和 package-level 验证，下一步应进入 Module 4 级别审查：Standards/Spec/code-quality/architecture review；不通过则回到 TDD/debug 修复，通过后再 commit Module 4。
