# Module 4 review fixes — Responses search stream/output fidelity

## 初次模块审查结论

Module 4 初审未通过。

子代理审查共指出：

1. P1：search `output_item.added/done` replay 与 final search metadata 双 owner，导致 `response.completed.output` 可能重复 search item，且 aggregator 生成的重复项缺 `action` / `queries` / `results`。
2. P1/P2：新 key `openai_responses_search_calls` 与 legacy key `openai_responses_web_search_calls` 同时存在时会重复 append。
3. P2：search metadata helper 放在 `outbound_convert.go`，实际被 inbound/outbound/stream 多处共享，模块归属错误。
4. P2：官方 required 空值字段会被 `omitempty` 丢失：
   - `file_search_call.queries: []`；
   - `web_search_call.action.type == "search"` 时 `query: ""`；
   - 同时验证 `open_page.url: null` 应继续保留。

## 回到 TDD/debug 后新增回归测试

新增测试：

- `TestInboundTransformer_TransformStream_MergesSearchOutputItemEventsAndFinalMetadata`
  - 复现 search stream replay + final metadata 同时存在时 completed output 重复的问题；
  - 修复后断言 final output 只有一个 search item，且 payload 完整。
- `TestInboundTransformer_TransformStream_DeduplicatesNewAndLegacySearchMetadata`
  - 复现新旧 metadata key 同时存在时重复输出；
  - 修复后按 search call identity 去重。
- `TestInboundTransformer_TransformStream_PreservesRequiredEmptySearchFields`
  - 复现 `queries:[]`、`query:""` 被 `omitempty` 丢失；
  - 修复后断言 JSON 输出保留 required 空值字段和 `url:null`。

红测试失败证据：

```text
TestInboundTransformer_TransformStream_MergesSearchOutputItemEventsAndFinalMetadata:
should have 1 item(s), but has 2

TestInboundTransformer_TransformStream_DeduplicatesNewAndLegacySearchMetadata:
should have 1 item(s), but has 2

TestInboundTransformer_TransformStream_PreservesRequiredEmptySearchFields:
actual missing queries:[] and action.query:""
```

## 修复实现

### 1. search metadata owner 收拢

新增：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/search_metadata.go`

集中放置：

- metadata keys：
  - `openai_responses_search_calls`
  - legacy read key `openai_responses_web_search_calls`
- search item copy/filter：
  - `responseSearchCallItemForMetadata`
  - `appendResponseSearchCallMetadata`
  - `getResponseSearchCallsFromMetadata`
- final output merge/dedupe：
  - `mergeResponseSearchCallsIntoOutput`
  - `responseSearchCallIdentity`

效果：

- inbound 不再依赖 `outbound_convert.go` 内的私有 helper；
- search metadata 作为独立 seam，职责清晰。

### 2. completed output merge/dedupe

修复前：

```go
response.Output = append(append([]Item(nil), calls...), response.Output...)
```

修复后：

```go
response.Output = mergeResponseSearchCallsIntoOutput(calls, response.Output)
```

策略：

- 若 aggregator output 已有同 `type:id` 的 search item，用 metadata 中完整 item 替换，避免丢 payload；
- metadata 中缺失于 aggregator output 的 search item prepend 到 output 前面，保留旧行为；
- 新旧 metadata key 同时存在时按 identity 去重。

### 3. required empty field 保真

修改：

- `WebSearchAction.MarshalJSON`
  - `type == "search"` 时强制输出 `query`，即使为空字符串；
  - `type == "find_in_page"` 时强制输出 `pattern`。
- `Item.MarshalJSON`
  - `type == "file_search_call"` 时强制输出 `queries`，nil 时输出 `[]`；
  - `results:null` 用 `json.RawMessage("null")` 保留。

## 验证命令

在 `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm` 执行：

```bash
go test ./transformer/openai/responses -run 'TestInboundTransformer_TransformStream_(MergesSearchOutputItemEventsAndFinalMetadata|DeduplicatesNewAndLegacySearchMetadata|PreservesRequiredEmptySearchFields)$' -count=1
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesWebSearchLifecycleEvents|InboundTransformer_TransformStream_ReplaysWebSearchLifecycleEventsFromMetadata|OutboundTransformer_TransformStream_PreservesFileSearchLifecycleEvents|InboundTransformer_TransformStream_ReplaysFileSearchLifecycleEventsFromMetadata|OutboundTransformer_TransformStream_PreservesSearchOutputItemEvents|InboundTransformer_TransformStream_ReplaysSearchOutputItemEventsFromMetadata|InboundTransformer_TransformStream_PreservesFileSearchCallsFromChunkMetadata|InboundTransformer_TransformStream_MergesSearchOutputItemEventsAndFinalMetadata|InboundTransformer_TransformStream_DeduplicatesNewAndLegacySearchMetadata|InboundTransformer_TransformStream_PreservesRequiredEmptySearchFields)$' -count=1
go test ./transformer/openai/responses -count=1
git diff --check
```

结果：

```text
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.755s
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.451s
ok  	github.com/looplj/axonhub/llm/transformer/openai/responses	0.723s
```

`git diff --check` 无输出。

## codebase-memory CLI 证据

按当前 goal，已使用 codebase-memory CLI 刷新索引和搜索：

```bash
codebase-memory-mcp cli --json index_repository '{"repo_path":"/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean","mode":"fast"}'
codebase-memory-mcp cli --json search_graph '{"project":"Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean","query":"search_metadata responseSearchCallItemForMetadata mergeResponseSearchCallsIntoOutput","limit":8}'
codebase-memory-mcp cli --json trace_path '{"project":"Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean","function_name":"mergeResponseSearchCallsIntoOutput","direction":"both","depth":2,"include_tests":true}'
```

CLI 结果确认：

- `responseSearchCallItemForMetadata`、`mergeResponseSearchCallsIntoOutput` 已位于 `search_metadata.go`；
- `mergeResponseSearchCallsIntoOutput` 仅被 inbound stream / inbound response assembly 调用；
- 三个回归测试均可被 CLI 搜索到。

## 当前状态

Focused re-review 最终结论：

- [x] focused behavior re-review pass — 2026-07-07, subagent 019f3b16-d083-7130-b3bc-6984648edea0
- [x] focused architecture re-review pass — 2026-07-07, subagent 019f3b16-d0fc-7570-b7e6-480eb359df78
