# 总交接 — AxonHub 协议转换 23-bug 审计修复(全 7 切片结清)

> 交接时间:2026-06-30　分支:`codex-transformer-field-fixes`　仓库:`/Users/asuan/项目/AI/axonhub`
> 状态:**全部 23 项结清**(14 项真修复 + 4 项复核不修 + 5 项子项归并)

## 项目目标(一句话)
以 OpenRouter 官方 spec 为唯一规范,找出 AxonHub 三协议互转(chat_completions / anthropic_messages / openai_responses)中所有不符合规范的 bug,逐条 TDD 修复,每条同模独立 agent 按 7 标准验收。

## 最终结清总表

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
| δ | #4 | chat reasoning 对象丢 + responses reasoning.enabled 丢(🔴最高危) | ✅ 已修·已验收 | `6bf3b379` |
| δ | #5 | thinking utils.go:34 覆盖范围 | ⏭ 复核无 bug·不修 | — |
| ε | #13 | user 桥接(chat/responses ↔ anthropic metadata.user_id) | ✅ 已修·已验收 | `e12f1fc3` |
| ε | #10 | session_id body 变体 | ⏭ 设计性不修·文档化 | — |
| ε | #11 | chat/responses 顶层 cache_control | ✅ 已修·已验收 | `72d07e00` |
| ζ | F2 | stream_options 跨格式 + convertStreamOptions 早 return | ⏭ 复核无 bug·不修 | — |
| η | D1/#1a | namespace 工具组经 TransformerMetadata 映射往返(P0 压轴) | ✅ 已修·已验收 | `397c7bce` |

**统计:14 项真修复 + 4 项复核不修 = 18 原子全结清。** C7/C8/C9 三 bug 归并为 1 原子,故 bug 项 > 原子数。

## 红线(全程守住)
- canonical `llm.Request` / `llm.Function` 不加协议独有顶层槽——所有协议独有字段(Namespace/TopK/采样旋钮/Prompt/ContextManagement/OutputConfig/Reasoning 对象/ReasoningEnabled/CacheControl)均只走 `TransformerMetadata` 透传往返。
- 不重构作者架构;跨包结构体透传统一用 `json.RawMessage`(F21/#11 范式)。
- 每原子 TDD 红→绿→同模验收 7 标准→commit;提交卫生:只暂存本原子文件。

## 关键修复范式(供后续维护参考)
1. **TransformerMetadata 往返**:协议独有字段入站 stash → metadata key → 出站 restore。key 命名仿 `openai_responses_*` / `anthropic_*` 范式。
2. **跨包类型障碍**:结构体跨包透传不能存指针,用 `json.RawMessage`(cache_control/context_management 范式)。
3. **查表还原禁字符串切分**:namespace 组名含 `__` 不可切分,必须建映射表查表还原(#1a 范式)。
4. **流式 metadata 传播**:outbound stream 从请求 metadata 取值存 state,发到首个 chunk;inbound `mergeTransformerMetadata` 透传;装配点查表(#1a 流式范式)。

## 已知限制(非本次引入,记录备查)
- **跨协议 metadata 传播缺口**:responses outbound 在 `TransformResponse` 中 `maps.Clone(httpResp.Request.TransformerMetadata)` 闭环完整;但 chat/anthropic outbound **不克隆**请求 TransformerMetadata 到响应。故 TransformerMetadata 类字段(namespace map/cache_control/reasoning 等)在 responses 客户端 → chat/anthropic 上游 的跨协议往返中会丢失。这是既有传播基础设施局限,影响所有 TransformerMetadata 字段,非任一单切片引入。若实际部署含跨协议场景,需另起切片让各 outbound 通用克隆 request TransformerMetadata。
- **compact 路径**:namespace 工具声明只在标准 inbound,compact 无 tools 声明;`convertInputFromMessages` 传 nil 即可(扁平工具保持原名)。

## 权威文档
- 字段命运权威表:`docs/specs/master-conversion-table.md`
- 规范基准:`docs/specs/openrouter-chat-messages-responses.min.yaml` + `openrouter-openapi.yaml`
- 修复跟踪:`docs/fix-tracker.md`　总报告:`docs/audit-bug-report.md`
- 切片 PRD:`docs/specs/slices/{alpha,beta,gamma,delta,epsilon,zeta,eta}-prd.md`
- 审计方法纪律:`.agent/rules/spec-audit-method.md`

## 环境/MCP 状态
- MCP codebase-memory `Users-asuan-AI-axonhub-llm` 项目可用(search_graph/trace_path/get_code_snippet),图谱相对工作树有滞后(作者 commit `812c9077` 后未重建),实时源码为准。
- gofmt -w 对齐结构体可接受。AGENTS.md 规定不主动跑 lint/build,go test 验证允许。
