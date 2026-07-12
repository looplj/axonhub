# AxonHub 三协议转换严格确认矩阵（INCOMPLETE）

- 创建日期：2026-07-09
- 状态：**INCOMPLETE / 未完成**
- 规则：只要任意一行存在 `UNCHECKED` / `PARTIAL` / `BLOCKED`，本文档就不能作为“协议转换已完成”的证据。
- 目的：把 OpenAI Responses、OpenAI Chat Completions、Anthropic Messages、Codex Responses 扩展拆成可审计行，逐项确认含义、内部归属、跨协议转换、真实链路测试。

## 0. 当前结论

本文档整体状态仍是 **INCOMPLETE**：任意 `UNCHECKED` / 未闭环跨协议行存在时，不能作为“三协议转换已全部完成”的证据。

截至 2026-07-12（G1–G7 implementation modules + 主矩阵同步）：

1. **Same-protocol 已闭环（有 code + targeted test）**：见 §5 中已从 `UNCHECKED` 提升为 `PARTIAL` 的 G1–G7 行，以及 §9 叠加证据表。覆盖范围包括：
   - Chat：`n`、`prompt_cache_retention`、`audio`/`prediction`/`moderation`、`web_search_options`、deprecated `functions`/`function_call`
   - Anthropic：`container`、`inference_geo`、`mcp_servers`、`tools[].type=mcp_toolset`
   - Responses：`reasoning` 对象关键子路径（context / generate_summary / content / stream text / unknown nested）
2. **历史已确认闭环**：Codex Responses `tools[].type = "namespace"` 在 Responses -> Chat 展开、reverse map、回包/流式恢复 `name + namespace`（仍是 P1/Codex 扩展叙事，不抬成公共 P0 全矩阵完成）。
3. **未声称**：101 行全 `CONFIRMED`、全方向跨协议语义等价、Codex P1 = 公共 P0、或 residual fixture-only 项已实现为 feature。
4. **Residual**：见 §9.3 与 `.trellis/tasks/07-10-protocol-high-priority-fixtures-matrix-sync/research/residual-gaps.md`。

权威阅读顺序：§5 行是主状态；§9 是 G1–G7 实现证据索引（与 §5 对齐，不是“仅附录、不提升主表”）。

## 1. 权威来源

| 来源 ID | 协议/范围 | 本地文件 | 用途 | 状态 |
|---|---|---|---|---|
| SRC-RSP-BASE | OpenAI Responses baseline | `docs/specs/protocols/openai-responses-protocol.md` | Responses 字段、input/output/tool/stream 基线 | CONFIRMED_SOURCE |
| SRC-RSP-RAW | OpenAI Responses official snapshot | `docs/specs/vendor/protocol-canonical-2026-07-06/openai-responses-create.platform-snapshot.md` | 详细 schema / event 查证 | CONFIRMED_SOURCE |
| SRC-RSP-API | OpenAI API machine-readable definition | `docs/specs/vendor/protocol-canonical-2026-07-06/openai-api-definition.fetch.yaml` | `/responses` 与 `/chat/completions` 机器可读定义 | CONFIRMED_SOURCE |
| SRC-CHAT-BASE | OpenAI Chat Completions baseline | `docs/specs/protocols/openai-chat-completions-protocol.md` | Chat 字段、messages/tools/stream 基线 | CONFIRMED_SOURCE |
| SRC-CHAT-RAW | OpenAI Chat official snapshot | `docs/specs/vendor/protocol-canonical-2026-07-06/openai-chat-completions-create.developers-snapshot.md` | 详细 Chat create schema 查证 | CONFIRMED_SOURCE |
| SRC-ANT-BASE | Anthropic Messages baseline | `docs/specs/protocols/anthropic-claude-messages-protocol.md` | Messages 字段、content/tool/stream 基线 | CONFIRMED_SOURCE |
| SRC-ANT-RAW | Anthropic Messages official raw | `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-messages-api.official-raw.md` | 详细 Messages schema 查证 | CONFIRMED_SOURCE |
| SRC-ANT-STREAM | Anthropic Messages streaming official raw | `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-messages-streaming.official-raw.md` | Anthropic stream event 查证 | CONFIRMED_SOURCE |
| SRC-ANT-MCP | Anthropic MCP connector official raw | `docs/specs/vendor/protocol-canonical-2026-07-06/anthropic-mcp-connector.official-raw.md` | `mcp_servers` / `mcp_toolset` 查证 | CONFIRMED_SOURCE |
| SRC-CODEX | OpenAI Codex source | `/Users/asuan/项目/AI/axonhub/.agent/research/openai-codex` | Codex Responses 扩展、namespace tools、客户端行为查证 | CONFIRMED_SOURCE |

### 1.1 事实优先级（已确认）

本矩阵采用固定事实优先级。低优先级来源不得推翻高优先级来源；第三方资料只能辅助发现问题，不能作为最终协议标准。

| Priority | 来源 | 可用于确认什么 | 不可用于确认什么 |
|---|---|---|---|
| P0 | OpenAI 官方协议文档、OpenAI 官方 OpenAPI/schema、Anthropic 官方文档 | 字段是否存在、字段官方含义、值形态、枚举/union、stream event 官方形状 | Codex 客户端专属行为、AxonHub 内部归属 |
| P1 | Codex 源码拆包 | Codex 客户端扩展行为，例如 namespace tools、客户端 tool spec、懒加载 wire shape | OpenAI/Anthropic 公共协议标准 |
| P2 | AxonHub 作者源码 | 当前实现、架构归属、字段实际存放位置、转换代码路径 | 官方字段含义本身 |
| P3 | 数据库样本、历史请求、真实运行日志 | 验证真实链路是否按预期运行、复现 bug、确认修复是否生效 | 定义协议标准 |
| P4 | OpenRouter / 第三方文档 / 社区资料 | 辅助发现字段、对照兼容性、提出调查线索 | 最终准信、最终字段语义、最终转换规则 |

### 1.2 `CONFIRMED` 门槛（已确认）

字段/结构/行为行只有同时满足以下条件，才允许标为 `CONFIRMED`：

1. P0/P1 级来源确认字段或行为存在；Codex 专属行为必须由 P1 确认。
2. P0/P1 级来源确认字段值类型、枚举值、union variant 或 event shape。
3. 已明确 AxonHub 架构归属：`llm.Request` / `llm.Response` / `ProviderExtensions` / `TransformerMetadata` / raw fragment / `LossyDowngrade` / `unsupported_error`。
4. 已明确相关方向的转换策略：`direct` / `rename` / `value_map` / `structural_transform` / `same_protocol_only` / `drop_with_diagnostic` / `unsupported_error`。
5. 有 P2 级代码证据，精确到文件/函数。
6. 有测试证据或 P3 级真实样本证据；stream 行必须覆盖 delta、done、final response，按适用范围覆盖 usage/error。

缺任意一条时，状态只能是 `PARTIAL`、`UNCHECKED`、`BLOCKED` 或带原因的 `N/A`，不能标为 `CONFIRMED`。

### 1.3 文档边界（已确认）

本文档是长期维护的**协议转换审计矩阵**，不是实现计划。

本文档只回答：

- 字段/结构/行为是什么；
- 官方含义和值形态是什么；
- 在 AxonHub 作者架构里应归属哪里；
- 每个协议方向应该如何转换或为什么不能转换；
- 当前证据是否足够把该行标为 `CONFIRMED`。

本文档不直接承载：

- 当前 sprint/slice 的实现排期；
- 具体提交计划；
- 临时调试记录；
- 一次性 handoff；
- 针对单个 bug 的短期执行清单。

实现计划应放在 task / issue / PRD / handoff / slice 文档中，并引用本矩阵的稳定行 ID。

### 1.4 行粒度规则（已确认）

字段总览行不能掩盖 object / array / union / typed union 内部变体。

规则：

1. 顶层字段可以有总览行，例如 `RSP.TOP.tools`、`RSP.TOP.input`、`ANT.TOP.thinking`。
2. 只要字段值是 object / array / union / typed union，就必须继续拆子行。
3. 子行必须拆到每个 variant 都能独立判断：官方含义、值形态、作者架构归属、转换方式、测试证据。
4. 总览行不能单独标为 `CONFIRMED`。
5. 只有所有必须子行均为 `CONFIRMED` 或带原因的 `N/A`，总览行才允许标为 `CONFIRMED`。

例如 `tools` 必须至少拆为：

- `RSP.TOOL.function`
- `RSP.TOOL.custom`
- `RSP.TOOL.web_search`
- `RSP.TOOL.image_generation`
- `RSP.TOOL.tool_search`
- `CODEX.TOOL.namespace`
- `RSP.TOOL.unknown_future`

同理，以下结构必须拆子矩阵：

- Responses `input[]`
- Responses `output[]`
- Responses `tool_choice`
- Chat `messages[].content[]`
- Chat `tool_calls[]`
- Chat `response_format`
- Anthropic `messages[].content[]`
- Anthropic `tools[]`
- Anthropic `tool_choice`
- Anthropic `thinking`
- 三协议所有 stream event / delta / done / final response 结构

### 1.5 转换方向规则（已确认）

每一行都必须填写六个方向的转换策略：

- Responses -> Chat
- Chat -> Responses
- Responses -> Anthropic
- Anthropic -> Responses
- Chat -> Anthropic
- Anthropic -> Chat

规则：

1. 六个方向列必须都写，不能留空。
2. 不要求六个方向都实现。
3. 实现不了也必须写明原因。
4. 不支持必须显式写成 `same_protocol_only`、`drop_with_diagnostic`、`unsupported_error`、`N/A` 或 `UNCHECKED`。
5. 不能用空白、缺列、或者模糊的 `not supported` 代替原因。

方向策略枚举：

| Strategy | 含义 | 必须补充什么 |
|---|---|---|
| `direct` | 语义和值形态都可直接承载 | 代码/测试证据 |
| `rename` | 语义一致，字段名不同 | 字段名映射和测试证据 |
| `value_map` | 语义接近，但值需要映射 | 值映射表、未知值策略 |
| `structural_transform` | 结构不同，需要专门转换 | 子结构矩阵和真实链路测试 |
| `sidecar_only` | 目标协议不能表达，只能保存在 sidecar | sidecar 路径和 replay 规则 |
| `same_protocol_only` | 只能同协议恢复，跨协议无等价语义 | 为什么无等价物 |
| `drop_with_diagnostic` | 转换时丢弃，但记录 LossyDowngrade | 诊断字段、严重级别、原因 |
| `unsupported_error` | 不允许继续转换，应报错 | 报错条件和错误路径 |
| `N/A` | 该方向逻辑上不适用 | 不适用原因 |
| `UNCHECKED` | 尚未确认 | 待查证据来源 |

示例：Responses `previous_response_id` 这类 server-side state 字段，如果目标是 Chat/Anthropic，不能空着，应写成类似：

```text
Responses -> Chat: same_protocol_only; Chat lacks server-side Responses state continuation.
Responses -> Anthropic: same_protocol_only; Anthropic Messages has no equivalent previous_response_id continuation.
```

### 1.6 字段值确认规则（已确认）

字段名确认不等于字段确认。每个字段/结构/行为行必须独立确认字段值形态和值语义。

必填值列：

- `Value Shape`：`string` / `boolean` / `number` / `object` / `array` / `union` / `typed_union` / `event`。
- `Allowed Values / Variants`：官方允许值、枚举值、object variant、content block type、tool type、event type。
- `Default Semantics`：字段省略时的官方默认行为。
- `Null / Omitted Semantics`：`null`、空字符串、空数组、省略字段是否有不同含义。
- `Unknown Value Policy`：未知 enum / 未来 variant 的处理策略。
- `Value Mapping Rule`：跨协议值怎么映射；不能映射时写原因。

规则：

1. 值形态没确认，该行不能 `CONFIRMED`。
2. enum / union / typed union 没拆 variant，该总览行不能 `CONFIRMED`。
3. 不能用字段名相同推导值语义相同。
4. 对 reasoning/thinking/token limit/tool choice/stream options 这类高风险字段，必须有官方值证据和代码/测试证据。

典型高风险值字段：

- OpenAI Chat `reasoning_effort`
- OpenAI Responses `reasoning`
- Anthropic `thinking`
- OpenAI Chat `max_completion_tokens` / deprecated `max_tokens`
- OpenAI Responses `max_output_tokens`
- Anthropic `max_tokens`
- 三协议 `tool_choice`
- 三协议 `stream_options` / stream event variants
- Chat `response_format`
- Responses `text`

### 1.7 未知字段 / 未来字段策略（已确认）

默认策略：同协议 raw preserve 优先；跨协议不猜。

规则：

1. 对 top-level unknown fields：
   - 同协议 replay：保存到对应协议的 `RawTopLevelFields`，尽量原样恢复。
   - 跨协议：不盲传；没有明确等价物时写 `sidecar_only`、`same_protocol_only`、`drop_with_diagnostic` 或 `unsupported_error`。
2. 对 typed union unknown variants，例如未知 tool type、content block type、stream event type：
   - 同协议 replay：保存 raw fragment、原始 index、raw JSON。
   - 跨协议：除非已有明确桥接策略，否则不转换。
3. 对 unknown enum values：
   - 同协议：优先保真。
   - 跨协议：如果目标协议值域没有官方确认，不能盲传。
4. 禁止把未知字段硬塞进相似名字的公共字段。
5. 禁止静默丢弃未知字段；如果目标协议无法表达，必须写诊断或明确 unsupported。

示例：Responses 未来新增 `x_future_feature`：

```text
Responses -> Responses: protocol_native_sidecar via RawTopLevelFields; replay when same-protocol.
Responses -> Chat: same_protocol_only or drop_with_diagnostic; reason = Chat has no confirmed equivalent.
Responses -> Anthropic: same_protocol_only or drop_with_diagnostic; reason = Anthropic has no confirmed equivalent.
```

### 1.8 第一批正式确认顺序（已确认）

第一批正式字段确认只处理最简单、最能建立模板的顶层字段组：

1. `model`
2. `stream`：只确认 request 层 boolean，不确认 stream event 子结构。
3. `temperature`
4. `top_p`
5. `metadata`

第一批暂不处理：

- `tools`
- `tool_choice`
- `reasoning` / `reasoning_effort` / `thinking`
- `input[]` / `messages[]` / `content[]`
- stream event / delta / done / final response 子矩阵
- token limit 字段
- cache 字段

第一批每个字段必须按固定步骤确认：

1. 查 OpenAI Responses 官方说明。
2. 查 OpenAI Chat 官方说明。
3. 查 Anthropic Messages 官方说明。
4. 查 AxonHub Owner / Storage Path。
5. 写 `Value Shape` / `Allowed Values` / `Default Semantics` / `Null/Omitted Semantics` / `Unknown Value Policy`。
6. 写六方向转换策略和不能转换原因。
7. 写代码证据。
8. 写测试证据；没有测试证据则保持 `PARTIAL`。

### 1.9 字段确认批次分类（已确认）

后续确认必须一批一批推进。每批只处理一类字段/结构，完成该批审计后再进入下一批，避免一次性处理过多导致误判。

| Batch | 名称 | 范围 | 代表字段 / 结构 | 为什么这一批放一起 | 进入下一批前的出口条件 |
|---:|---|---|---|---|---|
| 1 | 简单公共顶层字段 | 三协议都有或高度相似的简单 request 顶层字段 | `model`, request-level `stream`, `temperature`, `top_p`, `metadata` | 值形态简单，适合打磨表格模板 | 每个字段写完官方来源、值语义、架构归属、六方向策略、代码证据；无测试则保持 `PARTIAL` |
| 2 | 简单但有值域差异的顶层字段 | 字段名/用途接近，但 enum、默认值或协议语义可能不同 | `service_tier`, `safety_identifier`, deprecated `user`, Chat `stop` / Anthropic `stop_sequences` | 看似可直接转，实际容易因值域/弃用语义误判 | 必须补值映射表和 unknown value policy |
| 3 | token limit / 预算字段 | 输出 token、工具调用上限等限制类字段 | Responses `max_output_tokens`, Chat `max_completion_tokens` / deprecated `max_tokens`, Anthropic `max_tokens`, Responses `max_tool_calls` | 字段名相似但 reasoning/thinking 是否计入不同，风险高 | 必须确认官方计数口径、reasoning/thinking 影响、目标协议无等价物时的诊断策略 |
| 4 | 协议状态 / 缓存 / server-side continuation | server state、conversation、cache、store、background 等 | Responses `previous_response_id`, `conversation`, `prompt`, `background`, `store`, `prompt_cache_key`, `prompt_cache_retention`, `truncation`, `context_management`, Anthropic `container`, `inference_geo` | 多数无法跨协议无损，只能 sidecar 或 diagnostic | 必须明确 same-protocol replay 路径和跨协议不能转换原因 |
| 5 | 输出配置 / 模态 | 控制输出格式、音频、文本、预测输出等 | Responses `text`, Chat `response_format`, Chat `modalities`, Chat `audio`, Chat `prediction`, Chat `verbosity`, Anthropic `output_config` | object/union 多，不能只按字段名映射 | 必须拆子 variant；总览行不能 `CONFIRMED` |
| 6 | 消息 / 输入 / 内容块 | 三协议 conversation/input/content 结构 | Responses `input[]`, Responses `output[]`, Chat `messages[]`, Chat content parts, Anthropic `messages[].content[]`, Anthropic `system` | 结构差异最大，涉及 role、顺序、多模态、tool result | 必须按 role/content block/item type 拆子矩阵 |
| 7 | 工具定义 / 工具选择 / 工具调用 | tools、tool_choice、tool call、tool result、MCP/namespace | Responses `tools[]`, Responses `tool_choice`, Chat `tools[]`, Chat `tool_calls[]`, deprecated `functions/function_call`, Anthropic `tools[]`, Anthropic `tool_use/tool_result`, Codex `namespace`, Anthropic `mcp_servers/mcp_toolset`, Chat `web_search_options` | 容易漏掉 variant；这次 namespace bug 就属于这里 | 必须按 tool family 和 call/result lifecycle 拆子矩阵 |
| 8 | reasoning / thinking | 推理配置、思考输出、签名、加密 reasoning | Responses `reasoning`, Responses reasoning item/encrypted content, Chat `reasoning_effort`, Anthropic `thinking`, `thinking_delta`, `signature_delta`, `redacted_thinking` | 值域/预算/回放要求复杂，不能直接按名字映射 | 必须确认官方值、预算语义、stream 和 non-stream 回放 |
| 9 | stream event 矩阵 | 三协议 stream 事件/增量/完成/usage/error | Responses semantic SSE events, Chat `chat.completion.chunk`, Anthropic `message_start/content_block_delta/message_delta/message_stop` | event 形状完全不同，必须独立审计 | 每类 event 必须覆盖 delta、done、final response、usage/error 如适用 |
| 10 | deprecated / unknown / future compatibility | 已弃用字段、未知字段、未来变体、diagnostics | Chat `functions`, `function_call`, `seed`, deprecated `max_tokens`, unknown top-level fields, unknown enum, unknown typed union variant | 维护性和向前兼容要求，不应混入业务字段批次 | 必须确认 raw preserve、LossyDowngrade、unsupported_error 策略 |

批次规则：

1. 批次编号不是实现优先级，而是审计推进顺序。
2. 每批可以拆成更小 slice；一个 slice 只处理少量字段。
3. 当前批次没有完成前，不把下一批字段标为 `CONFIRMED`。
4. 发现分类错误时，先更新本节分类，再继续填矩阵。
5. 任意字段如果同时属于多个批次，按风险最高的批次处理。例如 `stream_options` 同时是顶层字段和 stream 相关配置，应放入 Batch 9 或单独拆子行。

### 1.10 顶层字段数量与切片计划（已确认）

当前字段数量只统计本地官方 baseline 的**请求顶层字段**，不包括 nested content block、tool variant、stream event 子字段。嵌套结构必须到对应批次再拆子矩阵，不能提前用一个粗略数字假装完成。

| 协议 | 顶层字段条目数 | 说明 |
|---|---:|---|
| OpenAI Responses | 29 | 来自 SRC-RSP-BASE §2 |
| OpenAI Chat Completions | 36 | 31 个 current + 5 个 deprecated，来自 SRC-CHAT-BASE §2 |
| Anthropic Messages | 18 | 来自 SRC-ANT-BASE §2 |
| Anthropic MCP companion | 1 | `mcp_servers`，来自 SRC-ANT-MCP |
| 合计协议字段条目 | 84 | 同名字段在不同协议中分别计数 |
| 去重顶层字段名 | 58 | 只用于规划，不表示语义相同 |

顶层字段第一轮规划为 10 个 batch、约 39 个小切片。每个切片只处理 1~3 个字段或一个小结构族；如果过程中发现字段值/variant 比预期复杂，必须继续拆小，不得硬塞进原切片。

| Batch | 范围 | 顶层字段/结构 | 计划切片 | 说明 |
|---:|---|---|---:|---|
| 1 | 简单公共顶层字段 | `model`, request `stream`, `temperature`, `top_p`, `metadata` | 4 | 先建立表格模板；request `stream` 不含 stream event |
| 2 | 简单但值域有差异 | `service_tier`, `safety_identifier`, `user`, `stop`, `stop_sequences` | 4 | 重点做值域/弃用语义/rename 策略 |
| 3 | token limit / 预算 | `max_output_tokens`, `max_completion_tokens`, `max_tokens`, `max_tool_calls` | 3 | 必须确认 reasoning/thinking 是否计入 |
| 4 | 状态 / 缓存 / server-side continuation | `background`, `context_management`, `conversation`, `previous_response_id`, `prompt`, `prompt_cache_key`, `prompt_cache_retention`, `store`, `truncation`, `container`, `inference_geo` | 6 | 多数可能 same-protocol-only 或 diagnostic |
| 5 | 输出配置 / 模态 / 生成控制 | `text`, `response_format`, `modalities`, `audio`, `prediction`, `verbosity`, `output_config`, `moderation` | 5 | object/union 必须继续拆 variant |
| 6 | 消息 / 输入 / 内容入口 | `input`, `instructions`, `messages`, `system` | 4 | 这里只审顶层入口；content block 子矩阵另拆 |
| 7 | 工具定义 / 选择 / 调用入口 | `tools`, `tool_choice`, `parallel_tool_calls`, `function_call`, `functions`, `web_search_options`, `mcp_servers` | 7 | 必须拆 tool family；Codex namespace 属于本批子项 |
| 8 | reasoning / thinking | `reasoning`, `reasoning_effort`, `thinking` | 4 | high-risk；值域、预算、stream/signature 单独确认 |
| 9 | stream 配置与事件矩阵入口 | `stream_options` + 三协议 stream event families | 3+ | 顶层字段少，但 event 子矩阵会很大，按事件族另拆 |
| 10 | 采样/概率/兼容兜底 | `frequency_penalty`, `presence_penalty`, `logit_bias`, `logprobs`, `top_logprobs`, `top_k`, `seed`, `n`, unknown/future variants | 6 | deprecated/unknown/future 策略一起兜底 |

切片完成规则：

1. 每个切片结束时只更新矩阵文档，不顺手改代码。
2. 如果发现代码与矩阵规则不一致，只记录架构/实现问题，不在字段审计切片里直接修。
3. 每个 batch 完成后做一次小审查：字段是否漏列、值是否漏确认、方向是否空白、证据是否足够。
4. 所有 batch 第一轮完成后，再进入架构复盘：判断作者现有 module/seam 是否需要 deepening。

## 2. 行分类

| 类别 ID | 类别 | 定义 | 典型例子 | 审计重点 |
|---|---|---|---|---|
| TOP | 请求顶层字段 | request body 顶层参数 | `model`, `stream`, `metadata`, `max_output_tokens` | 是否接住、是否映射、不能映射是否保真 |
| MSG | 消息/输入结构 | conversation/input/messages/content blocks | Responses `input[]`, Chat `messages[]`, Anthropic `content[]` | 顺序、role、typed content 是否保留 |
| TOOL_DEF | 工具定义 | 请求里的可用工具声明 | Responses `tools[]`, Chat `tools[]`, Anthropic `tools[]` | tool type、schema、strict、defer/lazy 是否保留 |
| TOOL_CALL | 工具调用/工具结果 | 模型发起工具调用、客户端回填结果 | Chat `tool_calls`, Responses `function_call`, Anthropic `tool_use/tool_result` | call id、name、namespace、arguments、typed output |
| REASONING | 思考/推理 | reasoning/thinking 配置和输出 | Responses `reasoning`, Chat `reasoning_effort`, Anthropic `thinking` | token budget/effort/signature/encrypted content |
| STREAM | 流式事件 | SSE/chunk 事件协议 | Responses semantic events, Chat chunks, Anthropic named events | delta/done 成对、最终对象、usage、error |
| STATE | 状态/上下文/缓存 | server-side state/context/cache | `previous_response_id`, `conversation`, `prompt_cache_key` | 是否有等价物；无等价物时是否保存/诊断 |
| OUTPUT_CFG | 输出配置/模态 | response format/audio/text/modalities | `text`, `response_format`, `modalities`, `audio` | JSON schema、audio/image/file 模态是否降级 |
| SAMPLING | 采样/限制/路由 | temperature/top_p/top_k/penalties/token limit | `temperature`, `top_p`, `max_tokens` | 字段名相似但语义不同的问题 |
| META_USAGE | metadata/usage/error | metadata、usage、stop/error | `usage`, `stop_reason`, `service_tier` | 计费/停止原因/错误结构是否保留 |
| CLIENT_EXT | 客户端/伴生扩展 | Codex、MCP connector 等客户端扩展 | Codex `namespace`, Anthropic `mcp_servers` | 是否官方主协议；是否需要桥接状态 |

## 3. 状态定义

| 状态 | 含义 | 能否算完成 |
|---|---|---|
| CONFIRMED | 官方含义、内部归属、转换策略、代码路径、真实测试全部确认 | 是 |
| PARTIAL | 只确认了部分方向或部分测试 | 否 |
| UNCHECKED | 还没有源码级/测试级确认 | 否 |
| BLOCKED | 缺官方证据、缺样本或目标协议无承载方案 | 否 |
| N/A | 该方向不存在或明确不适用，并已记录原因 | 可接受，但必须写原因 |

## 4. 矩阵列定义

| 列名 | 必填 | 说明 |
|---|---|---|
| ID | 是 | 稳定行号，如 `RSP.TOP.background` |
| Category | 是 | 使用第 2 节分类 ID |
| Source Protocol | 是 | `responses` / `chat` / `anthropic` / `codex-responses-ext` |
| Wire Path | 是 | 原协议路径，如 `tools[].tools[]`, `choices[].delta.tool_calls[]` |
| Official Meaning | 是 | 官方/源码含义，不能靠猜 |
| Source Evidence | 是 | 本地来源 ID + 文件/章节 |
| AxonHub Owner | 是 | `llm.Request`, `llm.Response`, `ProviderExtensions`, `TransformerMetadata`, `RawTopLevelFields`, `UNMAPPED` |
| Preservation Class | 是 | `native`, `raw_sidecar`, `bridge_metadata`, `lossy_map`, `drop_with_diagnostic`, `unsupported` |
| Rsp->Chat | 是 | 转换策略/状态 |
| Chat->Rsp | 是 | 转换策略/状态 |
| Rsp->Anthropic | 是 | 转换策略/状态 |
| Anthropic->Rsp | 是 | 转换策略/状态 |
| Chat->Anthropic | 是 | 转换策略/状态 |
| Anthropic->Chat | 是 | 转换策略/状态 |
| Stream Impact | 是 | `none`, `delta`, `done`, `final_response`, `usage`, `error`, `unchecked` |
| Code Evidence | 是 | 具体文件/函数；未查写 `UNCHECKED` |
| Test Evidence | 是 | 具体测试名/样本；未测写 `UNCHECKED` |
| Status | 是 | 第 3 节状态 |
| Uncertainty / TODO | 是 | 不确定点；CONFIRMED 时写 `none` |

## 4.1 第一批执行范围：只确认顶层字段清单和好确认归类

本批不判断所有嵌套结构，不判断所有 stream event，不判断所有 tool variant。只做三件事：

1. 基于本地官方 baseline 确认三个协议的**顶层请求字段清单**。
2. 给好确认的顶层字段补上“字段负责什么 / 值大概是什么 / 作者架构处理方式”。
3. 把不能直接确认的字段显式标为 `UNCHECKED` 或 `PARTIAL`，不伪装完成。

### 4.1.1 顶层字段清单完整性状态

| 协议 | 当前本地 baseline 字段数 | 字段来源 | 本批完整性结论 | 不包括什么 |
|---|---:|---|---|---|
| OpenAI Responses | 29 | SRC-RSP-BASE §2 | `BASELINE_COMPLETE`：相对本地官方 baseline 顶层字段已列出 | 不包括所有 `input[]` item variant、所有 `tools[]` variant、所有 stream event 字段 |
| OpenAI Chat Completions | 31 current + 5 deprecated | SRC-CHAT-BASE §2 | `BASELINE_COMPLETE`：相对本地官方 baseline 顶层字段已列出 | 不包括所有 message content part、tool call delta、response choice/logprob 子字段 |
| Anthropic Messages | 18 base + 1 companion extension | SRC-ANT-BASE §2, SRC-ANT-MCP | `BASELINE_COMPLETE_WITH_COMPANION`：base 字段已列出，`mcp_servers` 来自 companion MCP 文档 | 不包括 content block 子字段、tool schema 子字段、stream delta 子字段 |

这里的 `BASELINE_COMPLETE` 只表示“本地官方 baseline 顶层字段清单已覆盖”，不表示转换完成。

### 4.1.2 作者架构处理方式枚举

| Handling Mode | 归属 | 什么时候用 | 例子 | 是否可跨协议无损 |
|---|---|---|---|---|
| `common_native` | `llm.Request` / `llm.Response` | 三协议语义基本一致，可以进入公共模型 | `model`, `temperature`, `top_p` | 通常可以，但仍需检查值域/默认值 |
| `common_native_with_value_caveat` | `llm.Request` / `llm.Response` + 诊断 | 字段名/用途相近，但值域或默认语义不同 | `service_tier`, token limit 字段 | 不能默认无损 |
| `protocol_native_sidecar` | `ProviderExtensions` | 协议原生字段，公共模型放不下，但同协议 replay 必须保真 | Responses `background`, `include` | 跨协议通常不可直接表达 |
| `raw_fragment_ordered` | `ProviderExtensions` raw fragment | 数组内 typed union/未知 item，需要保留原始 index 和 raw JSON | Responses `RawTools`, `RawInputItems` | 只保证同协议顺序保真 |
| `bridge_metadata` | `TransformerMetadata` | 跨协议转换时恢复结构所需的临时桥接状态，不是协议字段 | Codex namespace reverse map | 只服务转换闭环，不发给 provider |
| `structural_transform` | 转换函数 | 两边语义相近但结构不同，必须写专门转换 | `input[]` ↔ `messages[]`, `tool_use` ↔ `tool_calls[]` | 取决于具体子结构 |
| `semantic_downgrade` | 转换函数 + `LossyDowngrade` | 只能表达近似含义 | `instructions` ↔ system/developer | 有损或近似 |
| `same_protocol_only` | `ProviderExtensions` | 目标协议无等价物，只能同协议恢复 | `previous_response_id`, `conversation` | 不可跨协议无损 |
| `drop_with_diagnostic` | `DiagnosticsProviderExtensions.LossyDowngrades` | 目标协议无等价物，转换时必须记录 | Anthropic `container` -> OpenAI | 不可无损 |
| `unsupported_error` | 转换入口 | 继续转换会误导或破坏语义，应报错 | 强依赖 server state 的不可降级请求 | 不转换 |
| `unchecked` | 未定 | 未查清 | 当前大量 nested 字段 | 不算完成 |

### 4.1.3 字段值确认列（后续每行必须补齐）

| 值相关列 | 说明 |
|---|---|
| `Value Shape` | `string` / `boolean` / `number` / `object` / `array` / `union` / `typed_union` |
| `Allowed Values / Variants` | 官方允许值、枚举值、union 分支；没查清写 `UNCHECKED` |
| `Default Semantics` | 字段缺失时的默认含义；没查清写 `UNCHECKED` |
| `Null / Omitted Semantics` | `null`、空数组、字段缺失是否不同；没查清写 `UNCHECKED` |
| `Unknown Value Policy` | 未知 enum / 未来 object variant 如何处理：raw preserve、diagnostic、error、drop |
| `Semantic Equivalence` | `exact` / `equivalent` / `approximate` / `none` / `unchecked` |
| `Lossiness` | `none` / `partial` / `total` / `unchecked` |

## 4.2 第一批：好确认顶层字段归类

| 语义组 | Responses 字段 | Chat 字段 | Anthropic 字段 | 值形态 | 字段负责什么 | 作者架构处理方式 | 初步转换判断 | 本批状态 | 后续必须确认 |
|---|---|---|---|---|---|---|---|---|---|
| 模型标识 | `model` | `model` | `model` | string | 选择目标模型/路由模型 | `common_native` -> `llm.Request.Model` | 字段可直接承载；模型别名/渠道规则另审 | PARTIAL | 模型别名、provider model rewrite、响应 `model` 回填 |
| 是否流式 | `stream` | `stream` | `stream` | boolean | 决定响应是否以 stream 返回 | `common_native` -> `llm.Request.Stream` + `structural_transform` for events | request 字段可承载；stream event 不能直接等同 | PARTIAL | 三协议 stream event 子矩阵 |
| 采样温度 | `temperature` | `temperature` | `temperature` | number | 控制随机性 | `common_native_with_value_caveat` | 可映射，但范围/default 未逐源确认 | PARTIAL | 官方范围、缺省语义、provider clamp |
| nucleus sampling | `top_p` | `top_p` | `top_p` | number | nucleus sampling | `common_native_with_value_caveat` | 可映射，但范围/default 未逐源确认 | PARTIAL | 官方范围、缺省语义、与 temperature 联动建议 |
| metadata | `metadata` | `metadata` | `metadata` | object | 请求附加元数据/用户标记 | `common_native_with_value_caveat` | 可承载；限制和含义可能不同 | PARTIAL | key/value 限制、是否传 provider、隐私字段 |
| service tier | `service_tier` | `service_tier` | `service_tier` | enum/string | 服务层级/优先级/批处理 | `common_native_with_value_caveat` / `value_map` | 字段名相同但值域不同，不能盲直传 | PARTIAL | 三协议允许值映射表 |
| 输出 token 上限 | `max_output_tokens` | `max_completion_tokens`, deprecated `max_tokens` | `max_tokens` | integer | 限制输出 token；不同协议计数口径可能不同 | `common_native_with_value_caveat` / `value_map` | 可近似映射，不可默认语义完全一致 | PARTIAL | reasoning/thinking token 是否计入、deprecated 行为 |
| 停止序列 | 无当前 baseline 顶层同名字段 | `stop` | `stop_sequences` | string or array | 自定义停止序列 | `value_map` | Chat ↔ Anthropic 可 rename；Responses 当前顶层无同名字段 | PARTIAL | Responses 是否存在 nested/兼容 stop，数组长度限制 |
| 系统/指令 | `instructions` | `messages[].role=developer/system` | `system` | string / content blocks / message | 高优先级指令 | `structural_transform` + `semantic_downgrade` | 需要专门规则，不能直接字段拷贝 | PARTIAL | developer vs system 优先级、content block 保真 |
| 并行工具调用 | `parallel_tool_calls` | `parallel_tool_calls` | 无明确同名顶层字段 | boolean | 是否允许并行工具调用 | `common_native_with_value_caveat` | Responses ↔ Chat 可承载；Anthropic 方向未确认 | PARTIAL | Anthropic tool_use 是否有等价控制 |
| 工具定义 | `tools` | `tools` | `tools` | typed array | 声明可用工具 | `structural_transform` + `raw_fragment_ordered` + `bridge_metadata` | 只能分 tool family 判断 | PARTIAL | function/custom/hosted/namespace/mcp_toolset 子矩阵 |
| 工具选择 | `tool_choice` | `tool_choice` | `tool_choice` | union/object/string | 限制/指定工具选择 | `structural_transform` + `protocol_native_sidecar` | 字段名相同但形状不同 | PARTIAL | object variant、raw preserve、unsupported forms |
| reasoning/thinking | `reasoning` | `reasoning_effort` | `thinking` | object/string/object | 控制或承载模型推理/思考行为 | `structural_transform` + `value_map` + sidecar | 绝不能只按名字硬转 | PARTIAL | effort 值、budget、signature、encrypted content |
| prompt cache key | `prompt_cache_key` | `prompt_cache_key` | 无同名顶层；content `cache_control` 是另一层 | string | prompt cache routing key | `common_native_with_value_caveat` / sidecar | Responses ↔ Chat 可能可承载；Anthropic 不等价 | PARTIAL | 与 Anthropic cache_control 区分 |
| prompt cache retention | `prompt_cache_retention` | `prompt_cache_retention` | 无同名顶层 | enum/string | prompt cache 保留策略 | `protocol_native_sidecar` / `value_map` | OpenAI 内部可保真；Anthropic 不等价 | PARTIAL | 允许值、raw replay 路径 |
| 安全/用户标识 | `safety_identifier`, deprecated `user` | `safety_identifier`, deprecated `user` | `metadata` 内用户相关字段 | string/object | 终端用户/安全识别 | `common_native_with_value_caveat` | OpenAI 字段可近似；Anthropic 不是同形字段 | PARTIAL | user deprecation、metadata 具体字段 |


## 5. 顶层请求字段清单（待逐行转换确认）

> 说明：本节先列官方 baseline 中的顶层字段。除明确标为 `CONFIRMED` 的行外，其余都是转换审计任务，不是完成证明。

### 5.1 OpenAI Responses 顶层字段

| ID | Category | Source Protocol | Wire Path | Official Meaning | Source Evidence | AxonHub Owner | Preservation Class | Rsp->Chat | Chat->Rsp | Rsp->Anthropic | Anthropic->Rsp | Chat->Anthropic | Anthropic->Chat | Stream Impact | Code Evidence | Test Evidence | Status | Uncertainty / TODO |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| RSP.TOP.background | TOP | responses | `background` | 后台运行 response | SRC-RSP-BASE §2 | Responses `Request.Background` + `TransformerMetadata[background]` | native+metadata | no-synth | N/A | no-synth | N/A | N/A | N/A | final_response | `responses/model.go`, `responses/inbound.go`, `responses/outbound.go` | `TestConvertToLLMRequest_Background`（true/false/absent） | PARTIAL | Responses 同协议 typed+metadata 保真；外协议无语义桥，不从 raw/metadata 推导等价支持 |
| RSP.TOP.context_management | STATE | responses | `context_management` | 请求级上下文管理/压缩配置 | SRC-RSP-BASE §2 | `ProviderExtensions.OpenAIResponses.Request.RawTopLevelFields` generic fallback | raw_fallback_only | UNCHECKED | N/A | UNCHECKED | N/A | N/A | N/A | unchecked | `responses/request_extensions.go` generic unknown-top-level capture/merge | 无字段专测；仅 generic raw-top-level tests | UNCHECKED | code-partial：只能同协议保留未建模 bytes；不等于理解 `context_management` 语义，仍需字段级专测/审计 |
| RSP.TOP.conversation | STATE | responses | `conversation` | Responses conversation/state attachment | SRC-RSP-BASE §2 | Responses `Request.Conversation` 仍 commented out；generic `RawTopLevelFields` fallback | raw_fallback_only | UNCHECKED | N/A | UNCHECKED | N/A | N/A | N/A | final_response | `responses/model.go`, `responses/request_extensions.go` | 无字段专测；仅 generic raw-top-level tests | UNCHECKED | code-partial/raw fallback；无 Chat/Anthropic 直接等价物，不得写成语义支持 |
| RSP.TOP.include | TOP | responses | `include` | 请求额外输出数据，如 encrypted reasoning/search results | SRC-RSP-BASE §2 | Responses `Request.Include []string` + `TransformerMetadata[shared.MetadataKeyInclude]` | native+metadata | no-synth | N/A | no-synth | N/A | N/A | N/A | final_response/stream | `responses/model.go`, `responses/inbound.go`, `responses/outbound.go` | include transformer-metadata + outbound include tests | PARTIAL | Responses 入出站保留已证实；具体 include 成员语义/外协议映射仍需逐值审计 |
| RSP.TOP.input | MSG | responses | `input` | string 或 typed input/message/tool item list | SRC-RSP-BASE §2/§3 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | N/A | N/A | final_response | UNCHECKED | UNCHECKED | PARTIAL | 已有转换但 typed item 全量闭环未确认 |
| RSP.TOP.instructions | MSG | responses | `instructions` | 顶层 instruction string | SRC-RSP-BASE §2 | UNCHECKED | native/lossy_map? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | N/A | N/A | none | UNCHECKED | UNCHECKED | PARTIAL | 需确认 system/developer 映射策略 |
| RSP.TOP.max_output_tokens | SAMPLING | responses | `max_output_tokens` | Responses 输出 token 上限 | SRC-RSP-BASE §2 | UNCHECKED | native/lossy_map? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | N/A | N/A | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Chat `max_completion_tokens` / Anthropic `max_tokens` 语义需逐项确认 |
| RSP.TOP.max_tool_calls | TOOL_DEF | responses | `max_tool_calls` | 工具调用数量上限 | SRC-RSP-BASE §2 | Responses `Request.MaxToolCalls` + `TransformerMetadata[max_tool_calls]` | native+metadata | no-synth | N/A | no-synth | N/A | N/A | N/A | final_response | `responses/model.go`, `responses/inbound.go`, `responses/outbound.go` | max_tool_calls metadata roundtrip tests | PARTIAL | Responses 同协议入出站；Chat/Anthropic 无明确等价物，不合成 |
| RSP.TOP.metadata | TOP | responses | `metadata` | metadata map | SRC-RSP-BASE §2 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 需确认大小/类型限制差异 |
| RSP.TOP.model | TOP | responses | `model` | 模型 ID | SRC-RSP-BASE §2 | `llm.Request.Model` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 基础字段，但路由/模型别名未逐项审计 |
| RSP.TOP.parallel_tool_calls | TOOL_DEF | responses | `parallel_tool_calls` | 是否允许并行工具调用 | SRC-RSP-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | UNCHECKED | UNCHECKED | none | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 等价策略未确认 |
| RSP.TOP.previous_response_id | STATE | responses | `previous_response_id` | Responses server-side state continuation | SRC-RSP-BASE §2 | `llm.Request.PreviousResponseID` direct | native/common-direct | no-synth | N/A | no-synth | N/A | N/A | N/A | final_response/stream | `llm/model.go`, `responses/inbound.go`, `responses/outbound.go`, `responses/aggregator.go` | outbound/websocket tests；`TestAggregateStreamChunks_PreservesPreviousResponseID` | PARTIAL | Responses state id 直接承载；不能盲目转成 Chat/Anthropic context |
| RSP.TOP.prompt | STATE | responses | `prompt` | prompt object/reference | SRC-RSP-BASE §2 | Responses `Request.Prompt` + `TransformerMetadata[prompt]` | native+metadata | no-synth | N/A | no-synth | N/A | N/A | N/A | none | `responses/model.go`, `responses/inbound.go`, `responses/outbound.go` | `TestConvertToLLMRequest_Prompt`（inbound/outbound/absent） | PARTIAL | 完整 Prompt object 同协议 metadata roundtrip；外协议无等价字段 |
| RSP.TOP.prompt_cache_key | STATE | responses | `prompt_cache_key` | prompt cache routing key | SRC-RSP-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Anthropic cache_control 不等价 |
| RSP.TOP.prompt_cache_retention | STATE | responses | `prompt_cache_retention` | prompt cache retention policy | SRC-RSP-BASE §2 | UNCHECKED | raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 需确认允许值和保真路径 |
| RSP.TOP.reasoning | REASONING | responses | `reasoning` | reasoning configuration/state | SRC-RSP-BASE §2 | Responses reasoning model + metadata/raw merge (G7) | native+raw_sidecar | PARTIAL | PARTIAL | LossyDowngrade | LossyDowngrade | LossyDowngrade | LossyDowngrade | delta/final_response | `llm/transformer/openai/responses` reasoning handlers | `reasoning_context_test.go` / `reasoning_g7_test.go` + stream/aggregator tests | PARTIAL | G7 same-protocol：context / generate_summary 身份 / content[] / stream prefer-text / unknown nested 已测；跨协议不与 Chat `reasoning_effort`、Anthropic `thinking` 简单等同 |
| RSP.TOP.safety_identifier | META_USAGE | responses | `safety_identifier` | stable safety/user identifier | SRC-RSP-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 deprecated `user` 差异需确认 |
| RSP.TOP.service_tier | META_USAGE | responses | `service_tier` | 服务层级 | SRC-RSP-BASE §2 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 各协议允许值不同 |
| RSP.TOP.store | STATE | responses | `store` | 是否存储 response | SRC-RSP-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 无直接等价物 |
| RSP.TOP.stream | STREAM | responses | `stream` | 是否使用 Responses semantic SSE | SRC-RSP-BASE §2 | `llm.Request.Stream` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/done/final_response | UNCHECKED | UNCHECKED | PARTIAL | 三协议 stream 形状完全不同，需按事件矩阵确认 |
| RSP.TOP.stream_options | STREAM | responses | `stream_options` | Responses stream options | SRC-RSP-BASE §2 | UNCHECKED | raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | delta/final_response | UNCHECKED | UNCHECKED | PARTIAL | nested options 保真需确认 |
| RSP.TOP.temperature | SAMPLING | responses | `temperature` | 采样温度 | SRC-RSP-BASE §2 | `llm.Request.Temperature` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 基础映射但边界/默认值未审计 |
| RSP.TOP.text | OUTPUT_CFG | responses | `text` | text output configuration | SRC-RSP-BASE §2 | UNCHECKED | raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 与 Chat `response_format` 不完全等价 |
| RSP.TOP.tool_choice | TOOL_DEF | responses | `tool_choice` | Responses tool choice | SRC-RSP-BASE §2 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | object forms 全量未确认 |
| RSP.TOP.tools | TOOL_DEF | responses | `tools` | Responses native tools array | SRC-RSP-BASE §2/§4 | UNCHECKED | native+raw_sidecar+bridge_metadata | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | delta/done | `llm/transformer/openai/responses/inbound.go`, `llm/transformer/openai/outbound.go` | namespace tests confirmed only | PARTIAL | namespace confirmed；其他 tool families 未全量确认 |
| RSP.TOP.top_logprobs | SAMPLING | responses | `top_logprobs` | 输出 logprob 配置 | SRC-RSP-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 与 Chat `logprobs` dependency 需确认 |
| RSP.TOP.top_p | SAMPLING | responses | `top_p` | nucleus sampling | SRC-RSP-BASE §2 | `llm.Request.TopP` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 基础映射但默认/范围未审计 |
| RSP.TOP.truncation | STATE | responses | `truncation` | deprecated truncation mode | SRC-RSP-BASE §2 | UNCHECKED | raw_sidecar? | PARTIAL | N/A | UNCHECKED | N/A | N/A | N/A | none | UNCHECKED | UNCHECKED | PARTIAL | deprecated 但必须保真/诊断 |
| RSP.TOP.user | META_USAGE | responses | `user` | deprecated user identifier | SRC-RSP-BASE §2 | `llm.Request.User`? | native/lossy_map? | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 `safety_identifier` 替代关系需确认 |

### 5.2 OpenAI Chat Completions 顶层字段

| ID | Category | Source Protocol | Wire Path | Official Meaning | Source Evidence | AxonHub Owner | Preservation Class | Rsp->Chat | Chat->Rsp | Rsp->Anthropic | Anthropic->Rsp | Chat->Anthropic | Anthropic->Chat | Stream Impact | Code Evidence | Test Evidence | Status | Uncertainty / TODO |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| CHAT.TOP.messages | MSG | chat | `messages` | Chat ordered role-message array | SRC-CHAT-BASE §2/§3 | UNCHECKED | native | PARTIAL | PARTIAL | N/A | N/A | PARTIAL | PARTIAL | delta/final_response | UNCHECKED | UNCHECKED | PARTIAL | typed content/tool/result 全量未确认 |
| CHAT.TOP.model | TOP | chat | `model` | model identifier | SRC-CHAT-BASE §2 | `llm.Request.Model` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 基础字段，路由别名未审计 |
| CHAT.TOP.audio | OUTPUT_CFG | chat | `audio` | audio modalities parameters | SRC-CHAT-BASE §2 | `openAIChatRawPreserveFields` (`chat_n.go`) | raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | delta/final_response | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` output-controls | PARTIAL | G4 same-protocol raw preserve；不 bridge 到 Responses/Anthropic 音频形态 |
| CHAT.TOP.frequency_penalty | SAMPLING | chat | `frequency_penalty` | frequency penalty | SRC-CHAT-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 无直接等价物 |
| CHAT.TOP.logit_bias | SAMPLING | chat | `logit_bias` | token logit bias map | SRC-CHAT-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 目标协议支持差异需确认 |
| CHAT.TOP.logprobs | SAMPLING | chat | `logprobs` | return output-token log probabilities | SRC-CHAT-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 与 Responses `top_logprobs` 联动未确认 |
| CHAT.TOP.max_completion_tokens | SAMPLING | chat | `max_completion_tokens` | Chat generated-token maximum | SRC-CHAT-BASE §2 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Responses/Anthropic token 上限语义差异需确认 |
| CHAT.TOP.metadata | TOP | chat | `metadata` | metadata map | SRC-CHAT-BASE §2 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 限制差异未审计 |
| CHAT.TOP.modalities | OUTPUT_CFG | chat | `modalities` | output modalities | SRC-CHAT-BASE §2 | `llm.Request.Modalities` + OpenAI Chat/Responses typed fields（typed code path located） | typed_code_path_located/code-partial | UNCHECKED | UNCHECKED | no-synth | N/A | no-synth | N/A | final_response | `llm/model.go`, `openai/model.go`, `responses/model.go`, OpenAI converters | 仅 Responses/Gemini 相关：`TestConvertToLLMRequest_Modalities`、Responses outbound modalities case、Gemini `TestModalitiesRoundTrip`；缺 Chat 字段级同协议专测 | UNCHECKED | code-partial；不得把 Responses/Gemini 测试当作 Chat same-protocol 证据；Anthropic 无等价字段，Chat roundtrip/值域/组合仍需审计 |
| CHAT.TOP.moderation | META_USAGE | chat | `moderation` | moderation config | SRC-CHAT-BASE §2 | `openAIChatRawPreserveFields` (`chat_n.go`) | raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | error/final_response | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` output-controls | PARTIAL | G4 same-protocol raw preserve；跨协议 no-synth |
| CHAT.TOP.n | TOP | chat | `n` | number of choices | SRC-CHAT-BASE §2 | `openAIChatRawPreserveFields` (`chat_n.go`) | raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | final_response | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` (`TestOpenAIChatRequestN*`) | PARTIAL | G1 same-protocol preserve；Responses/Anthropic 多 choice no-synth / LossyDowngrade |
| CHAT.TOP.parallel_tool_calls | TOOL_DEF | chat | `parallel_tool_calls` | whether tools may run in parallel | SRC-CHAT-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Anthropic 工具流差异未确认 |
| CHAT.TOP.prediction | OUTPUT_CFG | chat | `prediction` | predicted output config | SRC-CHAT-BASE §2 | `openAIChatRawPreserveFields` (`chat_n.go`) | raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | final_response | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` output-controls | PARTIAL | G4 same-protocol raw preserve；跨协议 no-synth |
| CHAT.TOP.presence_penalty | SAMPLING | chat | `presence_penalty` | presence penalty | SRC-CHAT-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 无直接等价物 |
| CHAT.TOP.prompt_cache_key | STATE | chat | `prompt_cache_key` | prompt cache routing key | SRC-CHAT-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Anthropic cache_control 不等价 |
| CHAT.TOP.prompt_cache_retention | STATE | chat | `prompt_cache_retention` | prompt cache retention policy | SRC-CHAT-BASE §2 | `openAIChatRawPreserveFields` (`chat_n.go`) | raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | none | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` prompt_cache_retention | PARTIAL | G2 same-protocol preserve；与 Anthropic cache_control 不等价，不伪映射 |
| CHAT.TOP.reasoning_effort | REASONING | chat | `reasoning_effort` | reasoning effort shorthand | SRC-CHAT-BASE §2 | UNCHECKED | native/lossy_map? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | UNCHECKED | UNCHECKED | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Responses `reasoning`/Anthropic `thinking` 转换未确认 |
| CHAT.TOP.response_format | OUTPUT_CFG | chat | `response_format` | text/JSON schema/object format | SRC-CHAT-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 与 Responses `text.format` 关系未全量确认 |
| CHAT.TOP.safety_identifier | META_USAGE | chat | `safety_identifier` | stable safety/user identifier | SRC-CHAT-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 `user` 替代关系未确认 |
| CHAT.TOP.service_tier | META_USAGE | chat | `service_tier` | service tier selection | SRC-CHAT-BASE §2 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 各协议值域不同 |
| CHAT.TOP.stop | SAMPLING | chat | `stop` | stop sequence(s) | SRC-CHAT-BASE §2 | UNCHECKED | native/lossy_map? | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 是 `stop_sequences` |
| CHAT.TOP.store | STATE | chat | `store` | whether to store output | SRC-CHAT-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 无直接等价物 |
| CHAT.TOP.stream | STREAM | chat | `stream` | stream chat chunks | SRC-CHAT-BASE §2 | `llm.Request.Stream` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/done/final_response | UNCHECKED | UNCHECKED | PARTIAL | 事件形状不同，需单独矩阵 |
| CHAT.TOP.stream_options | STREAM | chat | `stream_options` | Chat stream options | SRC-CHAT-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | usage/delta | UNCHECKED | UNCHECKED | PARTIAL | nested options 未确认 |
| CHAT.TOP.temperature | SAMPLING | chat | `temperature` | sampling temperature | SRC-CHAT-BASE §2 | `llm.Request.Temperature` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 基础映射但范围/default 未审计 |
| CHAT.TOP.tool_choice | TOOL_DEF | chat | `tool_choice` | Chat tool-choice config | SRC-CHAT-BASE §2 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | custom/tool-specific forms 未确认 |
| CHAT.TOP.tools | TOOL_DEF | chat | `tools` | Chat tool definitions | SRC-CHAT-BASE §2 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | delta/done | UNCHECKED | UNCHECKED | PARTIAL | function/custom/lazy 行为未全量确认 |
| CHAT.TOP.top_logprobs | SAMPLING | chat | `top_logprobs` | top log probabilities per token | SRC-CHAT-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | requires `logprobs` |
| CHAT.TOP.top_p | SAMPLING | chat | `top_p` | nucleus sampling | SRC-CHAT-BASE §2 | `llm.Request.TopP` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 基础映射但范围/default 未审计 |
| CHAT.TOP.verbosity | OUTPUT_CFG | chat | `verbosity` | verbosity setting | SRC-CHAT-BASE §2 | UNCHECKED | raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 与 Responses text verbosity 关系未确认 |
| CHAT.TOP.web_search_options | TOOL_DEF | chat | `web_search_options` | Chat web search options, not Responses web_search tool shape | SRC-CHAT-BASE §2 | `openAIChatRawPreserveFields` (`chat_n.go`) | raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | delta/final_response | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` + anthropic lossy notes | PARTIAL | G5a same-protocol preserve；禁止改名为 Responses `web_search` tool |
| CHAT.TOP.function_call | TOOL_DEF | chat | `function_call` | deprecated tool choice predecessor | SRC-CHAT-BASE §2 deprecated | Chat deprecated origin metadata + raw preserve (G5b) | compatibility/raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | none | `llm/transformer/openai` deprecated functions path | `chat_deprecated_functions_test.go` | PARTIAL | G5b：request/stream/history 可回写 legacy shape；不破坏 modern tools |
| CHAT.TOP.functions | TOOL_DEF | chat | `functions` | deprecated tool definitions | SRC-CHAT-BASE §2 deprecated | Chat deprecated origin metadata + raw preserve (G5b) | compatibility/raw_preserve | no-synth | PARTIAL (same Chat) | LossyDowngrade | N/A | LossyDowngrade | N/A | delta/done | `llm/transformer/openai` deprecated functions path | `chat_deprecated_functions_test.go` | PARTIAL | G5b same-protocol legacy roundtrip；不与 modern `tools` 合并 |
| CHAT.TOP.max_tokens | SAMPLING | chat | `max_tokens` | deprecated output token limit | SRC-CHAT-BASE §2 deprecated | UNCHECKED | compatibility | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 reasoning models 不兼容风险 |
| CHAT.TOP.seed | SAMPLING | chat | `seed` | deprecated/compat deterministic seed | SRC-CHAT-BASE §2 deprecated | UNCHECKED | compatibility/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 目标协议支持差异未确认 |
| CHAT.TOP.user | META_USAGE | chat | `user` | deprecated user identifier | SRC-CHAT-BASE §2 deprecated | `llm.Request.User`? | compatibility | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 `safety_identifier` 替代关系未确认 |

### 5.3 Anthropic Messages 顶层字段

| ID | Category | Source Protocol | Wire Path | Official Meaning | Source Evidence | AxonHub Owner | Preservation Class | Rsp->Chat | Chat->Rsp | Rsp->Anthropic | Anthropic->Rsp | Chat->Anthropic | Anthropic->Chat | Stream Impact | Code Evidence | Test Evidence | Status | Uncertainty / TODO |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ANT.TOP.max_tokens | SAMPLING | anthropic | `max_tokens` | required output token maximum | SRC-ANT-BASE §2 | UNCHECKED | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 OpenAI token 字段语义需确认 |
| ANT.TOP.messages | MSG | anthropic | `messages` | ordered user/assistant messages | SRC-ANT-BASE §2/§3 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | content block 全量未确认 |
| ANT.TOP.model | TOP | anthropic | `model` | Claude model identifier | SRC-ANT-BASE §2 | `llm.Request.Model` | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 模型别名/渠道映射未确认 |
| ANT.TOP.cache_control | STATE | anthropic | `cache_control` | 顶层 ephemeral prompt-cache control；不同于 content-block `cache_control` | SRC-ANT-RAW (`cache_control: optional CacheControlEphemeral`) | `MessageRequest.CacheControl` + `TransformerMetadata[anthropic_cache_control]` opaque restore（production path located） | native+metadata/code-partial | N/A | N/A | no-equivalent | no-equivalent | no-equivalent | no-equivalent | none | `anthropic/model.go`, `anthropic/inbound_convert.go`, `anthropic/outbound_convert.go` | 顶层字段级 same-protocol targeted roundtrip missing；不得用 content-block `cache_control` 测试替代 | UNCHECKED | official raw confirmed + production code-partial；缺顶层 targeted roundtrip，且仍需值域、null/omitted、TTL 审计；不等价于 OpenAI prompt cache 字段 |
| ANT.TOP.container | STATE | anthropic | `container` | container/context feature | SRC-ANT-BASE §2 | Anthropic model + metadata (G3) | opaque JSON + metadata | N/A | N/A | no-synth | PARTIAL (same Anthropic) | no-synth | PARTIAL (same Anthropic) | final_response | `llm/transformer/anthropic` model/metadata | `container_inference_geo_test.go` | PARTIAL | G3 same-protocol opaque preserve；OpenAI 无直接等价物，不伪映射 |
| ANT.TOP.inference_geo | META_USAGE | anthropic | `inference_geo` | inference geography | SRC-ANT-BASE §2 | Anthropic model + metadata (G3) | opaque JSON + metadata | N/A | N/A | no-synth | PARTIAL (same Anthropic) | no-synth | PARTIAL (same Anthropic) | final_response | `llm/transformer/anthropic` model/metadata | `container_inference_geo_test.go` | PARTIAL | G3 same-protocol opaque preserve；跨 OpenAI no-synth |
| ANT.TOP.metadata | TOP | anthropic | `metadata` | metadata object | SRC-ANT-BASE §2 | UNCHECKED | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 类型/限制差异未确认 |
| ANT.TOP.output_config | OUTPUT_CFG | anthropic | `output_config` | output config object | SRC-ANT-BASE §2 | Anthropic `MessageRequest.OutputConfig` + full object `TransformerMetadata[anthropic_output_config]` | native+metadata_full_object | N/A | N/A | PARTIAL (effort only) | PARTIAL (effort only) | PARTIAL (effort only) | PARTIAL (effort only) | final_response | `anthropic/model.go`, `anthropic/inbound_convert.go`, `anthropic/outbound_convert.go` | `TestOutputConfig_FormatTaskBudgetRoundTrip`, `TestOutputConfig_Inbound`, `TestOutputConfig_Outbound` | PARTIAL | Anthropic 同协议保留完整 object（effort/format/Hub extension task_budget）；跨协议仅 effort 部分映射，format/task_budget 不宣称等价 |
| ANT.TOP.service_tier | META_USAGE | anthropic | `service_tier` | standard/priority/batch tier | SRC-ANT-BASE §2/§7 | UNCHECKED | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 值域不同 |
| ANT.TOP.stop_sequences | SAMPLING | anthropic | `stop_sequences` | custom stop sequences | SRC-ANT-BASE §2 | UNCHECKED | native/lossy_map? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | maps to Chat `stop`? 需验证 |
| ANT.TOP.stream | STREAM | anthropic | `stream` | named SSE events | SRC-ANT-BASE §2/§6 | `llm.Request.Stream` | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/done/final_response | UNCHECKED | UNCHECKED | PARTIAL | event shape 完全不同 |
| ANT.TOP.system | MSG | anthropic | `system` | top-level system prompt; no system role in messages | SRC-ANT-BASE §2/§3 | UNCHECKED | native/lossy_map? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 Chat system/developer、Responses instructions 关系需确认 |
| ANT.TOP.temperature | SAMPLING | anthropic | `temperature` | sampling temperature | SRC-ANT-BASE §2 | `llm.Request.Temperature` | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 范围/default 未审计 |
| ANT.TOP.thinking | REASONING | anthropic | `thinking` | extended/adaptive thinking config | SRC-ANT-BASE §2/§3 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | thinking_delta/signature_delta/final_response | UNCHECKED | UNCHECKED | PARTIAL | 当前任务正在修；不能标完成 |
| ANT.TOP.tool_choice | TOOL_DEF | anthropic | `tool_choice` | tool choice config | SRC-ANT-BASE §2/§4 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 OpenAI tool_choice 形状不同 |
| ANT.TOP.tools | TOOL_DEF | anthropic | `tools` | client/server/toolset definitions | SRC-ANT-BASE §2/§4/§5 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | input_json_delta/done | UNCHECKED | UNCHECKED | PARTIAL | raw tool fields input_schema/strict/defer_loading 等未全量确认 |
| ANT.TOP.top_k | SAMPLING | anthropic | `top_k` | top-k sampling | SRC-ANT-BASE §2 | UNCHECKED | native/raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | UNCHECKED | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | OpenAI 无直接等价物 |
| ANT.TOP.top_p | SAMPLING | anthropic | `top_p` | nucleus sampling | SRC-ANT-BASE §2 | `llm.Request.TopP` | native | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 范围/default 未审计 |
| ANT.TOP.mcp_servers | CLIENT_EXT | anthropic | `mcp_servers` | Anthropic MCP connector companion parameter | SRC-ANT-MCP | Anthropic opaque top-level (G6) | raw_preserve | N/A | N/A | no-synth | PARTIAL (same Anthropic) | no-synth | PARTIAL (same Anthropic) | tool events | `llm/transformer/anthropic` MCP connector path | `mcp_connector_test.go` | PARTIAL | G6 same-protocol preserve；**禁止**桥到 Responses `mcp` / namespace |

## 6. 嵌套结构/行为矩阵（必须单独确认）

| ID | Category | Source Protocol | Wire Path | Official Meaning | Source Evidence | AxonHub Owner | Preservation Class | Rsp->Chat | Chat->Rsp | Rsp->Anthropic | Anthropic->Rsp | Chat->Anthropic | Anthropic->Chat | Stream Impact | Code Evidence | Test Evidence | Status | Uncertainty / TODO |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| RSP.MSG.input_items | MSG | responses | `input[]` | typed input/message/tool item list | SRC-RSP-BASE §3 | UNCHECKED | native+raw_sidecar | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | N/A | N/A | final_response | UNCHECKED | UNCHECKED | PARTIAL | 必须按 item type 展开子矩阵 |
| RSP.TOOL.function | TOOL_DEF | responses | `tools[].type=function` | function tool | SRC-RSP-BASE §4 | UNCHECKED | native | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | delta/done | UNCHECKED | UNCHECKED | PARTIAL | strict/defer_loading/schema 需确认 |
| RSP.TOOL.hosted | TOOL_DEF | responses | `tools[]` hosted families | web/file/computer/code/image hosted tools | SRC-RSP-BASE §4 | specific native tool raw families + generic Responses raw-tool merge | native_raw_family_partial | UNCHECKED | PARTIAL | UNCHECKED | UNCHECKED | UNCHECKED | UNCHECKED | delta/final_response | `responses/request_extensions.go`, native-tool classification/model paths | raw/native tool family tests exist；无统一 hosted 语义专测 | UNCHECKED | code-partial：具体 native raw family 可保真，但未证明统一 hosted 语义或全 family 覆盖；raw fidelity 不是语义支持 |
| RSP.TOOL.tool_search | TOOL_DEF | responses | `tool_search` | Responses tool search/lazy tool selection | SRC-RSP-BASE §4 | Responses known native raw tool + provider extensions | native_raw_preserve | LossyDowngrade | N/A | LossyDowngrade | N/A | N/A | N/A | delta/final_response | `llm/openai_responses_classification.go`, `responses/inbound.go`, `responses/request_extensions.go`, shared lossy downgrade | classification/inbound tests；`TestOutboundTransformer_TransformRequest_PreservesToolSearchDeclaration`；cross-protocol diagnostic tests | PARTIAL | known native raw same-protocol preserve + explicit lossy diagnostic；不等同 Codex namespace/lazy-loading 语义 |
| CODEX.TOOL.namespace | CLIENT_EXT | codex-responses-ext | `tools[].type=namespace`, `tools[].tools[]` | Codex namespace tool group; child function tools under namespace | SRC-CODEX | `TransformerMetadata` bridge map + flattened `llm.Tool` | bridge_metadata | CONFIRMED | CONFIRMED | UNCHECKED | UNCHECKED | UNCHECKED | UNCHECKED | delta/done/final_response | `llm/transformer/openai/responses/inbound.go`, `llm/transformer/openai/outbound.go`, `llm/transformer/openai/responses/inbound_stream.go` | `TestResponsesNamespaceToolRoundTripThroughOpenAIChat`, `TestInboundTransformer_TransformStream_RestoresNamespaceToolCallFromMetadata` | PARTIAL | Responses<->Chat confirmed；Anthropic directions 未确认 |
| CHAT.MSG.roles | MSG | chat | `messages[].role` | developer/system/user/assistant/tool/function roles | SRC-CHAT-BASE §3 | UNCHECKED | native/lossy_map | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | developer vs system vs Anthropic top-level system 需确认 |
| CHAT.MSG.content_parts | MSG | chat | `messages[].content[]` | text/image_url/input_audio/file/refusal content parts | SRC-CHAT-BASE §3 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | delta/final_response | UNCHECKED | UNCHECKED | PARTIAL | multimodal 不能降成纯文本 |
| CHAT.TOOL.tool_calls | TOOL_CALL | chat | `choices[].message.tool_calls[]`, `choices[].delta.tool_calls[]` | Chat assistant tool calls | SRC-CHAT-BASE §4/§6 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/done | UNCHECKED | UNCHECKED | PARTIAL | ID/name/arguments 增量闭合需确认 |
| ANT.MSG.content_blocks | MSG | anthropic | `messages[].content[]`, response `content[]` | typed content blocks | SRC-ANT-BASE §3 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | content_block_delta/final_response | UNCHECKED | UNCHECKED | PARTIAL | text/image/document/search_result/thinking/tool_use/tool_result/server-tool blocks 需拆子矩阵 |
| ANT.TOOL.tool_use_result | TOOL_CALL | anthropic | `tool_use`, `tool_result` | model emits tool_use; client replies with tool_result | SRC-ANT-BASE §4 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | input_json_delta/done | UNCHECKED | UNCHECKED | PARTIAL | 与 Chat tool role / Responses function_call_output 差异需确认 |
| ANT.TOOL.mcp_toolset | CLIENT_EXT | anthropic | `tools[].type=mcp_toolset` | Anthropic remote MCP toolset | SRC-ANT-MCP | `anthropic.Tool.Raw -> TransformerMetadata[anthropic_raw_tools] []anthropicRawToolFragment{OriginalIndex,Raw} -> appendAnthropicRawTools ordered merge` | raw_fragment_ordered | N/A | N/A | no-synth | PARTIAL (same Anthropic) | no-synth | PARTIAL (same Anthropic) | tool events | `llm/transformer/anthropic` MCP toolset path | `mcp_connector_test.go` order test | PARTIAL | G6 保序；不是 Responses namespace 同义词，不伪桥 |
| RSP.STREAM.events | STREAM | responses | `response.*` SSE events | semantic response event lifecycle | SRC-RSP-BASE §5 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | N/A | N/A | delta/done/final_response | UNCHECKED | UNCHECKED | PARTIAL | 需列 57 类事件逐项确认 |
| CHAT.STREAM.chunks | STREAM | chat | `chat.completion.chunk` | Chat delta chunks | SRC-CHAT-BASE §6 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/usage/done | UNCHECKED | UNCHECKED | PARTIAL | usage chunk、tool_call delta、finish_reason 需确认 |
| ANT.STREAM.events | STREAM | anthropic | `message_start/content_block_delta/...` | Anthropic named SSE events | SRC-ANT-BASE §6, SRC-ANT-STREAM | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/done/usage/error | UNCHECKED | UNCHECKED | PARTIAL | thinking_delta/signature_delta/input_json_delta 必须单独测 |
| RSP.RESP.output | MSG | responses | response `output[]` | typed output items | SRC-RSP-BASE §5 | UNCHECKED | native+raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | N/A | N/A | final_response | UNCHECKED | UNCHECKED | PARTIAL | function_call/status/namespace/custom/tool outputs 未全量确认 |
| CHAT.RESP.choices | MSG | chat | response `choices[]` | Chat choices/message/delta output | SRC-CHAT-BASE §6 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | n>1/finish_reason/logprobs 未全量确认 |
| ANT.RESP.message | MSG | anthropic | response `message` | assistant message object | SRC-ANT-BASE §7 | UNCHECKED | native+raw_sidecar? | N/A | N/A | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | stop_details/usage/thinking/server_tool_use 未全量确认 |

## 7. 完成判定

本文档当前完成度：**未完成**。

完成前必须做到：

1. 第 5 节所有顶层字段行 `Status = CONFIRMED` 或有明确 `N/A` 原因。
2. 第 6 节每个嵌套结构继续拆成子矩阵，直到没有“工具族/事件族/content block 族”这种粗粒度占位。
3. 每行至少有一个代码证据；转换行必须有真实链路测试证据。
4. Stream 相关行必须同时覆盖 `delta`、`done`、最终 response、usage/error 如适用。
5. 不允许用“字段存在”替代“转换闭环”。

## 8. Batch 1: simple common top-level fields

Scope for this section is limited to request top-level `model`, request-level `stream` boolean, `temperature`, `top_p`, and `metadata`. Stream event shapes, token limits, cache, messages/input/content, tools/tool_choice, and reasoning/thinking remain out of scope for Batch 1.

Status rule applied here: a field can be described as architecturally direct when code assigns it directly through `llm.Request`, but it remains `PARTIAL` unless official default/null semantics and targeted six-direction tests or P3 samples are both present.

### 8.1 `model`

#### Official Sources

| Protocol | Field | P0 Evidence | Confirmed Meaning |
|---|---|---|---|
| OpenAI Responses | `model` | SRC-RSP-BASE lines 45-46; SRC-RSP-RAW lines 8108-8118 | Model ID used to generate the response; schema accepts a string or listed model IDs. |
| OpenAI Chat Completions | `model` | SRC-CHAT-BASE line 37; SRC-CHAT-RAW lines 1730-1742 | Model ID used to generate the response; schema accepts a string or listed model IDs. |
| Anthropic Messages | `model` | SRC-ANT-BASE line 42; SRC-ANT-RAW line 1 field fragment `model: Model` | Claude model identifier / model that completes the prompt. |

#### Value Semantics

| Required Value Column | Finding |
|---|---|
| Value Shape | `string` / provider model enum-or-string. |
| Allowed Values / Variants | OpenAI Responses/Chat: string or documented OpenAI model IDs. Anthropic: `Model` with Claude model IDs plus string fallback in raw snapshot. Exact current model catalog is not exhaustively audited here. |
| Default Semantics | `UNCHECKED`: Chat and Anthropic treat `model` as required in AxonHub validation; Responses raw snapshot labels request `model` optional, but no Batch 1 decision was made on whether AxonHub may omit it. |
| Null / Omitted Semantics | `UNCHECKED`: no confirmed local P0/P2 evidence for accepting `null`; AxonHub request structs use `string`, so `null` behavior depends on JSON unmarshal/default empty string and validation path. |
| Unknown Value Policy | Preserve as literal string through `llm.Request.Model`; do not translate provider model names without explicit routing/model-alias policy. Unknown/provider-unavailable model handling is outside this field conversion matrix. |
| Value Mapping Rule | No semantic model-name mapping confirmed. Direct string carry only; provider/channel routing may later reject unsupported models. |

#### AxonHub Architecture

Primary owner: `llm.Request.Model` (`llm/model.go:40-45`). Protocol-native request structs also carry `model` directly: OpenAI Chat `Request.Model` (`llm/transformer/openai/model.go:17-24`), OpenAI Responses `Request.Model` (`llm/transformer/openai/responses/model.go:92-95`), Anthropic `MessageRequest.Model` (`llm/transformer/anthropic/model.go:12-17`).

#### Direction Matrix

| Direction | Strategy | Field Decision |
|---|---|---|
| Responses -> Chat | `direct` | `req.Model` -> `llm.Request.Model` -> Chat `Request.Model`; no model alias translation confirmed. |
| Chat -> Responses | `direct` | Chat `Request.Model` -> `llm.Request.Model` -> Responses `Request.Model`; no model alias translation confirmed. |
| Responses -> Anthropic | `direct` with provider caveat | String is carried to Anthropic `MessageRequest.Model`; whether that OpenAI-style model ID is accepted by Anthropic is a routing/model-selection concern, not confirmed here. |
| Anthropic -> Responses | `direct` with provider caveat | Claude model string is carried to Responses `model`; acceptance by OpenAI Responses endpoint is not confirmed. |
| Chat -> Anthropic | `direct` with provider caveat | Chat model string is carried to Anthropic `model`; no provider model translation confirmed. |
| Anthropic -> Chat | `direct` with provider caveat | Claude model string is carried to Chat `model`; no provider model translation confirmed. |

#### Evidence

- P2 current source:
  - Chat inbound direct assignment: `llm/transformer/openai/inbound_convert.go:45-47` (`Model: r.Model`).
  - Chat outbound direct assignment: `llm/transformer/openai/outbound_convert.go:17-18` (`Model: r.Model`).
  - Responses inbound direct assignment: `llm/transformer/openai/responses/inbound.go:171-172` (`Model: req.Model`).
  - Responses outbound direct assignment: `llm/transformer/openai/responses/outbound.go:246-248` (`Model: llmReq.Model`).
  - Anthropic inbound direct assignment: `llm/transformer/anthropic/inbound_convert.go:96-98` (`Model: anthropicReq.Model`).
  - Anthropic outbound direct assignment: `llm/transformer/anthropic/outbound_convert.go:129-132` (`Model: chatReq.Model`).
- P2 upstream worktree read-only check:
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/inbound_convert.go:54-56`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/outbound_convert.go:10-17`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/responses/inbound.go:169-171`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/inbound_convert.go:54-56`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/outbound_convert.go:115-118`.
- Test Evidence: `UNCHECKED`; no Batch 1-specific six-direction model round-trip test was verified in this pass.

#### Status

`PARTIAL`: official existence/value shape and P2 direct assignments are confirmed, but default/null semantics, model alias/routing policy, and targeted six-direction test/P3 evidence are not confirmed.

### 8.2 request-level `stream`

#### Official Sources

| Protocol | Field | P0 Evidence | Confirmed Meaning |
|---|---|---|---|
| OpenAI Responses | `stream` | SRC-RSP-BASE line 56; SRC-RSP-RAW lines 8861-8865 | Optional boolean; when true, response data is streamed using SSE. Responses semantic event shapes are out of scope here. |
| OpenAI Chat Completions | `stream` | SRC-CHAT-BASE line 58; SRC-CHAT-RAW lines 2615-2619 | Optional boolean; when true, chat completion data is streamed using SSE. Chat chunk shape is out of scope here. |
| Anthropic Messages | `stream` | SRC-ANT-BASE line 49; SRC-ANT-RAW line 1 field fragment `stream: optional boolean` | Optional boolean; when true, Messages response is incrementally streamed using SSE. Anthropic event shape is out of scope here. |

#### Value Semantics

| Required Value Column | Finding |
|---|---|
| Value Shape | `boolean`. |
| Allowed Values / Variants | `true` / `false`. |
| Default Semantics | `PARTIAL`: P0 confirms optional boolean and true behavior. This pass did not find/record a complete P0 statement for omitted behavior across all three protocols. |
| Null / Omitted Semantics | `UNCHECKED`: no confirmed `null` semantics. AxonHub uses pointer booleans, so omitted can remain nil internally. |
| Unknown Value Policy | Non-boolean is schema-invalid; AxonHub-specific validation behavior for malformed JSON types was not audited for Batch 1. |
| Value Mapping Rule | Request-level boolean can be carried directly. Stream event/chunk conversion is explicitly excluded from this row. |

#### AxonHub Architecture

Primary owner: `llm.Request.Stream` (`llm/model.go:196-198`). Protocol-native request structs carry pointer booleans: OpenAI Chat `Request.Stream` (`llm/transformer/openai/model.go:113-114`), OpenAI Responses `Request.Stream` (`llm/transformer/openai/responses/model.go:117-118`), Anthropic `MessageRequest.Stream` (`llm/transformer/anthropic/model.go:95-96`).

#### Direction Matrix

| Direction | Strategy | Field Decision |
|---|---|---|
| Responses -> Chat | `direct` | Copy request boolean only; do not infer Responses semantic event compatibility. |
| Chat -> Responses | `direct` | Copy request boolean only; do not infer Chat chunk -> Responses event compatibility. |
| Responses -> Anthropic | `direct` | Copy request boolean only; Responses event stream conversion remains separate Batch 9 work. |
| Anthropic -> Responses | `direct` | Copy request boolean only; Anthropic named event conversion remains separate Batch 9 work. |
| Chat -> Anthropic | `direct` | Copy request boolean only; Chat chunk conversion remains separate Batch 9 work. |
| Anthropic -> Chat | `direct` | Copy request boolean only; Anthropic named event conversion remains separate Batch 9 work. |

#### Evidence

- P2 current source:
  - Chat inbound/outbound: `llm/transformer/openai/inbound_convert.go:45-68`, `llm/transformer/openai/outbound_convert.go:17-40`.
  - Responses inbound/outbound: `llm/transformer/openai/responses/inbound.go:171-177`, `llm/transformer/openai/responses/outbound.go:246-252`.
  - Anthropic inbound/outbound: `llm/transformer/anthropic/inbound_convert.go:96-102`, `llm/transformer/anthropic/outbound_convert.go:129-135`.
- P2 upstream worktree read-only check:
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/inbound_convert.go:54-65`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/outbound_convert.go:16-35`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/responses/inbound.go:171-174`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/inbound_convert.go:56-61`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/outbound_convert.go:117-120`.
- Test Evidence: `UNCHECKED` for request-level boolean six-direction matrix. Existing stream tests cover event/chunk behavior and are intentionally not used to confirm this row.

#### Status

`PARTIAL`: request-level boolean storage and direct code assignment are confirmed; default/null semantics and targeted request-boolean conversion tests are not confirmed. Stream event compatibility remains out of scope.

### 8.3 `temperature`

#### Official Sources

| Protocol | Field | P0 Evidence | Confirmed Meaning |
|---|---|---|---|
| OpenAI Responses | `temperature` | SRC-RSP-BASE line 58; SRC-RSP-RAW lines 8879-8887 | Optional number; sampling temperature between 0 and 2; usually alter this or `top_p`, not both. |
| OpenAI Chat Completions | `temperature` | SRC-CHAT-BASE line 60; SRC-CHAT-RAW lines 2641-2648 | Optional number; sampling temperature between 0 and 2; usually alter this or `top_p`, not both. |
| Anthropic Messages | `temperature` | SRC-ANT-BASE line 51; SRC-ANT-RAW line 1 field fragment `temperature: optional number` | Optional number; amount of randomness; defaults to `1.0`; range `0.0` to `1.0`; even `0.0` is not fully deterministic. |

#### Value Semantics

| Required Value Column | Finding |
|---|---|
| Value Shape | `number`. |
| Allowed Values / Variants | OpenAI Responses/Chat: `0 <= temperature <= 2`. Anthropic: `0.0 <= temperature <= 1.0`. |
| Default Semantics | Anthropic default `1.0` confirmed by SRC-ANT-RAW. OpenAI default for omitted request value was not confirmed in this pass, although stored Chat response examples may show `1.0`; request default remains `UNCHECKED`. |
| Null / Omitted Semantics | `UNCHECKED`: no complete P0/P2 confirmation for `null`; AxonHub uses `*float64`, so omitted remains nil internally. |
| Unknown Value Policy | Numeric out-of-range should not be blindly forwarded. P2 current code does not show a guard/clamp in the direct assignments audited here. |
| Value Mapping Rule | OpenAI<->OpenAI can pass unchanged. Anthropic->OpenAI can pass unchanged because Anthropic's confirmed range is a subset of OpenAI's. OpenAI->Anthropic only values in `0..1` can pass unchanged; values `>1` require an explicit policy (`unsupported_error` or diagnostic) not confirmed in current code. |

#### AxonHub Architecture

Primary owner: `llm.Request.Temperature` (`llm/model.go:100-104`). Protocol-native request structs carry `*float64` for all three protocols.

#### Direction Matrix

| Direction | Strategy | Field Decision |
|---|---|---|
| Responses -> Chat | `direct` | Same OpenAI value range `0..2`; code carries pointer directly. |
| Chat -> Responses | `direct` | Same OpenAI value range `0..2`; code carries pointer directly. |
| Responses -> Anthropic | `value_map` / `PARTIAL` | `0..1` can pass unchanged; `>1` has no confirmed Anthropic-valid value. Current code appears to pass directly without an audited guard. |
| Anthropic -> Responses | `direct` | Anthropic `0..1` is within OpenAI `0..2`; code carries pointer directly. |
| Chat -> Anthropic | `value_map` / `PARTIAL` | `0..1` can pass unchanged; `>1` requires explicit unsupported/diagnostic policy not confirmed. |
| Anthropic -> Chat | `direct` | Anthropic `0..1` is within Chat `0..2`; code carries pointer directly. |

#### Evidence

- P2 current source:
  - Common field: `llm/model.go:100-104`.
  - Chat inbound/outbound direct assignment: `llm/transformer/openai/inbound_convert.go:45-56`, `llm/transformer/openai/outbound_convert.go:17-28`.
  - Responses inbound/outbound direct assignment: `llm/transformer/openai/responses/inbound.go:171-184`, `llm/transformer/openai/responses/outbound.go:246-264`.
  - Anthropic inbound/outbound direct assignment: `llm/transformer/anthropic/inbound_convert.go:96-100`, `llm/transformer/anthropic/outbound_convert.go:129-133`.
- P2 upstream worktree read-only check:
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/model.go:104`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/inbound_convert.go:56-59`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/outbound_convert.go:16-26`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/responses/inbound.go:171-181`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/inbound_convert.go:56-59`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/outbound_convert.go:117-119`.
- Test Evidence: `UNCHECKED`; no targeted Batch 1 tests were verified for OpenAI `temperature > 1` routed to Anthropic or for six-direction preservation.

#### Status

`PARTIAL`: P0 semantics reveal a real value-range mismatch. P2 code evidence confirms direct assignment, but no guard/diagnostic/default/null semantics or targeted tests were confirmed.

### 8.4 `top_p`

#### Official Sources

| Protocol | Field | P0 Evidence | Confirmed Meaning |
|---|---|---|---|
| OpenAI Responses | `top_p` | SRC-RSP-BASE line 63; SRC-RSP-RAW lines 10965-10975 | Optional number; nucleus sampling; range `0..1`; usually alter this or `temperature`, not both. |
| OpenAI Chat Completions | `top_p` | SRC-CHAT-BASE line 64; SRC-CHAT-RAW lines 2928-2938 | Optional number; nucleus sampling; range `0..1`; usually alter this or `temperature`, not both. |
| Anthropic Messages | `top_p` | SRC-ANT-BASE line 56; SRC-ANT-RAW line 1 field fragment `top_p: optional number` | Optional number; nucleus sampling; recommended for advanced use cases only. Range/default not confirmed in this pass from the local raw snapshot. |

#### Value Semantics

| Required Value Column | Finding |
|---|---|
| Value Shape | `number`. |
| Allowed Values / Variants | OpenAI Responses/Chat: `0 <= top_p <= 1`. Anthropic: `number`, range `UNCHECKED` in this pass. |
| Default Semantics | `UNCHECKED` for all three request fields in this pass. |
| Null / Omitted Semantics | `UNCHECKED`: AxonHub uses `*float64`; omitted remains nil internally, but wire-level `null` semantics were not confirmed. |
| Unknown Value Policy | Out-of-range or non-number should not be blindly forwarded; target-protocol rejection/diagnostic behavior is not audited. |
| Value Mapping Rule | OpenAI Responses<->Chat direct. Anthropic directions are `PARTIAL` until Anthropic range/default/null semantics are confirmed from a P0 source. |

#### AxonHub Architecture

Primary owner: `llm.Request.TopP` (`llm/model.go:111-116`). Protocol-native request structs carry `*float64` for all three protocols.

#### Direction Matrix

| Direction | Strategy | Field Decision |
|---|---|---|
| Responses -> Chat | `direct` | Same OpenAI value range `0..1`; code carries pointer directly. |
| Chat -> Responses | `direct` | Same OpenAI value range `0..1`; code carries pointer directly. |
| Responses -> Anthropic | `UNCHECKED` / current-code `direct` | P2 code carries value directly, but Anthropic allowed range/default not confirmed here. |
| Anthropic -> Responses | `UNCHECKED` / current-code `direct` | P2 code carries value directly, but source Anthropic value constraints are incomplete. |
| Chat -> Anthropic | `UNCHECKED` / current-code `direct` | P2 code carries value directly, but Anthropic allowed range/default not confirmed here. |
| Anthropic -> Chat | `UNCHECKED` / current-code `direct` | P2 code carries value directly, but source Anthropic value constraints are incomplete. |

#### Evidence

- P2 current source:
  - Common field: `llm/model.go:111-116`.
  - Chat inbound/outbound direct assignment: `llm/transformer/openai/inbound_convert.go:45-57`, `llm/transformer/openai/outbound_convert.go:17-29`.
  - Responses inbound/outbound direct assignment: `llm/transformer/openai/responses/inbound.go:171-184`, `llm/transformer/openai/responses/outbound.go:246-264`.
  - Anthropic inbound/outbound direct assignment: `llm/transformer/anthropic/inbound_convert.go:96-101`, `llm/transformer/anthropic/outbound_convert.go:129-134`.
- P2 upstream worktree read-only check:
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/model.go:116`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/inbound_convert.go:56-60`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/outbound_convert.go:16-27`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/responses/inbound.go:171-181`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/inbound_convert.go:56-60`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/outbound_convert.go:117-120`.
- Test Evidence: `UNCHECKED`; no targeted six-direction `top_p` test or Anthropic range/default test was verified.

#### Status

`PARTIAL`: OpenAI value semantics and P2 direct assignments are confirmed, but Anthropic range/default/null semantics and targeted tests are not confirmed.

### 8.5 `metadata`

#### Official Sources

| Protocol | Field | P0 Evidence | Confirmed Meaning |
|---|---|---|---|
| OpenAI Responses | `metadata` | SRC-RSP-BASE line 45; SRC-RSP-RAW lines 8100-8106 | Optional metadata map: up to 16 key-value pairs; keys are strings up to 64 chars, values strings up to 512 chars. |
| OpenAI Chat Completions | `metadata` | SRC-CHAT-BASE line 43; SRC-CHAT-RAW lines 2282-2288 | Optional metadata map: up to 16 key-value pairs; keys are strings up to 64 chars, values strings up to 512 chars. |
| Anthropic Messages | `metadata` | SRC-ANT-BASE line 45; SRC-ANT-RAW line 1 field fragment `metadata: optional Metadata` | Optional metadata object; local raw snapshot confirms `user_id` as optional string external user identifier. |

#### Value Semantics

| Required Value Column | Finding |
|---|---|
| Value Shape | OpenAI Responses/Chat: `object` / map of string keys to string values. Anthropic: `object` with confirmed `user_id` optional string. |
| Allowed Values / Variants | OpenAI: max 16 pairs, key length <=64, value length <=512. Anthropic: `user_id` should be UUID/hash/opaque identifier; do not include personal identifying information. Other Anthropic metadata keys are `UNCHECKED`. |
| Default Semantics | `UNCHECKED`: request omitted behavior not fully confirmed. |
| Null / Omitted Semantics | `UNCHECKED`: AxonHub common field is `map[string]string`; Anthropic native field is pointer object. Wire-level `null` semantics were not confirmed. |
| Unknown Value Policy | OpenAI arbitrary metadata keys must not be blindly mapped to Anthropic. Anthropic unknown metadata fields are not represented by current `AnthropicMetadata` struct. |
| Value Mapping Rule | OpenAI Responses<->Chat direct map. OpenAI->Anthropic only `metadata.user_id` has a confirmed carrier; arbitrary OpenAI metadata requires sidecar/diagnostic policy not confirmed. Anthropic->OpenAI can map `user_id` into common metadata map key `user_id`. |

#### AxonHub Architecture

Primary common owner: `llm.Request.Metadata map[string]string` (`llm/model.go:156-162`). Anthropic native owner is narrower: `AnthropicMetadata.UserID` only (`llm/transformer/anthropic/model.go:115-117`). This is not a fully symmetric metadata object across all protocols.

#### Direction Matrix

| Direction | Strategy | Field Decision |
|---|---|---|
| Responses -> Chat | `direct` | OpenAI metadata map can be carried through `llm.Request.Metadata` and emitted as Chat metadata. |
| Chat -> Responses | `direct` | OpenAI metadata map can be carried through `llm.Request.Metadata` and emitted as Responses metadata. |
| Responses -> Anthropic | `structural_transform` / `PARTIAL` | Only key `user_id` can be emitted as Anthropic `metadata.user_id`; other key-value pairs have no confirmed Anthropic carrier in current native struct. Need sidecar/diagnostic policy. |
| Anthropic -> Responses | `structural_transform` / `PARTIAL` | Anthropic `metadata.user_id` is stored as common metadata key `user_id` and can be emitted to Responses metadata map. Other Anthropic metadata keys are not represented. |
| Chat -> Anthropic | `structural_transform` / `PARTIAL` | Only key `user_id` can be emitted as Anthropic `metadata.user_id`; other Chat metadata keys need sidecar/diagnostic policy. |
| Anthropic -> Chat | `structural_transform` / `PARTIAL` | Anthropic `metadata.user_id` is stored as common metadata key `user_id` and can be emitted to Chat metadata map. Other Anthropic metadata keys are not represented. |

#### Evidence

- P2 current source:
  - Common OpenAI map owner: `llm/model.go:156-162`.
  - Chat inbound/outbound direct map assignment: `llm/transformer/openai/inbound_convert.go:45-62`, `llm/transformer/openai/outbound_convert.go:17-34`.
  - Responses inbound/outbound direct map assignment: `llm/transformer/openai/responses/inbound.go:171-178`, `llm/transformer/openai/responses/outbound.go:246-258`.
  - Anthropic native metadata shape: `llm/transformer/anthropic/model.go:115-117`.
  - Anthropic inbound maps only `user_id`: `llm/transformer/anthropic/inbound_convert.go:96-119`.
  - Anthropic outbound emits only `user_id`, preferring `Metadata["user_id"]` then `User`: `llm/transformer/anthropic/outbound_convert.go:144-156`.
- P2 upstream worktree read-only check:
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/model.go:162`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/inbound_convert.go:56-61`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/outbound_convert.go:16-31`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/openai/responses/inbound.go:171-174`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/model.go:109-111`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/inbound_convert.go:56-68`.
  - `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes/llm/transformer/anthropic/outbound_convert.go:125-127`.
- Test Evidence: `UNCHECKED` for full metadata object six-direction behavior. Anthropic `user_id` bridge behavior is visible in code, but arbitrary metadata preservation/loss diagnostics were not verified by tests in this pass.

#### Status

`PARTIAL`: OpenAI metadata semantics and direct OpenAI<->OpenAI code path are confirmed. Anthropic metadata is narrower (`user_id` only in current native model); arbitrary metadata cross-protocol preservation, diagnostics, default/null semantics, and targeted tests remain unconfirmed.


## 9. 2026-07-12 Implementation Sync (G1–G7)

This section updates the main matrix after the protocol-transformer field-fix modules completed. It does **not** claim all 101 rows are `CONFIRMED`; it only elevates rows with code+test evidence from G1–G7.

### 9.1 Rows with same-protocol implementation + targeted tests

| ID | Field | Module | Code evidence | Test evidence | Same-protocol | Cross-protocol policy |
|---|---|---|---|---|---|---|
| CHAT.TOP.n | `n` | G1 | `llm/transformer/openai/chat_n.go` | `chat_n_test.go` (`TestOpenAIChatRequestN*`) | raw preserve | no-synth to Responses; Anthropic LossyDowngrade |
| CHAT.TOP.prompt_cache_retention | `prompt_cache_retention` | G2 | `chat_n.go` raw preserve | `chat_n_test.go` prompt_cache_retention tests | raw preserve | no-synth to Responses; Anthropic LossyDowngrade |
| ANT.TOP.container | `container` | G3 | `anthropic/model.go` + metadata | `container_inference_geo_test.go` | opaque JSON + metadata | no-synth to OpenAI |
| ANT.TOP.inference_geo | `inference_geo` | G3 | same | same | opaque JSON + metadata | no-synth to OpenAI |
| CHAT.TOP.audio | `audio` | G4 | `chat_n.go` | `chat_n_test.go` output-controls | raw preserve | no-synth / lossy |
| CHAT.TOP.prediction | `prediction` | G4 | `chat_n.go` | same | raw preserve | no-synth / lossy |
| CHAT.TOP.moderation | `moderation` | G4 | `chat_n.go` | same | raw preserve | no-synth / lossy |
| CHAT.TOP.web_search_options | `web_search_options` | G5a | `chat_n.go` | web_search_options tests | raw preserve | no-synth Responses; Anthropic LossyDowngrade |
| CHAT.TOP.functions | `functions` (deprecated) | G5b | `chat_n.go` | `chat_deprecated_functions_test.go` | raw preserve | no fake modern rewrite unless explicit bridge |
| CHAT.TOP.function_call | request `function_call` (deprecated) | G5b | `chat_n.go` | same | raw preserve | no fake modern rewrite |
| CHAT.MSG.function_call | response `message.function_call` | G5b | `openai/model.go` + convert | response/history/stream tests | bridge+origin metadata | modern tool path preserved |
| ANT.TOP.mcp_servers | `mcp_servers` | G6 | `anthropic/model.go` MCPServers | `mcp_connector_test.go` | opaque JSON | not Responses `mcp` |
| ANT.TOOL.mcp_toolset | `tools[].type=mcp_toolset` | G6 | `anthropic.Tool.Raw -> TransformerMetadata[anthropic_raw_tools] []anthropicRawToolFragment{OriginalIndex,Raw} -> appendAnthropicRawTools ordered merge` | same + order test | raw fragment ordered | not Responses namespace/mcp; never stored in public `llm.Tool.Raw` |
| RSP.TOP.reasoning.context | `reasoning.context` | G7 | Responses model + metadata | `reasoning_context_test.go` | native+sidecar | effort/budget not conflated |
| RSP.TOP.reasoning.generate_summary | deprecated `generate_summary` | G7 | origin/value sidecars | `reasoning_g7_test.go` | distinct wire identity | not mixed into summary permanently |
| RSP.RESP.reasoning.content | output `content[]`/`reasoning_text` | G7 | Item JSON dual-path | output content test | content preferred | summary fallback documented |
| RSP.STREAM.reasoning_text | `reasoning_text.*` events | G7 | inbound/outbound stream + aggregator | stream tests | gated prefer-text production path | default summary path retained |
| RSP.TOP.reasoning.unknown_nested | unknown nested keys | G7 | raw object merge | unknown-nested test | raw-first merge | no silent drop |

### 9.2 Status language for these rows

§5 主表中对应 G1–G7 行已于 2026-07-12 同步更新为 `PARTIAL`，并写入 Code/Test Evidence 路径。§9.1 是同一批证据的紧凑索引，**不是**替代主表的“仅 overlay”。

阅读规则：

- **§5 行状态为准**（same-protocol `PARTIAL` + 跨协议 no-synth/LossyDowngrade 标注）。
- **§9.1** 提供 module → path 快速索引。
- 未列入 G1–G7 的行保持原状态；整体矩阵仍 **INCOMPLETE**。
- **Uncertainty**: remaining uncertainty is mostly cross-protocol equivalence / full event family coverage, not “field not implemented”.

### 9.3 Residual high-priority fixture-only (not reopened as features)

See `.trellis/tasks/07-10-protocol-high-priority-fixtures-matrix-sync/research/residual-gaps.md`.

### 9.4 Sync provenance

- Branch: `codex-transformer-field-fixes`
- Modules: G1–G7 archives under `.trellis/tasks/archive/2026-07/`
- Date: 2026-07-12

