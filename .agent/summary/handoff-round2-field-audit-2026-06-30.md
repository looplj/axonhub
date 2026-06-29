# Handoff · 第二轮字段命运审计(fork-agent 五路复核)

> 日期:2026-06-30  
> 执行模型:`cf/@cf/zai-org/glm-5.2`,fork_context 继承主 thread 方法论上下文  
> 规则基准:`.agent/rules/spec-audit-method.md`(本 thread 当场立,三层可信度+四判据)  
> 唯一规范源:`docs/specs/openrouter-chat-messages-responses.min.yaml`

## 本轮做了什么

延续第一轮单线审计,本轮派 4 路 fork-agent(A/C/D/F)+ 主线程自理 B 区,对各 part 表中所有 ⚠️／❌／「待X」格子逐一以 `get_code_snippet` 实时源码 + min.yaml 枚举**双证重核**(spec-audit-method 判据④)。图谱关系边一律作废(已知滞后于 fix 提交),全凭磁盘实读定论。

| 区 | agent 昵称 | 翻盘 | 坐实 | 仍悬 |
|---|---|---|---|---|
| A 工具身份 | Halley | 1(#1d→✅) | 5(#3/#1a-D1-P0/#1b-D4/#1c-D11D12/#1e-P1) | 4(#1/#2/#1f/#1g 业务决策) |
| B 内容载体 | 主线程自审 | 0(B2-04/B2-05b 维持⚠P2) | — | 2(P2 一致性议题) |
| C 采样限长 | James | 1(C13/D25 →❌查实硬失败解码路径) | 5(C3-D23/C7-C9-D24/C4-precedence) | 0 |
| D 推理缓存 | Pasteur | 4(#4-enabled/#5新bug/#6-format-task_budget/#9-const) | 5(#3/#4-chat-drop/#10/#11/#13) | 2(#7业务/#9-stateful D14) |
| F 流控平台 | Noether | 1(F2a ✅→⚠规范缺证降级) | 9(F19/F21等) | 9(含F2链路) |

合计翻盘 **7**,坐实一批原 ❌ 并补强双证;被 fix(`812c9077`)真正改变的格子 ≈ 0(仅 parse 侧 #1c 局部 + anthropic 自环 top_k 保命)。

## 三条红线状态

- canonical 中间层不动 ✅(Halley 确认 `llm.Function` 未加 Namespace 字段,diff 仅扩 FunctionCall.ResponseItemID/Status 与 ResponseCustomToolCall.Namespace)
- OpenRouter min.yaml 为唯一规范源 ✅
- 禁脚本批量推断、须人工 MCP 取证 ✅(全程 get_code_snippet/rg/sed 直读)

## 重磅元发现(commit 名实不符)

commit `812c9077` 信息号称修了 K1(D1/P0 namespace)、#10(custom tool ns) 等 21 处,经实时源码核实:

- **A区 D1(P0)**:容器展开仍是 `工具名__子工具` 压扁不记分组身份,**fix 反而删除唯一的反向还原机制 `splitNamespaceAndName` 却无替代 = 负优化**
- **A区 #1c**:仅在 parse 侧新增 `ResponseCustomToolCall.Namespace` 字段并填值,但四处 history 路径(look-ahead merge / standalone convert / emit 输入项 / completed 快照)的 custom 分支仍未补 ns
- **C区 采样参数(D23/D24/D25/D26)**:零触及,B 类明确排除在 PR 外
- **D区 结构性缺口**(reasoning.enabled / output_config.format/task_budget / store const 校验 / session_id body / cache_control 顶层 DROP / user 桥接):无一闭合
- **F区 硬伤 F19(prompt 注释态)/ F21(context_management 全零)**:原样还在仓库里

即现存不少"以为已修"的 bug 实际仍在。后续修复务必以本次双证结论为准,勿信 commit message 或旧表格判定。

## 最终缺陷清单(分级)

### 🔴 P0 Critical
- **D1/#1a** namespace 容器往返断裂——证据 `responses/inbound.go:805-823`;修法走 TransformerMetadata 映射,勿破 canonical 加槽

### 🟠 P1 High
- #1e disable_parallel_tool_use 永久丢(anthropic 入不出 dptu)— `anthropic/{inbound,outbound}_convert.go`
- #3 parallel_tool_calls 在 anthropic 方向零读写 — `anthropic/model.go:205-206`
- #13 user 身份跨格式不桥接(canonical.User↔Metadata.user_id 互不拷贝,P1)— `anthropic/inbound_convert.go:73` 等
- #5 🆕 强制工具时 thinking 清空不全→仍发 adaptive 给 Anthropic,涉嫌违反禁令致上游 400 — `claudecode/utils.go:52-54`
- D23/C3 top_k 三方不对称(chat/responses 入站根本没收)
- F21 context_management 三协议全零实现
- F19 prompt 字段注释态模板引用彻底丢失(Response 有活跃 Prompt@841=半成品迹象)— `responses/model.go:134-136`

### 🟡 P2 Medium/Low
- D25/C13 logit_bias 锁死 map[int64],浮点值必整包 400(spec 允许 double)— `openai/inbound.go:65`
- D24/C7-9 repetition_penalty/min_p/top_a 静默丢(lenient 弃)
- #4-resp reasoning`.enabled` 未捕获(`ReasoningConfig@12579` 有 bool enabled)
- #6 output_config format(JSON Schema 结构化输出)/task_budget 整个没建模→structured outputs 功能失效
- #4-chat / #3-chat chat 整丢 reasoning 对象与 summary
- #9-store 缺 const:false 校验 + stateful 续接功能空白(D14 MED-SUSP)
- #10 session_id body 变体三协议 NOT_FOUND(违 body 优先规则,D20)
- #11 chat/responses 顶层 cache_control DROP(D20 关联)
- #1b builtin 工具(file_search/mcp/computer_use/bash_/text_editor_)静默丢无告警(D4)
- #1c custom_tool_call Namespace 四处 history 路径未跟进(D11/D12);流式侧(outbound_stream.go:269/287/370)疑似也残留,待补查
- F2 stream_options 跨协议 usage 双向不闭合 + 🆕隐患:convertStreamOptions(include_obfuscation==nil 即 return nil)连带吞合法 stream_options — `outbound_convert.go:486-502`

### 🔵 业务决议跟踪 —— 已于 2026-06-30 处理两项
- ✅[决] F8/F9/F10/F20:**已采纳「维持现状」分支**(2026-06-30 用户拍板)。AxonHub 不开 OR 客户端入口、服务端 channel/select_candidates 接管选路,维持 DROP(lenient Unmarshal 吞,D20),master 表 & part-F 各格均已升 ✅[决]。README 补说明列 follow-up。
- ✅[决] #7 verbosity:**已采纳「维持默认丢弃」分支**(2026-06-30)。取证 `rg Verbosity llm/transformer/anthropic/`=0 命中,native Anthropic 无此概念,判合规并入主表 F5 行。
- C4/D26 max_tokens 双槽优先级两端相反(anthropic MT>MCT vs responses MCT>MT),可选 P2

### ✅ 翻盘为非缺陷
- #1d caller↔namespace:spec 二者语义本异,AxonHub 未混用
- B2-06 developer 角色(早前结案)

## 待办

1. 各 part 文件判定列与本增订段落落地进行中(交由各区 fork-agent,Halley/Jame/Pasteur/Noether 分别落 A/C/D/F 的 part-*.md)
2. master-conversion-table.md 由主线程在四区 part 完成后统刷(避免并发写冲突)
3. 修复队列(TDD)建议优先序:P0 D1 → P1 群(#1e/#3/#13/#5/D23/F21/F19)→ P2 群
4. **【已决议 2026-06-30】** 原「OR 兼容承诺」与「verbosity 跨格式必要性」两项业务口径均已采纳维持默认,F8-F20 + #7 全部结案(见上 🔵 块)。该条不再作为待办。
5. 补查项:#1c 流式 custom 路径 ns 残留、logit_bias 是否存在 pass-through 直通短路绕过 TransformRequest(James 建议 trace_path 追 orchestrator.Process→Inbound.TransformRequest 前置条件)

## MCP 使用纪律回顾(详见 spec-audit-method.md)

- `get_code_snippet`=可定论(读磁盘真实源码);`search_graph/query_graph/trace_path`=仅定位且边会过期勿信;`search_code` enrich 解读打折
- 判字段丢失只认赋值右值行,不信 grep 命中数
- 由 case 标签/函数名推出行为必跳进去读 return 兜底(B2-06 convertUserMessage 反例)
- 三协议×出入站六象限逐一取证才算闭环
- 任一格 ✅/❌ 须双证并存:源码赋值 file:line + min.yaml 枚举行号;缺一只能 ⚠️
