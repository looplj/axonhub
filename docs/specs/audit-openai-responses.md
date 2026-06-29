# AxonHub · openai/responses 客户端协议顶层请求字段命运审计
- 日期：2026-06-29(v2:MCP 图谱核对)
- 范围:`llm/transformer/openai/responses/{inbound,outbound,outbound_convert,request_extensions}.go`(及 `model.go`)对 client→canonical(`llm.Request`)→provider 的请求字段处理
- canonical 基准槽位:`messages,model,frequency_penalty,…,response_format,verbosity,reasoning_effort/reasoning_budget/reasoning_summary,prompt_cache_key,previous_response_id,safety_identifier,user,metadata,modalities,service_tier,store,stream,stream_options,parallel_tool_calls,tools,tool_choice,max_completion_tokens,top_logprobs,top_p,temperature,extra_body…`
- 方法:MCP 图谱定位 + 源码精读 + 调用关系图谱核对(trace_path/query_graph)+ 对 routing 类字段的 repo-wide 非测试 Go 字面量核查
- 只读不改源码;事实不确定者标 NOT_FOUND 并注「待复核」
## 图例(类别六选一)
| 标签 | 含义 |
|---|---|
| DIRECT | 直转同名进 canonical 已有槽 |
| RENAME | 改名/重组进 canonical 已有槽 |
| MERGE | 合并进别的结构层(如 messages[]) |
| PASSTHROUGH | 经 TransformerMetadata / ProviderExtensions 原始片段透传 |
| DROP | 默认丢弃(lenient Unmarshal 且无任何载体) |
| NOT_FOUND | 未找到处理点(如实标注,待复核) |
> 说明:`attachOpenAIResponsesRequestExtensions`(request_extensions.go:9)**只**把非结构化的 `tools`/`tool_choice`/`input.items` 三类原始片段存入 `ProviderExtensions.OpenAIRequests`,**不会**泛化回收任意未知顶层字段;`convertToLLMRequest` 也从不填充 `ExtraBody`(repo 核查:`.ExtraBody=` 全仓仅 gemini 两处,responses 包为零)。故凡不在 `Request` 结构(model.go)中的顶层字段,一律被宽松 `json.Unmarshal`(无 DisallowUnknownFields,D20)静默丢弃。
## 字段命运表(共 39 个顶层请求字段)
| spec字段 | 类别 | inbound处理(client→canonical) | outbound还原(canonical→provider) | 备注 |
|---|---|---|---|---|
| background | PASSTHROUGH | inbound.go:214 存 `TransformerMetadata[background]=*req.Background` | outbound.go:276 `xmap.GetBoolPtr` 回填 Background | 非 canonical 标量槽,经元数据透传 |
| cache_control | DROP | 无建模、无扩展捕获;lenient Unmarshal 丢弃(D20) | 不还原 | 全仓零引用(cache_control 是 Anthropic 概念,responses 路径不收) |
| debug | DROP | 无建模、无扩展捕获;全仓零引用 | 不还原 | 调试开关未实现 |
| frequency_penalty | DIRECT | inbound.go:173 直转 `chatReq.FrequencyPenalty` | outbound.go:262 原样写 payload | ⚠️ D19:Responses 规范 CreateResponseRequest 本不含此参(Chat 泄漏),仍接受并转发 |
| image_config | DROP | 无建模、无扩展捕获;全仓零引用 | 不还原 | 顶层缺失;图像配置仅在 `tools[].image_generation` 子结构内承载 |
| include | PASSTHROUGH | inbound.go:196 存 `TransformerMetadata[include]=req.Include`([]string) | outbound.go:272 `xmap.GetStringSlice` 回填 Include | 经元数据透传 |
| input | RENAME | inbound.go:260 设 ArrayInputs 标志;inbound.go:264 `convertInputToMessages` 展开 text 或 items→`[]llm.Message` | outbound_convert.go:95 `convertInputFromMessages` 反向重建 Input(Text/items) | ✅ 关键转换正确,字符串与数组两态均支持 |
| instructions | MERGE | inbound.go:250 作为 role=system 的 Message 注入 messages 头部 | outbound_convert.go:52 `convertInstructionsFromMessages` 抽取全部 system 消息拼回 Instructions | 归宿=首条 system message;多轮往返会把多条 system 文本拼接 |
| max_output_tokens | RENAME | inbound.go:179 映射到 `chatReq.MaxCompletionTokens` | outbound.go:260 回填 `payload.MaxOutputTokens`;outbound.go:291 空则兜底用 MaxTokens | 名义改名映射,双向闭合 |
| max_tool_calls | PASSTHROUGH | inbound.go:200 存 `TransformerMetadata[max_tool_calls]=req.MaxToolCalls`(*int64) | outbound.go:273 `xmap.GetInt64Ptr` 回填 | 经元数据透传 |
| metadata | DIRECT | inbound.go:176 `maps.Clone(req.Metadata)`→`chatReq.Metadata`(map[string]string) | outbound.go:259 原样写 payload.Metadata | 直接同名克隆 |
| modalities | DIRECT | inbound.go:184 直转 `chatReq.Modalities` | outbound.go:266 原样回填 | 直接同名 |
| model | DIRECT | inbound.go:171 直转 `chatReq.Model`(TransformRequest 校验非空 inbound.go:59) | outbound.go:248 写 payload.Model | 必填,直通 |
| models | DROP | 无建模(Request 仅单数 Model)、无扩展捕获;lenient Unmarshal 丢弃(D20);repo 核查 `"models"` 键仅为 GraphQL/诊断输出,无消费者读客户端 fallback 列表 | 不还原 | 客户端候选模型列表不被采用;选路走服务端 candidate 选择(select_candidates.go) |
| parallel_tool_calls | DIRECT | inbound.go:187 直转 `chatReq.ParallelToolCalls` | outbound.go:252 回填;outbound.go:286 当 Tools 为空时置 nil(ResponseAPI 兼容) | 直通 + 无工具清空逻辑 |
| plugins | DROP | 无建模、无扩展捕获;lenient Unmarshal 丢弃(D20) | 不还原 | 本包零引用 |
| presence_penalty | DIRECT | inbound.go:174 直转 `chatReq.PresencePenalty` | outbound.go:263 原样回填 | ⚠️ D19:同 frequency_penalty,Chat 泄漏但仍转发 |
| previous_response_id | DIRECT | inbound.go:189 直转 `chatReq.PreviousResponseID` | outbound.go:271 原样回填 | 直通;⚠️ 但 stateful 续接历史未服务端展开(D14) |
| prompt | DROP | Request 结构中该字段已注释禁用(model.go `// Prompt … // TODO`),无建模、不入 canonical | 不还原 | 提示模板引用功能未实现 |
| prompt_cache_key | DIRECT | inbound.go:188 直转 `chatReq.PromptCacheKey` | outbound.go:270 回填;若空且 ctx 有 SessionID 则以会话 ID 兜底(outbound.go:279 GetSessionID) | 直通 + 会话兜底 |
| provider | DROP | 无建模、无扩展捕获;lenient Unmarshal 丢弃(D20);repo 核查 internal/llm/cmd 内无非测试 Go 代码读取此客户端字段(OIDC/quota 日志命中均为无关) | 不还原 | 渠道/提供商选择由服务端 channel 配置决定;客户端或可借 model 前缀(如 openai/gpt-4o)表达偏好,但不经此字段 |
| reasoning | RENAME | inbound.go:219-231 将对象拆解:effort→ReasoningEffort(:220)、max_tokens→ReasoningBudget(:224)、summary/generate_summary→ReasoningSummary(:229/:231)(优先 summary) | outbound_convert.go:506 `convertReasoning` 由三槽重组 Reasoning{Effort,MaxTokens,Summary};effort 与 budget 同时存在时丢 budget | 对象↔三个 canonical 辅助槽的双向分解/重组 |
| route | DROP | 无建模、无扩展捕获;lenient Unmarshal 丢弃(D20);repo 同名命中仅为 xredis RouteByLatency/RouteRandomly 配置标志,与请求体无关 | 不还原 | 平台级路由策略未接入该字段 |
| safety_identifier | DIRECT | inbound.go:185 直转 `chatReq.SafetyIdentifier` | outbound.go:257 原样回填 | 直接同名 |
| service_tier | DIRECT | inbound.go:186 直转 `chatReq.ServiceTier` | outbound.go:256 原样回填 | 枚举未强校验(minor) |
| session_id | NOT_FOUND | body 级 session_id 在本包无建模/无捕获(lenient Unmarshal 丢弃,D20);功能上的会话身份另由中间件经 `shared.WithSessionID(ctx,...)`(源自 HTTP header 等,**非** body 字段)注入(shared/session.go:21) | 不直接还原;ctx 会话 ID 经 outbound.go:280 `GetSessionID` 用于 PromptCacheKey 兜底 | body 字段与 ctx 会话 ID 未证实关联,**待复核**(是否应打通属业务决策) |
| stop_server_tools_when | DROP | 无建模、无扩展捕获;lenient Unmarshal 丢弃(D20) | 不还原 | 服务端工具停止条件未实现 |
| store | DIRECT | inbound.go:181 直转 `chatReq.Store` | outbound.go:255 原样回填 | 值直通;⚠️ 有状态续接(store=false 时缺口)见 D14 |
| stream | DIRECT | inbound.go:175 直转 `chatReq.Stream` | outbound.go:253 原样回填 | 直接同名 |
| temperature | DIRECT | inbound.go:172 直转 `chatReq.Temperature` | outbound.go:261 原样回填 | 直接同名 |
| text | RENAME | inbound.go:283-297:text.format.type→ResponseFormat.Type(json_schema 时从 Name/Description/Schema/Strict 重构 JSONSchema);inbound.go:304 text.verbosity→Verbosity | outbound_convert.go:15 `convertToTextOptions` 由 ResponseFormat+Verbosity 重建 Text{Format,Verbosity} | ResponseFormat 等价物,json_schema 双向重构闭合 |
| tool_choice | RENAME | inbound.go:315 `convertToolChoiceToLLM`:mode/named 入 canonical;**忽略 choice.Tools[] 多选数组形式**(改为 raw 片段旁路 request_extensions.go:142) | outbound_convert.go:467 `convertToolChoice` 还原 mode/named;数组形式经 marshalRequestPayload(request_extensions.go:225)按签名匹配后原始注入 | ⚠️ 受 D1 影响:named 选择用的 Function.Name 可能是复合名(grp__leaf) |
| tools | RENAME | inbound.go:730 `convertToolsToLLM`:function/image_generation/web_search/custom 结构化建模;**namespace 组压扁成 grp__leaf 单函数(inbound.go:807/:819)且从不设 Function.Namespace**(rg 枚举证实展开分支无 Namespace 赋值);mcp/file_search 等 builtin 走 default 跳过但作为 RawTools 片段存活(request_extensions.go:95) | outbound.go:222-245 按 Type 逐类重建(function 用 src.Function.Name 即复合名,无 ns 拆分);marshalRequestPayload(request_extensions.go:197 mergeRawOnlyTools)合并回 raw-only 工具 | 🔴 **D1 核心**:ns 往返断裂致 MCP 工具回收箱无法映射→unsupported;builtin 类型见 D4 |
| top_k | DROP | 无建模(Request 结构无 top_k)、无扩展捕获;lenient Unmarshal 丢弃 | 不还原 | chat/responses 不收 top_k(仅 anthropic 保命侧使用) |
| top_logprobs | DIRECT | inbound.go:182 直转 `chatReq.TopLogprobs` | outbound.go:264 原样回填 | 直接同名 |
| top_p | DIRECT | inbound.go:183 直转 `chatReq.TopP` | outbound.go:265 原样回填 | 直接同名 |
| trace | DROP | 无建模、无扩展捕获;lenient Unmarshal 丢弃(D20);trace.go 中间件做的是 AH-Trace-Id 头传播,与本 body 字段无关 | 不还原 | 追踪标识 body 字段未接入 |
| truncation | PASSTHROUGH | inbound.go:208 存 `TransformerMetadata[truncation]=req.Truncation`(*string) | outbound.go:275 `xmap.GetStringPtr` 回填 | 经元数据透传(auto/disabled) |
| user | DIRECT | inbound.go:180 直转 `chatReq.User` | outbound.go:258 原样回填 | 直接同名 |
## 高风险丢映射小结
下列项虽不一定都属 DROP/NOT_FOUND,但其“映射”在功能完整性上构成真实风险:
- 🔴 **tools(namespace 往返断裂)= 当前修复重点根因**:Codex 以 `{type:namespace,name:<grp>,tools:[{name:<leaf>}]}` 声明每个 MCP server,inbound 展开成单函数名 `<grp>+__+<leaf>` 发往上游却**从不设置 `Function.Namespace`**,出站也无任何按命名空间还原的逻辑(named 选择与响应装配同样读到恒空的 Namespace)。机制完美对应 wire_api=responses 下 MCP 工具报 unsupported。(呼应 D1/D11/D12)
- 🟠 **tool_choice named 形式受连累**:`NamedToolChoice.Function.Name` 取自可能已被压扁的复合名,导致指定某 MCP 工具时名称口径不一致。(呼应 D1)
- 🟠 **builtin 工具类型(mcp/computer_use/file_search/code_interpreter 等)**:走 `default continue` 静默跳过结构化建模,仅当 responses→responses 同线时以 RawTools 原始片段存活;一旦下游换其它 wire 格式即彻底丢失。(呼应 D4)
- 🟡 **top_k DROP**:采样参数丢失,但 Responses 协议本身不接受 top_k(anthropic 保命侧特有),影响有限。
- 🟡 **provider / route / models = DROP**:repo 核查(internal+llm+cmd 非测试 Go)确认这些客户端顶层键在响应路径无可达消费者,lenient Unmarshal 即丢(D20)。其网关路由语义属独立子系统(channel/candidate 选择 + openrouter 出站转换器),不经过 canonical llm.Request 往返——客户端指定的 provider/model 列表不影响实际选路。
- 🟡 **session_id = NOT_FOUND**:OpenRouter 规范的 body 级 session_id 不被解析;AxonHub 另有基于 HTTP 头→`shared.WithSessionID` 的会话机制用于 prompt cache key 兜底(outbound.go:280)。两者是否应打通需业务决策。
- 🟡 **prompt / image_config / debug / cache_control / plugins / trace / stop_server_tools_when = DROP**:均为功能缺口(prompt 模板、调试、追踪、服务端工具停控等),非当前 bug 焦点,属于 lenient Unmarshal 盲区(D20)。
## 与已记录偏差(.agent/summary/responses-spec-audit.md)的关联
| 本次字段判定 | 关联 D 编号 | 说明 |
|---|---|---|
| tools(namespace 压扁、Namespace 恒空、composite name 往返断裂) | **D1** | CRITICAL 根因:声明期建 compositeName→{leaf,namespace} 映射来回填,不能靠 split(组名本身含 __ 如 mcp__node_repl) |
| tools(builtin 类型 default 静默跳过、仅 RawTools passthrough 存活) | **D4** | HIGH:mcp/computer_use/file_search/code_interpreter 应为一等工具类型,不得静默丢 |
| tool_choice / tools 中 custom_tool_call 的 Namespace 入站解析与历史重建不对称 | **D11** | MED-HIGH:custom_tool_call 两处补对称透传(已有相关测试 outbound_convert_test.go:1473) |
| aggregator 最终 completed 快照 custom_tool_call item 缺 Namespace(response 侧相邻) | **D12** | LOW-MED:function_call case 有 namespace(aggregator.go:631)而 custom 没有,append 补齐 |
| store / previous_response_id / include(stateful 续接未服务端展开) | **D14** | MED-SUSP:Codex store=false 未命中,缺 stateful 实现 |
| frequency_penalty / presence_penalty(Chat 参数泄漏仍被接受并转发) | **D19** | LOW:CreateResponseRequest 规范仅 temperature/top_p,应移除或文档标注 |
| 所有 DROP 未知顶层字段(provider/route/models/cache_control/debug/image_config/plugins/prompt/stop_server_tools_when/top_k/trace 等) | **D20** | LOW-MED:lenient `json.Unmarshal` 无 DisallowUnknownFields,未知与新 native 字段静默丢,建议白名单+告警 |
## MCP 图谱核对记录(call-graph evidence)
- **inbound 调用链(query_graph)**:`convertToLLMRequest`(inbound.go:169)的直接 CALLS 目标恰为四个:`attachOpenAIResponsesRequestExtensions`、`convertToolChoiceToLLM`、`convertInputToMessages`、`convertToolsToLLM`——与表中各字段入口一一吻合,不存在第四方隐藏通路消费其它顶层字段。
- **outbound 调用链(query_graph)**:`OutboundTransformer.TransformRequest`(outbound.go:191)CALLS 20 个目标,涵盖 `convertToTextOptions/convertInstructionsFromMessages/convertInputFromMessages/marshalRequestPayload/convertImageGenerationToTool/convertWebSearchToTool/convertCustomToTool/convertFunctionToTool/convertToolChoice/convertStreamOptions/convertReasoning/GetSessionID/xmap.*` 等——与表中 outbound 还原列完全对应。
- **D1 实锤(rg 枚举 Namespace)**:全包非测试文件的所有 `Namespace` 出现点中,**没有任何一个落在 `convertToolsToLLM` 的 namespace 展开分支(inbound.go 约 :807-822)**;该分支构造 `llm.Function{Name: grp+__+leaf}` 时确省略 Namespace。其余 Namespace 点均为(a) 历史 input item 还原(inbound.go:433/:538)、(b) 响应装配读取恒空值(inbound.go:990/:1008)、(c) 流式聚合/aggregator——均不构成声明期回填。
- **DROP 收敛证据**:`.ExtraBody =` 全仓非测试 Go 仅两处(gemini/openai/outbound.go:341、gemini/inbound_convert.go:71),responses 包为零;结合 `Request` 结构无相应字段,确认 cache_control/debug/image_config/plugins/provider/route/models/prompt/stop_server_tools_when/top_k/trace 在本协议下无可达载体而被 lenient Unmarshal 丢弃。
> 附:aggregator.go 负责 RESPONSE 流聚合(输出项快照),不参与 REQUEST 字段映射;compact_inbound/outbound 变体复用同一套字段映射规则,差异仅在 compaction 消息形态,不影响上述 39 字段的类别判定。
<!-- AUDIT-FINAL-V2-MCP-CHECKED-SENTINEL -->
