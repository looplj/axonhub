# G7 Module Review

结论: **PASS**

agent id: g7-module-review

commit: `7a1d1cfe4338c715b7a20492cd26721ccb137f40`

branch: `codex-transformer-field-fixes`

scope: `fix(responses): preserve reasoning context, generate_summary, content, stream`

review mode: code-only（不改生产代码）

date: 2026-07-12

## blocker / major / minor

### Blocker
无

### Major
1. **M1** outbound stream 对 `response.reasoning_text.delta/done` silent skip（`outbound_stream.go` default；`aggregator.go` 仅 summary_*）
2. **M2** `openai_responses_reasoning_prefer_text_stream` 无生产写入点（仅 `reasoning_g7_test.go` 注入）

### Minor
1. **m1** `inbound_test.go` 仍写 “generate_summary merged to summary”，未反映 origin/value 身份分离
2. **m2** content[] + summary 并存时 common `ReasoningContent` 只保留 content 文本（summary 进 metadata；same-protocol 可重建）
3. **m3** commit 夹带 classification / lossy diagnostics / additional_tools 回放 companion 改动

## 证据

### 设计意图核对

| # | Intent | Result | Evidence |
|---|---|---|---|
| 1 | `reasoning.context` same-protocol 保留 | PASS | typed `Reasoning.Context`；inbound 写 `openai_responses_reasoning_context`；outbound `convertReasoning` 回写；`TestResponsesReasoningContextSameProtocolRoundTrip` |
| 2 | deprecated `generate_summary` 与 `summary` 身份分离 | PASS | origin/value 双 sidecar；generateOnly 只写 `generate_summary`；双字段并存都回写；G7 only/both tests |
| 3 | response reasoning item `content[]` / `reasoning_text` 保留 | PASS | `Item.ReasoningContent` + custom JSON；outbound 优先 content；metadata 保存 parts；`buildReasoningItem` 重发；`TestResponsesReasoningOutputContentPreserved` |
| 4 | stream：`reasoning_text.*` 有路径；默认 summary 不回归 | PASS* | inbound `handleReasoningContent` 门控；默认 `reasoning_summary_*`；`TestResponsesReasoningTextStreamEvents`；*见 M1/M2 |
| 5 | unknown nested reasoning 不 silent drop | PASS | `extractRawReasoningObject` + `mergeRawReasoningObject` raw-first；`TestResponsesReasoningUnknownNestedPreserved` |
| 6 | request reasoning 不进 TransformerMetadata 当 protocol body field | PASS | 仅 context / generate_summary origin-value / raw object / content-parts 等 sidecar；未扩展 llm.Request body |
| 7 | 不把 effort 当 Anthropic budget | PASS | effort→`ReasoningEffort`，max_tokens→`ReasoningBudget`；本 commit 无 anthropic 文件 |

### 测试

在 `/Users/asuan/项目/AI/axonhub/llm`：

```text
go test ./transformer/openai/responses -count=1 -run 'TestResponsesReasoning'  → ok
go test ./transformer/openai/responses -count=1                               → ok
go test ./transformer/openai ./transformer/anthropic -count=1                 → ok / ok
```

关键用例：
- `TestResponsesReasoningContextSameProtocolRoundTrip`
- `TestResponsesReasoningGenerateSummaryKeepsDeprecatedIdentity`
- `TestResponsesReasoningSummaryAndGenerateSummaryBothPreserved`
- `TestResponsesReasoningOutputContentPreserved`
- `TestResponsesReasoningUnknownNestedPreserved`
- `TestResponsesReasoningTextStreamEvents`

### 代码落点

- context: `inbound.go` metadata write；`outbound_convert.go` convertReasoning restore；`model.go` `Context` field
- generate_summary: inbound origin/value sidecars；outbound generateOnly 分支
- content[]: `model.go` custom JSON；`outbound_convert.go` prefer content；`inbound.go` `buildReasoningItem`
- stream inbound: `inbound_stream.go` prefer-text gate；`stream_event.go` 常量
- unknown nested: `extractRawReasoningObject` + `mergeRawReasoningObject`
- effort≠budget: Responses 分槽；commit 无 anthropic 改动

## 修复建议

1. **P1 / M1**：`outbound_stream.go` case `ReasoningTextDelta` → `Delta.ReasoningContent`；`ReasoningTextDone` 对齐 summary done；`aggregator.go` 累积 content[]；加上游 SSE fixture
2. **P1 / M2**：当 outbound 解析到 reasoning item 含 `content[]/reasoning_text` 时自动写 prefer-text 或 text-content metadata；补端到端 stream test（非纯 unit 注入）
3. **P2 / m1**：刷新 inbound_test 注释与 sidecar 断言
4. **P3 / m2,m3**：文档化 content/summary common 文本策略；ledger 标注 companion 变更

## 详细说明

### 1. reasoning.context
- Model 增加 `Context string \`json:"context,omitempty"\``
- Inbound 仅写 sidecar，不并入 effort/summary 公共字段
- Outbound `convertReasoning` 回写 `reasoning.context`
- Fixture：`effort=high / summary=auto / context=current_turn` round-trip

### 2. generate_summary identity
- Inbound：summary 优先映射 `ReasoningSummary`；仅 generate_summary 时投影 + origin=true；只要有 generate_summary 就存 value
- Outbound：origin=true 只发射 `generate_summary`；否则 summary + 可选 generate_summary 并存
- 不把 deprecated 字段永久 rewrite 成 summary

### 3. output content[] / reasoning_text
- `Item.ReasoningContent` 用 `json:"-"`，reasoning type 走 wire `content`
- Outbound 优先 content 文本，summary 仅 fallback，parts 进 response metadata
- Inbound rebuild：有 raw text parts → content[]；common-only → summary_text 兼容

### 4. stream
**已实现（common→Responses inbound client）：**
```text
prefer-text OR text-content metadata
  → response.reasoning_text.delta / .done
  → output_item.done 带 content[] reasoning_text
else
  → response.reasoning_summary_*（默认，不回归）
```

**缺口：**
- Outbound stream（Responses upstream→common）未处理 reasoning_text.*，落入 default skip
- prefer-text key 生产无写入者

### 5. unknown nested
raw object 保存；merge 时 raw 作底、structured 覆盖 known keys；future_nested / another_future 存活

### 6. metadata 边界
允许 sidecar：context / generate_summary origin-value / raw object / text-content / summary-content / prefer-text / enabled / reasoning item  
不允许：把 request reasoning 整体当作 llm.Request protocol body field

### 7. effort ≠ Anthropic budget
effort 与 max_tokens 分槽；effort+budget 同时存在时 Responses outbound 优先 effort；本 commit 未改 anthropic

## 结论再述

**PASS** — 五个 micro-slice（context / generate_summary / output content / gated stream path / unknown nested）达到可合并质量；测试全绿。合并后应排期 M1+M2，否则 `reasoning_text` stream family 仍半成品（inbound 有路径、outbound/生产门控未齐）。

## Files Reviewed

- `llm/transformer/openai/responses/{model,inbound,inbound_stream,outbound_convert,request_extensions,stream_event}.go`
- `llm/transformer/openai/responses/{reasoning_g7_test,reasoning_context_test,outbound_test,cross_protocol_test}.go`
- `llm/transformer/openai/responses/testdata/reasoning-*.json`
- `llm/transformer/shared/responses_lossy_downgrade.go`
- `llm/openai_responses_classification.go` (+ test)
- 对照：`outbound_stream.go` / `aggregator.go`（未改但与 stream intent 相关）
