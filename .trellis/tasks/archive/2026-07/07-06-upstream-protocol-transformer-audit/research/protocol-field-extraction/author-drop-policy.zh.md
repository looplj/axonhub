# 作者字段丢弃模式与后续判定规则

本文件用于实现前判断：某个字段到底应该补、保留、桥接、诊断后丢弃，还是设计性不支持。

## 作者当前常见丢弃/弱保真模式

| 类型 | 作者当前表现 | 例子 | 风险 | 后续判定 |
|---|---|---|---|---|
| 目标协议不支持 | 在 common -> target 时直接不输出 | OpenAI Chat builder 过滤非 function tools | 协议更新后可能从“正确丢弃”变成“协议漂移 bug” | 重新查官方 schema；若目标协议已支持则补 native/provider-specific，否则 lossy diagnostic + drop |
| 旧版 deprecated 字段 | 不建模或不输出 | Chat `function_call` / `functions` | 旧客户端兼容性下降 | 优先映射到 `tools` / `tool_choice`；仅同协议需要 raw 保真；新版跨协议可诊断后丢弃 |
| 改变统一语义的字段 | 明确不支持或没有 common 表达 | Chat `n` 多候选 | Hub common response 偏单候选，支持它会牵动 response/usage/stream | 可设计性不支持，但必须文档化；如果要支持，需要单独任务 |
| Provider 私有字段 | 靠 metadata/raw 或直接丢 | OpenRouter sampling knobs、Codex client metadata | 可能影响特定 provider 能力 | 不进 `llm.Request`；放 native/provider-specific/ProviderExtensions；跨 provider 默认不透传 |
| 服务端状态字段 | 跨协议没有等价物 | Responses `conversation`、`previous_response_id`、Claude `container` | 静默丢会导致上下文断裂或状态丢失 | 同协议必须保；跨协议只能明确桥接或 lossy diagnostic + drop |
| 工具生态字段 | 不同协议语义相似但 schema 不同 | OpenAI Responses `mcp` vs Anthropic `mcp_servers` | 误映射会造成工具不可用或错误调用 | 不自动互转；同协议 native/raw 保真；跨协议先诊断后丢弃，除非有明确桥接设计 |
| Stream 细粒度事件 | 只处理部分事件或泛化聚合 | Responses `response.mcp_call.*`、`response.reasoning_text.*` | 流式丢字段比非流式更隐蔽，可能导致“看起来断流/缺内容” | 不能用 request 逻辑判断；必须审计 stream model/inbound_stream/outbound_stream/aggregator |
| 便捷派生字段 | response struct 不一定保存 | Responses `output_text` | 如果可由 `output` 聚合派生，缺 native 字段未必致命 | 若客户端同协议期望原字段，应 native/aggregator 生成；否则可派生不存 |

## 判定顺序

1. **是不是官方当前字段？**
   - 是：继续判断同协议是否必须保真。
   - 否：看是否 provider/Codex 兼容字段；不是则不补。

2. **是不是同协议 round-trip？**
   - 是：默认不应丢。优先 native struct，其次 ProviderExtensions/raw fallback。
   - 否：进入跨协议判断。

3. **目标协议有没有等价语义？**
   - 有：显式桥接，写测试。
   - 没有：lossy diagnostic + drop。

4. **字段是否改变 Hub common 语义？**
   - 是：不要偷偷支持；单独设计，例如 `n` 多候选。
   - 否：可进入 common 或 metadata。

5. **字段是否 provider-specific？**
   - 是：不要放进所有 OpenAI-compatible 共享 builder；放 provider outbound 或 gated native emission。

6. **字段是否 stream/event？**
   - 是：不要放进 request/response 字段处理；去 stream 层处理。

## 当前应优先补的“不应丢”类别

| 协议 | 字段/事件 | 原因 | 推荐路径 |
|---|---|---|---|
| OpenAI Responses | `conversation` | 服务端会话挂载，同协议丢失会断状态 | Responses native request 字段，跨协议 diagnostic |
| OpenAI Responses | `context_management` | 上下文压缩/管理配置，同协议必须保真 | Responses native/opaque request 字段，跨协议 diagnostic |
| OpenAI Responses | `prompt` | 官方 request 字段，作者 upstream 缺 | Responses native request 字段 |
| OpenAI Chat | `web_search_options` | 官方 Chat request 字段，作者缺 | Chat native request 字段；provider-specific/gated emission |
| OpenAI Chat | `prediction` | 官方 Chat request 字段，作者缺 | Chat native request 字段；跨协议 diagnostic |
| OpenAI Chat | top-level `audio` | 官方 Chat request 字段，作者只有 message audio | Chat native request 字段 |
| Anthropic | `container` | 官方 request/response 字段 | Anthropic native request/response 字段 |
| Anthropic | `inference_geo` | 官方 request/usage 相关字段 | Anthropic native request 字段 |
| Anthropic | `mcp_servers` | MCP connector companion 字段 | Anthropic native/provider extension；不自动转 OpenAI mcp |
| OpenAI Responses stream | `response.mcp_*`, `response.reasoning_*`, `response.audio_*` 等 | 官方 stream 事件大量未显式覆盖 | stream model/aggregator 单独审计 |

## 当前可以设计性不支持或延后

| 字段 | 原因 | 条件 |
|---|---|---|
| Chat `n` | 多候选会影响 common response、stream、usage 结构 | 文档化不支持；如要支持另开任务 |
| Chat `function_call` / `functions` | deprecated | 可转新版 tools/tool_choice；旧版同协议再考虑 raw/native |
| 无来源确认的字段 | 防止再次被错误文档污染 | 必须先补来源，再进实现 |
