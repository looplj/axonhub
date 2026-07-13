# 全量字段缺口清单（第一轮源码取证后）

- 日期：2026-07-13
- 依据：严格矩阵、三个官方协议基准文档、五份分域只读盘点报告、`Users-asuan-AI-axonhub-llm` CLI 图谱。
- 边界：`UNCHECKED`/`PARTIAL` 仅表示 FDR、测试或方向证据不完整；**不自动等于生产 bug**。

## 分类结果

| 分类 | 含义 | 本轮处置 |
|---|---|---|
| `IMPLEMENTATION` | 当前源码已证明同协议字段/结构被丢失或错误重放 | 进入本轮 TDD 实现切片 |
| `DIAGNOSTIC` | 跨协议没有等价字段，当前会静默丢失 | 仅对已有无歧义策略的字段补 `LossyDowngrade` |
| `TEST_ONLY` | 路径存在，缺 public-seam fixture | 不伪装成代码缺陷；随相邻切片补或留待后续 fixture 批次 |
| `DOC_ONLY` | owner、FDR、默认/null/值域或粗粒度矩阵证据不完整 | 本轮实施后反填；无代码改动 |
| `EXPLICIT_NO_IMPLEMENT` | client-only、无协议等价物或已经正确 same-protocol raw 保真 | 写明原因，不建立伪桥接 |
| `BLOCKED_PRODUCT_DECISION` | 实际不兼容，但错误/丢弃/降级的产品策略未决定 | 不擅自改变上游请求行为 |

## 初始已确认、可直接验证的实现缺口（历史发现记录）

下表记录的是 2026-07-13 首轮源码盘点时的发现，**不是当前开放项清单**。当前完成状态以紧随其后的闭环表和 `protocol-compliance-loop-ledger.md` 为准；不得从下表的历史 `IMPLEMENTATION` 标签推导“代码仍缺失”。

| 模块 | Field ID / 派生字段 | 证据摘要 | 切片 | 允许的最小修复 |
|---|---|---|---|---|
| Chat | `CHAT.TOP.tools` / `CHAT.TOP.tool_choice` / `CHAT.TOOL.tool_calls` | Chat `custom` 工具、named custom choice、custom call 在当前 function-only model 中无 carrier | C1 | Chat-native typed/raw sidecar；同协议回放；不把 Chat custom schema 当作 Responses schema |
| Chat | `CHAT.MSG.content_parts` | `file` 与 content-array `refusal` 没有 payload carrier/raw sidecar | C2 | Chat-native part raw/typed preservation；保持顺序和 omission |
| Responses | `RSP.RESP.status` / `RSP.RESP.incomplete_details` / output text refusal | queued/in_progress 被重建为 completed；non-stream incomplete details、refusal 丢失 | R1 | canonical response 的最小 presence carrier 或 Responses-native sidecar；非流式 round-trip fixture |
| Responses | `RSP.STREAM.completed` / top-level error | completed 依赖 usage；standalone error 被聚合器吞掉 | R2 | 修复终态状态机和显式 error 契约；不得伪造未知事件转换 |
| Anthropic | `ANT.MSG.content_blocks` / tool_result nested blocks | unknown request block 与 tool-result 未知 child 在 closed switch 中丢失 | A1 | Anthropic-native ordered raw fragment sidecar；同协议回放 |
| Anthropic | `ANT.TOP.tools` | 普通 function tool 的 Anthropic children 与非 web-search/mcp server tools 无 carrier | A2 | Anthropic-native raw declaration preservation；不扩宽公共 `llm.Tool` |
| Anthropic | `ANT.RESP.message` stop sequence/details + future usage | matched stop sequence、stop details、未建模 usage detail 丢失 | A3 | Anthropic response native/raw sidecar；先覆盖已有官方形状 |
| Common diagnostics | penalties / seed / selected no-equivalent fields | 存在明确 no-equivalent 且当前静默丢弃；已有 `LossyDowngrade` infra | D1 | 逐字段、显式 allowlist 的 diagnostic；不做反射式全字段猜测 |

## 当前实施闭环（2026-07-13）

下列是上述历史发现及随后独立审查新增的可实现问题的**当前状态**。`PASS` 只表示列出的 public seam 已验证；不把严格矩阵的其他 `PARTIAL`/`UNCHECKED` 行提升为 `CONFIRMED`。

| Slice | 已关闭的字段/行为 | 当前 owner / 处理方式 | Public-seam 证据 | 审查状态 |
|---|---|---|---|---|
| C1/C2 | Chat custom tool/choice/call、`allowed_tools`、file/refusal part | Chat adapter-local typed carrier；不借用 Responses schema | `chat_custom_fidelity_test.go` | 模块行为复审 PASS |
| R1 | Responses queued/in_progress、`incomplete_details`（含 explicit null）、refusal、`completed_at`、`output_text`、非终态 empty output | canonical 已建模字段 + `OpenAIResponses.Response.Status` / `RawTopLevelFields`；Responses-native status 不借用 Chat finish_reason | `r1_envelope_refusal_test.go` | 模块行为/架构复审 PASS（父级复审待更新） |
| R2 | 无 usage 完成、`length/error/cancelled/canceled` 终态、aggregated top-level error | Responses stream aggregator + inbound terminal helper；实时 SSE `error` 契约保持独立 | `r2_stream_terminal_error_test.go`、既有 inbound stream error fixture | 模块行为/架构复审 PASS |
| R3/R4 | raw-only output item、unknown/raw stream event | `OpenAIResponses.Response.RawOutputItems` / `RawStreamEvents`，仅 Responses 同协议 replay | `r3_raw_output_items_test.go`、`r4_raw_stream_event_test.go` | 模块行为/架构复审 PASS |
| A1 | unknown request block、tool_result unknown child、Anthropic raw placeholder 不泄漏到 Chat | `Anthropic.Request.RawContentFragments` + Anthropic-only temporary hydrate；Chat target 显式 drop placeholder | `a1_unknown_content_blocks_test.go` | 模块行为/架构复审 PASS |
| A2 | Anthropic function/native tool declaration children | Anthropic-native declaration preservation，不扩宽 common `llm.Tool` | `a2_tool_declaration_preservation_test.go` | 模块行为复审 PASS |
| A3–A8 | response stop/usage/content sidecars、raw stream lifecycle index、stop reasons、citation detail | `Anthropic.Response` named sidecars；stream fidelity code 保留短生命周期 metadata | `a3_response_stop_usage_test.go` 至 `a8_citation_native_details_test.go` | 模块行为/架构复审 PASS |
| D1 | 明确无等价的 sampling/seed loss | `ProviderExtensions.Diagnostics.LossyDowngrades`；不进入 payload | existing D1 diagnostics tests | 模块行为复审 PASS |

### 当前开放实现项

- **父级审查 P1（2026-07-13，已闭合）**：Responses non-terminal `status` 已迁移到 `ProviderExtensions.OpenAIResponses.Response.Status`，不再借用公共 `Choice.FinishReason`；同协议 replay、native-over-shared precedence、deep clone 与 Responses→Chat negative fixture均已通过，并已获父级复审 PASS。
- 仍未实现的项必须继续保持原分类：无精确语义的 hosted/MCP future family 是 `DOC_ONLY`/后续独立 fixture slice；`temperature > 1`→Anthropic 与跨协议 `service_tier` 是 `BLOCKED_PRODUCT_DECISION`；client-only control plane 是 `EXPLICIT_NO_IMPLEMENT`。

## 有证据但本轮不擅自实现的项

| Field / 类别 | 分类 | 原因与下一步 |
|---|---|---|
| Responses hosted/MCP output item families、future Responses SSE | `DOC_ONLY` / `TEST_ONLY` | R3/R4 已将没有 canonical owner 的 output item 与 unknown/future SSE 以 raw sidecar 做**同协议** replay；未闭环的是每个 hosted/MCP family 的精确官方 union、terminal aggregation 与跨协议语义，必须按 family 单独建 fixture，不能做“万能 raw”桥。 |
| Anthropic future SSE event | `DOC_ONLY` / `TEST_ONLY` | A4 已将 unknown event 和 unknown content-block lifecycle 以 `Anthropic.Response.RawStreamEvents` 做**同协议** replay，并覆盖后续 `contentIndex`；未冻结的是跨协议 diagnostic/unsupported 外部契约，不能把 raw 字节桥到 Chat/Responses。 |
| OpenAI `temperature > 1` → Anthropic | `BLOCKED_PRODUCT_DECISION` | 已知范围不兼容，但应 error、drop 还是 diagnostic 未有已批准策略；禁止 clamp。 |
| 跨协议 `service_tier` | `BLOCKED_PRODUCT_DECISION` | 枚举不等价；禁止按字符串猜映射。 |
| official null/default、Anthropic top_p 范围、模型省略语义 | `DOC_ONLY` | 需补官方源/FDR，不能以 Go 指针行为臆断 wire null。 |
| Responses context_management/conversation、Codex approval/OAuth/sandbox/multi-agent/telemetry | `EXPLICIT_NO_IMPLEMENT` | 前者已有 same-protocol raw replay且无跨协议语义；后者是 client control plane，不属于 Hub 协议字段。 |

## 仅测试 / FDR backlog（不阻塞上述实施）

- Chat：roles、response_format、supported chunk/choices、function legacy forms；
- Responses request：background false、store、truncation、prompt cache retention、parallel tool calls、text nested fields、input/output 子类型矩阵；
- Responses response：supported item mixed order、usage precedence、annotation 子型；
- Anthropic：known content-block order、system string/array、known stream lifecycle、modeled usage；
- Common：模型/stream/top_p/token-limit precedence/metadata identity/stop structural presence。

这些项目必须保留为 `TEST_ONLY` 或 `DOC_ONLY`，不得因本轮实现通过而把总览 Field ID 提升为 `CONFIRMED`。

## 本轮并行切片与写入边界

| Slice | 负责人 | 生产写入范围 | 不得触碰 |
|---|---|---|---|
| C1 | Chat custom tools | `llm/transformer/openai/{model,inbound_convert,outbound_convert}.go` 与专用 tests | C2 part payload、Responses/Anthropic |
| C2 | Chat content parts | Chat part model/converters 与专用 tests | C1 tools/choices/calls、其他协议 |
| R1/R2 | Responses envelope/terminal stream | `llm/transformer/openai/responses/{inbound,outbound,aggregator,model}.go` 与 tests | 未具 fixture shape 的 hosted/MCP broad raw architecture |
| A1/A3 | Anthropic content/response | Anthropic content/response model+converters 与 tests | A2 tools declarations |
| A2/D1 | Anthropic tools + safe diagnostics | Anthropic tool adapters、明确 loss recorder 与 tests | temperature/service-tier policy、反射式诊断 |

每个切片均须：先 RED fixture → 最小实现 → 定向 `go test`（在 `llm/`）→ 自审。完成后主会话进行冲突审查与模块级多轴审查。

## 证据来源

- `inventory-chat.md`
- `inventory-responses-request.md`
- `inventory-responses-output-stream.md`
- `inventory-anthropic.md`
- `inventory-common.md`
- `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md`
- `.trellis/spec/backend/protocol-transformer-guidelines.md`
