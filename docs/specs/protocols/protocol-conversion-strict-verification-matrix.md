# AxonHub 三协议转换严格确认矩阵（INCOMPLETE）

- 创建日期：2026-07-09
- 状态：**INCOMPLETE / 未完成**
- 规则：只要任意一行存在 `UNCHECKED` / `PARTIAL` / `BLOCKED`，本文档就不能作为“协议转换已完成”的证据。
- 目的：把 OpenAI Responses、OpenAI Chat Completions、Anthropic Messages、Codex Responses 扩展拆成可审计行，逐项确认含义、内部归属、跨协议转换、真实链路测试。

## 0. 当前结论

本文档整体状态仍是 **INCOMPLETE**：任意 `UNCHECKED` / 未闭环跨协议行存在时，不能作为“协议转换已全部完成”的证据。

截至 2026-07-13（G1–G8 边界证据 + G13–G15 Codex delta 文档规范化）：

1. **Same-protocol 已有证据（有 code + targeted test，状态多为 `PARTIAL`）**：见 §5 / §6 主状态行与 §9 / §10 / §13。覆盖范围包括：
   - Chat：`n`、`prompt_cache_retention`、`audio`/`prediction`/`moderation`、`web_search_options`、deprecated `functions`/`function_call`、`modalities`（G8）
   - Anthropic：`container`、`inference_geo`、`mcp_servers`、`tools[].type=mcp_toolset`、top-level `cache_control`（G8）
   - Responses：`reasoning` 对象关键子路径（G7）、`include`/`reasoning`/`stream_options` 请求保真（G13/G14）、`input[]` item identity（G15）、`context_management`/`conversation` raw fallback 与 hosted tools inventory（G8）
2. **历史已确认闭环（P1 叙事）**：Codex Responses `tools[].type = "namespace"` 在 Responses -> Chat 展开、reverse map、回包/流式恢复 `name + namespace`（不抬成公共 P0 全矩阵完成）。
3. **未声称**：全部 Field ID 已按 FDR 填完、全部行 `CONFIRMED`、全方向跨协议语义等价、Codex P1 = 公共 P0、或 residual fixture-only 项已实现为 feature。
4. **规范化账本**：§11 FDR 规则、§12 Codex usage-profile delta register、§13 实施符合性账本、§14 后续 diff/slice 流程。这些章节**不替代** §5/§6 主状态，也不把 schema 新要求伪称为已完成。
5. **Field ID 计数口径（唯一权威）**：
   - §5 顶层请求行 = **84**
   - §6 嵌套/结构/行为行 = **17**
   - §5 + §6 主状态 Field ID = **101**
   - §9 额外子 Field ID（未单独列入 §5/§6 主键行）= **6**
   - 总唯一 Field ID = **107**
   - **禁止**写“§5 主表 107”，也**禁止**只数 §5 而漏 §6。
6. **Residual**：见 §9.3、§14.4 与 `.trellis/tasks/07-10-protocol-high-priority-fixtures-matrix-sync/research/residual-gaps.md`。

权威阅读顺序：

1. **§5 + §6 主状态行**（Field ID 是规范主键；§5 顶层 + §6 嵌套/结构）
2. **§9 / §10**（G1–G7 / G13–G15 证据索引；§9 子 ID 是索引，不替代主状态行）
3. **§11–§14**（FDR 规则、Codex delta 登记、符合性账本、后续流程与 FDR 优先清单）
4. G 编号只是变更批次标签；**不能**用 G 编号替代 Field ID，也不能把 G9–G12 helper 编号当作字段完成状态

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
| RSP.TOP.context_management | STATE | responses | `context_management` | 请求级上下文管理/压缩配置 | SRC-RSP-BASE §2 | `ProviderExtensions.OpenAIResponses.Request.RawTopLevelFields` generic fallback | raw_fallback_same_protocol | no-synth | N/A | no-synth | N/A | N/A | N/A | unchecked | `responses/request_extensions.go` raw top-level capture/merge | `TestResponsesContextManagement_SameProtocolRawTopLevelFallback` | PARTIAL | Field-specific same-protocol raw fidelity only; not typed semantic support; do not bridge to Anthropic context_management |
| RSP.TOP.conversation | STATE | responses | `conversation` | Responses conversation/state attachment | SRC-RSP-BASE §2 | Request field still commented; generic `RawTopLevelFields` fallback | raw_fallback_same_protocol | no-synth | N/A | no-synth | N/A | N/A | N/A | final_response | `responses/model.go` (commented field), `responses/request_extensions.go` | `TestResponsesConversation_SameProtocolRawTopLevelFallback` (string+object) | PARTIAL | Same-protocol raw fidelity for string id and object forms; not Chat/Anthropic messages state; typed request support still not enabled |
| RSP.TOP.include | TOP | responses | `include` | 请求额外输出数据，如 encrypted reasoning/search results | SRC-RSP-BASE §2 | Responses `Request.Include []string` + `TransformerMetadata[shared.MetadataKeyInclude]` | native+metadata | no-synth | N/A | no-synth | N/A | N/A | N/A | final_response/stream | `responses/model.go`, `responses/inbound.go`, `responses/outbound.go` | include transformer-metadata + outbound include tests；`g13a_reasoning_include_test.go`（含 `reasoning.encrypted_content` 保真与默认省略） | PARTIAL | G13 same-protocol：客户端实际 include 顺序/值保真，不注入 Codex 默认；具体 include 成员语义/外协议映射仍需逐值审计 |
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
| RSP.TOP.reasoning | REASONING | responses | `reasoning` | reasoning configuration/state | SRC-RSP-BASE §2 | Responses reasoning model + metadata/raw merge (G7/G13/G14) | native+raw_sidecar | PARTIAL | PARTIAL | LossyDowngrade | LossyDowngrade | LossyDowngrade | LossyDowngrade | delta/final_response | `llm/transformer/openai/responses` reasoning handlers | `reasoning_context_test.go` / `reasoning_g7_test.go` + `g13a_reasoning_include_test.go` / `g14a_summary_stream_options_test.go` / `reasoning_effort_forward_compat_test.go` | PARTIAL | G7 + G13/G14 same-protocol：supplied reasoning/summary 保真；未知 effort 开串；不注入 Codex 默认；跨协议不与 Chat `reasoning_effort`、Anthropic `thinking` 简单等同 |
| RSP.TOP.safety_identifier | META_USAGE | responses | `safety_identifier` | stable safety/user identifier | SRC-RSP-BASE §2 | UNCHECKED | native? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | 与 deprecated `user` 差异需确认 |
| RSP.TOP.service_tier | META_USAGE | responses | `service_tier` | 服务层级 | SRC-RSP-BASE §2 | UNCHECKED | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | final_response | UNCHECKED | UNCHECKED | PARTIAL | 各协议允许值不同 |
| RSP.TOP.store | STATE | responses | `store` | 是否存储 response | SRC-RSP-BASE §2 | UNCHECKED | native/raw_sidecar? | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | none | UNCHECKED | UNCHECKED | PARTIAL | Anthropic 无直接等价物 |
| RSP.TOP.stream | STREAM | responses | `stream` | 是否使用 Responses semantic SSE | SRC-RSP-BASE §2 | `llm.Request.Stream` | native | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | PARTIAL | delta/done/final_response | UNCHECKED | UNCHECKED | PARTIAL | 三协议 stream 形状完全不同，需按事件矩阵确认 |
| RSP.TOP.stream_options | STREAM | responses | `stream_options` | Responses stream options | SRC-RSP-BASE §2 | typed `StreamOptions` + `ProviderExtensions.OpenAIResponses.Request.RawStreamOptions` (G9/G14) | typed+raw_sidecar | PARTIAL | PARTIAL | no-synth | N/A | PARTIAL | PARTIAL | delta/final_response | `responses/request_extensions.go`, `provider_extensions.go` | `g14a_summary_stream_options_test.go`, `g14b_stream_options_sidecar_test.go` | PARTIAL | G14 same-protocol：typed+raw nested merge / summary delivery / 默认省略 / 非 false unknown top-level；跨协议不合成 |
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
| CHAT.TOP.modalities | OUTPUT_CFG | chat | `modalities` | output modalities | SRC-CHAT-BASE §2 | `llm.Request.Modalities` + OpenAI Chat typed fields | common_typed | PARTIAL | PARTIAL | no-synth | N/A | no-synth | N/A | final_response | `llm/model.go`, `openai/model.go`, `inbound_convert.go`, `outbound_convert.go` | `TestInboundTransformer_TransformRequest_ModalitiesRoundTripChat`, `TestInboundTransformer_TransformRequest_ModalitiesOmittedChat` | PARTIAL | Chat same-protocol typed round-trip + omitted no-synth covered; Responses/Gemini tests are supplementary only; Anthropic has no equivalent top-level field |
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
| ANT.TOP.cache_control | STATE | anthropic | `cache_control` | 顶层 ephemeral prompt-cache control；不同于 content-block `cache_control` | SRC-ANT-RAW (`cache_control: optional CacheControlEphemeral`) | `MessageRequest.CacheControl` + `TransformerMetadata[anthropic_cache_control]` opaque restore | native+metadata | N/A | N/A | no-equivalent | no-equivalent | no-equivalent | no-equivalent | none | `anthropic/model.go`, `anthropic/inbound_convert.go`, `anthropic/outbound_convert.go` | `TestTopLevelCacheControlRoundTrip` (+ legacy TopLevel passthrough/e2e) | PARTIAL | Top-level same-protocol RT, TTL 5m/1h, omitted no-synth, isolation from content-block cache_control, no OpenAI prompt-cache bridge; not equivalent to OpenAI prompt_cache_* |
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
| RSP.MSG.input_items | MSG | responses | `input[]` | typed input/message/tool item list | SRC-RSP-BASE §3 | Responses inbound/outbound item convert + raw input sidecar | native+raw_sidecar | PARTIAL | PARTIAL | no-synth | no-synth | N/A | N/A | final_response | `responses/inbound.go`, `responses/outbound_convert.go` | G15a/b/c public fixtures（message/function/custom/reasoning identity） | PARTIAL | G15 request input item id same-protocol 已测；仍需按 item type 展开更广子矩阵；跨协议 no-synth |
| RSP.TOOL.function | TOOL_DEF | responses | `tools[].type=function` | function tool | SRC-RSP-BASE §4 | UNCHECKED | native | PARTIAL | PARTIAL | UNCHECKED | UNCHECKED | PARTIAL | PARTIAL | delta/done | UNCHECKED | UNCHECKED | PARTIAL | strict/defer_loading/schema 需确认 |
| RSP.TOOL.hosted | TOOL_DEF | responses | `tools[]` hosted families | web/file/computer/code/image hosted tools | SRC-RSP-BASE §4 | specific native tool raw families + generic Responses raw-tool merge | native_raw_family_partial | LossyDowngrade | PARTIAL | no-synth / lossy (Chat proven; Anthropic family-level no unified bridge) | N/A | N/A | N/A | delta/final_response | `llm/openai_responses_classification.go`, `responses/request_extensions.go` | `TestResponsesHostedTools_SameProtocolRawPreserveAndChatLossy` + existing tool_search/web_search coverage | PARTIAL | Inventory + same-protocol raw preserve for raw-only hosted types; Chat lossy/no-synth explicit; Anthropic not given a blanket LossyDowngrade claim beyond no unified bridge; no unified cross-protocol hosted abstraction |
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

This section updates the main matrix after the protocol-transformer field-fix modules completed. It does **not** claim all 101 rows are `CONFIRMED`; it only elevates rows with code+test evidence from G1–G7. Codex delta G13–G15 request-side evidence is indexed in §10.

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
- 未列入 G1–G7 的行保持原状态；整体矩阵仍 **INCOMPLETE**（G8 边界项已补字段级证据，但未宣称 101 行全 CONFIRMED）。
- **Uncertainty**: remaining uncertainty is mostly cross-protocol equivalence / full event family coverage, not “field not implemented”.

### 9.3 Residual high-priority fixture-only (not reopened as features)

See `.trellis/tasks/07-10-protocol-high-priority-fixtures-matrix-sync/research/residual-gaps.md`.

### 9.4 Sync provenance

- Branch: `codex-transformer-field-fixes`
- Modules: G1–G7 archives under `.trellis/tasks/archive/2026-07/`
- Date: 2026-07-12

## 10. 2026-07-12/13 Codex delta sync (G13–G15)

Source delta: Codex `1f0566d..9e552e9d1`（含终点 `9e552e9d1`）。Scope is OpenAI Responses **request** same-protocol preservation only.

G13–G15 public-seam fixtures / independent module review：见 §10.1 与 §13.4。跨模块 parent review 的可复核持久化证据为 `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`，结论 PASS；实现批次锚点为 scoped local commit `5c63811d`。这些事实只证明本 delta 的 request same-protocol 范围，不等于 §5/§6 全矩阵完成。

完整 Codex usage-profile 差分登记（含 client-only / no-delta / text-content / wire-neutral）见 **§12**；实施符合性对照见 **§13**。本节只索引 G13–G15 的 public-seam fixture 证据，不把 Codex 客户端策略写成 Hub 强制字段。

### 10.1 Fixtures and status

| Group / slice | Field focus | Status | Test evidence | Fixture evidence | Notes |
|---|---|---|---|---|---|
| G13a | request `reasoning` + `include` (incl. `reasoning.encrypted_content`) | PASS | `g13a_reasoning_include_test.go` | `g13a-reasoning-include.request.json`, `g13a-default-omission.request.json` | preserve supplied; no Codex default injection |
| G14a | `reasoning.summary` + summary-delivery `stream_options` | PASS | `g14a_summary_stream_options_test.go` | `g14a-summary-stream-options.request.json`, `g14a-default-omission.request.json` | no model-capability gate |
| G14b | `RawStreamOptions` sidecar / merge / false G11 | PASS | `g14b_stream_options_sidecar_test.go` | known stream_options body cases in tests | dedicated deep-clone sidecar |
| G15a | message / function_call / function_call_output item id | PASS | `g15a_input_item_identity_test.go` | `g15a-input-item-identity.request.json`, `g15a-default-omission.request.json` | no synthetic ids |
| G15b | custom_tool_call(_output); reasoning→tool tool ids | PASS | `g15b_input_item_identity_test.go` | `g15b-custom-tool-item-identity.request.json`, `g15b-reasoning-following-tool-identity.request.json` | tool id independent of call_id |
| G15c standalone | reasoning item id with following message | PASS | `g15c_reasoning_item_identity_test.go` | `g15c-standalone-reasoning-item-identity.request.json` | reasoning id ≠ Message.ID |
| G15c pure standalone | reasoning last item, no follower | PASS | same | `g15c-pure-standalone-reasoning-item-identity.request.json` | must not drop item |
| G15c summary-only | summary present, encrypted_content absent | PASS | same | `g15c-summary-only-reasoning-item-identity.request.json` | presence-aware `*string` carrier |
| G15c reasoning→tool | reasoning before function/custom tool | PASS | same | `g15c-reasoning-following-tool-item-identity.request.json` | reasoning id independent of tool ids |
| G15c no cross invent | bare `ReasoningContent` | PASS | same | inline body in test | must not invent Responses reasoning item |
| Pre-G13 note | unknown same-family effort string | PASS (aux) | `reasoning_effort_forward_compat_test.go`, Chat inbound unknown-effort tests | `future-effort` | not a cross-protocol mapping claim |

### 10.2 Status language

- Same-protocol request fidelity for G13–G15 is evidence-backed `PARTIAL`/`PASS` at the public seam（fixture + module review 索引见 §10.1 / §13.4）。
- Cross-protocol remains **no-synth** for these item identities and Codex client policies.
- Residual “waiting for G15 fixture coverage” language is retired.
- Scoped local commit `5c63811d` 可作为实现批次 code/test 锚点；parent review PASS 见 `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`。
- 本附录**不等于** §5+§6 共 101 个主状态 Field ID（或含 §9 子 ID 的 107 唯一 ID）已全部按 FDR 填完，也**不等于**全方向/`CONFIRMED`。



## 11. 规范化字段决策记录（FDR）规则

本节定义后续如何把矩阵行提升为可验收的 Field Decision Record。**它是流程规范，不是已完成证明。**

### 11.1 主键与编号

| 概念 | 含义 | 可否当作完成状态 |
|---|---|---|
| **Field ID** | 稳定主键，如 `RSP.TOP.include`、`RSP.MSG.input_items` | 是：验收与反填都以 Field ID 为键 |
| **G 编号** | 变更批次 / 实现切片标签（G1–G8 字段批次；G9–G12 helper；G13–G15 Codex delta） | 否：只能索引“哪次改动”，不能替代 Field ID 完成态 |
| **§5 顶层行** | 顶层请求字段主状态（**84** 行） | 是：顶层 Status / Evidence 权威源之一 |
| **§6 嵌套/结构行** | 嵌套结构/行为主状态（**17** 行） | 是：与 §5 合计 **101** 个主状态 Field ID |
| **§9 子 Field ID** | 批次证据索引中的额外子 ID（**6** 个，不替代 §5/§6 主键行） | 否：索引用；计入总唯一 **107**，但不能写成“§5=107” |
| **§9 / §10 / §13** | 批次证据索引 / 符合性账本 | 否：不能单独宣称字段完成 |

### 11.2 每行 FDR 必填项

后续任何“可按文档验收”的矩阵行，都必须能追溯到下列条目。已有 §5+§6 主状态行（101）与 §9 子 ID（+6 → 总唯一 107）**尚未**全部按 FDR 逐项写满；**不得**把本节 schema 要求伪称为已完成，也**不得**写“§5 主表 107”。

| FDR 字段 | 必填 | 说明 / 允许值 |
|---|---|---|
| Field ID | 是 | 稳定 ID；总览行与子行不得混用 |
| Official source | 是 | `SRC-*` + 本地章节/路径；Codex 扩展另标 P1 |
| Owner | 是 | 如 `llm.Request`、`ProviderExtensions.*`、`TransformerMetadata`、`RawTopLevelFields`、`UNMAPPED` |
| Preservation class | 是 | `native` / `raw_sidecar` / `typed+raw_sidecar` / `bridge_metadata` / `lossy_map` / `drop_with_diagnostic` / `unsupported` / `same_protocol_only` 等 |
| 六方向策略 | 是 | Rsp↔Chat、Rsp↔Anthropic、Chat↔Anthropic；无路径写 `N/A` / `no-synth` / `same_protocol_only`，禁止留白装完成 |
| Stream impact | 是 | `none` / `delta` / `done` / `final_response` / `usage` / `error` / `unchecked` |
| Value semantics | 是 | 类型、枚举/union、缺省/null/省略差异、是否 open-string |
| Code evidence | 是 | 文件/函数；未查写 `UNCHECKED` |
| Test evidence | 是 | 测试名/fixture；未测写 `UNCHECKED` |
| Review evidence | 是 | 可引用的 durable review 路径/链接；无则写 `见 task durable review report（待落盘）` 或省略具体结论，**不要**把未落盘 review 写成 PASS，也**不要**用 `PENDING` 充当矩阵字段 Status |
| Implementation disposition | 是 | 见 §11.3 |

### 11.3 Implementation disposition 枚举

> **硬约束**：disposition **不是**平行处理体系。它只能**引用或组合**既有 §4.1.2 Handling Mode 与 §1.5 方向 Strategy；不得发明与二者冲突的新处理语义。

| Disposition | 含义 | 何时使用 |
|---|---|---|
| `preserve_same_protocol` | 同协议保真；跨协议不伪造 | 绝大多数 Responses/Chat/Anthropic 原生字段 |
| `raw_or_sidecar_only` | 仅 raw/sidecar 同协议恢复 | 未进公共模型的协议私有字段 |
| `explicit_bridge` | 有文档化桥接规则 | 仅当官方/产品明确要求且有证据 |
| `lossy_with_diagnostic` | 有损并记录 LossyDowngrade | 已知无法等价表达 |
| `client_only_exclude` | 不进入 Hub transformer | Codex approval/sandbox/OAuth/multi-agent/telemetry 等 |
| `no_delta` | 本次源 diff 无协议变化 | delta register 排除项 |
| `text_content_only` | 只改文本内容，不改 schema | personality instructions、tool output truncation 文案 |
| `wire_neutral_type_shell` | 类型换壳，wire 仍同形 | 如 `ResponseItemId` transparent string |
| `unchecked_pending_evidence` | 尚无足够 code/test/review 证据 | 默认；不得标 `CONFIRMED`；**不是**矩阵 Status=`PENDING` |

#### 11.3.1 Disposition → 既有 Handling Mode / 方向 Strategy 映射

| Disposition | 允许引用的 §4.1.2 Handling Mode | 允许引用的 §1.5 Strategy（跨协议列） | 组合规则 |
|---|---|---|---|
| `preserve_same_protocol` | `common_native`, `common_native_with_value_caveat`, `protocol_native_sidecar`, `raw_fragment_ordered`, `same_protocol_only` | 同协议方向可用 `direct` / `rename` / `value_map` / `structural_transform`；跨协议无等价时用 `same_protocol_only` / `sidecar_only` / `drop_with_diagnostic` / `N/A` | 同协议保真优先；禁止“保真”名义下伪造跨协议等价 |
| `raw_or_sidecar_only` | `protocol_native_sidecar`, `raw_fragment_ordered`, `same_protocol_only` | `sidecar_only`, `same_protocol_only`, `N/A` | 不得把 raw fallback 写成 typed semantic 完成 |
| `explicit_bridge` | `bridge_metadata`, `structural_transform`, `semantic_downgrade` | `structural_transform`, `value_map`, `rename`, `direct`（仅当有证据） | 必须有文档化桥与测试；禁止隐形桥 |
| `lossy_with_diagnostic` | `semantic_downgrade`, `drop_with_diagnostic` | `drop_with_diagnostic`, `value_map`（有损） | 必须挂 LossyDowngrade / 诊断原因 |
| `client_only_exclude` | 不进入 Handling Mode（非 Hub 字段） | 不填六方向 / 或显式 `N/A`（非协议字段） | **禁止**新建 Hub Field ID |
| `no_delta` | 保持既有 mode 不变 | 保持既有 strategy 不变 | 只登记排除，不改 Status |
| `text_content_only` | 落在既有内容字段的 `common_native` / `structural_transform` / raw preserve | 通常 `direct` 原文透传或 `N/A` | **禁止**因文案变化新增 schema 行 |
| `wire_neutral_type_shell` | 保持既有 owner/mode | 保持既有 strategy | wire 同形则不新开 schema G |
| `unchecked_pending_evidence` | `unchecked` | `UNCHECKED` | 缺证据时的默认 disposition；字段 Status 仍用 §3 的 `PARTIAL`/`UNCHECKED` 等 |

### 11.4 完成门槛与诚实声明

1. FDR 必填项缺任意一项 → 该行不得 `CONFIRMED`。
2. 仅有 G 批次 public-seam PASS、但 §5/§6 行仍 `PARTIAL`/`UNCHECKED` → 只能写“批次有证据”，不能写“字段已完成”。
3. 新增 FDR 列/账本**不回溯宣称**旧行已填完。
4. 当前计数口径：§5=**84**，§6=**17**，主状态 **101**；§9 额外子 ID **6**；总唯一 **107**。**绝大多数尚未按本节逐项 FDR 闭环**。
5. disposition 若无法映射到 §4.1.2 / §1.5 已有类别 → 非法，必须先扩展既有枚举（另开文档变更），不得在 FDR 私自发明。

### 11.5 与现有列的关系

§4 矩阵列已覆盖 ID / Owner / Preservation Class / 六方向 / Stream / Code / Test / Status。FDR 是对这些列的**验收契约**，并额外要求：

- Official source 可复核；
- Value semantics 显式写出；
- Review evidence 与 Implementation disposition 可审计；
- disposition 必须能回链 §4.1.2 Handling Mode 与 §1.5 Strategy；
- 任何实现切片结束后必须反填 §5/§6 + §12/§13（见 §14）。

---

## 12. Codex Responses usage-profile delta register

### 12.1 基线与范围

| 项 | 值 |
|---|---|
| Codex base | `1f0566d3f59298d1bb88820a0d35294f1eeb07ea`（短写 `1f0566d`） |
| Codex head | `9e552e9d15ba52bed7077d5357f3e18e330f8f38`（短写 `9e552e9d1`） |
| 范围写法 | `1f0566d..9e552e9d1` + 终点 `9e552e9d1` |
| 研究来源 | `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/codex-delta-request-response-1f0566d-to-9e552e9d1.md` |
|  | `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/codex-delta-streaming-metadata-1f0566d-to-9e552e9d1.md` |
|  | `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/codex-delta-tools-mcp-1f0566d-to-9e552e9d1.md` |
| 辅助基线 | `.agent/research/codex-reasoning-effort-latest-2026-07-12.md`（effort 值域；本 range 内无新 enum 成员） |

**分层标签（Layer）**：

| Layer | 含义 | 可否写成 Hub 矩阵字段完成 |
|---|---|---|
| `wire` | 实际改变发往/来自 OpenAI Responses 的 body/event 内容或出现策略 | 可关联 Field ID；Hub 通常只保真 supplied values |
| `schema` | 公开/生产 tool 或 request schema 形状变化 | 本 range **未发现** 新 public tool/MCP schema |
| `text-content` | 仅文本内容策略变化 | 不新增 Field ID |
| `wire-neutral` | 内部类型/命名变化，serde wire 同形 | 不新增 Field ID |
| `client-only` | 仅 Codex 本地控制面 | **禁止**写成 Hub 字段 |
| `no-delta` | 审计范围内无相关变化 | 记录排除即可 |

### 12.2 编号消歧（研究别名 → 权威编号）

三份 research 初稿曾把 wire 项临时标成 G9–G11。权威编号以任务 PRD / §10 为准：

| 权威 G | 主题 | 研究别名（作废作主键） | 历史同号但不同义 |
|---|---|---|---|
| **G13** | always-on `reasoning` + `include: reasoning.encrypted_content` | research “G9” | 历史 G9 = `stream_options` raw nested helper |
| **G14** | capability-gated `reasoning.summary` + summary `stream_options` | research “G10” | 历史 G10 = raw JSON clone helper |
| **G15** | outbound item id 前缀过滤 / identity presence | research “G11” | 历史 G11 = lossy-downgrade recorder |
| G9–G12 | 架构 helper / repair（已完成，非本 delta 字段批次） | — | **不得**当作字段完成状态或本 delta 新缺口 |

### 12.3 Delta 登记表

| Delta ID | 主题 | Layer | Codex evidence (commit / area) | Matrix Field ID(s) | G group | Hub strategy / disposition |
|---|---|---|---|---|---|---|
| CD-REQ-001 | 每个 Responses 请求始终携带 `reasoning`，并始终 `include: ["reasoning.encrypted_content"]` | `wire`（emission policy） | `d2d00b663`；`client.rs` build_reasoning / include | `RSP.TOP.reasoning`, `RSP.TOP.include` | **G13** | `preserve_same_protocol`：只保真客户端实际发送的值/顺序；**禁止**因 Codex 常用而由 Hub 注入默认 |
| CD-REQ-002 | 仅当模型 catalog 允许时发送 `reasoning.summary` 与 summary-delivery `stream_options` | `wire`（conditional emission） | `dffe1f02a`；`supports_reasoning_summary_parameter` | `RSP.TOP.reasoning`, `RSP.TOP.stream_options` | **G14** | `preserve_same_protocol`：有则保留、无则保持省略；**禁止**在 Hub 复制 Codex model catalog gate |
| CD-REQ-003 | `input[]` item `id`：仅合法前缀出站；空/无前缀省略 | `wire` | `c9d52de5c`；`prepare_response_items_for_request` | `RSP.MSG.input_items`（总览 `RSP.TOP.input` 相关） | **G15** | `preserve_same_protocol`：保留已有非空 id、允许无 id；**禁止**强制 Codex 前缀表或合成 id |
| CD-REQ-004 | `ResponseItemId` 类型换壳；serde transparent string | `wire-neutral` | `c9d52de5c`；`response_item_id.rs` | `RSP.MSG.input_items`（identity carrier，非新 wire key） | **G15**（并入，不单开 schema G） | `wire_neutral_type_shell`：Hub 仍按 string identity 处理 |
| CD-REQ-005 | `personality = "none"` 改写 instructions 文本 | `text-content` | `09ccae2c0` | `RSP.TOP.instructions`（仅内容策略备注；**非**新 schema 行） | — | `text_content_only` / `client_only_exclude`：不立项 Hub schema 映射 |
| CD-TOOL-001 | tool/exec 输出 truncation 增加 bytes-omitted 提示文本 | `text-content` | `6138909d6`；unified_exec / `function_call_output` 文本 | 无新 Field ID；落在既有 tool result / `function_call_output` 内容保真 | — | `text_content_only`：原文透传即可；**禁止**提升为新协议字段 |
| CD-TOOL-002 | tool_search / defer_loading / namespace / apply_patch / web_search / MCP declaration schema | `no-delta` | tools crate 生产代码无功能 diff | 既有 `RSP.TOOL.tool_search`, `CODEX.TOOL.namespace`, `RSP.TOOL.hosted`, `ANT.TOP.mcp_servers`, `ANT.TOOL.mcp_toolset` 等 | 非本 delta 新 G | `no_delta`：继续既有矩阵残差，不因本 range 重开 |
| CD-CLI-001 | approval 中心化 / guardian / permission hooks | `client-only` | `tools/approvals.rs` 等 | **无 Hub Field ID** | — | `client_only_exclude`；禁止桥到 Responses MCP approval items |
| CD-CLI-002 | sandbox workspace roots / exec-server 本地沙箱 | `client-only` | `sandboxing.rs` 等 | **无 Hub Field ID** | — | `client_only_exclude` |
| CD-CLI-003 | MCP OAuth refresh / 持久化策略 | `client-only` | `rmcp-client/src/oauth/**` | **无 Hub Field ID** | — | `client_only_exclude`；不改变 Responses MCP tool schema |
| CD-CLI-004 | multi-agent 终端事件 `error`/`started_at`、InterAgentCommunication 等 | `client-only` | `protocol.rs` TurnComplete/Aborted 等 | **无 Hub Field ID** | — | `client_only_exclude` |
| CD-CLI-005 | WebSocket timing telemetry / 本地 trace | `client-only` | `9993fb838` 等 | **无 Hub Field ID** | — | `client_only_exclude` |
| CD-STR-001 | SSE / stream event 解析 | `no-delta` | `codex-api/src/sse/responses.rs` 0 字节 diff | `RSP.STREAM.events`（无本 delta 变化） | — | `no_delta`：不声称新 stream event 修复 |
| CD-META-001 | model catalog 字段改名（`supports_reasoning_summaries` → `supports_reasoning_summary_parameter`） | `client-only` | `openai_models.rs` | **无 Hub Field ID** | 支撑 G14 客户端策略 | `client_only_exclude` |
| CD-NOTE-001 | `ReasoningEffort` 自定义字符串 / `ultra→max` | `wire` 值域注意（本 range **无 enum delta**；`ultra→max` 在 base 已存在且未改） | 既有 `client.rs` + 辅助研究 | `RSP.TOP.reasoning`, `CHAT.TOP.reasoning_effort` | G13/G14 note | same-family open-string 保真；**禁止**把 `ultra→max` 写成全局协议映射 |
| CD-CLI-006 | `RolloutLine.ordinal` 分页/本地 rollout 序号 | `client-only` | `5c19155cb`；`protocol.rs` rollout persistence | **无 Hub Field ID** | — | `client_only_exclude`：本地持久化/分页元数据，不进入 Responses/Chat/Anthropic transformer |
| CD-CLI-007 | `event_mapping` / `stream_events_utils` 仅适配 `ResponseItemId` 类型 | `wire-neutral` | `c9d52de5c` 连带；`event_mapping.rs` / `stream_events_utils.rs` | **无 Hub Field ID** | — | `wire_neutral_type_shell`：本地 TurnItem/日志映射类型换壳；**不是** SSE/event schema delta，勿桥到 `RSP.STREAM.events` |
| CD-CLI-008 | web-search history 仅适配 `ResponseItemId` API | `wire-neutral` / `client-only` | `ext/web-search/src/history.rs` | **无 Hub Field ID** | — | `wire_neutral_type_shell` + `client_only_exclude`：客户端 history 类型适配；不改变 public web_search tool/item schema |
| CD-CLI-009 | apply_patch 审批路径保留 `PathUri` + `environment_id`（延迟绝对路径） | `client-only` | `b66c25c6a`；`apply_patch.rs` / `approvals.rs` | **无 Hub Field ID** | — | `client_only_exclude`：本地 approval/guardian 路径；**禁止**桥到 Responses MCP approval / tool schema |

### 12.4 覆盖核对（三份 research → 本表）

| Research 主题 | 覆盖 Delta ID | 结果 |
|---|---|---|
| reasoning / include always-on | CD-REQ-001 | 已登记 → G13 |
| summary / stream_options capability gate | CD-REQ-002 | 已登记 → G14 |
| ResponseItemId / id filtering | CD-REQ-003, CD-REQ-004 | 已登记 → G15 |
| personality none instructions text | CD-REQ-005 | 已登记为 text-content，非 Hub schema |
| tool output truncation text | CD-TOOL-001 | 已登记为 text-content |
| ResponseItemId transparent type | CD-REQ-004 | 已登记为 wire-neutral |
| approval / sandbox / OAuth / multi-agent / telemetry | CD-CLI-001..005 | 已登记为 client-only，**无 Hub Field ID** |
| `RolloutLine.ordinal` | CD-CLI-006 | 已点名登记为 client-only，**无 Hub Field ID** |
| `event_mapping` / `stream_events_utils` type adaptation | CD-CLI-007 | 已点名登记为 wire-neutral，**无 Hub Field ID**；勿误桥 stream 字段 |
| web-search history `ResponseItemId` adaptation | CD-CLI-008 | 已点名登记为 wire-neutral/client-only，**无 Hub Field ID** |
| apply_patch `PathUri` / `environment_id` approval path | CD-CLI-009 | 已点名登记为 client-only，**无 Hub Field ID**；勿桥 MCP approval |
| tool/MCP no schema delta | CD-TOOL-002 | 已登记为 no-delta |
| SSE parser / stream metadata 无变更 | CD-STR-001 | 已登记为 no-delta |

### 12.5 硬性禁令

1. **禁止**把 `client-only` 项写成 Hub §5 字段或伪造成 `CONFIRMED`。
2. **禁止**把 Codex “始终发送 / 按 catalog 省略 / 前缀过滤”原样复制为 Hub 全局强制策略。
3. **禁止**把 research 临时 G9–G11 与历史 helper G9–G12、权威字段批次 G13–G15 混用。
4. 后续任何 Codex diff 必须先增补本 register，再决定是否开实现 slice（§14）。

---

## 13. 实施符合性账本（G1–G15）

### 13.1 阅读规则

- 本账本追溯**实现批次**与 **Field ID** 的符合性，不重写 §5/§6 全表。
- **G1–G8、G13–G15**：字段/边界相关批次，必须有 Field ID 映射。
- **G9–G12**：历史 helper/repair 编号（stream_options raw nested、raw JSON clone、lossy recorder、raw top-level capture）。它们是基础设施，**不能**作为“某字段已完成”的状态位。
- `Review/conformance` 仅反映当前文档可引用的证据；无法确认则标 `UNCHECKED`，不杜撰。
- `Actual owner/mode` 来自 §5 / §9 / §10 与既有测试索引；若与 required 不一致，记 gap，不在本 docs-only 任务中改生产代码。

### 13.2 G1–G8 字段批次

| G | Matrix Field ID(s) | Required owner / mode | Actual owner / mode (docs evidence) | Same-protocol evidence | Cross-protocol policy | Review / conformance |
|---|---|---|---|---|---|---|
| G1 | `CHAT.TOP.n` | Chat raw/native preserve；多 choice 不跨协议伪造 | `openAIChatRawPreserveFields` (`chat_n.go`) / `raw_preserve` | `chat_n_test.go` | no-synth → Responses；Anthropic LossyDowngrade | §9.1 索引；模块审查细节 `UNCHECKED`（本账本未重跑审查原文） |
| G2 | `CHAT.TOP.prompt_cache_retention` | Chat raw preserve；≠ Anthropic cache_control | same raw preserve path / `raw_preserve` | `chat_n_test.go` retention cases | no-synth；不伪映射 Anthropic | 同 G1：证据在 §9；审查原文 `UNCHECKED` |
| G3 | `ANT.TOP.container`, `ANT.TOP.inference_geo` | Anthropic opaque + metadata；OpenAI no-synth | Anthropic model + metadata / opaque JSON+metadata | `container_inference_geo_test.go` | no-synth to OpenAI | §9.1；审查原文 `UNCHECKED` |
| G4 | `CHAT.TOP.audio`, `CHAT.TOP.prediction`, `CHAT.TOP.moderation` | Chat raw preserve output-controls | `chat_n.go` raw preserve | `chat_n_test.go` output-controls | no-synth / lossy | §9.1；审查原文 `UNCHECKED` |
| G5a | `CHAT.TOP.web_search_options` | Chat raw preserve；禁止改名为 Responses web_search tool | `chat_n.go` raw preserve | web_search_options tests | no-synth Responses；Anthropic LossyDowngrade | §9.1；审查原文 `UNCHECKED` |
| G5b | `CHAT.TOP.functions`, `CHAT.TOP.function_call`, `CHAT.MSG.function_call` | legacy shape 可往返；不破坏 modern tools | deprecated origin metadata + raw preserve / bridge+origin metadata | `chat_deprecated_functions_test.go` 等 | no fake modern rewrite unless explicit bridge | §9.1 含 `CHAT.MSG.function_call`；**注意**：`CHAT.MSG.function_call` 主要出现在 §9 索引，§5 是否有对等响应行需后续 FDR 对齐（文档缝） |
| G6 | `ANT.TOP.mcp_servers`, `ANT.TOOL.mcp_toolset` | Anthropic MCP 隔离；禁止桥 Responses mcp/namespace | opaque JSON；`anthropic_raw_tools` ordered fragments | `mcp_connector_test.go` | not Responses mcp/namespace | §9.1；审查原文 `UNCHECKED` |
| G7 | `RSP.TOP.reasoning` 及子证据 `RSP.TOP.reasoning.context`, `RSP.TOP.reasoning.generate_summary`, `RSP.RESP.reasoning.content`, `RSP.STREAM.reasoning_text`, `RSP.TOP.reasoning.unknown_nested` | reasoning 对象/输出/stream 分路径；effort≠budget | native+raw_sidecar / stream aggregator | `reasoning_context_test.go`, `reasoning_g7_test.go` 等 | 不与 Chat effort / Anthropic thinking 伪等价 | §9.1；子 Field ID 已在索引出现，主表仍以 `RSP.TOP.reasoning` 总览行为主 |
| G8 | `CHAT.TOP.modalities` | Chat same-protocol typed modalities | `llm.Request.Modalities` + Chat typed / `common_typed` | `TestInboundTransformer_TransformRequest_ModalitiesRoundTripChat` 等 | 六向未闭环，保持 PARTIAL | G8 review PASS（`.trellis/tasks/07-12-protocol-residual-five-boundary-fields/research/reviews/g8-residual-five-fields-review.md`） |
| G8 | `ANT.TOP.cache_control` | top-level cache_control 与 content-block 隔离；≠ OpenAI prompt_cache_* | `MessageRequest.CacheControl` + metadata / native+metadata | `TestTopLevelCacheControlRoundTrip` | no OpenAI prompt-cache bridge | G8 review PASS |
| G8 | `RSP.TOP.context_management` | same-protocol raw top-level fallback only | `RawTopLevelFields` / `raw_fallback_same_protocol` | `TestResponsesContextManagement_SameProtocolRawTopLevelFallback` | no-synth to Anthropic | G8 review PASS；typed semantic support **未**宣称 |
| G8 | `RSP.TOP.conversation` | string+object raw fallback；非跨协议 messages state | commented request field + `RawTopLevelFields` | `TestResponsesConversation_SameProtocolRawTopLevelFallback` | no-synth | G8 review PASS；typed request field仍未启用 |
| G8 | `RSP.TOOL.hosted` | hosted inventory + same-protocol raw；无统一跨协议抽象 | native raw families + raw-tool merge / `native_raw_family_partial` | `TestResponsesHostedTools_SameProtocolRawPreserveAndChatLossy` | Chat lossy/no-synth；Anthropic 无统一 bridge | G8 review PASS w/ minors（Rsp→Anthropic 列曾偏乐观；以 no unified bridge 为准） |

#### 13.2.1 G8 细节 `UNCHECKED` 清单（不杜撰）

下列细节在现有文档/审查中**未能**于本规范化任务内重新核到可引用的逐文件实现结论，故保持 `UNCHECKED`：

| 项 | 状态 | 说明 |
|---|---|---|
| G8 是否改动过生产 transformer 代码 | 任务 ledger 写 “No production transformer changes” | 本账本采信任务记录，但**未**在本任务重跑 `git show 5260e558` 逐文件核对 |
| Hosted 各具体 family（file/computer/code/image 等）是否逐一有独立 fixture | `UNCHECKED` | 现有证据是 inventory + 代表性 raw preserve/Chat lossy；不能写成每个 hosted variant 全 CONFIRMED |
| G8 与 G12 raw top-level capture helper 的代码所有权边界 | `UNCHECKED` | G12 是 helper 批次；G8 复用其行为但批次号不可混为字段完成态 |
| G8 跨协议六方向是否因 review minors 需要改 §5 单元格 | `UNCHECKED` | review 指出 hosted Rsp→Anthropic 表述风险；§5 当前已偏 no-synth，是否仍有残留乐观措辞需另开 docs 校对 |

### 13.3 G9–G12 helper 批次（非字段完成状态）

| G | 主题 | 关联 Field ID（若有） | Required role | Actual role | Conformance note |
|---|---|---|---|---|---|
| G9 | Responses `stream_options` raw nested preservation | `RSP.TOP.stream_options` | helper：typed+raw nested 同协议保真 | 被 G14 证据复用（`RawStreamOptions` sidecar） | **不是**“stream_options 字段全矩阵完成” |
| G10 | raw JSON clone helper | 多字段 raw 路径 | helper | 基础设施 | **禁止**标为字段 Status |
| G11 | lossy-downgrade recorder | 跨协议 lossy 行 | helper | 基础设施；G14b 文档曾写 “false G11” 指误报/非本 helper 回归 | **禁止**与 research 临时 G11（item id）混淆 |
| G12 | raw top-level capture helper | `RSP.TOP.context_management`, `RSP.TOP.conversation` 等 | helper | G8 raw fallback 依赖其能力 | **禁止**用 G12 完成替代 G8/字段 FDR |

### 13.4 G13–G15 Codex delta 字段批次

| G | Matrix Field ID(s) | Required owner / mode | Actual owner / mode | Same-protocol evidence | Cross-protocol policy | Review / conformance |
|---|---|---|---|---|---|---|
| G13 | `RSP.TOP.reasoning`, `RSP.TOP.include` | 保真 supplied `reasoning`/`include`；不注入 Codex always-on 默认 | Responses native include/reasoning + metadata/raw merge | `g13a_reasoning_include_test.go` + fixtures；aux `reasoning_effort_forward_compat_test.go` | no-synth；不跨协议伪造 encrypted include | independent module review PASS；parent review PASS（`.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`）；scoped local commit `5c63811d` |
| G14 | `RSP.TOP.reasoning`（summary 子路径）, `RSP.TOP.stream_options` | 保真 summary + summary-delivery options；不复制 catalog gate | typed `StreamOptions` + `RawStreamOptions` deep-clone sidecar | `g14a_summary_stream_options_test.go`, `g14b_stream_options_sidecar_test.go` | no-synth | independent module review PASS；parent review PASS（`.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`）；scoped local commit `5c63811d` |
| G15 | `RSP.MSG.input_items`（及 `RSP.TOP.input` 总览相关 identity） | presence-aware 非空 id 保真；无 id 保持无；不合成、不强制前缀 | `Message.ID` / `ToolCall.ResponseItemID` / `Message.ResponseReasoningItemID(*string)` 等 | `g15a/b/c_*_test.go` + fixtures | 跨协议不得发明 Responses item id | independent module review PASS；parent review PASS（`.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`）；scoped local commit `5c63811d` |

### 13.5 批次状态总览

| 批次 | 类型 | 字段完成？ | 说明 |
|---|---|---|---|
| G1–G7 | 实现 + 测试 | **否（PARTIAL only）** | same-protocol 有证据，非全矩阵 CONFIRMED |
| G8 | 边界证据 + 测试（ledger：无生产代码改动） | **否（PARTIAL）** | 五边界项有字段级证据；细节见 §13.2.1 UNCHECKED |
| G9–G12 | helper/repair | **不适用** | 不得当字段完成状态 |
| G13–G15 | Codex delta same-protocol fixtures | **否（PARTIAL only）** | same-protocol public-seam PASS（module review + fixtures；parent review PASS；commit `5c63811d`）；仍不代表全字段/全方向完成。持久化证据见 `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`。 |


### 13.6 Review 证据与 task artifacts 的边界

- 任务目录中的 `protocol-compliance-loop-ledger.md` / `implement.md` / `prd.md` 是**执行记录**；父级审查的持久化证据必须有可引用报告或等价 artifact。
- G13–G15 的 parent review 已落盘：`.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md`；相关 task artifacts 已同步为 PASS，scoped commit 为 `5c63811d`。
- 未来任务在 review report 落盘前：不得主张 parent review PASS；字段 Status 也**不要**用 `PENDING`，仍只用 §3 的 `PARTIAL` / `UNCHECKED` / `CONFIRMED` / …。
- 符合性账本与 §5/§6 在“是否宣称全矩阵 CONFIRMED”上保持诚实：批次 public-seam PASS ≠ 101 主状态行 FDR 全填完，更 ≠ 107 唯一 ID 全完成。

---

## 14. 后续 Codex diff 与实现 slice 规范

### 14.1 强制顺序

```text
Codex / 协议源 diff
  → 先写入 §12 delta register（Layer + Field ID + disposition）
  → 再决定：文档-only / 开实现 slice / client-only 排除
  → 合并前反填：§5 主表 + §11 FDR 缺口 + §13 符合性账本 +（如适用）§9/§10 证据索引
```

### 14.2 规则

1. **任何后续 Codex diff** 必须先登记到 §12，再决定是否创建实现 slice。
2. **任何实现 slice** 必须反填：对应 Field ID 的 §5 行、§11 FDR 必填项、§13 符合性账本。
3. **稳定 Field ID 是规范主键**；G 编号只是变更批次。
4. 不得重用历史 G 号表达新语义；新批次应分配新 G 或明确 “no new G / existing Field ID only”。
5. client-only / no-delta / text-content / wire-neutral 必须保持分层，禁止上抬为 Hub schema 完成。
6. 文档更新本身若未补测试证据，不得把 Status 从 `PARTIAL` 抬到 `CONFIRMED`。

### 14.3 与相关文档的关系

| 文档 | 角色 |
|---|---|
| 本文 §5 + §6 | Field 主状态权威（84 + 17 = 101） |
| 本文 §9 子 ID | 批次证据索引（+6 → 总唯一 107）；不替代主状态行 |
| 本文 §11–§14 | 规范化验收、Codex delta 账本、FDR 优先清单 |
| `docs/specs/protocols/hub-protocol-field-matrix.md` | Hub carrier 视角摘要；冲突时回查本文 §5/§6 + 代码 |
| `.trellis/spec/backend/protocol-transformer-guidelines.md` | 实现/审查行为准则；不替代 Field ID 主键 |
| 任务 research 三份 delta 报告 | §12 事实来源；编号冲突以本文 §12.2 为准 |
| `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/g13-g15-parent-review.md` | G13–G15 parent review PASS 的持久化证据；未来任务沿用同类可引用 artifact |

### 14.4 下一批 FDR 优先审查 Field ID 清单

基于既有 residual（§9.3、`residual-gaps.md`、§5/§6 粗粒度占位行），**先补 FDR/证据，不默认新开 feature**。下列为优先顺序建议：

| 优先 Field ID | 为何优先 | 先补什么证据（FDR 最小集） |
|---|---|---|
| `CHAT.TOP.tools`（含 custom tool 形态） | residual 明确 Chat custom tool 源缺口；顶层 tools 总览行过粗，易误桥 Responses/Anthropic | 拆 function vs custom（及不支持形态）；same-protocol raw/typed 边界；跨协议 no-synth / lossy 策略；禁止发明未证实 custom 支持 |
| `RSP.TOP.context_management` | G8 仅 raw fallback PARTIAL；易被误读为 typed semantic 完成 | Official value shape；typed vs `RawTopLevelFields` disposition 映射；跨协议 no-synth 原因；是否需要 typed request 字段的产品决策记录 |
| `RSP.TOP.conversation` | 同 G8：string/object raw 有证据，但 request 字段仍 commented | string id vs object 两种 value semantics；same_protocol_only 六方向；与 Chat/Anthropic messages state 的非等价声明 |
| `RSP.MSG.input_items`（子矩阵） | G15 只闭环 identity/presence；item variant 全量仍是结构主风险 | 按 item type 拆子 FDR（message / function_call(_output) / custom_tool / reasoning / 其他 raw）；顺序与 raw fragment；跨协议 no-synth id |
| `RSP.STREAM.events` | 主状态仍是事件族总览；Codex range 确认 SSE parser **无 delta**，但 Hub 事件族保真未 FDR 化 | 事件族清单与 stream impact；delta/done/final/usage/error 分测策略；与 `stream_options`（请求控件）严格分离 |
| `ANT.MSG.content_blocks` | content block 族是跨协议最大结构差之一；总览行无法 CONFIRMED | 按 block type 拆子 FDR（text/image/thinking/tool_use/tool_result/…）；thinking signature；与 Chat parts / Responses items 的非伪映射 |
| `ANT.STREAM.events` | named SSE 与 OpenAI 事件模型不同；thinking/signature/input_json delta 必须单测 | 事件名清单；content_block_delta 子类型；usage/error；与 Chat chunk / Responses SSE 的 strategy 映射 |
| `RSP.RESP.output` | 输出 item 族与请求 input 对称风险；function/custom/status/namespace 未全量 | output item type 子矩阵；final_response 证据；与 stream event 的对应关系；跨协议降级策略 |

规则：

1. 优先清单**不是**新 G 编号分配表；仍以 Field ID 为键。
2. 进入实现 slice 前，必须先有 §11 FDR 最小集 +（若源自 Codex）§12 登记。
3. 粗粒度总览行在子 FDR 未齐前，Status 最高 `PARTIAL`，不得 `CONFIRMED`。
