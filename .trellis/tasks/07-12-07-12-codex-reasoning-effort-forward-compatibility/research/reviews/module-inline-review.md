# 模块级内联审查（独立代理不可用时的替代证据）

> **已退役 / superseded（2026-07-13）**：这是 A4/R4 之前的历史内联审查记录，不能作为当前模块状态。后续 A4/R4 及 reopen 修复已加入 Anthropic/Responses same-protocol raw SSE replay；独立复审结果与当前状态以 `protocol-compliance-loop-ledger.md` 的“Module re-review”和父级审查记录为准。保留本文仅用于追溯当时的 401 认证阻塞和初始 finding。

- 日期：2026-07-13
- 原因：行为/架构/规范独立 `trellis-check` 代理均无法完成启动，实际错误为 `http://127.0.0.1:8090/v1/responses` 的 `401 Unauthorized`；未将失败伪称为独立审查通过。
- 方法：主会话以 CLI 图谱、精确 diff、官方本地协议基准、严格矩阵、每个 slice 的 public-seam 定向测试，按 review-router 的三个轴逐项审查。

## 已接受并修复的 finding

| Finding | Axis | 证据 | Owner slice | Route | 复验 |
|---|---|---|---|---|---|
| Chat 初版遗漏官方 `tool_choice.allowed_tools` union | Spec / Behaviour | Chat 官方 snapshot `2650–2725`；`inventory-chat.md` 已标注；初版 wire model 无 carrier | C1 | TDD | 新增 RED 后失败 `invalid tool choice type`；补 Chat-only carrier 后 `TestOpenAIChatRequest_AllowedToolsChoiceRoundTrip` 通过 |
| R1 将 Responses `incomplete_details` 放在 `TransformerMetadata` | Architecture / Spec | guideline “Responses Body Field Storage” §规则 71–77 | R1 | architecture repair | 迁移至 `ProviderExtensions.OpenAIResponses.Response.RawTopLevelFields`；R1 完整通过 |
| 并行写同一 `responses/outbound.go` 导致 R1 queued/in_progress branch 被覆盖 | Behaviour / Integration | R1 定向 test 失败，finish reason nil | R1/R2+D1 integration | TDD | 恢复分支后 R1/R2/D1 定向 test 及 Responses package 通过 |
| Anthropic A1 残留无效 `rawContentFrags` / `_ =` | Architecture | diff inspection | A1 | implementation cleanup | 删除后 A1 test 通过 |
| Chat ToolChoice union reuse 未清空其他成员 | Behaviour / Maintainability | `UnmarshalJSON` branch state inspection | C1 | implementation cleanup | 每个 union branch 清空其他 carrier；Chat package 通过 |

## 行为 / bug 轴

- Chat：custom declaration/custom choice/custom calls/file/refusal/allowed_tools 均从 public inbound→canonical→outbound seam 覆盖；Responses custom tool 仍被 Chat outbound filter 排除。
- Responses：queued/in_progress、incomplete_details、refusal、无 usage completed、top-level error 的 R1/R2 tests 通过。`error.param`、hosted/MCP raw output、`response.queued` SSE re-emit 明确保留为未实现，不被误标为完成。
- Anthropic：unknown request/tool-result blocks、Anthropic-only tool declaration children、native tool order、stop sequence/details、usage raw 细节均有 public same-family fixture。**本文历史时点**的 unknown SSE 尚未实现；当前状态已由 A4 raw same-protocol replay 取代，见文件顶部 superseded 说明。
- Diagnostics：typed penalties 和 Chat seed 仅记录 `LossyDowngrade`，目标 body 仍省略，未改变 payload。

## 架构 / 可维护性轴

- Responses 原生 response field 已从 metadata 迁移到有命名 owner 的 response sidecar；clone 支持深拷贝该 sidecar。
- Anthropic native data 使用 adapter-local raw fragments / response native helper，未扩宽 `llm.Request` / `llm.Tool`。
- Chat response/stream 必须经过 canonical model；采用明确命名的 `OpenAIChat*` carriers，且 converter 仅由 OpenAI Chat adapter 消费。未将其用于跨协议映射。此为现有 `llm.Tool`/`MessageContentPart` canonical transport 的最小局部扩展，不是新 AST。
- D1 使用显式 allowlist；无反射扫描、无诊断与 native sidecar 混用。

## 规范轴

- 未处理 `temperature > 1` → Anthropic 或 `service_tier` 值映射；二者仍是 `BLOCKED_PRODUCT_DECISION`。
- 未将 client-only Codex approval/OAuth/sandbox/multi-agent/telemetry 或无等价字段桥接。
- 总览字段状态仍是 `PARTIAL`，本批不能宣称 101/107 矩阵全完成。
- 残余 TEST_ONLY/DOC_ONLY/EXPLICIT_NO_IMPLEMENT 已留在 `full-field-gap-inventory.md`。

## 结论

本文历史内联三轴审查在当时通过；独立多代理审查当时仍为**待补**，原因是 8090 认证错误。当前独立复审状态以文件顶部 superseded 说明和任务 ledger 为准。
