# Handoff — δ 切片(推理簇)收尾

> 交接时间:2026-06-30　分支:`codex-transformer-field-fixes`　llm 为独立 Go 子模块(`cd llm && ...`)

## δ 切片范围与结论
推理控制字段命运的五原子,全部结清:

| 原子 | bug | 结论 | commit | 验收代理 |
|---|---|---|---|---|
| δ-1 F19 | responses `prompt` 存储模板引用静默丢 | ✅ 已修 | `b1698131` | Anscombe |
| δ-2 F21 | anthropic `context_management` 上下文压缩策略丢 | ✅ 已修 | `d9fe8bed` | Confucius |
| δ-3 #6 | anthropic `output_config` format/task_budget 子项丢 | ✅ 已修 | `0e8f0120` | Lovelace |
| δ-4 #4 | chat `reasoning` 对象整体丢 + responses `reasoning.enabled` 丢(🔴最高危) | ✅ 已修 | `6bf3b379` | Kant |
| δ-5 #5 | thinking `utils.go:34` 覆盖范围复核 | ⏭ 复核无 bug·不修(类 #1b) | `f1bff385` | — |

## #4 修复要点(本切片最后一项,🔴最高危)
- **根因(grill 已坐实)**:
  - chat `openai/model.go` Request 仅有平铺 `ReasoningEffort`/`ReasoningSummary`,**无 `reasoning` 对象字段**→客户端发 OpenRouter 规范对象 `{effort,summary}`(yaml:4884)时 lenient Unmarshal 整对象丢弃。
  - responses `responses/model.go` Reasoning 结构体仅 Effort/GenerateSummary/Summary/MaxTokens,**无 enabled**→`reasoning.enabled`(yaml:12579 ReasoningConfig.enabled nullable bool)入站丢弃。
- **修复(守红线:不加 canonical 新顶层槽,继续借现有 ReasoningEffort/Budget/Summary 三槽 + TransformerMetadata)**:
  - chat:Request 加 `Reasoning *ChatReasoningConfig{Effort,Summary}`;inbound 对象 effort/summary 覆盖平铺 shorthand(对象优先);outbound 不改(继续发平铺,对象→平铺归一化经 canonical 三槽存活,语义无损)。
  - responses:Reasoning 加 `Enabled *bool`;inbound stash 进 `TransformerMetadata[responsesReasoningEnabledTransformerMetadataKey]`;outbound `convertReasoning` 用 `xmap.GetBoolPtr` 还原,并纳入 `hasReasoningFields` 早返回守卫(enabled 单独存在也往返)。
- **测试**:7 新例(chat 对象捕获/对象优先/fallback/往返 + responses enabled 捕获/往返/缺省守卫),4 包全绿。
- **验收**:代理 Kant 7 标准 APPROVED;canonical 红线未破;全仓范围完整性扫描无残留。
- **范围决策(不偷选,留作可选 follow-up)**:chat outbound 仍发平铺 `reasoning_effort`/`reasoning_summary`(保 OpenAI 兼容),未发 `reasoning` 对象。若上游(严格 OpenRouter-spec 方)要求对象往返,需另议 outbound wire 决策——两条路径后果不同,未在本原子内擅改。

## 已修复总进度(11 原子已结清)
- α:D12(namespace split 安全化)
- β:#1e dptu / #3 parallel_tool_calls / #1c(D11) / #1b builtin(⏭按作者设计不修)
- γ:C13 logit_bias 浮点容错 / C3 top_k 三方对称化 / C7·C8·C9 rep_penalty·min_p·top_a 保留
- δ:F19 / F21 / #6 / #4 / #5(⏭复核不修)

## 剩余工作
- **ε 身份会话缓存**(3 原子):
  - #13 user 桥接(chat/responses `user` ↔ anthropic `metadata.user_id` 往返)
  - #10 session_id body 变体(responses body 内 session_id 透传)
  - #11 chat·responses 顶层 cache_control
- **ζ 流式杂项**(1 原子):F2 stream_options 双向 usage 闭合 + convertStreamOptions 早 return 守卫
- **η P0 压轴**(2 原子):D1/#1a namespace 容器经 TransformerMetadata 映射往返(最高危结构性缺口)
- **收官**:`cd llm && go test ./...` 全量回归 + 总 handoff + 终审 commit

## 流程纪律(每原子严格执行,勿跳)
1. `/grill-with-docs` → MCP/rg 探根因(必须对照 master-conversion-table.md,别搜窄——#1c 教训)
2. `docs/specs/slices/<切片>-prd.md` 写 PRD
3. `/tdd` 红→绿
4. 同模验收代理 7 标准(bug消除/无回归/最小修复/无屎山[improve-codebase-architecture 实扫]/符合作者风格/守AGENTS与canonical红线/范围完整性[rg 全仓无残留])
5. 绿项跳 /diagnosing-bugs 但显式写理由
6. `docs/fix-tracker.md` 归档 + commit(只暂存本原子文件;AGENTS.md/CONTEXT.md/docker/yaml/untracked 留本地;specs md 需 git add)
7. 切片末 handoff

## 环境提示
- MCP codebase-memory 传输此前断开(`Transport closed`),已全程用 rg/sed 回退(正常)。若恢复可用 search_graph/get_code_snippet 加速。
- commit 消息禁用内嵌双引号(破坏 shell `-m "..."` 引号),用单引号或去引号。
