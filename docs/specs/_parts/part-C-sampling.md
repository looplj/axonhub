## 第3区 · 采样与限长

> 规范基准:`docs/specs/openrouter-chat-messages-responses.min.yaml`(三请求 schema 锚点 `ChatRequest`@yaml:4757、`MessagesRequest`@yaml:8924、`ResponsesRequest`@yaml:13051)。
> canonical 对照:`llm/model.go`(`Request` 结构体)。**canonical 无 TopK / RepetitionPenalty / MinP / TopA 字段**,故此类参数须走 `TransformerMetadata` 通道(参 ADR-D1 思路)。
> 类别图例:DIRECT 直传闭合 / RENAME 改名往返闭合 / MERGE 多名归一槽 / PASSTHROUGH 元数据透传 / DROP 已收而失 / NOT_FOUND 协议侧本无此概念。⑦=流式与非流式路径说明。

| # | 概念 | Chat名(类型) | Messages名(类型) | Responses名(类型) | Canonical槽 | 作者现状[类别](证据) | 合规判定 | 正确做法 & 关联D编号 |
|---|---|---|---|---|---|---|---|---|
| C1 | temperature | temperature(number·0–2)(yaml:4991) | temperature(number·double)(yaml:9105) | temperature(number·double)(yaml:13191) | Temperature *float64(model.go:104) | 三协议入站直拷同槽、出站直发同名。<br>a)chat 入 inbound_convert.go:53 / 出 outbound_convert.go:24<br>b)msg 入 anthropic/inbound_convert.go:58 / 出 anthropic/outbound_convert.go:116<br>c)resp 入 responses/inbound.go:172 / 出 responses/outbound.go:261<br>[DIRECT] ⑦均在请求 init 阶段一次性写入,聚合器/aggregator 不触及采样参数;流式非流式同路径。 | ✅ | 无需改动。值全程存活且数值原样不裁剪(spec 范围校验未实现但属 pass-through 保真,不计缺陷)。 |
| C2 | top_p | top_p(number·0–1)(yaml:5025) | top_p(number·double)(yaml:9354) | top_p(number·double)(yaml:13255) | TopP *float64(model.go:116) | 同 C1 对称直传。<br>a)chat 入 openai/model.go:50→inbound_convert.go:55 / 出 outbound_convert.go:26<br>b)msg 入 anthropic/model.go:54→anthropic/inbound_convert.go:59 / 出 anthropic/outbound_convert.go:117<br>c)resp 入 responses/model.go:163→responses/inbound.go:183 / 出 responses/outbound.go:271<br>[DIRECT] ⑦同 C1。 | ✅ | 无需改动。 |
| C3 ★ | top_k(三方不对称 bug) | top_k(integer)(yaml:5015) | top_k(integer)(yaml:9352) | top_k(integer)(yaml:13250) | **无顶层 TopK 槽**(model.go 缺);仅 metadata 键 TransformerMetadataKeyTopK="anthropic_top_k"(anthropic/model.go:184) | a)msg 自环保命:入站写 chatReq.TransformerMetadata["anthropic_top_k"]=*int64(anthropic/inbound_convert.go:86-88),出站还原 req.TopK(anthropic/outbound_convert.go:195) → 仅 msg-in→msg-out 往返存活 [PASSTHROUGH]<br>b)chat 入站:openai.Request(openai/model.go:15 起)**无 TopK 字段** → Go JSON 解码静默剥离未知键;ToLLMRequest(inbound_convert.go:38)/RequestFromLLM(outbound_convert.go:10)均零处理,**全包 grep 零引用** [DROP]<br>c)resp 入站:responses.Request(responses/model.go:96 起)**无 TopK 字段**;convertToLLMRequest(responses/inbound.go:166)/payload(responses/outbound.go:247)均零处理,**全包 grep 零引用** [DROP]<br>后果:spec 三协议皆收 top_k,AxonHub 却只让 anthropic 来源活下来;任何 chat/responses 客户端显式设的 top_k 必丢——即便路由到 anthropic provider 也救不回(metadata 为空)。⑦init 阶段即定生死。 | ❌ | D23(P1):对称化方案照搬 ADR-D1 metadata 通道。(1)将常量提升至 transformer/shared 层并改中性 key(如 "top_k"),保留对旧串 "anthropic_top_k" 的读取兼容——**注意 shared 层当前并无此常量,需新建,并非复用既有**;(2)给 openai/chat Request 与 responses Request 各加 TopK 字段并在各自入站写入同一 metadata key;(3)在所有支持 top_k 的上游(chat/openrouter 上游、responses 上游、anthropic 已有)从该 key 还原发送。不改 canonical 架构(守红线)。 |
| C4 | max_tokens | max_tokens(integer·deprecated→max_completion_tokens)(yaml:4800) | max_tokens(integer·required gte=1)(anthropic/model.go:12)(yaml:9029) | —(spec 以 max_output_tokens 表达,yaml:13094 属另一概念) | MaxTokens *int64(model.go:73)+并存 MaxCompletionTokens *int64(model.go:64) | a)chat 入 r.MaxTokens→canon.MaxTokens(inbound_convert.go:48);出反向(oconv:20)[DIRECT]<br>b)msg 入 &req.MaxTokens→canon.MaxTokens(anth_in:57);出 resolveMaxTokens 优先 MaxTokens 否则回落 MaxCompletionTokens(anth_out:202-209)[RENAME+fallback]<br>c)resp 入 不读 max_tokens(spec 无);出 当 MaxOutputTokens 缺失时把 canon.MaxTokens 灌进 max_output_tokens(resp_out:291-292)[MERGE-fallback]<br>d)⚠️双槽并存且目标间优先级不一致:anthropic 偏 MaxTokens>MaxComp,responses 偏 MaxComp>MaxTokens;客户端同时设两值不同时会按目标产生分歧(退化场景)。⑦init 统一。 | ✅(值全程存活)/⚠️precedence | 主链路 length-cap 值跨六口闭合,无需紧急修。边缘:D26(P2 可选)统一双槽优先级语义或在文档标注取舍规则。关联 D25?否。 |
| C5 | max_completion_tokens | max_completion_tokens(integer)(yaml:4795) | —(spec 无) | —(spec 以 max_output_tokens 表达) | MaxCompletionTokens *int64(model.go:64) | a)chat 入 canon.MaxCompletionTokens(inbound_convert.go:47);出(oconv:19)[DIRECT]<br>b)msg 入 spec 无 [NOT_FOUND];出 作 resolveMaxTokens 次选回落源(anth_out:208)使 chat 来源长度预算仍可送达 anthropic[MERGE-fallback]<br>c)resp 入 以 max_output_tokens 之名灌入本槽(resp_in:179)[MERGE];出 反向 emit 为 max_output_tokens(resp_out:260)[MERGE-renamed roundtrip]<br>结论:命名分裂但经共享 MaxCompletionTokens 槽跨六口闭合。⑦init。 | ✅ | 无需改动。关联 D26(precedence 同 C4)。 |
| C6 | max_output_tokens | —(spec 无) | —(spec 无) | max_output_tokens(integer)(yaml:13094) | 复用 MaxCompletionTokens *int64(model.go:64)(无独立 MaxOutputTokens 槽) | a)resp 入 req.MaxOutputTokens→canon.MaxCompletionTokens(resp_in:179)[MERGE]<br>b)resp 出 canon.MaxCompletionTokens→payload.MaxOutputTokens(resp_out:260);canon.MaxTokens 作后备(resp_out:291-292)[MERGE-fallback]<br>c)chat/msg 侧 spec 无 [NOT_FOUND],但其 canonical 槽被借用实现跨协议 length 透传<br>⑦init。 | ✅ | 无需改动。关联 D26。 |
| C7 | repetition_penalty | repetition_penalty(number·OpenRouter 扩展)(yaml:4923) | —(spec 无) | —(spec 无) | 无 RepetitionPenalty 槽(model.go 缺) | a)chat 入 openai.Request(openai/model.go:15 起)**未列字段** → 解码静默剥离;全包零引用 [DROP]<br>b)msg spec 无 [NOT_FOUND]<br>c)resp spec 无 [NOT_FOUND]<br>OpenRouter 扩展 knob,多 provider 原生支持;丢失降低相对 OR 的保真度。⑦n/a(init 即丢)。 | ❌ | D24(P2):中性 metadata key("repetition_penalty")于 chat 入站捕获,在上游支持的出站还原发送(机制同 D23)。 |
| C8 | min_p | min_p(number·扩展)(yaml:4822) | —(spec 无) | —(spec 无) | 无 MinP 槽(model.go 缺) | 同 C7 模式。openai.Request 无 min_p 字段,全包零引用 [DROP]/[NOT_FOUND×2]。⑦n/a。 | ❌ | D24(P2):key="min_p",机制同 C7。 |
| C9 | top_a | top_a(number·扩展)(yaml:5009) | —(spec 无) | —(spec 无) | 无 TopA 槽(model.go 缺) | 同 C7 模式。openai.Request 无 top_a 字段,全包零引用 [DROP]/[NOT_FOUND×2]。⑦n/a。 | ❌ | D24(P2):key="top_a",机制同 C7。 |
| C10 | frequency_penalty | frequency_penalty(number·−2..2)(yaml:4773) | —(spec 无) | frequency_penalty(number)(yaml:13078) | FrequencyPenalty *float64(model.go:54) | a)chat 入 canon.FrequencyPenalty(inbound_convert.go:46);出(oconv:17)[DIRECT]<br>b)msg spec 无 [NOT_FOUND];anthropic 出站 buildBaseRequest 只发 temp/topp/maxtokens(anth_out:109-119)不发 freq → 凡路由至 anthropic 者 freq 丢弃(target API 亦不接受,属协议限制而非作者 bug)<br>c)resp 入 canon.FrequencyPenalty(resp_in:173);出(resp_out:262)[DIRECT]<br>⇒chat↔responses 双向闭合 ✓;anthropic-bound 不可避免丢弃。⑦init。 | ✅(附注 anthropic-bound drop) | 无需改动。若希望面向支持 freq 的非 OpenAI/Anthropic 上游也透传,可在相应出站补发(超出本区范围)。 |
| C11 | presence_penalty | presence_penalty(number·−2..2)(yaml:4876) | —(spec 无) | presence_penalty(number)(yaml:13145) | PresencePenalty *float64(model.go:84) | 镜像 C10。chat 入 inbound_convert.go:49 / 出 oconv:21;resp 入 resp_in:174 / 出 resp_out:263;msg [NOT_FOUND];anthropic 出站不发 pres。⑦init。 | ✅(附注同 C10) | 同 C10。 |
| C12 | seed | seed(integer)(yaml:4949) | —(spec 无) | —(spec 无) | Seed *int64(model.go:91) | a)chat 入 canon.Seed(inbound_convert.go:50);出(oconv:22)[DIRECT]<br>b)msg spec 无 [NOT_FOUND];anthropic 出站不发 seed(target 不支持)<br>c)resp spec 无 [NOT_FOUND];responses 出入站不读(seed slot 恒空)<br>⇒仅 chat↔chat 闭合;跨协议因 target API 普遍不支持 seed 而丢弃(可接受)。未走 metadata 透传,单跳路由下不影响。⑦init。 | ✅(附注跨协议 drop) | 无需改动。可选增强:metadata 透传以备将来跨格式续接。 |
| C13 | logit_bias | logit_bias(object{token:number/double})(yaml:4781;additionalProperties format double/type number) | —(spec 无) | —(spec 无) | LogitBias map[string]int64(model.go:154)—**类型锁死整数** | a)chat 入 r.LogitBias(map[int64])直拷入 canon(inbound_convert.go:61);出反向(oconv:30)[DIRECT-but-type-locked]<br>b)msg spec 无 [NOT_FOUND]<br>c)resp spec 无 [NOT_FOUND]<br>⚠️隐患:spec 允许 value 为浮点(double),Go 端 map[string]int64 严格反序列化遇小数(如 "-100.5")会报 cannot unmarshal number into int64。是否导致整包请求失败取决于入口 decoder 实现(strict encoding/json vs 容错 xjson)——**本区未核实 decode 路径**,标待复核。⑦decoder/init 阶段。 | ⚠️ 待复核(若 strict 则升级为❌) | D25(P2):将 canonical + openai LogitBias 放宽为 map[string]float64 或自定义 UnmarshalJSON 兼纳 int/float,消除解码中断风险;bias 本就是 logits 加减,浮点更精确,语义无损。先确认 decode 路径再定级。 |
| C14 | stop / stop_sequences(停止序列) | stop(string\|array<string>≤4)(yaml:4971) | stop_sequences(array<string>)(yaml:9091) | —(spec ResponsesRequest 无顶级 stop/stop_sequences) | Stop *Stop{Stop,MultipleStop}(model.go:194 / type Stop @ model.go:298) | a)chat 入 r.Stop→canon.Stop{Stop,MultipleStop}(inbound_convert.go:77-84);出反向(oconv:48-53)[DIRECT]<br>b)msg 入 StopSequences[]string→canon.Stop(单项填 Stop、多项填 MultipleStop)(anth_in:331-339);出 convertStopSequences(canon.Stop)→[]string(anth_out:23,func@outbound_convert.go:317-331)[RENAME-roundtrip closes]<br>c)resp spec 无独立 stop [NOT_FOUND];responses 出入站不读,payload 无 stop 字段(responses/outbound.go:247-289 未现)→ routing 至 responses provider 时停止序列丢弃(provider API 层面 Responses 确无顶级 stop,属协议缺口)<br>⇒chat↔messages 经 canonical.Stop 改名往返闭合 ✓;responses-bound 不可用(协议本身无)。⑦init。 | ✅(附注 responses-bound protocol gap) | 无需改动。 |

### 本区自检

- **取证方法**:逐格人工核对 MCP 图谱定位的三 transformer 包源码 + min.yaml 三 schema 字段块,禁脚本批量推断。每条 `(file:行)` 均来自实读 sed 输出,非图谱推测。
- **合规判定计数**:✅ = 9(C1,C2,C4,C5,C6,C10,C11,C12,C14);❌ = 4(C3,C7,C8,C9);⚠️ = 1(C13)。共 14 行。
- **行主类别分布**:DIRECT ×6(C1,C2,C10,C11,C12,C13)、DROP ×4(C3,C7,C8,C9)、MERGE ×3(C4,C5,C6)、RENAME ×1(C14)。另:PASSTHROUGH 作为内嵌子项出现于 C3-msg 自环;NOT_FOUND 作为内嵌子项贯穿各 chat-only/resp-absent 列(C5/C6/C7–C9/C10–C14 的对应协议侧)。六类俱全。
- **新增缺陷编号**(避开已占用 D1–D22):
  - D23 P1 — top_k 三方不对称(chat/responses 全程缺失,仅 anthropic 自环保命)
  - D24 P2 — repetition_penalty/min_p/top_a OpenRouter 扩展参数 chat 入站静默剥离
  - D25 P2 — logit_bias 类型锁死 map[string]int64(spec 允许 double,潜在解码中断)
  - D26 P2(可选)— max_tokens 双槽(MaxTokens/MaxCompletionTokens)目标间优先级不一致
- **待复核项 = 1**:C13(logit_bias)实际 decode 路径未实测,strict decoder 下是否整包失败存疑;其余 13 行均已据源码坐实。
- **关键事实校正记录**(防先前误判复发):(a)responses OUTBOUND 确实发出 TopP(responses/outbound.go:271),首轮结构字面量扫描漏看差点误判 top_p 断裂,经全文复读纠正为 ✅;(b)"shared 层既有 TransformerMetadataKeyTopK" 并不存在——唯一常量定义位于 transformer/anthropic/model.go:184 且值为 "anthropic_top_k",对称化修复需新建 shared 中性键,本表据此如实表述,不沿用任务描述中的"复用既有"措辞。
- **边界声明**:本区只读源码、未做运行时验证;streaming 路径(⑦)依据架构判断(采样/限长均为 request-body 参数,init 写入后 aggregator 不重写),未单独跑流式回归。

## 2026-06-30 fork-agent(James)复核增订

> 本节由主线程代为落盘(James 子代理两次 `completed:null` 未成功写入文件)。**本节结论 supersede 上方表格中对应格子的旧判定，遇冲突以本节为准。**

### 方法
全程 `get_code_snippet`/rg/sed 直读实时源码 + min.yaml 枚举双证(`.agent/rules/spec-audit-method.md` 四判据④)；图谱关系边作废(fix 提交滞后)，所有 `(file:line)` 来自本轮实读。

### 逐格新定性

| 格号 | 旧判定 | 新定性 | 源码证据(file:line) | min.yaml 行号 |
|---|---|---|---|---|
| C3 ★ top_k | ❌D23 P1 不对称 | ❌确违(D23,P1)维持 | anthropic 自环 metadata 存(`inbound_convert.go:86-88`)／还原(`outbound_convert.go:195-196`)仅保命；chat/responses model 无 TopK 字段(`openai/model.go:15-98`、`responses/model.go:94-167`)；路由到 anthropic 上游时 metadata 空 也救不回 | ChatRequest 5015／Messages 9352／Responses 13250 均 integer |
| C4 max_tokens | ✅值存活／⚠precedence | 维持 ✅主链闭合 ＋ ⚠precedence 成立(D26 P2 可选) | canonical 双槽 `model.go:73`(MaxTokens)+`:64`(MaxCompletionTokens)；anthropic 出站 `resolveMaxTokens`=outbound_convert.go:204-214 取 MT>MCT>默认8192；responses 出站 outbound.go:260 主取 MCT 缺省回填 MT(:291-292)=MCT>MT，**两端优先级相反** | chat 4800(deprecated)／messages 9029(required≥1)／responses max_output_tokens@13094 |
| C7 repetition_penalty | ❌D24 P2 剥离 | ❌确违(D24,P2)静默 DROP 维持 | `openai/model.go` 无字段；`inbound.go:65` 裸 json.Unmarshal 未设 DisallowUnknownFields→未知键静默弃；字面量零引用 | 仅 ChatRequest 4923(number·double·nullable)，messages/responses schema 无 |
| C8 min_p | ❌D24 P2 剥离 | ❌确违(D24,P2)维持 | 同 C7 模式 | 仅 ChatRequest 4822(number·double) |
| C9 top_a | ❌D24 P2 剥离 | ❌确违(D24,P2)维持 | 同 C7 模式 | 仅 ChatRequest 5009(number·double) |
| C13 logit_bias | ⚠️待复核(decoder 路径未核实) | **❌确违(D25,P2)翻盘升级** | 类型锁死 `map[string]int64`(`model.go:154`／`openai/model.go:62`，`inbound_convert.go:61` 直拷)；解码入口 `openai/inbound.go:65` 严格 encoding/json.Unmarshal + `xjson.To`(`llm/internal/pkg/xjson/json.go:33` 标准封装)全程不容错、无自定义 UnmarshalJSON ⇒ 浮点 bias(如 `-100.5`)必报 `cannot unmarshal number … into int64` → ErrInvalidRequest → 整包 400 | ChatRequest 4781 `additionalProperties` format:double/type:number 允许浮点(messages/responses 无此项) |

### 四新增缺陷编号当前真实状态
- **D23 top_k 不对称**：未修。fix 只补强 anthropic 自环(metadata 写读已在位)+`top_k_test.go`；chat/responses 入站结构性缺失依旧，即便路由到 anthropic 上游也因 metadata 为空救不回。
- **D24 rep/min_p/top_a 剥离**：未触及。三者仅在 ChatRequest spec 单次出现，AxonHub 结构体无字段 + lenient 解码静默丢。
- **D25 logit_bias 类型锁死**：未触及，且本次首次查实硬失败解码路径。整数 bias(-100/+5)能过，小数必致整包 400。spec 明确允许 double 故违规。
- **D26 max_tokens 双槽优先级**：函数体实测两端优先级相反(anthropic MT>MCT vs responses MCT>MT)，客户端同时设两不同值会按目标产生分歧长度预算。维持「可选 P2」。

### 统计
翻盘 1(C13 ⚠️→❌)／坐实 5(C3/C7/C8/C9 维持❌ + C4 维持✅/⚠precedence)／仍悬 0。

### 诚实声明(per AGENTS.md §5 分开写)
- **已证明事实(C13)**：transformer/跨格式转换路径下，带分数 logit_bias 的请求会被 AxonHub 在自身解码阶段拒掉。
- **待确认影响半径**：是否存在 pass-through 直通模式绕过 TransformRequest(原始字节不经解码直达同格式上游)? 若存在则分数 bias 在纯直通场景可能幸存。本棒未验证 pass-through 是否短路。不影响合规性定性(spec 要求只要经 AxonHub 处理就该接受)，只影响线上触发频率估算。建议下一棒用 MCP `trace_path` 追 `orchestrator.Process→Inbound.TransformRequest` 调用前置条件确认。
- **图谱新鲜度处理**：per `.agent/rules/spec-audit-method.md`，凡 fix 动过的模块一律 rg/sed 直读磁盘真文件为准，未采信图谱关系边做影响分析。
- **pure-✅ 行抽样自洽**(C1/C2/C10/C11/C14 等)：顺带核对到 responses 出站确发 Temperature:261／FrequencyPenalty:262／PresencePenalty:263，印证文档此前纠正的「top_p 出站断裂误判」，结构与文档一致无需翻案。
- **关键事实校正记录**：(a) responses OUTBOUND 确发出 TopP(`responses/outbound.go:271`)，首轮结构字面量扫描漏看差点误判 top_p 断裂，经全文复读纠正为 ✅；(b)「shared 层既有 TransformerMetadataKeyTopK」并不存在——唯一常量定义位于 `transformer/anthropic/model.go:184` 值为 `"anthropic_top_k"`，对称化修复需新建 shared 中性键。
- **边界声明**：本区只读源码、未做运行时验证；streaming 路径(⑦)依据架构判断(采样/限长均为 request-body 参数，init 写入后 aggregator 不重写)，未单独跑流式回归。
