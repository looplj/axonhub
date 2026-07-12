# G13–G15 父级审查

| 项目 | 已核实事实 |
|---|---|
| 审查日期 | 2026-07-13 |
| 范围 | Codex `1f0566d..9e552e9d1` 触及的 AxonHub G13–G15 改动、测试、fixture、协议矩阵与 Trellis 规范 |
| 审查方式 | 主会话 CLI 源码/调用链核对 + 独立 `trellis-check` 审查 |
| 实现提交 | `5c63811d fix(llm): preserve Codex responses request fidelity` |
| 结论 | **PASS**：G13–G15 的目标范围无 P0/P1/P2 未关闭问题；不宣称三协议 101/107 Field ID 全部完成。 |

## 审查对象

- G13：Responses request `reasoning`、`include` 与未知同协议 `reasoning.effort` 字符串保真；禁止 Hub 注入 Codex 客户端默认。
- G14：`reasoning.summary`、`stream_options.reasoning_summary_delivery` 与未知嵌套 stream options 同协议保真；禁止复制 Codex 模型 catalog gate。
- G15：Responses request `input[]` 的 message、function/custom tool、tool output、reasoning item identity/presence 保真；禁止 Codex 前缀强制、ID 合成和错误 fallback。

## 架构核对

| 规则 | 结果 | 证据 |
|---|---|---|
| 不引入通用协议 AST | PASS | G14 使用 `ProviderExtensions.OpenAIResponses.Request.RawStreamOptions`；G15 使用与现有 `ToolCall.ResponseItemID` 对称的最小 identity carrier。 |
| 协议原生字段不塞进 metadata 垃圾桶 | PASS（本范围） | `stream_options` raw 字段进入专属 sidecar；G15 identity 只由 Responses inbound/outbound 消费。 |
| 同协议优先，跨协议不伪造 | PASS | G13/G14 无 model catalog 复制；G15 不合成 item ID，裸 `ReasoningContent` 不生成 Responses reasoning item。 |
| typed 与 raw 合并 | PASS | G14 raw nested stream options 可与 typed 字段共存，raw shared key 优先，且不污染 unknown-top-level diagnostics。 |

## 定向验证

在 `llm/` 独立 Go module 中执行并通过：

```bash
go test ./transformer/openai/responses/ \
  -run 'TestG13a_|TestG14a_|TestG14b_|TestG15a_|TestG15b_|TestG15c_|TestResponsesUnknownReasoningEffortSameProtocolRoundTrip|TestConvertReasoning_UnknownEffortPreserved|TestFunctionCallItem_IDAndStatusRoundTrip|TestConvertStreamOptions|TestCrossProtocol_ChatOutboundEmitsLossyDowngrade' \
  -count=1

go test ./transformer/openai/ -run 'TestInboundTransformer_UnknownReasoningEffort' -count=1
go test . -run 'TestCloneProviderExtensions_RawStreamOptionsDeepCopy' -count=1
git diff --check
```

结果均为 PASS。未运行全量 lint/build，符合仓库约束。

## 独立审查结论

独立 `trellis-check` 审查覆盖当前 G13–G15 diff，结论 PASS：

- G13：supplied `reasoning` / `include` 保真，省略时不注入；
- G14：summary、raw/typed stream options、clone 和诊断边界正确；
- G15：message/tool/reasoning identity 三态、summary-only、pure standalone、reasoning→tool 与 no-cross-invent 均通过 public seam；
- 未发现 P0/P1/P2；无关 Docker、`.agent/`、`.agents/`、`.codex/` 未进入 `5c63811d`。

## 已知边界（非阻塞）

1. G13–G15 是 OpenAI Responses request 同协议 public seam 修复，不证明三协议全方向语义等价。
2. Responses response `output[]` reasoning item identity、完整 stream event 家族和其他 Field ID 仍按严格矩阵单独审计。
3. 本审查不把 Codex approval、sandbox、OAuth、multi-agent、telemetry 等客户端控制面转为 Hub wire 字段。
