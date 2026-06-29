# AxonHub 协议转换修复跟踪表

> 每原子修复一项一行:TDD 红→绿后由同模型独立 agent 按 6 条标准验收,通过方归档。
> 流程:/ask-matt → /grill-with-docs → /to-prd → [/prototype] → /tdd → /diagnosing-bugs → /improve-codebase-architecture → 同模验收 → 本表归档 → 切片末 git commit。
> 6 标准:bug真没消除 / 无新错误 / 最小修复 / 无屎山 / 符合作者风格 / 守 AGENTS.md 与架构红线(canonical 不加 Namespace 顶层槽等)。

| 编号 | 切片 | 状态 | 改动文件(file:line) | 测试(red→green) | 验收结果 |
|---|---|---|---|---|---|
| D12(#1c·流式) | α | ✅ 已修·验收通过 | `llm/transformer/openai/responses/outbound_stream.go:272`(state init)、`:291`(首 delta emit)、`:375`(增量 delta emit);配套 `outbound_stream_test.go:+TestOutboundTransformer_TransformStream_CustomToolCallPreservesNamespace` | 先红(expected mcp__myserver got "")→补三处 Namespace 字段→绿;`go test ./transformer/openai/responses/...` 全 ok | 代理 Mendel(继承主线程模型)VERDICT APPROVED,六项均 PASS(2026-06-30)。补充观察:增量路径既存逻辑把 streamEvent.Namespace 写入 tc.Function.Namespace(:343),而本点读 tc.ResponseCustomToolCall.Namespace,custom_tool_call 场景后者为权威身份,链路自洽,非缺陷 |
| #3+#1e(parallel_tool_calls↔disable_parallel_tool_use) | β | ✅ 已修·验收通过 | `llm/transformer/anthropic/outbound_convert.go`(convertToAnthropicRequestWithConfig:合成/补 ToolChoice+注入 DisableParallelToolUse,极性反转、Tools 非空与 Type!=none 守卫);`inbound_convert.go`(convertToLLMRequest:读 ToolChoice.DisableParallelToolUse 反相写 ParallelToolCalls);配套 `outbound_test.go`(+三态极性+none 守卫)、`inbound_test.go`(+三态极性) | red→green;`go test ./transformer/anthropic/...` 全 ok(含 claudecode) | 代理 Banach 六项 APPROVED;后按建议去冗余内层 nil+补 none 守卫用例(测试锁定) |

## 待办切片
- β Anthropic 工具链:#1e dptu / #3 parallel_tool_calls / #1b builtin告警 / 非流侧#1c(D11)
- γ C区采样:top_k对称化 / rep_penalty·min_p·top_a保留 / logit_bias浮点容错
- δ 推理簇:#4 resp.reasoning.enabled / #5 thinking清空不全 / #6 output_config.format-task_budget / F19 prompt接线 / F21 ctx_mgmt
- ε 身份会话缓存:#13 user桥接 / #10 session_id body变体 / #11 chat·responses顶层cache_control
- ζ 流式杂项:F2 stream_options双向usage闭合 + convertStreamOptions早return守卫
- η P0压轴:D1/#1a namespace容器经 TransformerMetadata 映射往返
- 收官:cd llm 全量回归 + 总handoff + 终审commit
