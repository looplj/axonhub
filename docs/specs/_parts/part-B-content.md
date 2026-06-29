# 第2区 · 内容载体（messages / input / system / instructions / metadata）

> 审计范围：三协议 transformer 入站(client→canonical)+出站(canonical→upstream wire)。
> 规范基准：`docs/specs/openrouter-chat-messages-responses.min.yaml`（Chat=`ChatRequest`:4757、Anthropic Messages=`MessagesRequest`:8924/`MessagesMessageParam`:8679、Responses=`ResponsesRequest`:13051/`Inputs`:7798）。
> canonical 对照：`llm.Request.Messages []Message`(model.go:42)、`Message`(model.go:342 Role/Content/ToolCalls…)、`Metadata map[string]string`(model.go:162)；**无独立 System/Instruction 槽**，统一以 `role=="system"` 承载。
> 方法：纯人工逐格 MCP 核实(search_graph/get_code_snippet/trace_path/query_graph)，禁止脚本批量推断。

| # | 概念 | Chat名(类型) | Messages名(类型) | Responses名(类型) | Canonical槽 | 作者现状[类别](证据) | 合规判定 | 正确做法 & 关联D编号 |
|---|---|---|---|---|---|---|---|---|
| B2-01 | messages 会话主体 | messages(array<ChatMessages>, required minItems1)(yaml:4856) | messages(array<MessagesMessageParam>, nullable required)(yaml:9033) | —(无顶层 messages；由 input 承载见 B2-02) | Request.Messages []Message(model.go:42); Message.Role/Content/ToolCalls(model.go:342+) | [DIRECT] chat 入站 ToLLMRequest lo.Map m.ToLLMMessage() 按 role 直映(openai/inbound_convert.go:71)；chat 出站 RequestFromLLM 同向还原(openai/outbound_convert.go:43)。anthropic 入站逐条转 llm.Message 保留原 role(user/assistant)(inbound_convert.go:122 起)；anthropic 出站 convertMessages 过滤 system/developer 后按 user/tool/assistant 还原(outbound_convert.go:332/336)。三方均落到 canonical.Messages 且角色保真。 | ✅ | 维持现状；同协议双向对称，无需改。不涉 D 编号。 |
| B2-02 | input 输入载体(responses 独有) | — | — | input(Inputs: string/array<EasyInputMessage/InputMessageItem/FunctionCallItem…>)非必填(yaml:13054/7798) | 展开并入 Request.Messages | [MERGE] 入站 convertInputToMessages：Text→单条 user 消息；数组逐项 convertItemToMessage(reasoning 特殊合并后续簇)(responses/inbound.go:338-392)，由 convertToLLMRequest 在 instructions 之后 append(:296-303)，ArrayInputs 仅在 Items!=nil 时置 true(:300-302)。出站 convertInputFromMessages switch 仅 case user/developer/assistant/tool，**缺 system 分支→system 消息从 output.items 丢弃**(outbound_convert.go:114-148)，靠 instructions 接管回收；单条简单内容且非 array 格式回退 Text 字符串形式(:104-107)。 | ✅ 结构闭合 | input↔Messages 往返闭合(system 经 instructions 路径回收)。维持现状。仅响应对象层缺口关联 D10。 |
| B2-03 | system 系统提示词(msg 独立顶层；chat 内嵌 messages[0]) | —(无独立字段；规范以 messages[].role=system 即 ChatSystemMessage 承载,yaml:4686 discriminator) | system(anyOf<string/array<AnthropicTextBlockParam>> 可选)(yaml:9099) | —(无独立 system；等义走 instructions 见 B2-04) | 无独立槽→Messages 中 role=="system"/"developer" | [MERGE] anthropic 入站：Prompt(string)→1 条 system 消息；MultiplePrompts(array)→N 条 system 消息并置 ArrayInstructions=true(inbound_convert.go:95-118)。anthropic 出站 convertToAnthropicSystemPrompt 收集 system+developer，依 ArrayInstructions 与数量还原 Prompt 单字符串或 MultiplePrompts 数组(outbound_convert.go:812-869)；convertMessages 同时过滤 system/developer 防 duplicate(:336)。chat 两端直接以 role 保真透传。 | ✅ 同协议往返格式保持(string↔string/array↔array) | 维持。跨协议位置差异已被 canonical 统一吸收，无丢映射。不涉 D 编号。 |
| B2-04 | instructions 开发者指令(responses 独有) | — | — | instructions(nullable string)(yaml:13091) | 无独立槽→入站注入 Messages[0]{role:"system"}；出站从全部 role==system 文本拼回 | [MERGE] 入站 convertToLLMRequest：if req.Instructions!="" 前插一条 role=system 消息 Content=&Instructions(responses/inbound.go:250-258)。出站 TransformRequest 同时设 Instructions=convertInstructionsFromMessages()(扫描全部 system 消息取 text 以 \n 拼,outbound_convert.go:52-89)与 Input=convertInputFromMessages(...)(drop system)(responses/outbound.go:249-250)。请求转发路径往返闭合。 | ✅ 请求转发闭合 ❗响应结果对象不回显 instructions | 上游请求重建已正确双向；仅需修 BaseResponsesResult 回显缺失(**D10**,P1)。另：Instructions json tag 无 omitempty(responses/model.go:98)会恒输出空串""，spec 允许 null/省略——低优(P2)⚠️待 native 校验。 |
| └B2-05a | metadata 映射主链 | metadata(map<string,string> additionalProperties:string,max16/64键/512值)(yaml:4814) | metadata({user_id:string})(yaml:9036) | metadata($ref RequestMetadata=map<string,string>)(yaml:13100) | Request.Metadata map[string]string(model.go:162) | [PASSTHROUGH] chat 入 Metadata=r.Metadata(openai/inbound_convert.go:59)/出 r.Metadata=openai outbound_convert.go:31 直传；responses 入 maps.Clone(req.Metadata)(inbound.go:176)/出 payload.Metadata=llmReq.Metadata(outbound.go:252)直传。chat/responses/canonical 三者同为 map[string]string 主链无损。 | ✅ | 维持。主链合规且可逆。不涉 D 编号。 |
| └└B2-05b | metadata 解码鲁棒性不对称(高危子项) | 同上 | 同上 | 同上 | 同上 | [DROP] anthropic 入仅写 ["user_id"]=UserID(inbound_convert.go:73)、出仅 AnthropicMetadata{UserID}(outbound_convert.go:128-129)，其余键 DROP。三者入站统一标准 json.Unmarshal(chat openai/inbound.go:TransformRequest / responses inbound.go:54 / anthropic inbound.go:TransformRequest)，均未设 DisallowUnknownFields。Go 语义：(a) chat/responses 因值为 map[string]string，客户端发非字符串值(数字/对象/数组)触发类型错误致整包解码失败(spec 要求 string 故属合规拒绝，但硬失败而非优雅降级)；(b) anthropic *struct* 解码对 user_id 以外额外键静默丢弃、不会因其存在报错(仅当 user_id 自身类型不符才报错)；(c) 跨协议路由 chat→anthropic 时除 user_id 外元数据被设计性丢弃。 | ⚠️ 行为不对称(非违规) | 不新增 canonical 顶层字段。建议两端策略统一：一律严格拒收非法 metadata 返回明确400 或 一律宽松跳过坏键。当前三协议不一致系一致性问题(P2)。responses-spec-audit 未覆盖 metadata→无对应 D 编号，列新观察⚠️待定级。 |
| └└B2-06 | developer 角色跨协议往返保真 | messages[].role∈{system,user,developer,assistant,tool} | messages[].role∈{user,assistant,system(+x-speakeasy allow)}(yaml:8724)；原生 Anthropic 无 developer | input[]消息项 role∈{user,system,assistant,developer}(EasyInputMessage yaml:6358 / InputMessageItem yaml:7783) | Message.Role 取 "developer" | [PASSTHROUGH] 入站 chat `ToLLMMessage` 首句 `Role: m.Role`(openai/inbound_convert.go:124)、responses `convertItemToMessage`(type=="message")`Role: item.Role`(responses/inbound.go:498)均原样透传,developer 进 canonical 无损;Anthropic 原生无该角色 N/A。出站 chat 直传 developer(ChatRequest enum 含 developer,yaml:4472);responses `convertInputFromMessages` case"user","developer"共用 `convertUserMessage`,但其返回 `Item{Type:"message", Role: msg.Role}`(outbound_convert.go:188)**仍透传 developer**(函数名误导,行为无损),且 spec EasyInputMessage(yaml:6362)/InputMessageItem(yaml:7789)enum 显式允许 developer;anthropic 出站将 developer 并入 SystemPrompt(role→system)(outbound_convert.go:812-869)系其无 developer 概念下的合理适配。另 orchestrator 渠道开关 `ReplaceDeveloperRoleWithSystem`(默认关,transform_options.go:14)可为不支持 developer 的老模型强制降级。 | ✅ 已核实·非缺陷 | 维持现状。三方向往返角色保真、各出口均符目标协议能力与 spec 枚举;不新增 canonical 字段,不涉 D 编号。(2026-06-29 MCP search_graph/get_code_snippet 闭环) |

## 本区自检

- 共 7 数据行(B2-01~B2-04 主概念 4 行 + B2-05a/B2-05b/B2-06 高危子项 3 行)。
- 类别计数：DIRECT×1(B2-01)、MERGE×3(B2-02/B2-03/B2-04)、PASSTHROUGH×2(B2-05a/B2-06)、DROP×1(B2-05b)、RENAME×0、NOT_FOUND×0。
- ⚠️ 待复核清单共 2 项：
  1. B2-04 — Instructions 空""恒输出(json tag 无 omitempty)是否被原生 OpenAI Responses 拒绝；spec 允许 null/省略。(缺 native 报文校验)
  2. B2-05b — metadata 三协议解码行为不对称(硬失败 vs 优雅丢弃)的“缺陷”定性未决；硬失败本身符合 spec 故暂判非违规，是否升级为一例性整改项待确认。
- ✅ 已闭环(2026-06-29 MCP 复核)：B2-06 判定非缺陷——入站两端 Role 原样透传(openai/inbound_convert.go:124、responses/inbound.go:498);出端 chat/responses 同样透传,responses 出站 convertUserMessage 实为 Role: msg.Role(outbound_convert.go:188,函数名误导但行为无损),anthropic 出站并入 system 属协议无 developer 概念下的合理适配;OpenRouter spec EasyInputMessage(yaml:6362)/InputMessageItem(yaml:7789)enum 显式允许 developer。原表“responses 当 user 发/三种归宿不一”判断作废。
- 已闭环结论(高置信)：input/system/instructions 的同协议请求级往返均结构闭合(format-preserving)；metadata 主链 chat/responses 双向无损、anthropic 受 spec 限制仅留 user_id 属预期。
- 关联既有 D 编号：仅 **D10**(instructions 响应对象回显缺失,P1)；metadata/developer 为本区新观察,responses-spec-audit 尚无对应 D 编号。

## 2026-06-30 主线程复核增订（B 区自理）

> 本节为 B 区 B2-04／B2-05b 两项此前标「待复核」格子的双证闭环结果。**本节结论 supersede 上方对应旧判定，遇冲突以本节为准。**（B2-06 developer 角色已于 2026-06-29 早前结案为 ✅ 非缺陷，见上方表格该行，不再赘述。）

### 方法
`get_code_snippet`/rg 直读实时源码 + min.yaml 枚举(`spec-audit-method.md` 四判据④)；图谱关系边作废。

### 逐格新定性

| 格号 | 旧判定 | 新定性 | 源码证据(file:line) | min.yaml 行号 |
|---|---|---|---|---|
| B2-04 instructions 空""恒输出(json tag 无 omitempty) | ⚠️待复核(缺 native 报文校验) | **维持 ⚠️ P2**(合规但不优雅) | `responses/model.go:98` `Instructions string \`json:"instructions"\``(值类型无 omitempty → 客户端未传亦恒发空串 ""); 同文件 :805 另有 `*string`+omitempty 版本(出入站归属辨明留 follow-up) | ResponsesRequest.instructions@13091 `nullable:true type:string` → 空串属合法 string 值故技术合规、仅冗余不优雅。建议日后改 `*string`+omitempty；是否被原生 OpenAI 拒仍待真实报文验证 |
| B2-05b metadata 解码鲁棒性不对称 | ⚠️行为不对称(非违规)P2 待定级 | **维持 ⚠️ P2**(三方各符其 spec，不对称 = 不同合规策略) | chat `openai/model.go:65` 与 responses `model.go:122/:808` 均 `map[string]string`(强类型 → 非串值触发整包 400)；anthropic 用 `AnthropicMetadata{UserID}` 结构(lenient 吞额外键)，`outbound_convert.go` 仅取 `Metadata["user_id"]` 其余 DROP | ChatRequest.metadata@4816 `additionalProperties:{type:string}` 与 map&lt;string,string&gt;一致；MessagesRequest.metadata@9039 仅 `properties.user_id` 无 additionalProperties(struct 对齐)；ResponsesRequest `$ref RequestMetadata` |

### 结论
两项均维持原 ⚠️ P2 定性：技术上各自符合所对应协议规范。「不对称」属实(chat/responses 因强类型对非法值硬失败、anthropic 因弱类型 struct 静默吞额外键)，但双方都**不构成违规**——前者 spec 本就要求 all-string 故拒绝正确，后者 spec 本就没定义额外键故丢弃也对。是否升级为一例性整改(一律严格返回明确 400 或一律宽松跳过坏键)属业务取舍，代码层面无需强制改动。

### 诚实声明(per AGENTS.md §5 分开写)
- 未做运行时报文验证(native OpenAI 是否拒空串 instructions ／实际跨格式路由下多键 metadata 的线上命运)；
- `responses/model.go:98` 与 `:805` 两个 `Instructions` 定义所属 struct 的精确归属(outbound wire payload 还是 inbound parse model)未在本棒逐一打开核对，不影响「合规但可优化」之定性，留给后续修复阶段定位 patch 点时确认。
