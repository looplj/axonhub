# Module 3 review fix result — MCP stream events

时间：2026-07-07

## 初审反馈

Standards / Architecture / Spec 多 agent 审查发现：

- P1/P0 等级：`stream_extensions.go` 是未跟踪文件，若只看 tracked diff 会导致 diff 不自包含。
- P1：MCP stream events 的官方必填 `sequence_number` 未保真。metadata copy 未保存，inbound replay 通过 `enqueueEvent` 重排。
- P2：`outbound_stream.go` 多处重复 `resp.TransformerMetadata = map[string]any{}` + append，可抽 helper 提升 locality。
- P2：`TransformerMetadata` magic key 有张力，需要守住 Responses-native 白名单和同协议 replay seam。

## 修复

- 暂存全部 Module 3 文件，确认 `stream_extensions.go` 进入 staged diff：
  - `A llm/transformer/openai/responses/stream_extensions.go`
- `stream_extensions.go`
  - `responsesMCPStreamEventForMetadata` 现在保存 `SequenceNumber`。
- `inbound_stream.go`
  - 新增 `enqueuePreservedEvent`，用于 replay Responses-native MCP events，不覆盖原始 `sequence_number`。
  - replay 后将内部 sequence cursor 推进到 `max(current, preserved+1)`，避免后续生成事件倒退。
- `outbound_stream.go`
  - 新增 `preserveMCPStreamEvent` helper，集中 MCP metadata 写入。
- tests
  - outbound tests 断言 metadata 中保留原始 `sequence_number`。
  - inbound tests 断言 replay 出来的 SSE 保留 metadata 中的 `sequence_number`。

## 验证

执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'Test(OutboundTransformer_TransformStream_PreservesMCP|InboundTransformer_TransformStream_ReplaysMCP)' -count=1
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --cached --check
```

结果：均通过。

## 当前待复审点

- P1 `sequence_number` 是否已充分修复。
- `stream_extensions.go` 是否已经纳入 staged diff，不再有不自包含问题。
- P2 magic-key 张力是否已被白名单、Responses-native naming、helper seam 控制在可接受范围。

## Focused re-review

Spec focused re-review：

- P0/P1/P2：未发现仍存在。
- `sequence_number`：已修到足以通过。
- `stream_extensions.go`：已纳入 staged diff。
- 官方 MCP stream fields：未发现“声称覆盖但实际丢失”；`type/output_index/item_id/delta/arguments/sequence_number/item` 均有 metadata 保真路径与 staged tests 覆盖。

Architecture / Standards focused re-review：

- P0/P1：未见仍存在。
- `preserveMCPStreamEvent` 改善了 locality。
- `stream_extensions.go` 是可接受的 Responses-native preservation module，不是 raw bucket。
- 允许提交 Module 3，不需要先做架构重构。
- 轻微 nit：`copyStringPtr` 手写 pointer helper，可提交前小修。

## Final pre-commit nit fix

- 移除 `copyStringPtr` helper。
- `responsesMCPStreamEventForMetadata` 改为局部 `itemID` + `lo.ToPtr(*event.ItemID)`。
- 重新执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --cached --check
```

结果：均通过。
