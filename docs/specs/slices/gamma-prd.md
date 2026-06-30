# 切片 γ · PRD(采样参数)

> 规范基准:`docs/specs/openrouter-chat-messages-responses.min.yaml`;canonical `llm.Request`(`llm/model.go`)。守红线:不动 canonical 架构,仅放宽既有字段类型 / 加 protocol Request 字段 + metadata 通道。

## γ-1 · C13 — logit_bias 浮点解码中断
- **Problem**:canonical `llm.Request.LogitBias`(`llm/model.go:154`)、openai `Request.LogitBias`(`transformer/openai/model.go:62`)、openai `CompletionRequest.LogitBias`(`completion.go:21`)三处锁死 `map[string]int64`;chat 入站 `inbound.go:52` 与 completion 入站 `completion_inbound.go:47` 均用标准 `encoding/json.Unmarshal`。OpenRouter spec 允许 logit_bias value 为浮点(yaml:4781 additionalProperties double),遇 `-100.5` 等 → `cannot unmarshal number into int64` → 整包 400。
- **Solution**:三处 `map[string]int64` → `map[string]float64`(int→float 无损;bias 本就是 logits 加减,浮点更精确)。传播点(inbound_convert.go:58 / completion_inbound.go:80 / outbound_convert.go:30 / completion_outbound.go:85 均 copy)无需改。仅放宽既有字段类型,守红线。
- **Testing**:红:chat 入站发 `{"logit_bias":{"5043":-100.5}}` → 现 TransformRequest 报错;绿:放宽 float64 后解码成功且值保留 -100.5。附 completion 路径同测 + chat 往返保浮点测。
- **Out of scope**:logit_bias 跨 anthropic/responses(spec 无此字段,本就 DROP);自定义 UnmarshalJSON(直接放宽 float64 更简单,无需)。
- **diagnose 跳过理由**:根因坐实(标准 json + int64 map + spec 允许 double),红测可直接复现。
- **状态**:✅ 已完成·同模验收 Herschel APPROVED(7 标准,4 处 float64 全仓无 int64 残留),待 commit。

## γ-2 · C3 — top_k 三方不对称(P1)
- **Problem**:spec 三协议皆收 top_k,但作者仅让 Anthropic 来源活:常量 `TransformerMetadataKeyTopK="anthropic_top_k"`(anthropic/model.go:184),仅 anthropic 入站写(inbound_convert.go:86-88)、出站读(outbound_convert.go:209-211)。openai.Request 与 responses.Request **无 TopK 字段** → chat/responses 客户端显式设的 top_k 在 JSON 解码即丢,即便路由到 anthropic provider 也救不回(metadata 为空)。
- **Solution**(D23 对称化,守红线——不动 canonical,仅 metadata 通道 + protocol Request 加字段):
  1. `transformer/shared/`:新建中性常量 `TransformerMetadataKeyTopK = "top_k"`(shared 层现无,需新建,非复用)。
  2. anthropic 迁移:inbound 改写 `shared.TransformerMetadataKeyTopK`;outbound 读 `shared` key,回退旧串 `"anthropic_top_k"`(兼容已持久化 metadata);model.go 旧常量降级为 unexported legacy。
  3. `openai/model.go` Request 加 `TopK *int64 `json:"top_k,omitempty"``;`responses/model.go` Request 同。
  4. openai `ToLLMRequest`:r.TopK 非空时写 `req.TransformerMetadata[shared key]=&topK`(*int64,匹配 anthropic);`RequestFromLLM`:读 `shared key` 的 `*int64` 还原 `req.TopK`。
  5. responses `convertToLLMRequest`:同写;`outbound`:`TopK: xmap.GetInt64Ptr(metadata, shared key)`(xmap 兼容 *int64)。
- **Testing**:红:chat top_k→canonical→chat outbound 丢;chat top_k→anthropic outbound 丢;responses top_k 往返丢。绿:三路全通;anthropic 既有 top_k 往返不回归(回归守卫)。
- **Out of scope**:canonical 加 TopK 顶层槽(违红线);top_k 数值范围校验(pass-through 保真,不计)。
- **diagnose 跳过理由**:根因坐实(master 表 C3 + 源码),红测可直接复现三路丢失。
- **状态**:✅ 已完成·同模验收 Meitner APPROVED(7 标准,三入站写三出站读无漏接,canonical 红线未破),待 commit。

## γ-3/4/5 · C7/C8/C9 — rep_penalty/min_p/top_a(P2,同模式,待 γ-2 完成后展开)
- **状态**:⏳ 待开始。
