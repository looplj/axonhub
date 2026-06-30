# 切片 δ · PRD(推理控制与缓存会话)

> 规范基准:`docs/specs/openrouter-chat-messages-responses.min.yaml`(下记 `(yaml:L)`);canonical `llm.Request`(`llm/model.go`)。守红线:不加 canonical 顶层槽(Thinking / OutputConfig / Prompt / ContextManagement 等),仅经 `TransformerMetadata` 通道或既有 canonical 槽展开。
> 权威字段表:`docs/specs/master-conversion-table.md` 第4区(part-D 推理控制)+ 第5区(part-F F19/F21)。本切片每原子 grill 必须对照 master 表行 + 源码行三步交叉。
> 原子顺序(由清晰到复杂):F19 → F21 → #6 → #4 → #5。

## δ-1 · F19 — responses `prompt` 存储模板引用静默丢失(P2)
- **Problem**:OpenRouter Responses 请求体 `prompt`($ref StoredPromptTemplate,id/version/variables,yaml:13152)在 AxonHub 入站被静默丢弃:`responses/model.go:134-136` 的 `Prompt *Prompt` 字段被 `// TODO` 注释(结构体 `type Prompt` 已存于 `model.go:170` 但字段未接线),`convertToLLMRequest`(inbound.go:170+)全文零 `req.Prompt` 引用,lenient Unmarshal 吞掉。同区兄弟字段 F15(background)/F16(include)/F17(truncation)/F18(max_tool_calls)均为 responses 独有且均经 TransformerMetadata PASSTHROUGH(✅),F19 是该家族唯一 DROP,根因纯为字段被注释。master 表 F19 行判定 ❌。
- **Solution**(镜像 F15-F18,守红线——不加 canonical Prompt 槽):
  1. `responses/model.go`:取消注释 `Prompt *Prompt json:"prompt,omitempty"`(恢复既存结构体接线)。
  2. `responses/inbound.go`:背景 stash 块后加 `if req.Prompt != nil { chatReq.TransformerMetadata["prompt"] = req.Prompt }`。
  3. `responses/outbound.go`:出站 Request 字面量加 `Prompt: xmap.GetPtr[Prompt](llmReq.TransformerMetadata, "prompt")`(复用既有泛型助手,存 *T 走 case *T)。
- **Testing**:红:入站 `&Request{Prompt:&Prompt{...}}`→`convertToLLMRequest`→`TransformerMetadata["prompt"]` 缺失;出站 metadata 带 prompt→body 无 prompt。绿:双向保留;缺省不注入守卫。
- **Out of scope**:跨格式至 chat/anthropic(二者 spec 无存储模板引用概念,master F19 标 C/M N/A,该腿正确 DROP);模板内容解析(由上游服务端解析,网关只透传引用);canonical 加 Prompt 槽(违红线)。
- **diagnose 跳过理由**:根因本轮已坐实(master F19 + 源码 model.go:134-136 / inbound.go 零引用 实测),红测可直接复现静默丢。
- **状态**:✅ 已完成·同模验收 Anscombe 代码 6/7 PASS(标准1/2/4/5/6/7);标准3 仅提交卫生(AGENTS.md 预存无关编辑未入 commit、PRD 已 add、状态已更新),已按规则只暂存 F19 文件提交。

## δ-2 · F21 — anthropic `context_management` 上下文压缩策略丢失(P1)
- **Problem**:grill 已坐实:anthropic `context_management`(顶层请求字段,yaml:8920(edits 8936+),nullable,edits 数组:clear_tool_uses_20250919/clear_thinking_20251015 等判别式 oneOf)在 `MessageRequest`(model.go:11)无 `ContextManagement` 字段、inbound_convert.go 零 stash、outbound_convert.go 零 restore→lenient Unmarshal 静默吞,上下文压缩策略丢失。grep 确认 Go 代码零声明(仅 testdata 响应夹具命中 response 侧 applied_edits,属 D10 另计)。判定 ⚠️ P1。
- **Solution**(镜像 cache_control 透传,守红线——json.RawMessage 不透明透传,不加 canonical 槽):
  1. `anthropic/model.go`:MessageRequest 末加 `ContextManagement json.RawMessage json:"context_management,omitempty"`(edits 为版本化判别式 schema,网关不解释,按作者 Caller/tool_result json.RawMessage 透传先例 raw 往返);加常量 `TransformerMetadataKeyContextManagement="anthropic_context_management"`。
  2. `anthropic/inbound_convert.go`:cache_control stash 块后加 `if len(anthropicReq.ContextManagement) > 0 { chatReq.TransformerMetadata[key] = anthropicReq.ContextManagement }`。
  3. `anthropic/outbound_convert.go`:cache_control restore 块后加 `if cm := asJSONRawMessage(chatReq.TransformerMetadata[key]); len(cm) > 0 { req.ContextManagement = cm }`(复用 tool_blocks.go 既有 asJSONRawMessage 助手,多类型兼容,免新 import)。
- **Testing**:红:入站 stash 缺 + 出站 restore 缺;绿:json.RawMessage 往返保真 + 缺省不注入。镜像 top_k_test.go 四子测。
- **Out of scope**:跨格式至 chat/responses(二者 spec 无 context_management,该腿正确 DROP);建模 edits 子结构(网关只透传不解释,master 表允许"透传");response 侧 applied_edits 回显(属 D10,另计)。
- **状态**:✅ 已完成·同模验收 Confucius 7 标准 APPROVED(json.RawMessage 贴合 Caller/tool_result 先例、asJSONRawMessage 复用、范围完整、canonical 红线未破)。

## δ-3 · #6 — output_config format/task_budget 子项丢失(P1)
- **Problem**:grill 已坐实:anthropic `output_config`(MessagesOutputConfig,yaml:8855/9048)有 effort/format(json_schema)/task_budget(tokens)三子项,但作者 `OutputConfig` 结构体(model.go:210)仅 `Effort` 字段,inbound_convert.go:389 仅 stash effort + max→xhigh 映射,outbound 仅 restore effort。`format`/`task_budget` 未建模→lenient Unmarshal 静默吞→DROP。注:`effort=='max'→xhigh` 实为设计性跨格式适配(stash 保留原始 max,supportsOutputConfig 出站回切 max),非 bug,不动。
- **Solution**(补 format/task_budget 透传,守红线——不加 canonical 槽,json.RawMessage 不透明透传):
  1. `anthropic/model.go`:OutputConfig 加 `Format json.RawMessage`+`TaskBudget json.RawMessage`(网关不解释,与 F21/Caller 同范式);加常量 `TransformerMetadataKeyOutputConfig="anthropic_output_config"`(存完整 *OutputConfig)。
  2. `anthropic/inbound_convert.go`:条件放宽为 `OutputConfig != nil`,stash 完整 `*OutputConfig` 到新 key;effort 子项映射逻辑保留(effort 非空时仍写 effort key + ReasoningEffort)。
  3. `anthropic/outbound_convert.go`:supportsOutputConfig 分支优先回切完整 stashed `*OutputConfig`(带 format/task_budget,含 format-only 无 effort 边界),回退 effort-only(向后兼容);非 supportsOutputConfig 维持 thinking 降级。
- **Testing**:红:inbound 全量/format-only stash 缺 + outbound 全量/format-only restore 缺;绿:format/task_budget 往返保真 + format-only 边界 + effort-only 向后兼容。镜像 TestOutputConfig_Outbound/Inbound 范式。
- **Out of scope**:effort max→xhigh 跨格式适配(设计性,不动);跨格式至 chat/responses(二者无 output_config,该腿正确 DROP);effort validate 缺 xhigh(旁支,不顺手改)。
- **状态**:✅ 已完成·同模验收 Lovelace 7 标准 APPROVED(代码 6/7 PASS,标准3 仅 AGENTS.md 卫生;空 output_config:{} 由 DROP 改保真透传判定可接受;effort max→xhigh 未动)。

## δ-4 · #4 — chat `reasoning` 对象整体丢失 + responses `reasoning.enabled` 未捕获(🔴最高危)
- **Problem**:master 表 part-D 第4行:chat 线模型(openai/model.go)未声明 `reasoning` 对象字段→整个 reasoning 配置(effort+summary)入站丢失,客户端唯有改用平铺 `reasoning_effort`(及非规范 `reasoning_summary`)才能部分救活;responses 侧 `reasoning.enabled` bool 子项未捕获(`responses/model.go:177` Reasoning 结构体仅 Effort/GenerateSummary/Summary/MaxTokens)。判定 ❌ HIGH-RISK DROP(chat)+ ⚠️ responses.enabled 缺失。
- **Solution**(守红线——不加 canonical 新顶层槽,继续借现有 ReasoningEffort/Budget/Summary 三槽):chat 给线模型加 `reasoning` 对象并展开进三槽;responses 补 `enabled` 进 TransformerMetadata。关联 D20。待 grill 核实源码行。
- **状态**:待 grill(Problem 引自 master 表第4行已核实)。

## δ-5 · #5 — thinking 往返 + utils.go:34 覆盖范围复核
- **Problem**:master 表 part-D 第5行:anthropic `thinking` 自身往返无损(enabled/disabled/adaptive/display 全保)。⚠️ 待复核:claudecode `disableThinkingIfToolChoiceForcedStructured`(utils.go:34)可能误伤合法 adaptive 思考。
- **Solution**:待 grill 复核 utils.go:34 覆盖范围;主体维持。
- **状态**:待 grill。

---

## 切片进度
| 原子 | 状态 | 验收代理 |
|---|---|---|
| δ-1 F19 | ✅ 已完成·已验收 | Anscombe |
| δ-2 F21 | ✅ 已完成·已验收 | Confucius |
| δ-3 #6 | ✅ 已完成·已验收 | Lovelace |
| δ-4 #4 | 待 grill | — |
| δ-5 #5 | 待 grill | — |
