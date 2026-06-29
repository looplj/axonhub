# Audit: openai/chat_completions 协议顶层请求字段命运

> 审计范围:**仅** `openai/chat_completions` 这一种客户端协议。
> 入站(client→canonical `*llm.Request`) = `llm/transformer/openai/inbound.go::TransformRequest`
>   → 解析目标结构体 `openai.Request`(`llm/transformer/openai/model.go:15`)
>   → 映射方法 `(r *Request).ToLLMRequest()`(`llm/transformer/openai/inbound_convert.go:38`)
> 出站(canonical→provider) = `llm/transformer/openai/outbound.go::TransformRequest`
>   → `RequestFromLLM()`(`llm/transformer/openai/outbound_convert.go:10`)重建 `openai.Request`
>   → `json.Marshal(oaiReq)` 序列化为上游请求体(`outbound.go:71`)
>
> 关键前提(均已核验):
> - 全仓无 `DisallowUnknownFields`;`openai.Request` 无 catch-all `map`/`RawMessage` 兜底字段,
>   故客户端发送而结构体未声明的 JSON 键一律被 `encoding/json` 静默丢弃。
> - 此协议入站不向 `llm.Request.ExtraBody` 写入任何内容;出站 `RequestFromLLM` 也从不读取
>   `ExtraBody`,因此本路径不存在 extra_body 透传通道(gemini/embedding 等其他协议才有)。
> - 流式聚合器 `aggregator.go` / `completion_aggregator.go` 只组装响应侧 choice/delta,不触及请求字段。

## 类别图例

- **DIRECT** —— 直转同名:canonical 有同名槽,inbound 直接赋值,outbound 还原同名输出字段。
- **RENAME** —— 改名重组进 canonical 已有槽(跨协议改名场景)。
- **MERGE** —— 合并进别的结构层(如 system 并入 messages[0])。
- **PASSTHROUGH** —— 经 TransformerMetadata 或 ExtraBody 原样透传给上游。
- **DROP** —— 默认丢弃:canonical 无该槽且不透传(此处多为结构体未声明导致解码期即丢)。
- **NOT_FOUND** —— 在代码中找不到任何处理点(不确定如实标注)。

## 字段命运表(共 39 个)

| spec字段 | spec类型 | inbound处理(文件:行+一句话) | outbound还原(文件:行+一句话) | 类别 | 备注 |
|---|---|---|---|---|---|
| cache_control | object | 结构体未声明(model.go:15-98),入站解析即被丢弃 | canonical 无槽,RequestFromLLM 不重建 | DROP | 客户端缓存提示丢失 |
| debug | object | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 调试开关丢失 |
| frequency_penalty | number | inbound_convert.go:45 直接赋 req.FrequencyPenalty(tag model.go:23) | outbound_convert.go:17 还原至同名字段 | DIRECT | |
| image_config | object | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 图像生成配置丢失(chat 端口无关) |
| logit_bias | object(map) | inbound_convert.go:58 赋 req.LogitBias(tag model.go:62,类型 map[string]int64) | outbound_convert.go:30 还原同名字段 | DIRECT | 值须为整数;浮点偏置会使整包解码失败 |
| logprobs | boolean | inbound_convert.go:46 赋 req.Logprobs(tag model.go:26) | outbound_convert.go:18 还原 | DIRECT | |
| max_completion_tokens | integer | inbound_convert.go:47 赋 req.MaxCompletionTokens(tag model.go:29) | outbound_convert.go:19 还原 | DIRECT | 与 max_tokens 双槽并存各自直转 |
| max_tokens | integer | inbound_convert.go:48 赋 req.MaxTokens(tag model.go:32) | outbound_convert.go:20 还原 | DIRECT | 已弃用但仍双向往返 |
| messages | []ChatMessages | inbound_convert.go:71-73 lo.Map 调 m.ToLLMMessage()(tag model.go:17) | outbound_convert.go:43-45 MessageFromLLMWithConfig 按 reasoningField 转 | DIRECT | 顶层直达;内部 message/toolcall 子结构另行变换 |
| metadata | object | inbound_convert.go:59 赋 req.Metadata(tag model.go:65,map[string]string) | outbound_convert.go:31 还原同名字段 | DIRECT | 仅支持扁平 string->string;嵌套或非字符串值会致整包解码失败 |
| min_p | number | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 采样阈值参数丢失 |
| modalities | []string | inbound_convert.go:60 赋 req.Modalities(tag model.go:68) | outbound_convert.go:32 还原 | DIRECT | |
| model | string | inbound.go:50 解析 + inbound_convert.go:44 赋 req.Model(tag model.go:20;必填校验 inbound.go:58) | outbound_convert.go:16 还原 | DIRECT | 缺失返回 ErrInvalidRequest |
| models | array | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 多模型路由列表丢失(AxonHub 以自有路由替代) |
| parallel_tool_calls | boolean | inbound_convert.go:66 赋 req.ParallelToolCalls(tag model.go:90) | outbound_convert.go:38 还原;但出站过滤后无工具则强制置 nil(L92-94) | DIRECT | 出站条件性清零 |
| plugins | []?(数组) | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 插件配置丢失 |
| presence_penalty | number | inbound_convert.go:49 赋 req.PresencePenalty(tag model.go:35) | outbound_convert.go:21 还原 | DIRECT | |
| provider | object | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | OpenRouter 选商参数丢失 |
| reasoning | object | 顶层 Request(model.go:15-98)未声明 reasoning 键;仅 Message 层有同名 echo 字段(model.go:170),不承载配置 | canonical 无顶层 reasoning 槽,RequestFromLLM 不重建 | DROP | 顶层 {effort,max_tokens,exclude,…} 整套配置连同 effort 一起丢;客户端须改用平铺键(Message 级 reasoning 仅回显历史推理,与此无关) |
| reasoning_effort | string | inbound_convert.go:61 赋 req.ReasoningEffort(tag model.go:71) | outbound_convert.go:33 还原 | DIRECT | 与上方 reasoning(对象)是两个相互独立入口 |
| repetition_penalty | number | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 重复惩罚采样参数丢失 |
| response_format | union(object) | inbound_convert.go:111-116 取 {Type,JSONSchema}(tag model.go:94 / 类型 model.go:295) | outbound_convert.go:85-90 还原 {Type,JSONSchema} | DIRECT | 仅承载 Type 与 JSONSchema 两子字段,其余子项不保留 |
| route | string | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | OpenRouter 路由策略丢失 |
| seed | integer | inbound_convert.go:50 赋 req.Seed(tag model.go:38) | outbound_convert.go:22 还原 | DIRECT | |
| service_tier | string | inbound_convert.go:64 赋 req.ServiceTier(tag model.go:81) | outbound_convert.go:36 还原 | DIRECT | |
| session_id | string | chat 顶层 Request 未声明(codex 子协议 headers.go:18 另有同名字段,与本协议无关) | 无槽不重建 | DROP | 会话分组标识丢失 |
| stop | union(string \| []) | inbound_convert.go:76-81 经自定义编解码 Stop{Stop,MultipleStop}(codec model.go:125-147)归一化进 req.Stop | outbound_convert.go:48-53 重建并经 MarshalJSON(model.go:113-123)回写 string \| [] | DIRECT | 单协议内无线材改名;字符串与数组两态双向保形往返 |
| stop_server_tools_when | array | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 服务端工具停止条件丢失 |
| stream | boolean | inbound_convert.go:65 赋 req.Stream(tag model.go:86) | outbound_convert.go:37 还原 | DIRECT | |
| stream_options | object | inbound_convert.go:84-88 仅取 IncludeUsage(tag model.go:87 / 类型 model.go:101) | outbound_convert.go:56-60 还原 IncludeUsage | DIRECT | 仅 include_usage 一个子字段被承载 |
| temperature | number | inbound_convert.go:52 赋 req.Temperature(tag model.go:44) | outbound_convert.go:24 还原 | DIRECT | |
| tool_choice | union(string \| obj) | inbound_convert.go:96-108 经自定义解码 ToolChoice{ToolChoice,NamedToolChoice}(codec model.go:433-450)进 req.ToolChoice | outbound_convert.go:70-82 重建并经 MarshalJSON(model.go:425-431)回写 | DIRECT | 字符串或 {type,function{name}} 保形;更丰富对象只取 Function.Name |
| tools | []ChatFunctionTool | inbound_convert.go:91-93 全量 t.ToLLMTool() 转 llm.Tool(tag model.go:91) | outbound_convert.go:65-67 FilterMap 仅保留 t.Type=="function" 的工具 | DIRECT | 入站全收;出站剔除非函数类(image_generation/responses_custom_tool 等) |
| top_a | number | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 采样参数丢失 |
| top_k | integer | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 采样参数丢失(canonical 亦无 top_k 槽) |
| top_logprobs | integer | inbound_convert.go:53 赋 req.TopLogprobs(tag model.go:47) | outbound_convert.go:25 还原 | DIRECT | |
| top_p | number | inbound_convert.go:54 赋 req.TopP(tag model.go:50) | outbound_convert.go:26 还原 | DIRECT | |
| trace | object | 结构体未声明,默认丢弃 | 无槽不重建 | DROP | 追踪配置丢失 |
| user | string | inbound_convert.go:57 赋 req.User(tag model.go:59,*string) | outbound_convert.go:29 还原 | DIRECT | |

类别计数:DIRECT 24 · DROP 15 · RENAME 0 · MERGE 0 · PASSTHROUGH 0 · NOT_FOUND 0(合计 39)

## 高风险丢映射小结

下列字段在本协议路径被判为 **DROP**,且会影响功能完整性(按影响排序):

- **top_k / min_p / repetition_penalty / top_a**(采样参数):均为合法、部分上游提供商可识别的采样控制项;canonical 既无对应槽又无透传通道,客户端设置后被静默吞掉,会导致实际生成行为偏离预期。这是最直接的「功能性」损失。
- **reasoning(对象形态)**:OpenRouter 推荐以单一 `reasoning:{effort,max_tokens,exclude,...}` 对象表达推理控制;AxonHub 该协议仅识别平铺键 `reasoning_effort`/`reasoning_budget`/`reasoning_summary`,凡走对象形态的客户端其整套推理配置连同 effort 都会被整体丢弃。(注:`reasoning_effort` 平铺键本身是 DIRECT,正常工作——二者是相互独立的入口。)
- **session_id**:用于会话/对话分组的标识完全丢失,影响多轮上下文连续性与去重计费等下游能力。
- **cache_control**:Prompt-cache 提示丢失,对依赖缓存降低成本与延迟的场景有实质影响(AxonHub canonical 也无顶层 cache_control 槽)。

以下 DROP 属于 OpenRouter 特有的路由/编排语义,AxonHub 以自有路由体系替代,通常视为「设计性替换」而非缺陷,但仍需告知用户不可用:

- **provider** / **route** / **models**(选商与多模型 fallback 列表)、**plugins**、**stop_server_tools_when**、**trace**、**debug**、**image_config**。

### 附带提醒(DIRECT 但存在兼容隐患)

虽非丢映射,但以下已支持字段在边界条件下会造成请求失败或信息缩减,审计中一并记录:

- **metadata**:类型固定为 `map[string]string`;若客户端发送嵌套对象或非字符串值,会使整个请求体 JSON 解码失败(inbound.go:52),表现为整包 ErrInvalidRequest 而非优雅降级。
- **logit_bias**:类型固定为 `map[string]int64`;浮点偏置值同样触发整包解码失败。
- **tools**:入站接收全部工具类型,但出站 `outbound_convert.go:65-67` 用 `FilterMap` 只放行 `t.Type=="function"`;其余类型(image_generation、responses_custom_tool 等)在 canonical→provider 阶段被剔除。
- **parallel_tool_calls**:当出站过滤后无 function 工具时,outbound_convert.go:92-94 强制将其置 nil,即便客户端显式设了 true。
- **stream_options / response_format / tool_choice**:各自仅承载有限子字段(include_usage / Type+JSONSchema / Function.Name),多余子项不会往返到上游。


## MCP 复核记录

> 本节由二次核对产生,工具:MCP `get_code_snippet` + `search_graph` 辅助 grep 定位。

- 入站 `(r*Request).ToLLMRequest()` 取自 `inbound_convert.go:38-119`:表中每个 DIRECT/DROP 字段的入站行号与之逐行一致。
- 出站 `RequestFromLLM(r,reasoningField)` 取自 `outbound_convert.go:10-97`:表中每个还原行号与之逐行一致;
  其中 tools 经 `lo.FilterMap(... t.Type==llm.ToolTypeFunction)`(L65-67)、ParallelToolCalls 于无函数工具时强制 nil(L92-94)均已印证。
- DROP 缺席核查:对 15 个被判 DROP 的 spec 键执行 `rg 'json:"<key>"...' llm/transformer/openai --glob '!**/*_test.go' --glob '!responses/**'`,
  结果显示这些键在 chat 顶层 `Request`(model.go:15-98)上**零声明**;仅有的相关命中均不属本协议顶层:
    - `model.go:170 Reasoning *string json:"reasoning,omitempty"` —— 位于 **Message** 结构体(L149 起),用于回显历史推理,非顶层请求配置对象;
    - `codex/headers.go:18 SessionID json:"session_id"` —— 属 GitHub-Copilot/codex 子协议头,不经 chat_completions 入站;
    - `responses/model.go:* Reasoning ...` —— 属 openai/responses 子协议,已被 `--glob '!responses/**'` 排除。
- 关键前提:`DisallowUnknownFields` 全仓为零启用;`openai/inbound.go` 与 `inbound_convert.go` 均不写入 `llm.Request.ExtraBody`,
  且出站从不读取 ExtraBody —— 故本路径不存在 extra_body 兜底透传,未知顶层字段一律解码期丢弃。
- 计数复算:DIRECT 24 + DROP 15 = 39(RENAME/MERGE/PASSTHROUGH/NOT_FOUND 各 0),与正文一致。

