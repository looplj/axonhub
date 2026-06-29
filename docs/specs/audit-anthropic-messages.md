# AxonHub `anthropic/messages` 客户端协议字段命运审计

> 范围:仅 `anthropic/messages` 一种客户端协议。审 27 个顶层请求字段(spec 取自 OpenRouter MessagesRequest)在
> **入站(client→canonical `llm.Request`)** 与 **出站(canonical→provider,Anthropic 原生格式重建)** 的处理动作。
> 对照基准 = canonical 中间层 `llm.Request` 已有 JSON 槽位。只读审计;严格基于代码事实。

## 入口与核心函数(均位于 `llm/transformer/anthropic/`)

- 入站校验:`inbound.go:28` `InboundTransformer.TransformRequest`(解析 + 校验 model/messages/max_tokens/system/thinking/tool_choice)
- 入站映射:`inbound_convert.go:54` `convertToLLMRequest`(`*MessageRequest → *llm.Request`)
- 出站还原:`outbound_convert.go:18` `convertToAnthropicRequestWithConfig` → `buildBaseRequest`(:113)+ 各子转换器
- 子结构体定义:`model.go`(`MessageRequest`:11、`AnthropicMetadata`:105 仅 UserID、`Thinking`:186、`OutputConfig`:195、`ToolChoice`:202 含 `DisableParallelToolUse`、`CacheControl`:254 等)
- 平台特化:`claudecode/outbound.go` 为纯**出站**平台适配(OAuth),无独立入站转换器,复用基类 `buildBaseRequest`;其 `utils.go` 对 `*llm.Request` 做注入式后处理(强制工具时禁思考 :34、工具名加前缀 :63、系统消息注入等),不改变字段级映射结论
- 流式聚合:`aggregator.go:17 AggregateStreamChunks` 只组装响应流块,**不触及请求顶层字段**

## 图例(命运标签六选一)

| 标签 | 含义 |
|------|------|
| DIRECT | 直转同名槽位 |
| RENAME | 改名重组进 canonical 已有槽(stop↔Stop、any↔required、metadata.user_id 等) |
| MERGE | 合并进别的结构层(system 并入 messages[0]) |
| PASSTHROUGH | 经 TransformerMetadata / ExtraBody 原样透传给上游 |
| DROP | 默认丢弃(声明但有意不发往上游) |
| NOT_FOUND | 在 anthropic 转换管线中未找到任何处理点 |

## 字段命运表

| spec字段 | spec类型 | inbound处理(文件:行+一句话) | outbound还原(文件:行+一句话) | 类别 | 备注 |
|---|---|---|---|---|---|
| cache_control | object\|null (`ephemeral`+ttl) | `inbound_convert.go:80` 存入 `TransformerMetadata["anthropic_cache_control"]` 原值保留 | `outbound_convert.go:~180` 从该键取回赋 `req.CacheControl`,并跳过自身断点优化 | PASSTHROUGH | 顶层自动缓存标记经 metadata 往返成功(model.go:254)。注意:**块级** cache_control 走消息内容转换另算,本行仅指顶层字段 |
| context_management | object\|bool | 未在 `MessageRequest` 声明,`inbound_convert.go` 无引用,Go 解析静默忽略 | 无对应槽位与还原逻辑 | NOT_FOUND | 全仓零引用;OpenRouter 扩展,AxonHub 不建模 |
| fallbacks | array\<object\> | 同上未声明、不解析 | 无还原 | NOT_FOUND | 仓库内 `fallbacks` 仅命中 antigravity executor 内部端点回退(executor.go:310),非客户端请求字段 |
| max_tokens | int64 (必填) | `inbound_convert.go:57` `chatReq.MaxTokens=&anthropicReq.MaxTokens` 直传 | `outbound_convert.go:120` `resolveMaxTokens(chatReq)` 取回(含缺省兜底,:204) | DIRECT | 入站校验必填且 >0(inbound.go:46);写 canonical.max_tokens 非 max_completion_tokens |
| messages | array\<MessageParam\> (必填) | `inbound_convert.go:94-315` system 先并入为首条,再逐 block 转 `llm.Message`(text/image/tool_result/thinking/redacted_thinking/signature) | `outbound_convert.go:18` 经 convertMessages 反向重建 MessageParam 数组 | DIRECT | 内容结构双向变换;thinking 等子块有专门分支处理 |
| metadata | object{user_id} | `inbound_convert.go:73` 仅提取 `Metadata.UserID` 写入 `chatReq.Metadata["user_id"]` | `outbound_convert.go:~129` 由 `Metadata["user_id"]` 还原为 `AnthropicMetadata{UserID}` | RENAME | `AnthropicMetadata`(model.go:105)仅有 user_id 一字段故无损;用户身份走此通道而非顶层 user |
| model | string (必填) | `inbound_convert.go:56` `chatReq.Model=anthropicReq.Model` | `outbound_convert.go:115` `req.Model=chatReq.Model` | DIRECT | 入站校验非空(inbound.go:42) |
| models | array\<string\>\|array\<object\> | 未声明、不解析 | 无还原 | NOT_FOUND | 与单数 model 不同;属 OpenRouter 多候选路由指令,服务端按配置渠道选择,不接受客户端传入 |
| output_config | object{effort(low/medium/high/max)} | `inbound_convert.go:377` 存 `TransformerMetadata["output_config_effort"]=原始 effort`,并把 effort 映射到 ReasoningEffort(max→xhigh) | `outbound_convert.go:~168` 若 `supportsOutputConfig(config)` 则原值还原 OutputConfig.Effort,否则降级 enabled+budget 思考 | PASSTHROUGH | 原始 effort(含 "max")靠 metadata 保真;不支持平台则语义漂移。验证见 thinking_test.TestOutputConfig_Outbound |
| plugins | array | 未声明、不解析 | 无还原 | NOT_FOUND | anthropic 包及 server 层均无 json tag 或字面量引用 |
| provider | string\|object | 未声明、不解析 | 无还原 | NOT_FOUND | 服务端按渠道配置选上游,不接受客户端指定 provider;仓库内 provider 字段均为 webhook/auth/Gemini 模型拉取等无关用途 |
| route | string | 未声明、不解析 | 无还原 | NOT_FOUND | 路由由服务端决定,客户端不可控 |
| service_tier | enum(auto,standard_only) | `inbound_convert.go:70` 非空则 `lo.ToPtr` 存 `chatReq.ServiceTier` | `outbound_convert.go:~125` `*chatReq.ServiceTier` 回写 `req.ServiceTier` | DIRECT | 注释明确为跨格式转换保活而显式携带 |
| session_id | string | 不从消息体解析(MessageRequest 无此字段) | 无还原 | NOT_FOUND | 会话标识仅在 codex/opencode/openai-responses 路径经 header 提取(internal/server/middleware/trace.go),anthropic 消息体的 session_id 被忽略 |
| speed | enum(priority/speed?) | 未声明、不解析 | 无还原 | NOT_FOUND | 仓库无 `json:"speed"` 及相关字面量 |
| stop_sequences | array\<string\> | `inbound_convert.go:333-339` 单元素→`Stop.Stop`,多元素→`Stop.MultipleStop` | `outbound_convert.go:315` `convertStopSequences` 反向还原 []string | RENAME | ↔canonical.stop 双向往返成功 |
| stop_server_tools_when | object\|array | 未声明、不解析 | 无还原 | NOT_FOUND | 全仓零引用;web_search 等服务器工具的停止条件不被支持 |
| stream | bool\|null | `inbound_convert.go:60` `chatReq.Stream=anthropicReq.Stream` | `outbound_convert.go:118` `req.Stream=chatReq.Stream` | DIRECT | 控制流式开关贯穿管线 |
| system | string\|array\<SystemPromptPart\> | `inbound_convert.go:94-112` 作为 role:"system" 消息并入 Messages 开头(数组形式置 `TransformOptions.ArrayInstructions=true`) | `outbound_convert.go:812` `convertToAnthropicSystemPrompt` 抽出系统消息并据 ArrayInstructions 还原字符串或数组形态 | MERGE | 并入 messages[0],往返可逆(含文本/数组两种形态与每段 cache_control) |
| temperature | float\|null | `inbound_convert.go:58` `chatReq.Temperature=…` | `outbound_convert.go:116` `req.Temperature=…` | DIRECT | |
| thinking | object{type,budget_tokens,display} | `inbound_convert.go:351-373` enabled→ReasoningEffort(thinkingBudgetToReasoningEffort)+ReasoningBudget;display 进 thinking_display 键;disabled/adaptive→TransformerMetadata["thinking_type"](+ReasoningEffort none/"high") | `outbound_convert.go:139-165` 按 thinking_type(disabled/adaptive)/ReasoningEffort="none"/buildThinking(enabled,budget,:211)/display 综合重建 Thinking | RENAME | 主路径改名进 reasoning_effort/budget;adaptive·disabled·display 靠 TransformerMetadata 补全以无损往返。注:claudecode 平台在强制工具(tool/any)时会清空 reasoning(utils.go:34 disableThinkingIfToolChoiceForcedStructured)—平台级行为覆盖 |
| tool_choice | object{type,name?,disable_parallel_tool_use?} | `inbound_convert.go:392` convertAnthropicToolChoiceToLLM:auto/none 直传、"any"→"required"、"tool"→NamedToolChoice.Function.Name | `outbound_convert.go:284` convertToolChoiceToAnthropic:反向(required→any,type:"tool"+name) | RENAME | 类型互译往返正常;但 **DisableParallelToolUse 既不入 canonical.parallel_tool_calls 也未还原**(两处源码均未涉)→隐藏丢失(高险) |
| tools | array\<Tool\> | `inbound_convert.go:317-326` convertToolToLLM 转函数工具与 web_search 参数 | `outbound_convert.go:229` convertToolsAnthropic 反向;文档注明仅 web_search 为原生工具,其它原生工具(image_generation/google_* 等)被忽略 | DIRECT | 函数工具往返完整;原生非 web_search 工具静默丢弃(部分 DROP)。claudecode 另对工具名加前缀(utils.go:63) |
| top_k | int\|null | `inbound_convert.go:88` 因 canonical 无 TopK 槽,存 `TransformerMetadata["anthropic_top_k"]=&topK` | `outbound_convert.go:~187` 取回赋 `req.TopK` | PASSTHROUGH | 三方都有但 canonical 无同名槽,借 TransformerMetadata 完成无损往返 |
| top_p | float\|null | `inbound_convert.go:59` `chatReq.TopP=…` | `outbound_convert.go:117` `req.TopP=…` | DIRECT | |
| trace | object\|string(id?) | 未作为请求体字段声明、不解析(`tracing.Config` 属观测配置与此无关) | 无还原 | NOT_FOUND | 请求级 trace 标识不被建模;分布式追踪走 internal/server/config.go tracing 配置而非客户端字段 |
| user | string | 未声明顶层 user;用户身份实际经 metadata.user_id 流转(见 metadata 行),不走 canonical.User 槽 | 无独立还原(随 metadata 复原) | NOT_FOUND | 标准 Anthropic API 无顶层 user;OpenRouter 该字段在此实现中无入口。canonical 虽有 user 槽但未被 anthropic 入站写入 |

## 高风险丢映射小结

下列属于 DROP / NOT_FOUND 且影响功能完整性,以及虽整体已映射但在子维度存在静默丢失的字段:

### A. 整字段 NOT_FOUND —— 功能直接缺失

1. **路由委派类(provider / route / fallbacks / models)**:AxonHub 上游选择完全由服务端渠道配置决定,不接受客户端指定的 provider/route/fallback/model-list。若使用方期望类似 OpenRouter 的客户端驱动路由/回退/模型列表,这些会被静默忽略——迁移兼容性高风险。
2. **特性参数类(context_management / session_id-body / speed / stop_server_tools_when / plugins / trace / user-topLevel)**:
   - `context_management`、`stop_server_tools_when`:全仓零实现,功能彻底不存在;
   - `session_id`:仅 header/codex/opencode/openai-responses 路径生效,**anthropic 消息体内的 session_id 被丢弃**;
   - `speed`、`plugins`、`trace`(请求级):无任何建模;
   - `user`(顶层 body):标准 Anthropic 协议本身不含此字段,用户身份改由 `metadata.user_id` 承载,二者不可混淆。

### B. 隐藏子项丢失(整字段已映射但不完整)

3. **tool_choice.disable_parallel_tool_use**:canonical 有 `parallel_tool_calls` 槽却未被写入,出站也不还原 ⇒ 并行工具控制信号在跨格式转换中永久丢失。这是本次审计最隐蔽的功能缺口。
4. **tools 中的原生工具**:除 `web_search_20250305` 外,image_generation / google_* 等原生工具在出站被显式忽略(outbound_convert.go:229 文档注释),即客户端声明的此类工具会无声消失。
5. **ClaudeCode 平台的 thinking 强制关闭**:`utils.go:34 disableThinkingIfToolChoiceForcedStructured` 在 tool_choice 为 any/named-tool 时清空 ReasoningEffort/Budget,导致该平台上「强制工具调用」场景下扩展思考被意外抑制——行为级而非映射级风险,需在使用侧知晓。

### C. 语义漂移风险

6. **output_config.effort == "max"**:入站被映射成 `reasoning_effort="xhigh"`(有损),只有当目标平台 `supportsOutputConfig` 时才经 TransformerMetadata 用原值 "max" 还原;不支持的平台会把 "max" 降级为 budget-based 的 enabled-thinking,产生语义偏差。

---

附:类别计数 — DIRECT 8 · RENAME 4(metadata/stop_sequences/thinking/tool_choice)· MERGE 1(system)· PASSTHROUGH 3(cache_control/output_config/top_k)· DROP 0 · NOT_FOUND 11 · 合计 27。
