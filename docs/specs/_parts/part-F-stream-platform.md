<!-- part-F · 第5区·流控杂项与平台路由丢弃 · 纯人工逐格核实(MCP 图谱定位+源码精读+min.yaml 对照) -->
> 类别图例：DIRECT 直转同名进 canonical 已有槽｜RENAME 改名/重组进已有槽｜MERGE 合并进别的结构层｜PASSTHROUGH 经 TransformerMetadata 往返透传｜DROP 默认丢弃(lenient Unmarshal 无 DisallowUnknownFields，D20)｜NOT_FOUND 未找到处理点(如实标注)
> 规范基准：`docs/specs/openrouter-chat-messages-responses.min.yaml`(下记行号均指该文件)；canonical 缺 Background/Include/Truncation/MaxToolCalls/Prompt/ImageConfig/Debug 等标量槽，故这些走 metadata 或丢。
> 三 transformer 包入口：openai/chat=`llm/transformer/openai/{inbound_convert,outbound_convert,model}.go`；anthropic/messages=`llm/transformer/anthropic/{inbound_convert,inbound,model}.go`；openai/responses=`llm/transformer/openai/responses/{inbound,outbound,outbound_convert,request_extensions,model}.go`

## 第5区 · 流控杂项与平台路由丢弃

| # | 概念 | Chat名(类型) | Messages名(类型) | Responses名(类型) | Canonical槽 | 作者现状[类别](证据) | 合规判定 | 正确做法 & 关联D编号 |
|---|---|---|---|---|---|---|---|---|
| F1 | stream | `stream`(boolean,default false)(yaml:ChatRequest.stream@4984) | `stream`(boolean)(yaml:MessagesRequest.stream@9097) | `stream`(boolean,default false)(yaml:ResponsesRequest.stream@13188) | `llm.Request.Stream` | C:DIRECT(inbound_convert.go:65→outbound_convert.go:37)；M:DIRECT(inbound_convert.go:60→outbound_convert.go:118)；R:DIRECT(inbound.go:175→outbound.go:253)。流式⑦与非流式⑦路径一致 | ✅ | 三协议同名直转且出站还原，无需修复。D— |
| F2 | stream_options | `stream_options`($ref ChatStreamOptions)(yaml@4989) | —（规范 messages 体无此字段） | `stream_options`(见 model.go:148，含 include_obfuscation) | `llm.Request.StreamOptions`(仅 IncludeUsage，model.go:290-296) | C:DIRECT-subset——只取 include_usage 进 canonical.StreamOptions.IncludeUsage(inbound_convert.go:84-87→outbound_convert.go:56-59)，其余子项不往返；M:N/A(spec 无)；R:PASSTHROUGH——建空壳 `&llm.StreamOptions{}` 且把 include_obfuscation 存 TM(inbound.go:241-245)，经 convertStreamOptions 从 metadata 回填(outbound_convert.go:488-501→outbound.go:268) | ⚠️ | chat 仅承载 include_usage 子集、resp 经元数据透传 include_obfuscation；跨格式时另一端子项丢失。建议统一建模或白名单告警。关联 D20(相邻)、P3 |
| F2a | └ stream_options.include_obfuscation | —（chat 子项不含此项） | — | 见上(responses StreamOptions 子键，AxonHub 解析) | 无标量槽→TransformerMetadata["include_obfuscation"] | R:PASSTHROUGH(inbound.go:243-244 存→convertStreamOptions@488-501 取→outbound.go:268 还原)双向闭合 | ✅(基准确认见自检①) | 透传正确；**但 min.yaml 基准未在 ResponsesRequest 列出 stream_options/include_obfuscation**(来源待核)。关联 D20-邻接、P3 |
| F3 | modalities | `modalities`(array enum text/image/audio)(yaml@4828) | —（messages 体无） | `modalities`(array enum text/image)(yaml@13102) | `llm.Request.Modalities` | C:DIRECT(inbound_convert.go:60→outbound_convert.go:32)；R:DIRECT(inbound.go:184→outbound.go:266)；M:N/A | ✅ | 各定义协议同名直转，msg 无该概念故 N/A。D— |
| F4 | response_format | `response_format`(anyOf 格式配置 Type+JSONSchema 等)(yaml@4929) | —（messages 用 thinking/output_config 替代，body 无 response_format） | **无独立 response_format 字段**——经 `text.format` 表达(yaml:ResponsesRequest.text@13195) | `llm.Request.ResponseFormat`(Type+JSONSchema) | C:DIRECT(inbound_convert.go:110-114 取 Type+JSONSchema→outbound_convert.go:85-89 还原)；R:**RENAME**——入站把 text.format.Type+重建 JSONSchema(name/desc/schema/strict)写入 canonical.ResponseFormat(inbound.go:283-300)，出站 convertToTextOptions 反向还原(outbound_convert.go:15-47)；M:N/A | ✅ | chat↔responses 经 canonical.ResponseFormat 中转，json_schema 四子字段双向可逆(已实测闭合，呼应 v2 纠正)。D— |
| F5 | text(TextExtendedConfig:format+verbosity) | —（chat 顶层无 text 对象；但有平铺 `verbosity`) | — | `text`($ref TextExtendedConfig{format,verbosity})(yaml@13195/$ref@14072 区) | `ResponseFormat`(format 部分)+`llm.Request.Verbosity`(verbosity) | R:RENAME——format→ResponseFormat(@283-300)、verbosity→Verbosity(@303-304)，出站 convertToTextOptions 合并还原(outbound.go:254)；C:verbosity 平铺 DIRECT(openai/model.go:97→inbound_convert.go:67→outbound_convert.go:39)，text 对象本身 N/A；M:N/A | ✅(基准确认见自检②) | responses text 双向闭合(RENAME 非 DROP，v2 已纠正)；chat 仅承 verbosity。**M→Anthropic 方向**:`canonical.Verbosity`(`llm/model.go:223`)在 anthropic transformer 全目录零读写(`rg Verbosity llm/transformer/anthropic/`=0 命中)=静默丢,native Anthropic 无对应概念,**[业务决策2026-06-30 维持默认丢弃]**。D— |
| F6 | logprobs | `logprobs`(boolean)(yaml@4790) | —（请求体无；12969 为响应内容块子 schema，非 body） | —（请求体无 bool 开关；仅有 top_logprobs int@13252） | `llm.Request.Logprobs` | C:DIRECT(inbound_convert.go:46→outbound_convert.go:18)；M/R:N/A(spec 该协议无此请求字段) | ✅ | chat-only 同名直转；msg/resp 该概念不存在故 N/A。D— |
| F7 | top_logprobs | `top_logprobs`(integer 0-20)(yaml@5020) | —（无） | `top_logprobs`(integer nullable)(yaml@13252) | `llm.Request.TopLogprobs` | C:DIRECT(inbound_convert.go:53→outbound_convert.go:25)；R:DIRECT(inbound.go:182→outbound.go:264)；M:N/A | ✅ | 设 top_logprobs 即隐含启用日志概率(spec 口径)。D— |
| F8 | provider | `provider`($ref ProviderPreferences)(yaml@4882) | `provider`($ref ProviderPreferences)(yaml@9076) | `provider`($ref ProviderPreferences)(yaml@13157) | 无(canonical 无 Provider 槽) | C/M/R 全部 DROP——三包 Request 结构体零声明 `json:"provider"`(rg 核查 anthropic/root/responses 非测试 Go 均 0 命中)，lenient Unmarshal 静默吞(D20)；服务端渠道选上游替代客户端 provider 偏好(设计性替换) | ✅[决] | **[业务决策2026-06-30 维持默认]** 设计性替换 NOT_FOUND/DROP:AxonHub 不对外承诺完整 OR 平台路由兼容,服务端 channel/select_candidates 接管选路,客户端 provider/order/allow_fallbacks 偏好按 lenient Unmarshal 静默吞(D20)。边界明确不再列为待办;如未来口径变更需补 reject/warn。关联 D20 |
| F9 | route | `route`($ref DeprecatedRoute,deprecated)(yaml@4947) | `route`($ref DeprecatedRoute)(yaml@9078) | `route`($ref DeprecatedRoute)(yaml@13161) | 无 | C/M/R 全部 DROP(同 F8，结构体零声明，D20)；已被服务端 candidate 选择取代 | ✅[决] | **[业务决策2026-06-30 维持默认]** deprecated 字段静默丢;影响低,不实现 deprecation warn。关联 D20 |
| F10 | models(候选模型列表) | `models`($ref ChatModelNames)(yaml@4843) | `models`(array string)(yaml@9044) | `models`(array string)(yaml@13112) | 无(struct 仅单数 Model) | C/M/R 全部 DROP——结构体仅 `Model` 单数，无复数 `Models`；client fallback 列表不被采用(select_candidates.go 服务端选择替代) | ✅[决] | **[业务决策2026-06-30 维持默认]** 设计性替换;多模型冗余意图由服务端 candidate 体系承接,不保留 client fallback 列表。关联 D20 |
| F11 | plugins | `plugins`(array 插件配置 auto-router/moderation/web…)(yaml@4850) | `plugins`(同)(yaml@9050) | `plugins`(同)(yaml@13119) | 无 | C/M/R 全部 DROP(结构体零声明，D20)；plugin transforms 不生效 | ⚠️ | 功能缺口(plugin 变换链失效)。关联 D20、P2 |
| F12 | trace | `trace`($ref TraceConfig)(yaml@5031) | `trace`($ref TraceConfig)(yaml@9357) | `trace`($ref TraceConfig)(yaml@13259) | 无 | C/M/R 全部 DROP(结构体零声明，D20)；追踪配置丢失 | ⚠️ | 可观测性 hint 丢失。关联 D20、P2 |
| F13 | debug | `debug`($ref ChatDebugOptions)(yaml@4771) | —（messages 体无 debug） | `debug`($ref ChatDebugOptions)(yaml@13076) | 无 | C/R DROP(结构体零声明，D20)；M:N/A(spec 无) | ⚠️ | 调试开关未实现。关联 D20、P3 |
| F14 | image_config | `image_config`($ref ImageConfig)(yaml@4779) | —（messages 体无） | `image_config`($ref ImageConfig)(yaml@13082) | 无 | C/R DROP(结构体零声明，D20)；图像生成配置仅在 tools[].image_generation 子结构内承载；M:N/A | ⚠️ | 顶层图像配置丢失(下沉到 tool 子结构另计)。关联 D20、P2 |
| F15 | background | —（chat 体无） | —（messages 体无） | `background`(boolean nullable)(yaml@13071) | 无标量槽→TransformerMetadata["background"] | R:PASSTHROUGH(inbound.go:213-214 存 *bool→outbound.go:276 xmap.GetBoolPtr 回填)双向闭合；C/M:N/A | ✅ | 非 canonical 标量槽，经元数据往返；请求侧命运合规。(响应侧回显缺另有 D10) |
| F16 | include | —（chat 体无） | —（messages 体无） | `include`(array $ref ResponseIncludesEnum)(yaml@13084) | 无标量槽→TransformerMetadata["include"] | R:PASSTHROUGH(inbound.go:195-196 存 []string→outbound.go:272 xmap.GetStringSlice 回填)双向闭合；C/M:N/A | ✅ | 元数据透传合规。D— |
| F17 | truncation | —（chat 体无） | —（messages 体无） | `truncation`($ref OpenAIResponsesTruncation)(yaml@13262) | 无标量槽→TransformerMetadata["truncation"] | R:PASSTHROUGH(inbound.go:207-208 存 *string→outbound.go:275 xmap.GetStringPtr 回填)双向闭合；C/M:N/A | ✅ | 请求侧命运合规；(响应侧回显缺 D10) |
| F18 | max_tool_calls | —（chat 体无；14049 属 SubagentTool 嵌套定义，非主请求体） | —（messages 体无） | `max_tool_calls`(integer nullable)(yaml@13097) | 无标量槽→TransformerMetadata["max_tool_calls"] | R:PASSTHROUGH(inbound.go:199-200 存 *int64→outbound.go:273 xmap.GetInt64Ptr 回填)双向闭合；C/M:N/A | ✅ | 元数据透传合规。D— |
| F19 | prompt(stored prompt template 引用) | —（chat 体无） | —（messages 体无） | `prompt`($ref StoredPromptTemplate)(yaml@13152) | 无 | R:**真 DROP**——`responses/model.go:134-136` Prompt 字段被注释(`// TODO // Prompt *Prompt`)，整函数(convertToLLMRequest inbound.go:169-312)无 req.Prompt 引用，lenient Unmarshal 吞掉(D20)；C/M:N/A | ❌ | prompt 模板引用完全丢失(prompt/id/version/variables)。要么实现解析(结构体已存 type Prompt @model.go:170 但字段未接线)，要么明确报错而非静默吞。关联 D20、P2 |
| F20 | fallbacks | —（chat 体无顶层 fallbacks） | `fallbacks`(array $ref MessagesFallbackParam,max 3,cannot combine with models)(yaml@9021) | —（resp 体无） | 无 | M:DROP(MessageRequest 无 Fallbacks 字段,D20)；OR 多模型路由语义,AxonHub 以服务端 channel/candidate 替代(设计性替换)；C/R:N/A | ✅[决] | **[业务决策2026-06-30 维持默认]** 冗余兜底列表不被采用;models 与 fallbacks 均 DROP 故二者互斥约束无需校验。关联 D20 |
| F21 | context_management | —（chat 体无） | `context_management`(obj edits clear_tool_uses/compact 等)(yaml@8936) | —（resp 体无） | 无 | M:DROP(MessageRequest 无 ContextManagement 字段,D20)；上下文压缩策略丢失，可能致超长对话未被裁剪而触发限流/截断差异；C/R:N/A | ⚠️ | 功能性损失较纯路由更实质。应建模或透传至支持的上游，至少白名单告警。关联 D20、**P1** |
| F22 | speed | —（chat 体无） | `speed`($ref AnthropicSpeed fast/standard)(yaml@9086) | —（resp 体无） | 无 | M:DROP(MessageRequest 无 Speed 字段,D20)；fast 档推理/计费配置丢失；C/R:N/A | ⚠️ | 定价与速度档位偏好丢失。关联 D20、P2 |
| F23 | stop_server_tools_when | `stop_server_tools_when`($ref StopServerToolsWhen)(yaml@4982) | `stop_server_tools_when`($ref StopServerToolsWhen)(yaml@9095) | `stop_server_tools_when`($ref StopServerToolsWhen)(yaml@13182) | 无 | C/M/R 全部 DROP(三包结构体零声明，D20)；服务端工具停控条件失效 | ⚠️ | 工具流终止控制丢失。关联 D20、P2 |

### 本区自检

**方法学核对**：每字段均经 MCP(search_graph/get_code_snippet 定位符号 → 源码精读取证 → min.yaml 查官方名/类型/描述)三步交叉，禁止脚本批量推断。所有 file:line 引用均为当前 HEAD 实测值(非沿用旧审计近似值)：responses 入站点 inbound.go:{175,182,184,195-196,199-200,207-208,213-214,241-245,283-305} 与出站点 outbound.go:{253,254,264,266,268,272-276}/outbound_convert.go:{15-47,488-501} 逐一比对吻合；chat 入站点 inbound_convert.go:{46,53,60,65,67,84-87,110-114} 与出站点 outbound_convert.go:{18,25,32,37,39,56-59,85-89} 吻合；anthropic 入站点 inbound_convert.go:{60}(Stream) 与出站点 outbound_convert.go:{118} 吻合。canonical `StreamOptions` 仅含 IncludeUsage(llm/model.go:290-296)是 include_obfuscation 必须经 TransformerMetadata 透传的根因，已验证。

**类目计数(按概念行级主类目)**：
- DIRECT-pure：4(F1 stream、F3 modalities、F6 logprobs、F7 top_logprobs)
- PASSTHROUGH：5(F2a include_obfuscation、F15 background、F16 include、F17 truncation、F18 max_tool_calls)——均为 responses 独有的 TransformerMetadata 往返字段，双向闭合均已证实
- RENAME：1 族(F4 response_format ↔ F5 text.format 经 canonical.ResponseFormat 中转的双向重构，json_schema name/description/schema/strict 四子字段可逆)
- DROP：12(F8 provider、F9 route、F10 models、F11 plugins、F12 trace、F13 debug、F14 image_config、F19 prompt、F20 fallbacks、F21 context_management、F22 speed、F23 stop_server_tools_when)
- 跨协议分流(mixed，同一概念在不同协议载体不同故分属两类)：3(F2 stream_options=chat DIRECT-subset / resp PASSTHROUGH；F4 response_format=chat DIRECT / resp RENAME；F5 text=resp RENAME / chat verbosity DIRECT)
- MERGE：0；NOT_FOUND：0(凡 spec 有定义者均有明确去向判定，未留悬空)

**严重度分布**：P0=0；P1=1(F21 context_management)；P2=10(F8/F9/F10/F11/F12/F14/F19/F20/F22/F23)；P3=3(F2 subset、F2a baseline、F13 debug)
> 🔖 **2026-06-30 业务决议增量**:F8/F9/F10/F20 由 ⚠ 升 ✅[决](维持默认 DROP,见各行);上方聚合计数为复核前快照尚未刷新,故仍含此四项于 P2 计数,实际待修已减4。verbosity(#7)→Anthropic 默认丢经核实合规并已并入 F5 行。

**待复核项(共 3，绝不臆测)**：
1. **responses.stream_options / include_obfuscation 未出现于 min.yaml 基准**：awk 扫描 ResponsesRequest(13051–13267)属性块仅命中 `text@13195`，无 stream_options/include_obfuscation 键；但 AxonHub `responses/model.go:148/191` 显式建模并透传。来源待定——可能是 baseline 抓取遗漏，也可能是上游未文档化的 OpenRouter/OpenAI 扩展。需对照 openrouter-openapi.yaml 及上游 changelog 复核。当前透传行为无害，不影响功能，P3。
2. **chat 顶层 `verbosity` 未列于 min.yaml ChatRequest 属性块**：anchors 扫描未见 `^        verbosity:$` 于 chat 区(仅 14081 在 TextConfig 内)；但 AxonHub `openai/model.go:97` 接受并 DIRECT 往返。OpenAI 官方 chat completions 已支持 verbosity，疑基准滞后。需确认是否应补入基准，P3。
3. **【已结案 2026-06-30】** provider/route/models/fallbacks(F8/F9/F10/F20)+ #7 verbosity→Anthropic:用户拍板维持默认丢弃——AxonHub 不对外承诺完整 OR 平台路由兼容,服务端 channel/select_candidates 接管选路,lenient Unmarshal 静默吞(D20);native Anthropic 无 verbosity 概念故跨格式至 anthropic 端到端丢判合规。合规判定均已升 ✅[决]。源码证据:`llm.Request`(canonical)无 Provider/Route/Fallbacks 字段、struct 仅单数 Model 无复数 Models;`rg Verbosity llm/transformer/anthropic/`=0 命中。详见各行。(此项先前曾标 ⚠️ 待业务口径,现已撤销)

**附带说明(本区外比较器，不计入计数)**：用户提示中的 DROP 比较 example `cache_control(chat侧)`/`top_k(chat侧)` 经核属实——`openai/inbound_convert.go` 无 cache_control/top_k 映射、`openai/model.go` Request(15-98)亦无 TopK 字段，均落入 D20 盲区；但二者分别归属缓存族/采样参数族(他区管辖)，本区不立项，仅此处交叉印证 D20 模式的普遍性。同理 anthropic 侧 `MessageRequest` 确有 `TopK`(model.go:43)与 `CacheControl`(model.go:102)，方向相反(messages 收而 chat 丢)，佐证各协议建模不对齐。

**闭环结论**：第5区核心命题成立——background/include/truncation/max_tool_calls/stream_options.include_obfuscation 五项确为 PASSTHROUGH(TransformerMetadata 往返，已逐条双向取证)；prompt(image_config/debug/cache_control/chat侧 top_k 等)确为真 DROP(D20)；provider/route/models/fallbacks 为设计性替换 NOT_FOUND/DROP；response_format ↔ responses.text.format 经 canonical.ResponseFormat 重构 JSONSchema 后双向闭合(RENAME，v2 纠正无误)。唯一硬伤为 F19 prompt 字段被注释导致模板引用彻底丢失(❌)，及 F21 context_management 的功能性损失(P1)。


## 2026-06-30 fork-agent(Noether)复核增订（主线程代笔）

> 本节由主线程代为落盘(Noether 子代理最终消息 `completed:null` 回传异常，磁盘未见其自行写入)。**本节结论 supersede 上方表格中对应格子的旧判定，遇冲突以本节为准。**

### 方法
全程 get_code_snippet／rg 直读实时源码 + min.yaml 枚举双证(spec-audit-method 四判据④)；图谱关系边作废。

### 逐格新定性

| 格号 | 旧判定 | 新定性 | 源码证据(file:line 实测 HEAD=812c9077) | min.yaml 行号 | 被 fix 改变 |
|---|---|---|---|---|---|
| F2 stream_options 子集不对称 | ⚠️ | ⚠️仍悬 | responses 出站 `convertStreamOptions`(outbound_convert.go:486-502) 当 `metadata["include_obfuscation"]==nil` 即 return nil，丢弃 canonical.StreamOptions.IncludeUsage → 跨协议 usage 双向不闭合 | ChatRequest.stream_options@4989 | 否 |
| F2a include_obfuscation 透传 | ✅(自称基准确认) | **⚠️降级—闭环成立但规范缺证** | 入站 inbound.go:244 存 metadata→出站 outbound_convert.go:493 回填 `&StreamOptions{IncludeObfuscation}` 单向闭合成立；但 min.yaml ResponsesRequest 属性块实测无 stream_options/include_obfuscation 键(awk 扫 13064-13273 仅 background/include/max_tool_calls/truncation)，四判据④双证缺失 | 无(min.yaml 未列) | 否 |
| F8 provider 设计性替换 | ⚠️ | 🟡业务决策 | 三包+openrouter 包全树 `json:"provider"` 零声明(lenient 吞,D20)；canonical llm.Request 无 Provider 槽(仅 ProviderExtensions 边车@287)；服务端 select_candidates 替代选路 | provider@4882 | 否(不在A类21项内) |
| F9 route(deprecated) | ⚠️ | 🟡业务决策 | 同上零声明；弃用键静默丢，建议加 deprecation warn | route@4947 | 否 |
| F10 models 候选列表 | ⚠️ | 🟡业务决策 | 结构体仅单数 Model，无复数 Models；fallback 列表不被采用(select_candidates 接管)。合理性取决于是否承诺 OR 多模型路由兼容 | models@4843 等 | 否 |
| F11 plugins | ⚠️ | ⚠️仍悬(功能缺口) | 全树零声明；AxonHub 无 plugin 变换链实现 | plugins@4850 等 | 否 |
| F12 trace | ⚠️ | ⚠️仍悬(可观测 hint 丢) | 全树零声明 | trace@5031 等 | 否 |
| F13 debug | ⚠️ | ⚠️仍悬(P3 低值) | 全树零声明；messages 体本就 N/A | debug@4771 | 否 |
| F14 image_config | ⚠️ | ⚠️仍悬(顶层下沉 tool 子结构) | 全树零声明；图像配置仅在 Tool.image_generation 内承载 | image_config@4779 | 否 |
| F19 prompt 模板引用 | ❌ | **❌坐实(仍未修)** | 主 Request 第 134-136 行仍是 `// TODO // Prompt *Prompt json:"prompt,omitempty"` 注释态；type Prompt 已定义(model.go:170) 却未接线；convertToLLMRequest 全程不引 req.Prompt；注：Response 结构体(line 783 起)却有活跃 Prompt 字段@841 → 半成品迹象，倾向“未完工”而非有意丢弃 | prompt@13152 | **未被 fix 触碰**(diff 仅加 Tools.Tools/FrequencyPenalty/PresencePenalty/Modalities/InputAudio) |
| F20 fallbacks | ⚠️ | 🟡业务决策 | anthropic MessageRequest 无 Fallbacks 字段(rg 验证)；与 models 互斥约束亦无从校验 | fallbacks@9021 | 否 |
| F21 context_management | ⚠️P1 | **⚠️仍悬(P1 硬伤未解)** | anthropic/model.go 无 ContextManagement/Speeed/Speed 任一字段声明(rg 零命中)；上下文压缩策略丢失，超长对话可能触发上游限流差异 | context_management@8936 | 否 |
| F22 speed | ⚠️ | ⚠️仍悬(fast 档计费偏好丢) | anthropic 包无 Speed 字段(rg 零命中)；TopK@43/ServiceTier@66 存在但 Speed 缺失 | speed@9086 | 否 |
| F23 stop_server_tools_when | ⚠️ | ⚠️仍悬(工具流终止控制失效) | 全树零声明 | stop_server_tools_when@4982 等 | 否 |

### 额外发现(part-F 漏列，本次补登)
- **prompt_cache_retention 也是完整双向透传**：入站 inbound.go:204 写 TransformerMetadata["prompt_cache_retention"] → 出站 outbound.go:274 GetStringPtr 还原。行为正确，但 part-F 表格未立项，属文档遗漏非代码缺陷。
- **convertStreamOptions 提前返回风险**：outbound_convert.go:486-502 当 `metadata["include_obfuscation"]==nil` 即 `return nil`，导致即便客户端发了合法的 responses stream_options(无 obfuscation 子键) 也整体不发 stream_options 对象——放大 F2 的跨协议不对称，修复 F2 时须一并处理。

### 统计
翻盘 1(F2a 由 ✅ 自称基准降为 ⚠️ 规范缺证，因四判据④要求源码赋值点+min.yaml 枚举双证并存而后者实测不存在)／坐实 9(F2/F11/F12/F13/F14/F19/F21/F22/F23 旧判定方向经实时取证维持不变)／仍悬 9／业务决策挂起 0(**F8/F9/F10/F20 已于 2026-06-30 决议维持默认 DROP,升 ✅[决]**)。

### 业务口径裁定依据
核实依据：新增 `llm/transformer/openrouter/{aggregator,model,outbound}.go` 仅有 outbound 无 inbound，证明 AxonHub 不暴露 OpenRouter 客户端入口，故 F8/F9/F10/F20 按“设计性替换”文档化标注较稳妥，**已于 2026-06-30 经用户拍板采纳「维持默认 DROP」分支(不实现 OR 平台路由字段解析),各格升 ✅[决],无需再等产品口径**。如未来对外宣称完整 OR 兼容,届时须升级为必修解析或 reject-with-warn。

### 未决事项落盘建议(写入 docs/specs/未决事项清单.md)
1. F2a include_obfuscation 来源归属(OpenRouter 未公开/OR 私有扩展/AxonHub 自造)需对照 openrouter-openapi.yaml 全量及上游 changelog 复核——当前透传无害但命名权属不明。
2. F8/F9/F10/F20 四项需产品口径裁定：“AxonHub 定位为三协议网关且路由由服务端接管” **【已结案 2026-06-30】采纳「维持现状」分支**:F8/F9/F10/F20 维持默认 DROP 并已在各行升 ✅[决];README 补说明列 follow-up。原条件句失效。
3. F19 半成品判断(Response 有活跃 Prompt 而 Request 注释)：应问作者是有意搁置还是遗留 TODO。

### 未覆盖诚实标注
任务提及的四条跨区项(#7 verbosity 跨格式丢 ／ #10 session_id body 变体 D20 ／ #11 cache_control 顶层 chat/responses DROP ／ #13 user 身份桥接 P1)归他区管辖，已被 D 区 Pasteur 取证坐实(见 part-D 增订章节)，本区不再重复定性。
