# AxonHub 协议桥接领域词汇表

本文件只记录已达成共识的术语定义，不含实现细节或规格说明。

**词汇表刷新:** 2026-07-22（与 ADR 0001/0002、protocols README 权威层对齐）。

## 工具身份表示
- **LeafMethod（叶子方法）**: MCP 服务器暴露的单个工具的本名，例 `list_projects`。
- **NamespaceQualifier（命名空间限定符）**: 标识工具所属分组（MCP server 等）的独立字段值，例 `mcp__codebase_memory_mcp`。对应 Responses 规范的可选字段 `namespace`。
- **CompositeName（复合名）**: 把限定符与叶子拼接成的单串 `<ns>__<leaf>`，仅供不支持 namespace 概念的协议（chat_completions）传输使用；不得充当严格客户端的路由键。

## 客户端派发语义
- **DispatchRegistry（派发注册表）**: 严格客户端（如 Codex responses 模式）以 `(namespace, name)` 二元组检索本地处理器；两者需各自命中声明期句柄。
- **StructuralValidity（结构合法）**: 响应体满足 schema 必填字段即成立（含 type/name/call_id/arguments）；不蕴含可路由。
- **RoutableIdentity（可路由身份）**: 字段取值能使 DispatchRegistry 命中所期望处理器。结构合法 ≠ 可路由。
- **SpecOptionalImplRequired（规约可选·实现必填）**: 规范将 `namespace` 标为可选，但特定客户端将其视作 MCP 路由所必需——规约可空性与运行时必要性之间的落差。

## 桥接非对称
- **BridgeAsymmetry（桥接非对称）**: responses↔chat 桥接中，responses 端存在 namespace 概念而 chat 端不存在；若入站阶段压扁后未在回程还原，则限定符身份在往返中丢失。

## 协议面与归属
- **OpenAIResponsesNative（OpenAI Responses 原生协议）**: OpenAI Responses API 的完整协议语义集合，包括普通消息、工具、状态、推理、流式 item 与协议扩展字段。公开 wire 事实以 `docs/specs/protocols/openai-responses-protocol.md` 与 `vendor/protocol-canonical-2026-07-06/` 为准。
- **CodexResponsesProfile（Codex Responses 使用画像）**: Codex 客户端在 OpenAIResponsesNative 中实际使用的一组 agent 化字段和 item 形态；它不是独立私有协议。能对齐公开协议的先归一再转；不能对齐的停在 adapter/usage-profile，禁止假塞进 CrossProtocolCanonical。
- **CrossProtocolCanonical（跨协议通用抽象）**: 多种上游/下游协议都能稳定表达的模型调用语义，例如模型名、普通消息、采样参数和普通 function tool。载体为 `llm.Request` / `llm.Response`。
- **FieldOwnership（字段归属）**: 每个协议字段只能有一个主要归属层——CrossProtocolCanonical、协议 native 结构、`ProviderExtensions` 命名 owner、same-protocol raw fallback、或 LossyDowngrade；归属决定测试入口和允许的丢弃方式。
- **NativePreservation（原生保真）**: 同协议转发时保持协议原生结构、字段身份和未知扩展不被压扁或丢弃。它要求 transformer/native 层理解或携带字段，**不等同于** PassThroughBody。
- **SameProtocolPreservation（同协议保真）**: 源协议和目标协议相同的时候，Hub 解析、转换、重建后仍保留该协议可表达的原生字段和身份信息；优先于任何跨协议映射设计。
- **FullNativeRoundTrip（完整原生往返）**: 请求、响应和流式事件经 Hub 后仍保留该协议标准字段、工具身份、item 类型和未知扩展（在可 re-emit 范围内）。P0/P1 只代表实施顺序。
- **ResponsesNativeAST（Responses 原生 AST）**: Hub 对 OpenAI Responses 标准字段、tool/item variants 和流式事件的一等结构化表示；与 raw fallback 配合，避免只留字节无法诊断。
- **PassThroughBody（请求体透传）**: 复用客户端原始请求体并只 patch 必要字段的转发手段；缓解路径和对照基线，**不是**字段丢失的架构解。
- **LossyDowngrade（有损降级）**: 跨协议因目标缺少等价语义而改写/压扁/丢弃；必须可诊断（`ProviderExtensions.Diagnostics`），禁止静默。
- **PreservationSlice（保真切片）**: 只验证一组字段在一个协议方向上 preserve-or-diagnose 的小实施单元。历史上第一刀是 Responses→Responses request；**当前**切片可按 FieldOwnership 覆盖 Chat / Anthropic / 响应 / 流 residual，不得用「第一刀冻结」拒绝已有归属规则的修复。

## 文档权威（防误导）
- Wire 事实：`docs/specs/protocols/*-protocol.md` + vendor canonical。
- 架构决策：`docs/adr/0001-*.md`、`docs/adr/0002-*.md`、`.trellis/spec/backend/protocol-transformer-guidelines.md`。
- 完成度账本：`protocol-conversion-strict-verification-matrix.md`（仅 `CONFIRMED` 可当完成）。
- 实现真相：当前 `llm/**` 与定向测试。drafts / 旧 handoff / 过期 summary 不得压过以上层。
