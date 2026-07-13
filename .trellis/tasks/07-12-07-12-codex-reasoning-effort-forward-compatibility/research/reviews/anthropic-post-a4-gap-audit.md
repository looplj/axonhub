# Anthropic post-A4 gap audit

- 日期：2026-07-13
- 审计范围：`ANT.TOP`、`ANT.MSG`、`ANT.RESP`、`ANT.STREAM`；仅审计 `llm/transformer/anthropic/**`。
- 协议证据：
  - `docs/specs/protocols/anthropic-claude-messages-protocol.md`
  - `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-messages-api.official-raw.md`
  - `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-messages-streaming.official-raw.md`
- 代码取证：优先使用 `/Users/asuan/.local/bin/codebase-memory-mcp cli` 的 `search_graph` / `get_code_snippet`，关键结论再对照当前磁盘源码。
- 审计标准：只有“官方 shape 已确认 + 当前同协议路径确实丢失 + Anthropic adapter 内有明确 same-protocol owner”的项才实现；不增加 OpenAI 桥接、不改 `temperature > 1` / `service_tier` 未决策略。

## A1–A4 复核结果

| 范围 | 结论 | 当前 owner / 证据 | 分类 |
|---|---|---|---|
| `ANT.MSG.content_blocks` 与 `tool_result.content[]` future children | A1 已覆盖。未知 request block 及 nested child 以 Anthropic raw fragment 保存，并按原始顺序合并回 Anthropic wire body。 | `tool_blocks.go`、`inbound_convert.go`、`outbound_convert.go`；`a1_unknown_content_blocks_test.go`。 | 已实现；剩余 known-block 组合是 `TEST_ONLY`。 |
| `ANT.TOP.tools` | A2 已覆盖。普通 function 的 Anthropic-only children 与 server/native declaration 使用 raw declaration fragment；不能伪装成 `llm.Tool`。 | `tools.go`、`inbound_convert.go`、`outbound_convert.go`；`a2_tool_declaration_preservation_test.go`。 | 已实现；跨协议无等价是 `EXPLICIT_NO_IMPLEMENT`。 |
| `ANT.RESP.stop_sequence` / `stop_details` / future `usage` children | A3 的 non-stream response 路径已覆盖：stop fields 走 response metadata，usage 原始对象在有未建模 children 时 replay。 | `response_native.go`、`usage.go`、`inbound_convert.go`、`outbound_convert.go`；`a3_response_stop_usage_test.go`。 | 已实现；更多 usage variant 是 `TEST_ONLY`。 |
| unknown top-level semantic SSE | A4 已覆盖，未知 event type 写入 `ProviderExtensions.Anthropic.Response.RawStreamEvents`，仅 Anthropic inbound 重放。 | `outbound_stream.go` default 分支、`inbound_stream.go:enqueueRawAnthropicStreamEvents`；`a4_raw_stream_event_test.go`。 | 已实现。 |
| raw sidecar clone | 已覆盖。`CloneProviderExtensions` 深拷贝每个 `AnthropicRawStreamEvent.Raw`。 | `llm/provider_extensions.go:198-225`。 | 已实现；本次不改公共 owner。 |

## 本次确证并完成的缺口

### A4b：未知 block type 位于已知 stream lifecycle event 中

**官方 shape**：Anthropic stream 的 `content_block_start` / `content_block_delta` / `content_block_stop` 是合法 lifecycle event；内容块与 delta 的 union 会演进。

**原始问题**：A4 之前只在 `switch streamEvent.Type` 的 `default` 保存未知 event type。若 event type 已知、但 `content_block_start.content_block.type` 是 future/unknown type，则 `content_block_start` 的内层 `default` 直接返回 `nil`；其后 delta/stop 也没有 adapter-local owner，整段 lifecycle 丢失。

**修复**：

- `streamState.rawContentBlockIndexes` 标记 unknown block 的原始 index；
- 该 block 的 start/delta/stop 都写入 Anthropic response raw-stream sidecar；
- `ping` 仍过滤为 transport heartbeat；known text/thinking/tool lifecycle 仍走既有 canonical conversion；
- Anthropic inbound 在 replayed `content_block_stop` 后标记已关闭，终态不会再合成同 index 的重复 stop。

**RED → GREEN**：`TestA4_UnknownAnthropicContentBlockLifecycle_SameProtocolRawReplay` 初始失败（缺 `content_block_start`），初版 sidecar 修复后又暴露重复 stop；补充 inbound 状态同步后通过。该 fixture 同时断言 start/delta/stop JSON 等价和 stop 不重复。

### A5：stream `message_delta.delta.stop_sequence`

**官方 shape**：Anthropic `message_delta` 可同时携带 `delta.stop_reason = "stop_sequence"` 和 matched `delta.stop_sequence`。

**原始问题**：`outbound_stream` 只把 stop reason 放进 `TransformerMetadataKeyAnthropicStopReason`；`inbound_stream` 也只重发 stop reason，且没有从 metadata 恢复原生 reason。`stop_sequence` stream round-trip 退化成 `end_turn` 并丢失 matched string。

**修复**：

- outbound 保存 `TransformerMetadataKeyAnthropicStopReason` 与 `TransformerMetadataKeyAnthropicStopSequence`；
- inbound 优先恢复原始 Anthropic stop reason；在输出 `message_delta` 时写回 matched stop sequence；
- 普通 OpenAI finish reason 保留既有 fallback mapping。

**RED → GREEN**：`TestA5_StreamMatchedStopSequence_SameProtocolRoundTrip` 初始得到 `end_turn`，修复后准确输出 `stop_sequence` 与 `###END###`。

### D2：OpenAI → Anthropic 的三个无等价字段诊断

**确证字段**：`safety_identifier`、`prompt_cache_key`、`metadata` 的非 `user_id` 余项。

**原始问题**：canonical `llm.Request` 已有这些 OpenAI carriers；Anthropic `buildBaseRequest` 只把 `metadata.user_id` 映射到 `AnthropicMetadata.UserID`，这是正确的 no-fake-bridge 行为，但其余字段静默丢失，现有 `recordAnthropicChatNativeLossyDowngrades` 未记录诊断。

**修复**：显式 allowlist 调用现有 `llm.AddLossyDowngradeIfPresent`。不改 Anthropic request body：

- `safety_identifier` 不写入 `metadata.user_id`；
- `prompt_cache_key` 不写入 `cache_control`；
- metadata 只在包含非 `user_id` key 时记录一个 `metadata` downgrade。

**RED → GREEN**：`TestOutboundTransformer_TransformRequest_DiagnosesOpenAIMetadataOnlyLosses` 对 OpenAI Chat 和 Responses 两个来源初始没有诊断；修复后 body 仅保留 `metadata.user_id`，同时精确记录三条 `LossyDowngrade`。

## 明确不实现 / 后续项

| 项目 | 结论 | 理由 |
|---|---|---|
| `temperature > 1` → Anthropic | `BLOCKED_PRODUCT_DECISION` | 不可自行 clamp、报错或丢弃；没有批准的产品策略。 |
| `service_tier` 跨协议值域 | `BLOCKED_PRODUCT_DECISION` | OpenAI 与 Anthropic 枚举不等价；不按字符串猜映射。 |
| `ping` | `EXPLICIT_NO_IMPLEMENT` | 是 transport heartbeat，不是 response semantic event。 |
| known text/thinking/tool lifecycle | `TEST_ONLY` | 已有 canonical owner；本次不为了 raw replay 重放会被正常流状态机重建的事件。 |
| non-stream `usage` 全值域、所有官方 block/tool variants | `TEST_ONLY` / `DOC_ONLY` | A1–A3 已修复确认的 native loss，但严格矩阵仍是 `PARTIAL`，不可宣称完整协议已确认。 |

## 验证

```bash
cd /Users/asuan/项目/AI/axonhub/llm
go test ./transformer/anthropic -run '^TestOutboundTransformer_TransformRequest_DiagnosesOpenAIMetadataOnlyLosses$' -count=1 -v
go test ./transformer/anthropic -run '^TestA5_StreamMatchedStopSequence_SameProtocolRoundTrip$' -count=1 -v
go test ./transformer/anthropic -run '^TestA4_UnknownAnthropicContentBlockLifecycle_SameProtocolRawReplay$' -count=1 -v
go test ./transformer/anthropic -count=1
git diff --check
```

所有上述命令在本次改动后通过。

### A4 follow-up：raw lifecycle 与后续结构化 block 混排时的 index 同步

**独立行为审查发现的 P1**：A4 的 unknown content-block lifecycle 原样 replay 后，Anthropic inbound 只记录了“已回放 stop”，却没有推进内部 `contentIndex`。因此同一流中紧随其后的结构化 text block 会再次使用 `index=0`，而不是原始生命周期之后的 `index=1`。

**RED**：新增公开 stream round-trip fixture
`TestA4_UnknownContentBlockThenText_SynchronizesContentIndex`。它使用
`message_start → unknown block[0] start/delta/stop → text block[1] start/delta/stop`
的真实 Anthropic 流，经过 outbound canonicalization 和 inbound replay。修复前失败：
`text must start after the replayed raw block`，实际 index 为 `0`。

**最小 GREEN 机制**：raw sidecar replay 仅对 `content_block_stop` 的公开 JSON payload 解码；仅当 event type、payload type 和 index 均可验证时，单调更新 `contentIndex = max(contentIndex, stop.index + 1)`。它不假设 start/delta/stop 被装入同一个 canonical chunk，也不会对无 index 或格式不匹配的 raw event 猜测状态。已有的 replay stop 标记仍保留，避免终态重复合成 stop。

**GREEN 验证**：

```bash
cd llm && go test ./transformer/anthropic -run '^TestA4_UnknownContentBlockThenText_SynchronizesContentIndex$' -count=1 -v
cd llm && go test ./transformer/anthropic -count=1
git diff --check
```

以上命令均通过。

## 本次修改文件

```text
llm/transformer/anthropic/outbound.go
llm/transformer/anthropic/outbound_test.go
llm/transformer/anthropic/outbound_stream.go
llm/transformer/anthropic/inbound_stream.go
llm/transformer/anthropic/a4_raw_stream_event_test.go
llm/transformer/anthropic/a5_stream_stop_sequence_test.go
.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/anthropic-post-a4-gap-audit.md
```

### A1 follow-up：可转换 media block 与 unknown raw block 的 wire 顺序

**Owner**：Anthropic request adapter owns the source ordinal. `inbound_convert.go` writes the original top-level content-block index to each canonical `MessageContentPart` via the existing `setAnthropicBlockIndex` helper in `tool_blocks.go`; `outbound_convert.go:convertMultiplePartContent` already reads that metadata and merges typed blocks with `anthropic_raw_block` through the existing `sortOrderedContentBlocks` path. This remains Anthropic same-protocol preservation and does not create a cross-protocol bridge.

**RED**：新增 public request round-trip fixture
`TestA1_ConvertibleMediaBeforeUnknownBlock_PreservesExactOrder`，覆盖官方现有 request shape 中的 `[image, unknown]` 与 `[document, unknown]`，并对完整 `content` array 做 JSON 等价断言（array order 必须精确一致）。修复前两个子用例均失败：输出分别变成 `[unknown, image]` 与 `[unknown, document]`。原因是 unknown raw part 携带原始 index，而顶层可转换 image/document part 没有 index；既有排序会把有 index 的 unknown 排到无 index media 之前。

**GREEN**：只在 Anthropic request inbound 的顶层 `image` / `document` 成功转换分支写入 `blockIdx`。不改既有 ordered raw merge，不改变排序规则，不复制 raw block，也不向 OpenAI/其他协议伪造 media 或 unknown-block 映射。

验证结果：

```bash
cd llm && go test ./transformer/anthropic -run '^TestA1_ConvertibleMediaBeforeUnknownBlock_PreservesExactOrder$' -count=1 -v
# PASS: image / document

cd llm && go test ./transformer/anthropic -count=1
# ok github.com/looplj/axonhub/llm/transformer/anthropic

git diff --check
# exit 0
```

**本 follow-up 实际改动 owner 范围**：

- `llm/transformer/anthropic/inbound_convert.go`
- `llm/transformer/anthropic/a1_unknown_content_blocks_test.go`
- 本审计记录（append only）

**明确未改范围**：`llm/model.go`、provider extensions、Anthropic stream inbound/outbound、response-native、OpenAI Responses、OpenAI Chat、strict matrix；`outbound_convert.go` 与 `tool_blocks.go` 的既有 ordered-index mechanism 仅被复用，没有重写。未回退并行实现者的现存改动。

## A3 architecture follow-up — non-stream response-native sidecar owner (2026-07-13)

- **RED:** `TestA3_StopSequenceAndStopDetails_SameProtocolRoundTrip` was tightened to require `ProviderExtensions.Anthropic.Response` ownership for non-stream `stop_sequence`, structured `stop_details`, and full raw `usage`; before the migration the required sidecar fields did not exist. `TestCloneProviderExtensions_AnthropicResponseNativeFieldsDeepCopy` additionally requires pointer and raw JSON buffers not to alias.
- **GREEN owner:** `llm.ProviderExtensions.Anthropic.Response` now owns only these non-stream Anthropic-native response fragments: `StopSequence`, `StopDetails`, and `RawUsage`. The Anthropic outbound adapter captures cloned source values there; the inbound adapter restores only from that named sidecar.
- **Metadata boundary:** non-stream `Choice.TransformerMetadata` / `Response.TransformerMetadata` no longer stores these three fields. Existing stream lifecycle metadata (including stream stop sequence/reason) is intentionally unchanged and out of this A3 follow-up scope.
- **Clone boundary:** `CloneProviderExtensions` deep-copies the stop-sequence pointer and both raw JSON buffers, as well as the pre-existing raw stream event slice. No second storage model or cross-protocol bridge was introduced.
