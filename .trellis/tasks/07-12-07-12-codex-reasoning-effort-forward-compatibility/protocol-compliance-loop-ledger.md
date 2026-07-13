# Protocol compliance loop ledger — latest Codex delta

## Source baseline

- Codex delta: `1f0566d..9e552e9d1`.
- Research reports:
  - `research/codex-delta-request-response-1f0566d-to-9e552e9d1.md`
  - `research/codex-delta-streaming-metadata-1f0566d-to-9e552e9d1.md`
  - `research/codex-delta-tools-mcp-1f0566d-to-9e552e9d1.md`

## Numbering resolution

| Number | Status | Meaning |
|---|---|---|
| G9 | historical, complete | Responses `stream_options` raw nested preservation |
| G10 | historical, complete | raw JSON clone helper |
| G11 | historical, complete | lossy downgrade recorder |
| G12 | historical, complete | raw top-level capture helper |
| G13 | complete | Responses request `reasoning` + encrypted reasoning include preservation |
| G14 | complete | Responses reasoning summary + summary stream-options preservation, without Codex model gating |
| G15 | complete | Responses request `input[]` item id identity/presence preservation (message / tool / reasoning) |

## Delta classification table

| Delta | Classification | Why | Next action |
|---|---|---|---|
| Always `reasoning` + `include: reasoning.encrypted_content` | G13 | existing Responses fields, new Codex outbound policy | audit current Hub preservation only |
| Capability-gated `reasoning.summary` + summary delivery option | G14 | existing Responses fields, new Codex emission gate | audit preservation/absence, do not duplicate gate |
| Strip empty/unprefixed outbound `input[].id` | G15 | existing Responses item identity behavior | audit preservation/absence, do not import prefix policy |
| `ReasoningEffort` named values + custom support; `ultra -> max` | G13/G14 note | value-domain/client policy, not a new wire field | retain same-family open-string proof; no generic mapping |
| tool_search/defer_loading/namespace/apply_patch/web-search/MCP declaration | existing domains, no delta | no production wire shape change in range | no G13–G15 work |
| approval/MCP OAuth/sandbox/multi-agent events/telemetry | client-only | not Responses body | document exclusion only |

## Slice state

| G / slice | Seam | State | Evidence | Next route |
|---|---|---|---|---|
| Pre-G13 unknown effort same-family | Chat/Responses inbound → canonical → outbound | PASS, auxiliary evidence | `future-effort` fixtures (`reasoning_effort_forward_compat_test.go`, Chat inbound tests); no production change | retain as G13/G14 value-domain proof |
| G13a | Responses body → canonical → Responses body | PASS | `g13a_reasoning_include_test.go`; `g13a-*.request.json`; preserve + default omission; module review PASS | G14 |
| G14a/G14b | Responses body → canonical → Responses body | PASS | `g14a_summary_stream_options_test.go`, `g14b_stream_options_sidecar_test.go`; dedicated `RawStreamOptions` clone; module re-review PASS | G15 |
| G15a | request `input[]` message / function / function_output item id | PASS | `g15a_input_item_identity_test.go`; `g15a-*.request.json`; preserve + no synthetic ids | G15b |
| G15b | custom tool + reasoning-following tool item id | PASS | `g15b_input_item_identity_test.go`; `g15b-*.request.json`; custom_tool_call(_output) + reasoning→tool path | G15c |
| G15c | reasoning item id / presence | PASS | `g15c_reasoning_item_identity_test.go`; fixtures cover standalone, pure standalone, summary-only, reasoning→tool, and no cross-protocol invent | parent review |
| Parent review (G13–G15) | integration | PASS | `research/reviews/g13-g15-parent-review.md`; independent review; no P0/P1/P2 | complete |
| Final integration docs/commit | matrices + guidelines + scoped commit | PASS | targeted tests + `git diff --check`; scoped local commit `5c63811d` | complete |

## Checkpoint

G13, G14, G15a/b/c are complete with public-seam fixtures and independent module reviews. Residual-coverage language is retired: custom-tool, summary-only, pure standalone, and reasoning→tool identities now have dedicated G15b/G15c fixtures.

父级审查、最终定向验证和 scoped local commit 已完成。任务执行证据完整；严格矩阵整体仍不宣称所有 Field ID / 跨协议方向完成。

Rules retained: no Codex default injection; no model-capability gate; no item-id synthesis/fallback; no cross-protocol Codex id invention.

## 2026-07-13 R1/R2 implementation checkpoint

- Slice: R1/R2 Responses envelope / terminal stream only.
- Production scope: `llm/transformer/openai/responses/{model,outbound,outbound_convert,inbound,inbound_stream,aggregator}.go`.
- Tests: `r1_envelope_refusal_test.go`, `r2_stream_terminal_error_test.go`.
- Covered:
  - non-stream `status` queued/in_progress same-family preserve
  - non-stream `incomplete_details.reason` preserve via Responses-native metadata carrier
  - non-stream message content `type=refusal` preserve via `Message.Refusal`
  - inbound stream `response.completed` no longer requires a usage chunk (usage still preferred when present; usage-less completes at stream end)
  - `AggregateStreamChunks` top-level `error` becomes failed response with code/message
- Explicitly not claimed: hosted/MCP output families, response.queued SSE re-emit, broad raw output architecture, error.param schema expansion.
- Verification: `cd llm && go test ./transformer/openai/responses/ -count=1` PASS.

## 2026-07-13 C1/C2 self-review correction

- 初版 C1/C2 已覆盖 Chat custom declaration / named custom choice / custom call / file / refusal，但自审发现官方 `tool_choice` union 的 `allowed_tools` 分支漏实现。
- 已在**同一 Chat slice**内按 RED→GREEN 补齐 `allowed_tools.{mode,tools[]}` 的 Chat-only carrier 和 request public round-trip fixture；不使用 Responses schema，不建立跨协议映射。
- 该 finding 在 Chat 模块独立审查前关闭；不得把初版通过描述为全量 `CHAT.TOP.tool_choice` 覆盖。

## 2026-07-13 全字段盘点 checkpoint

- 证据产物：`research/full-field-gap-inventory.md` 与五份 `research/inventory-*.md`。
- 结论：矩阵的 `UNCHECKED`/`PARTIAL` 不能批量当 bug；已分为 implementation、diagnostic、test-only、doc-only、explicit-no-implement 与 blocked-product-decision。
- 下一阶段：执行 C1、C2、R1/R2、A1/A3、A2/D1 五个互不重叠的 TDD 切片；每个 slice 完成后先自审，整个模块完成后执行行为/架构/协议三轴独立审查。
- 明确阻塞：OpenAI `temperature > 1` 到 Anthropic 的错误/丢弃/诊断策略，以及跨协议 `service_tier` 值映射，均没有获批准的产品语义，不得擅自修复。

## 2026-07-13 R3 Responses raw output item checkpoint

- RED：`TestR3_FileSearchCallOutput_SameProtocolRawRoundTrip` 初始只得到一个结构化 message output，`file_search_call` 消失。
- GREEN：新增 `ProviderExtensions.OpenAIResponses.Response.RawOutputItems`，从 upstream `output[]` 捕获无 canonical owner 的 item raw JSON + 原始 index；Responses inbound 仅在同协议 response emission 时按 index 合并。`file_search_call` fixture 通过。
- 边界：该 sidecar 不由 Chat/Anthropic adapter 消费；不实现跨协议 MCP/hosted bridge；未主张所有 output item/stream event 全覆盖。

## 2026-07-13 模块审查回退与修复 checkpoint

- 模块级独立审查结论为 **FAIL**，不得以此前 R1–R4/A1–A8 测试通过宣称模块完成。
- 已接受并拆为最小 TDD slices 的 finding：
  1. **Responses terminal/replay**：raw-only `output[]` 和 `queued`/`in_progress` 的空 `output:[]` 被发明为空 assistant message；stream `length`/`error`/`cancelled` finish reason 被无条件重放成 `response.completed`。
  2. **Chat cross-protocol isolation**：Anthropic same-protocol raw placeholder `anthropic_raw_block` 可能被 Chat adapter 序列化成非法 Chat content-part type。
  3. **P3 cleanup**：已迁移的 Anthropic raw-content/usage TransformerMetadata 常量残留在生产模型中。
- Responses RED→GREEN public-seam tests：
  - `TestR3_RawOnlyOutput_DoesNotInventEmptyMessage`
  - `TestR1_NonTerminalEmptyOutput_DoesNotInventMessage`
  - `TestR2_InboundStream_PreservesTerminalStatus`
- Responses implementation boundary：仅在 completed、存在 canonical choice 且没有 Responses raw output sidecar 的历史路径保留合成 empty message；其他空 output 保持 `[]`。stream terminal helper 将 `length/error/cancelled|canceled` 分别映射为 `response.incomplete/response.failed/response.cancelled` 及对应 response status；未更改顶层 SSE `error` 的既有实时 `error` 事件契约。
- Chat RED→GREEN public cross-protocol test：
  - `TestA1_UnknownRequestContentBlock_DoesNotLeakToOpenAIChat`
- Chat implementation boundary：只显式过滤已定义为 Anthropic provider-native placeholder 的 `anthropic_raw_block`；不建立 raw byte 跨协议桥，也未新增无批准的 diagnostic policy。
- P3 cleanup：删除 `TransformerMetadataKeyRawContentBlocks` 与 `TransformerMetadataKeyAnthropicUsageRaw` 生产常量；A3 test 以旧 wire-key 字面量断言 canonical metadata 不再持久存 usage raw。
- 已验证：
  - `cd llm && go test ./transformer/openai/responses -run '^(TestR3_RawOnlyOutput_DoesNotInventEmptyMessage|TestR1_NonTerminalEmptyOutput_DoesNotInventMessage|TestR2_InboundStream_PreservesTerminalStatus)$' -count=1 -v` PASS
  - `cd llm && go test ./transformer/openai/responses -count=1` PASS
  - `cd llm && go test ./transformer/anthropic -run '^(TestA1_UnknownRequestContentBlock_DoesNotLeakToOpenAIChat|TestA3_StopSequenceAndStopDetails_SameProtocolRoundTrip)$' -count=1 -v` PASS
  - `git diff --check` PASS
- Next route: complete slice self-review, run full target-package regression, then reopen module review with independent agents. No module PASS yet.

### Slice self-review — 2026-07-13 reopen fixes

| Slice | Declared public seam | Actual write set | Verification | Self-review result |
|---|---|---|---|---|
| Responses terminal/replay | Responses HTTP response → canonical → Responses response; canonical chunk stream → Responses SSE | `responses/inbound.go`, `responses/inbound_stream.go`, R1/R2/R3 focused tests | three new RED tests then PASS; full `./transformer/openai/responses`; final four target package regression | PASS — no provider-agnostic abstraction, no sidecar cross-read, terminal mapping is a small local helper; top-level SSE `error` intentionally unchanged because existing inbound stream error contract covers it |
| Chat raw-placeholder isolation | Anthropic HTTP request → canonical → OpenAI Chat HTTP request | `openai/outbound_convert.go`, Anthropic A1 cross-protocol fixture | RED then PASS; full `./transformer/openai` and `./transformer/anthropic` | PASS — one named source placeholder is filtered at the target adapter; raw bytes remain Anthropic-only and no fake bridge/diagnostic policy was invented |
| P3 metadata cleanup | Anthropic response conversion public seam | `anthropic/model.go`, A3 assertion | A1/A3 targeted pass; full Anthropic package | PASS — removed only unreferenced production constants; test preserves migration contract with a literal legacy key |

- Cross-slice check: four target packages (`responses`, `openai`, `anthropic`, `llm`) PASS and `git diff --check` PASS.
- Checkpoint route: slices are self-reviewed; reopen module review on the exact three accepted findings. Module still cannot be marked PASS until independent behavior and architecture re-review return.

### Test-only P2 coverage closure — 2026-07-13

- Review residuals were coverage-only, not additional production defects. They were closed without production edits:
  - `TestR2_InboundStream_PreservesTerminalStatus` now covers both `cancelled` and `canceled`, asserts exactly one intended terminal event, and rejects any simultaneous `response.completed`.
  - `TestR1_NonTerminalEmptyOutput_DoesNotInventMessage` now covers `queued`, `in_progress`, `incomplete`, `failed`, and `canceled` empty-output envelopes.
- Verification: `cd llm && go test ./transformer/openai/responses -run '^(TestR1_NonStreamStatus_QueuedAndInProgress_RoundTrip|TestR1_NonTerminalEmptyOutput_DoesNotInventMessage|TestR2_InboundStream_PreservesTerminalStatus)$' -count=1 -v` PASS.
- Self-review: PASS. The write set is test-only, asserts client-visible HTTP/SSE contracts, does not calculate expectations from the implementation, and avoids expanding the protocol policy surface.

### Module re-review — 2026-07-13

- Architecture re-review (`Aquinas`): PASS for all accepted findings. Confirmed Chat placeholder filtering, dead-constant removal plus legacy migration assertion, and no ProviderExtensions/TransformerMetadata owner regression.
- Behavior re-reviews (`Confucius`, `Bohr`): PASS. Confirmed raw-only/non-terminal empty output replay, finish-reason lifecycle mapping, A1 same-family preservation, C1/C2 boundary, and existing top-level SSE error contract. No P0/P1 remained.
- Residuals after test-only closure: no known P0/P1/P2 implementation or fixture gap in this module. Product policy exclusions remain unchanged: cross-protocol raw-block diagnostics, `temperature > 1` → Anthropic, and service-tier mapping.
- Next route: update durable protocol documentation to separate historical discovery rows from current closure evidence, then run parent-level architecture/contract review.

### Durable knowledge update — 2026-07-13

- `research/full-field-gap-inventory.md` now separates the historical discovery table from the current closure table and explicitly lists the remaining non-implementation dispositions.
- `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` §13.7 indexes C1/C2, R1–R4, A1–A8, D1, and review hardening without promoting the full matrix to `CONFIRMED`.
- `.trellis/spec/backend/protocol-transformer-guidelines.md` routes these closures to matrix §13.7 and repeats the no-global-completion claim.
- Verification after documentation update: all four target Go packages PASS and `git diff --check` PASS.
- Next route: parent-level independent review is in progress. No local commit until it passes.

### Parent review failure route — 2026-07-13

- Two independent parent reviews returned FAIL. Accepted findings:
  1. **P1 code**: Responses `queued` / `in_progress` had been transported through shared `Choice.FinishReason`, leaking invalid Chat `finish_reason` values cross-protocol.
  2. **P2 code hygiene**: two obsolete Anthropic non-stream TransformerMetadata constants remained after migration.
  3. **P1/P2 documentation**: inventory residual rows still described A4/R4 raw SSE replay as unimplemented; matrix §0 and a historical inline review did not point readers to the current closure state.
- P1 RED fixtures: same-family canonical response must have nil shared finish reason plus `OpenAIResponses.Response.Status`; Responses→Chat must not emit `"queued"` or `"in_progress"` as `finish_reason`.
- P1 implementation: added named `OpenAIResponsesResponseExtensions.Status`, deep clone support, and non-terminal ingress/egress handling. Terminal status remains derived from shared finish reasons only; native status wins on same-protocol response replay.
- P2 implementation: removed obsolete `TransformerMetadataKeyAnthropicStopDetails` and `TransformerMetadataKeyAnthropicResponseContent`; negative migration assertions use legacy key literals.
- P1/P2 documentation repair: residual rows now distinguish same-protocol A4/R4 replay from future family/cross-protocol work; matrix §0 indexes §13.7; historical inline review is marked superseded.
- Verification: focused P1 + clone tests PASS; four target packages PASS; `git diff --check` PASS.
- Next route: self-review this repair slice, then reopen parent architecture/spec review. No parent PASS or local commit yet.

### Parent P1 repair self-review — 2026-07-13

| Check | Evidence | Result |
|---|---|---|
| Public seam | Responses HTTP response → canonical → Responses HTTP preserves `queued`/`in_progress`; Responses HTTP response → canonical → Chat HTTP never serializes those values as `finish_reason` | PASS (`TestR1_NonStreamStatus_QueuedAndInProgress_RoundTrip`, `TestR1_NonTerminalResponsesStatus_DoesNotLeakToOpenAIChat`) |
| Owner | Only `OpenAIResponses.Response.Status` owns native nonterminal lifecycle; shared `Choice.FinishReason` remains terminal Chat-compatible vocabulary | PASS (outbound/inbound source audit) |
| Precedence | Native status wins over a mixed canonical shared terminal reason instead of silently becoming failed/completed | PASS (`TestR1_NativeNonTerminalStatusPrecedesSharedFinishReason`) |
| Clone | provider extension status pointer is deep-cloned | PASS (`TestCloneProviderExtensions_ResponsesStatusDeepCopy`) |
| Scope | Production edits limited to Responses adapter owner/routing and obsolete Anthropic metadata constants; no cross-protocol bridge or new global abstraction | PASS |
| Documentation | inventory residual SSE descriptions, matrix §0/§13.7, guidelines storage rule, and superseded inline review now agree with code | PASS |

- Four target packages and `git diff --check` passed after this repair.
- Next route: rerun independent module/parent review; no local commit until PASS.

### Parent re-review PASS — 2026-07-13

- Reused independent parent reviewers returned PASS after the P1/P2 repair.
  - Architecture/protocol reviewer: nonterminal Responses status now has a named native owner, cannot leak into Chat `finish_reason`, obsolete Anthropic metadata constants are removed, and no new owner/cross-protocol violation was found.
  - Goal/spec reviewer: A4/R4 residual documentation now agrees with same-protocol raw replay; matrix §0/§13.7 and superseded historical review form a consistent evidence chain without claiming the full 101/107 matrix.
- Final parent-review contract: goal coverage PASS for identified implementation gaps; non-goals respected; accepted findings closed; required allowed checks PASS; remaining non-goals/blocked policies remain explicitly stated.
- Final known disposition: no known P0/P1/P2 implementation or fixture gap in the scoped closure. `PARTIAL`/`UNCHECKED` FDR rows, hosted/MCP family expansion, cross-protocol raw diagnostic policy, `temperature > 1`→Anthropic, `service_tier`, and client-only controls remain deliberately outside this closure.
- Next route: final four-package regression, diff check, scoped local commit only; do not stage user-owned `.agent/`, `.agents/`, `.codex/`, Docker, or compose files.
