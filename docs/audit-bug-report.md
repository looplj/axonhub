# AxonHub 协议转换 Bug 审计与修复总报告

> 唯一规范基准:`docs/specs/openrouter-chat-messages-responses.min.yaml`(OpenRouter 官方 chat/messages/responses 三合一 OpenAPI spec)
> 权威字段表:`docs/specs/master-conversion-table.md`　修复跟踪:`docs/fix-tracker.md`
> 分支:`codex-transformer-field-fixes`　llm 为独立 Go 子模块(`cd llm && ...`)
> 更新:2026-06-30

## 核心架构(作者设计,本次修复不偏离)
三协议(chat_completions / anthropic_messages / openai_responses)经 **canonical 中间层 `llm.Request`** 互转。协议独有字段不污染 canonical 顶层,只走 `TransformerMetadata` 透传往返。canonical 顶层仅保留三协议**最大公约数**槽(ReasoningEffort/Budget/Summary、采样族、Store/PromptCacheKey/SafetyIdentifier/User/ServiceTier 等)。

## 修复总览

| 切片 | 原子 | bug 一句话 | 状态 | commit |
|---|---|---|---|---|
| α | D12 | namespace split 反向还原断裂 | ✅ 已修·已验收 | (α) |
| β | #1e | dptu 透传 | ✅ 已修·已验收 | (β) |
| β | #3 | parallel_tool_calls 往返 | ✅ 已修·已验收 | (β) |
| β | #1c | 非流侧 session_id(D11) | ✅ 已修·已验收 | (β) |
| β | #1b | builtin 工具 | ⏭ 按作者设计·不修 | — |
| γ | C13 | logit_bias 浮点解码中断 | ✅ 已修·已验收 | `3a57d0ac` |
| γ | C3 | top_k 三方不对称 | ✅ 已修·已验收 | `9b0a2688` |
| γ | C7/C8/C9 | rep_penalty/min_p/top_a 丢 | ✅ 已修·已验收 | `7aeadd9f` |
| δ | F19 | responses prompt 存储模板引用丢 | ✅ 已修·已验收 | `b1698131` |
| δ | F21 | anthropic context_management 丢 | ✅ 已修·已验收 | `d9fe8bed` |
| δ | #6 | anthropic output_config format/task_budget 丢 | ✅ 已修·已验收 | `0e8f0120` |
| δ | #4 | chat reasoning 对象丢 + responses reasoning.enabled 丢(🔴) | ✅ 已修·已验收 | `6bf3b379` |
| δ | #5 | thinking utils.go:34 覆盖范围 | ⏭ 复核无 bug·不修 | — |
| ε | #13 | user 桥接(chat/responses user ↔ anthropic metadata.user_id) | ✅ 已修·已验收 | `e12f1fc3` |
| ε | #10 | session_id body 变体 | ⏭ 设计性不修·文档化 | — |
| ε | #11 | chat/responses 顶层 cache_control | ✅ 已修·已验收 | `72d07e00` |
| ζ | F2 | stream_options 跨格式 + convertStreamOptions 早 return | ⏭ 复核无 bug·不修 | — |
| η | D1/#1a | namespace 容器经 TransformerMetadata 往返(P0 最高危结构性缺口) | ✅ 已修·已验收 | (η) |
| 补遗 | 跨协议 | chat/anthropic/gemini 出口不传播请求 TransformerMetadata(非流式+流式) | ✅ 已修·已验收 | `a1b10836`+`9e3905b2` |

## 红线(全程守住)
- canonical `llm.Request` / `llm.Function` 不加协议独有顶层槽(Namespace/TopK/采样旋钮/Prompt/ContextManagement/OutputConfig/Reasoning 对象/ReasoningEnabled 均只走 TransformerMetadata)。
- 不碰 AGENTS.md/docker/yaml/CONTEXT.md 等非原子文件(留本地)。
- 每原子同模独立验收 7 标准,绿项才归档 commit。

## 验收纪律
7 标准:① bug 消除 ② 无回归(go test/build) ③ 最小修复 ④ 无屎山(improve-codebase-architecture 实扫) ⑤ 符合作者风格 ⑥ 守 AGENTS 与 canonical 红线 ⑦ 范围完整性(rg 全仓无残留,#1c 教训)。
