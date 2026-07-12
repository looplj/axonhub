# G8 Residual Five Boundary Fields Review

- **Date**: 2026-07-12
- **Branch**: `codex-transformer-field-fixes`
- **Scope**: G8 五边界项（非 101 行全量）
  1. Chat modalities
  2. Anthropic top-level cache_control
  3. Responses context_management
  4. Responses conversation
  5. Responses hosted tools inventory/coverage
- **Changed files reviewed** (working tree, uncommitted):
  - `llm/transformer/openai/inbound_test.go`
  - `llm/transformer/anthropic/cache_control_test.go`
  - `llm/transformer/openai/responses/g8_field_preservation_test.go` (untracked)
  - `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`
- **Review mode**: read-only; no production files modified by this review
- **Evidence sources**:
  - `git diff` / file contents
  - codebase-memory project `Users-asuan-AI-axonhub-llm`
  - production paths: Chat convert, Anthropic convert, Responses `request_extensions`, classification
  - targeted test run (passed)

## Verdict

# PASS

G8 五项整体通过。变更以**字段级同协议证据补齐 + 矩阵纠偏**为主，未改生产代码，且未把 raw 保真抬成跨协议语义等价，也未新建统一 hosted 抽象。

剩余问题为 **minor**，不构成整包 FAIL。

## Hard-check Summary

| Check | Result | Notes |
|---|---|---|
| 测试是否覆盖目标协议/字段 | PASS | 未用 Responses 测 Chat modalities；未用 content-block 替代 top-level cache_control；context/conversation/hosted 均走 Responses 路径 |
| 是否改生产代码；无改是否合理 | PASS | 仅测试 + 矩阵；对应生产路径本已存在 |
| 是否把 raw 保真写成跨协议语义等价 | PASS | 文案/测试注释明确 same-protocol raw only |
| 是否创建统一 hosted 跨协议抽象 | PASS | 无新增抽象；仍用 classification + RawTools |
| 矩阵状态/证据与代码一致 | PASS w/ minors | 主证据一致；hosted 的 Rsp→Anthropic 列偏乐观 |
| 架构/屎山/无用代码 | PASS | 新增测试聚焦；无生产旁路重构 |
| 虚假完成声明 | PASS | 五行均为 `PARTIAL`，未写成 `CONFIRMED`/全量闭环 |

## Per-item Findings

### 1) Chat modalities — PASS

**Matrix row**: `CHAT.TOP.modalities` → `PARTIAL` / `common_typed`

**Test evidence**:
- `TestInboundTransformer_TransformRequest_ModalitiesRoundTripChat`
- `TestInboundTransformer_TransformRequest_ModalitiesOmittedChat`
- 路径：Chat inbound `/v1/chat/completions` → `llm.Request.Modalities` → `RequestFromLLM` → Chat outbound wire JSON
- 覆盖 present `["text","audio"]` 与 omitted no-synth

**Production evidence (pre-existing)**:
- `llm.Request.Modalities`
- Chat `Request.Modalities`
- `ToLLMRequest()` / `RequestFromLLM()` 直接拷贝 typed 字段

**Assessment**:
- 正确纠正“仅 Responses/Gemini 测试不能证明 Chat same-protocol”
- 未宣称 Anthropic 有等价字段
- `Rsp↔Chat PARTIAL` 仍合理（G8 只补 Chat same-protocol，不假装六向完成）

**No production change**: 合理。typed 路径已在。

### 2) Anthropic top-level cache_control — PASS

**Matrix row**: `ANT.TOP.cache_control` → `PARTIAL` / `native+metadata`

**Test evidence**:
- `TestTopLevelCacheControlRoundTrip`
  - same-protocol type-only
  - TTL `5m` / `1h`
  - omitted no-synth
  - isolation: block-only 不发明 top-level；top-only 不发明 block；both 独立存活
  - 明确拒绝桥到 OpenAI `prompt_cache_*`

**Production evidence (pre-existing)**:
- inbound: `MessageRequest.CacheControl` → `TransformerMetadata[anthropic_cache_control]`
- outbound: metadata restore → `MessageRequest.CacheControl`
- 与 content-block/tool/system breakpoint 是不同层

**Assessment**:
- 真正测 top-level，不是 content-block 冒充
- 状态仍是 PARTIAL（值域/null 全量等未关闭），未虚假完成

**No production change**: 合理。

### 3) Responses context_management — PASS

**Matrix row**: `RSP.TOP.context_management` → `PARTIAL` / `raw_fallback_same_protocol`

**Test evidence**:
- `TestResponsesContextManagement_SameProtocolRawTopLevelFallback`
- 断言 `RawTopLevelFields["context_management"]` 与 outbound payload JSONEq
- 明确 not typed semantic support

**Production evidence (pre-existing)**:
- `buildRawUnknownTopLevelFields` 捕获非 `Request` 已知顶层字段
- `mergeRawUnknownTopLevelFields` 同协议回放
- 无 typed owner；Anthropic 另有 `context_management` typed/raw，但本行明确 no bridge

**Assessment**:
- 字段专测补齐了原先“仅 generic raw-top-level tests”
- 未把 raw 保真写成语义支持
- minor：测试断言 `ProviderExtensions.Diagnostics == nil` 对“无语义桥”帮助有限（该字段是另一类 diagnostics 容器），但不构成错误完成声明

**No production change**: 合理。

### 4) Responses conversation — PASS

**Matrix row**: `RSP.TOP.conversation` → `PARTIAL` / `raw_fallback_same_protocol`

**Test evidence**:
- `TestResponsesConversation_SameProtocolRawTopLevelFallback`
  - string id
  - object form（含 extra key 保真）

**Production evidence (pre-existing)**:
- `Request.Conversation` 仍 commented out → 不在 `knownRequestTopLevelFields`
- 因而走 generic `RawTopLevelFields` fallback
- 类型 `Conversation` 仍存在但未启用 request typed support

**Assessment**:
- 与代码状态一致：same-protocol raw only，typed 未启用
- 未写成 Chat/Anthropic messages state 等价

**No production change**: 合理。

### 5) Responses hosted tools inventory/coverage — PASS w/ minor

**Matrix row**: `RSP.TOOL.hosted` → `PARTIAL` / `native_raw_family_partial`

**Test evidence**:
- `TestResponsesHostedTools_SameProtocolRawPreserveAndChatLossy`
  - inventory 与 `llm.IsKnownOpenAIResponsesNativeToolType` 对齐：
    `function,image_generation,web_search,custom,namespace,tool_search,mcp,file_search,code_interpreter,computer_use_preview,local_shell,shell,apply_patch`
  - raw-only same-protocol RT：`file_search/code_interpreter/computer_use_preview/mcp/local_shell/shell/apply_patch`
  - 不折叠成 function tools
  - Chat outbound：lossy diagnostic + 不输出 `file_search` / 不 synth function

**Production evidence (pre-existing)**:
- classification list
- `buildRawOnlyToolFragments` + `isStructurallyRepresentedToolType`（function/image_generation/web_search/custom 结构性；其余 raw）
- Chat path `RecordResponsesLossyDowngradeDiagnostics`

**Assessment**:
- 未创建统一 hosted 跨协议抽象 — 正确
- raw fidelity ≠ semantic support — 文案正确
- 状态 PARTIAL — 正确

**Minor inconsistency**:
- 矩阵 `Rsp->Anthropic = LossyDowngrade`
- G8 新证据只明确 Chat lossy；Anthropic outbound 未见对 Responses `RawTools` 统一记录 `ResponsesLossyDowngradeDiagnostics` 的对应路径/测试
- 这里更准确应是“Chat explicit lossy verified；Anthropic drop/lossy diagnostic 仍未字段级证明”
- 这是矩阵方向列过宽，不是把 raw 吹成语义支持

## Production Code Delta

| Area | Production code changed? | Reasonable? |
|---|---|---|
| Chat modalities | No | typed 路径已在 |
| Anthropic top-level cache_control | No | metadata preserve 已在 |
| Responses context_management | No | generic raw top-level 已在 |
| Responses conversation | No | commented field + raw fallback 已在 |
| Hosted tools | No | classification + raw tools 已在 |

结论：本次是 **证据/矩阵收敛**，不是实现补洞；与 G8 边界项目标匹配。

## Matrix Consistency

| Row | Old Status | New Status | Evidence match | Overclaim? |
|---|---|---|---|---|
| CHAT.TOP.modalities | UNCHECKED | PARTIAL | Chat same-protocol tests 匹配 | No |
| ANT.TOP.cache_control | UNCHECKED | PARTIAL | top-level RT + isolation 匹配 | No |
| RSP.TOP.context_management | UNCHECKED | PARTIAL | field-specific raw RT 匹配 | No |
| RSP.TOP.conversation | UNCHECKED | PARTIAL | string/object raw RT 匹配 | No |
| RSP.TOOL.hosted | UNCHECKED | PARTIAL | inventory + raw-only + Chat lossy 匹配 | Minor: Rsp→Anthropic LossyDowngrade 证据不足 |

Preservation class 调整（`raw_fallback_same_protocol` / `common_typed` / `native+metadata`）与代码 owner 一致，且方向列多用 `no-synth` / `no-equivalent` / `LossyDowngrade`，没有写成跨协议语义等价。

## Architecture / Dead Code

- 新增 `g8_field_preservation_test.go`：聚焦 G8 字段，复用既有 `roundTripResponsesRawPayload` helper，无新抽象层。
- 无生产旁路、无重复实现、无“hosted 统一模型”屎山。
- Anthropic `cache_control_test.go` 增量较大，但分层清晰（top-level vs block isolation），可接受。

## Verification Executed

```text
cd llm
go test ./transformer/openai -run 'TestInboundTransformer_TransformRequest_Modalities' -count=1
go test ./transformer/anthropic -run 'TestTopLevelCacheControlRoundTrip' -count=1
go test ./transformer/openai/responses -run 'TestResponsesContextManagement_SameProtocolRawTopLevelFallback|TestResponsesConversation_SameProtocolRawTopLevelFallback|TestResponsesHostedTools_SameProtocolRawPreserveAndChatLossy' -count=1
```

全部 **ok**。

## Majors

**无 majors。**

## Minors

1. **RSP.TOOL.hosted 矩阵 `Rsp->Anthropic=LossyDowngrade` 证据不足**  
   G8 只证明 Chat outbound lossy/no-synth；Anthropic 方向缺少同等字段级诊断/丢弃证据。建议改为更窄表述，或补 Anthropic outbound 专测后再写 LossyDowngrade。

2. **context_management 测试对 `ProviderExtensions.Diagnostics == nil` 的断言偏弱/易误导**  
   与 raw top-level 保真主命题弱相关；不影响 PASS。

3. **CHAT.TOP.modalities 的 `Rsp↔Chat PARTIAL` 仍无 G8 新证据**  
   可接受（状态未抬到 CONFIRMED），但读者勿把 Chat same-protocol 专测当成六向完成。

4. **`g8_field_preservation_test.go` 目前 untracked**  
   矩阵已引用其测试名；合入前需确保文件随变更一起提交，否则证据断链。

## Residual Risks (out of G8 fail criteria)

- conversation typed support 仍关闭，仅 raw fallback
- context_management 无 typed 语义/stream 审计
- hosted tools 无跨协议语义映射，也无 Anthropic 等价 hosted 抽象
- modalities 值域组合（image-only 等）未扩测

这些是已知 PARTIAL 残差，不是本次虚假完成。

## Final Decision

**PASS**

G8 五边界项达到“同协议字段级证据 + 诚实 PARTIAL 矩阵”目标；无生产错误改动、无 raw=语义等价、无统一 hosted 抽象、无 majors。

