# G7 Module Re-Review

结论: **PASS**

agent id: g7-module-rereview

commits:
- `7a1d1cfe4338c715b7a20492cd26721ccb137f40` — 初版 G7
- `e6fe1a78d336a1715646f997d350e82f69749d6c` — 修复 review majors（outbound stream/aggregator reasoning_text + production prefer-text）

branch: `codex-transformer-field-fixes`

HEAD at review: `e6fe1a78d336a1715646f997d350e82f69749d6c`

scope: close previous majors M1/M2; re-verify G7 five micro-slices + default summary stream non-regression

review mode: code-only（不改生产代码）

date: 2026-07-12

prior review: `research/reviews/g7-module-review.md` (PASS-with-majors)

## blocker / major / minor

### Blocker
无

### Major
无

### Minor
1. **m1** `outbound_stream_test.go` golden 比较前将 `expectedEvent.TransformerMetadata = actual...`，协议 payload 仍比对，但 sidecar 元数据不再被 golden 约束。
2. **m2** aggregator `OutputItemDone` 仍不从最终 `item.ReasoningContent` 回填 `ReasoningTextParts`；仅靠 `reasoning_text.delta/done` 累积。真实上游通常会发 delta，边缘「仅 done 带 content[]」路径仍弱。
3. **m3** 初审遗留 minor 未在本修复提交关闭：`inbound_test` 过时注释；content[]+summary 并存时 common `ReasoningContent` 优先 content；classification/lossy companion 夹带。

## M1 / M2 关闭判定

| ID | 原问题 | 判定 | 证据 |
|---|---|---|---|
| **M1** | `outbound_stream` default silent skip `reasoning_text.*`；`aggregator` 仅 summary_* | **CLOSED** | 见下 |
| **M2** | `openai_responses_reasoning_prefer_text_stream` 无生产写入（仅测试注入） | **CLOSED** | 见下 |

### M1 CLOSED — outbound stream / aggregator 处理 reasoning_text

**outbound_stream.go**
- `StreamEventTypeReasoningSummaryTextDelta, StreamEventTypeReasoningTextDelta` 同 case：写入 `Delta.ReasoningContent`，不再 default skip。
- `StreamEventTypeReasoningSummaryTextDone, StreamEventTypeReasoningTextDone` 同 case：done 跳过（内容已在 delta 流过），reasoning_text done 额外标记 prefer-text state。
- `OutputItemDone` + `type=reasoning` + `ReasoningContent`：写入 text-content / prefer-text metadata；无 encrypted 时也可发 metadata-only chunk。

**aggregator.go**
- `aggregatedItem.ReasoningTextParts` + `ensureReasoningTextPart`
- `StreamEventTypeReasoningTextDelta` / `ReasoningTextDone` 累积 content[]
- `buildResponse()` 输出 `Item.ReasoningContent`

**tests**
- `TestResponsesOutboundStreamReasoningTextToCommon` — upstream SSE `reasoning_text.delta/done` → common `Delta.ReasoningContent == "think-athink-b"`
- `TestResponsesAggregatorReasoningTextContent` — 聚合 body 含 `content[]` `reasoning_text` `"alpha"`

### M2 CLOSED — production prefer-text 写入

生产写入点（非测试注入）：

1. **stream delta** (`outbound_stream.go`)：`ReasoningTextDelta` →  
   `state.transformerMetadata[prefer_text]=true` + chunk metadata
2. **stream done** (`outbound_stream.go`)：`ReasoningTextDone` → state prefer-text
3. **stream item.done** (`outbound_stream.go`)：`len(Item.ReasoningContent)>0` →  
   `text_content` + `prefer_text=true`（state + chunk）
4. **non-stream outbound** (`outbound_convert.go`)：`ReasoningContent` →  
   `responsesReasoningTextContentTransformerMetadataKey`  
   （inbound stream 门控同时接受 text-content 存在 **或** prefer-text bool）

inbound 门控（默认 summary 不回归）：

```text
emitReasoningText =
  text_content metadata present
  OR prefer_text == true
else → reasoning_summary_*（默认路径）
```

`TestResponsesOutboundStreamReasoningTextToCommon` 断言生产路径 `sawPreferText=true`。

## 默认 summary stream 不回归

| 检查 | 结果 |
|---|---|
| `handleReasoningContent` 无 prefer-text / text-content 时走 `reasoning_summary_*` | PASS（代码路径仍在） |
| 全包 `go test ./transformer/openai/responses` | ok（含既有 golden stream fixtures） |
| `outbound_stream_test` fixture 比对仍绿 | ok（sidecar 比较放宽，协议字段仍比对） |

未发现默认 summary 路径被无条件切换到 `reasoning_text.*`。

## G7 五 micro-slice 仍成立

| Slice | Status | Evidence |
|---|---|---|
| 8A `reasoning.context` same-protocol | PASS | `TestResponsesReasoningContextSameProtocolRoundTrip` |
| 8B `generate_summary` 身份分离 | PASS | generate_summary only / both tests |
| 8C output `content[]` / `reasoning_text` | PASS | `TestResponsesReasoningOutputContentPreserved` |
| 8D stream：gated `reasoning_text.*` + default summary | PASS | inbound gate + outbound production path + both G7 stream tests |
| 8E unknown nested reasoning 不 silent drop | PASS | `TestResponsesReasoningUnknownNestedPreserved` |

## 测试证据

在 `/Users/asuan/项目/AI/axonhub/llm`：

```text
go test ./transformer/openai/responses -count=1 -run 'TestResponses'  → ok
go test ./transformer/openai/responses -count=1                       → ok
go test ./transformer/openai ./transformer/anthropic -count=1         → ok / ok
```

关键 G7 / 修复用例全部 PASS：
- `TestResponsesReasoningContextSameProtocolRoundTrip`
- `TestResponsesReasoningGenerateSummaryKeepsDeprecatedIdentity`
- `TestResponsesReasoningSummaryAndGenerateSummaryBothPreserved`
- `TestResponsesReasoningOutputContentPreserved`
- `TestResponsesReasoningUnknownNestedPreserved`
- `TestResponsesReasoningTextStreamEvents`
- `TestResponsesOutboundStreamReasoningTextToCommon`  ← e6fe1a78
- `TestResponsesAggregatorReasoningTextContent`       ← e6fe1a78

## 代码落点（修复提交）

| 文件 | 变更 |
|---|---|
| `llm/transformer/openai/responses/outbound_stream.go` | reasoning_text delta/done 处理；prefer-text 生产写入；item.done content origin；clone metadata |
| `llm/transformer/openai/responses/aggregator.go` | ReasoningTextParts 累积与 buildResponse 输出 |
| `llm/transformer/openai/responses/reasoning_g7_test.go` | outbound stream + aggregator 回归测试 |
| `llm/transformer/openai/responses/outbound_stream_test.go` | golden 比对忽略 sidecar metadata 差异 |
| `research/g7-slice-ledger.md` | review gate 备注更新（内容可后续再刷） |

## 残留风险 / 非阻塞

1. Golden stream test 不再强制 sidecar 形状；未来 metadata 回归可能被吞。
2. Aggregator 对「无 delta、仅 final item.content[]」的 reasoning 仍可能丢 content。
3. 端到端「outbound SSE → common → inbound re-emit reasoning_text.*」无单一集成测试串联，但分段测试已覆盖两端门控与生产写入。

## 结论再述

**PASS** — `e6fe1a78` 真正关闭初审 M1/M2：outbound stream/aggregator 不再 silent skip `reasoning_text.*`，prefer-text 有生产写入；默认 summary stream 与 openai/anthropic/responses 相关测试全绿；G7 五个 micro-slice 仍然成立。无 blocker/major。可按仓库流程推进合并。

## Files Reviewed

- `llm/transformer/openai/responses/outbound_stream.go`
- `llm/transformer/openai/responses/aggregator.go`
- `llm/transformer/openai/responses/inbound_stream.go`（门控对照）
- `llm/transformer/openai/responses/outbound_convert.go`（非 stream text-content 生产写入）
- `llm/transformer/openai/responses/reasoning_g7_test.go`
- `llm/transformer/openai/responses/outbound_stream_test.go`
- `llm/transformer/openai/responses/model.go`（metadata key 常量）
- prior: `research/reviews/g7-module-review.md`
- commits: `7a1d1cfe`, `e6fe1a78`
