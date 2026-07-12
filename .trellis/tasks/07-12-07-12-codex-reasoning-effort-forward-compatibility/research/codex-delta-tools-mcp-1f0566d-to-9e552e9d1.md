# Research: Codex tools/MCP/approval delta `1f0566d..9e552e9d1`

- Query: 对比 `/Users/asuan/项目/ai工具/openai-codex` 的 `1f0566d..9e552e9d1`，只审计 Codex 工具、MCP、懒加载/工具搜索、apply_patch、web search、multi-agent、approval 是否改变对 OpenAI Responses 请求/响应或额外工具协议的输入/输出形状；严格区分 client-only 与会透传 Hub 的 wire 字段。
- Scope: mixed（Codex 源码只读 diff + Hub 协议矩阵/transformer 现状对照）
- Date: 2026-07-12
- Base: `1f0566d3f59298d1bb88820a0d35294f1eeb07ea`
- Head: `9e552e9d15ba52bed7077d5357f3e18e330f8f38`
- Focus paths: `codex-rs/core/src/tools/**`, `codex-rs/core/src/client.rs` tool/request assembly, `codex-rs/protocol/**`, `codex-rs/tools/**`, `codex-rs/rmcp-client/**`, related suite tests

## Executive summary

在该区间内，**工具声明/组装的 Responses wire shape 基本未变**。`codex-rs/tools` 的 tool definition / `responses_api` 转换代码无功能 diff（仅测试字段跟随 `supports_reasoning_summary_parameter` 改名）。MCP OAuth、approval 编排、apply_patch guardian 路径、sandbox workspace roots、multi-agent 终端事件、web-search history 适配，绝大多数是 **client-only**。

唯一明确会改变发往 Responses provider 的 tool/item 相关 wire 行为是：

1. **`input[]` item `id` 出站前缀策略**（#32312）：未前缀或空 ID 在 HTTP/WebSocket 请求中被省略；有前缀的 ID 在 store/item_ids 场景保留。
2. **`function_call_output` / exec 工具结果文本内容**可能多出 truncation / bytes-omitted 提示（#32150）：仍是普通 output 文本，不是新 JSON 字段。
3. 旁路但同属 `client.rs` 请求体：reasoning 始终发送、`include` 恒含 `reasoning.encrypted_content`、`stream_options` 条件收紧——**不是 tool 家族字段**，本范围只标注，不展开为 tool gap。

**未发现** tool_search / defer_loading / namespace / apply_patch tool schema / web_search tool schema / MCP tool declaration / multi-agent tool schema 的 Responses 请求/响应类型新增或改名。

## Files found

| Path | One-line description |
|---|---|
| `codex-rs/core/src/client.rs` | Responses 请求组装：reasoning、include、stream_options、`prepare_response_items_for_request` 去掉未前缀 item id |
| `codex-rs/protocol/src/response_item_id.rs` | 新 `ResponseItemId`：生成 `prefix_suffix`，`is_prefixed()` 判定 |
| `codex-rs/protocol/src/models.rs` | 所有 `ResponseItem` 变体的 `id: Option<ResponseItemId>` + `id_prefix()` 表 |
| `codex-rs/protocol/src/items.rs` | hook prompt message 生成 `msg_*` 前缀 ID |
| `codex-rs/protocol/src/protocol.rs` | client event：`TurnCompleteEvent.error/started_at`、`TurnAbortedEvent.started_at`、rollout ordinal；`InterAgentCommunication.id` 类型换壳 |
| `codex-rs/protocol/src/request_permissions.rs` | `strict_auto_review` 注释：允许 hook 先解决 |
| `codex-rs/core/src/tools/approvals.rs` | 新 approval 中心：hook → guardian/user；含 ApplyPatch/Shell/ExecCommand action |
| `codex-rs/core/src/tools/orchestrator.rs` | 把 approval 逻辑迁出；strict auto-review 仍可被 hook allow |
| `codex-rs/core/src/tools/runtimes/apply_patch.rs` | guardian 请求改为保留 `PathUri` + `environment_id`，延迟绝对路径转换 |
| `codex-rs/core/src/tools/sandboxing.rs` | 删除 `ApprovalCtx.guardian_review_id`；exec-server sandbox 传入 workspace roots |
| `codex-rs/core/src/tools/context.rs` | `ExecCommandToolOutput.output_omitted_bytes` + truncation 文本 marker |
| `codex-rs/core/src/tools/handlers/unified_exec/exec_command.rs` | 透传 omission 元数据到 model-facing output 文本 |
| `codex-rs/rmcp-client/src/oauth/**` | MCP OAuth refresh 跨进程串行与失败策略 |
| `codex-rs/ext/web-search/src/history.rs` | 仅跟随 `ResponseItemId` API 改动 |
| `codex-rs/tools/src/*_tests.rs` | 无 tool wire shape 变更，仅 model capability 字段改名 |
| Hub: `llm/openai_responses_classification.go` | 已知 native tool/item 类型分类（含 tool_search/apply_patch/mcp 等） |
| Hub: `llm/transformer/openai/responses/inbound.go` | `generateItemID()` 现为 `item_<16 alnum>`，非 Codex 前缀表 |
| Hub: `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` | G1–G8 后工具行仍 PARTIAL；hosted/tool_search/namespace 已有矩阵位 |

## Code patterns (file:line evidence)

### 1) Tool declaration / lazy load / tool_search / web_search / MCP tool schema

- `codex-rs/tools/**` 在该区间 **无生产代码 diff**；仅：
  - `tool_config_tests.rs`: `supports_reasoning_summaries` → `supports_reasoning_summary_parameter`
  - `image_detail_tests.rs`: 删除旧字段
- `tool_definition_to_responses_api_tool` 仍：`defer_loading: tool_definition.defer_loading.then_some(true)`（head 现状，区间无 diff）。
- `core/src/tools/handlers/**` 除 unified_exec 与 multi_agents **测试** 外无 handler 业务 diff。
- **结论**：tool_search / defer_loading / namespace / web_search tool config / MCP tool-to-Responses 转换 **wire or client-only = no delta**。

### 2) Outbound item IDs on Responses `input[]` (wire)

`prepare_response_items_for_request`（`client.rs`）：

```text
// first: strip unprefixed IDs
if item.id().is_some_and(|id| !id.is_prefixed()) { item.set_id(None); }
// then: if item_ids disabled and not store, strip all IDs
```

`ResponseItemId::is_prefixed`：存在 `_` 且 prefix/suffix 均非空。

`ResponseItem::id_prefix()`（`models.rs`）工具相关前缀：

| Item variant | Prefix |
|---|---|
| AdditionalTools | `at` |
| FunctionCall | `fc` |
| FunctionCallOutput | `fco` |
| ToolSearchCall | `tsc` |
| ToolSearchOutput | `tso` |
| CustomToolCall | `ctc` |
| CustomToolCallOutput | `ctco` |
| WebSearchCall | `ws` |
| LocalShellCall | `lsh` |
| Message | `msg` |
| Reasoning | `rs` |
| ImageGenerationCall | `ig` |

测试证据：

- `azure_responses_request_includes_store_and_prefixed_item_ids` 断言 store 场景保留 `msg_message-id` / `fc_function-id`。
- `responses_websocket_omits_unprefixed_item_ids_without_mutating_prompt` 断言未前缀/空 ID 不出现在请求体。

**Wire impact**：同一历史里若客户端曾发送无前缀 item id，现在会省略该字段；有前缀 id 的 shape 仍是字符串。JSON 类型未从 string 变成 object（`ResponseItemId` 为 transparent string）。

### 3) apply_patch

- Runtime 变更：`build_guardian_review_request` → `build_approval_action`，保留 `PathUri` 与 `environment_id`，把绝对路径转换推迟到 guardian 层（`approvals.rs` `guardian_cwd`）。
- 未改 apply_patch **tool schema**、call arguments、或 Responses custom/function tool 名称。
- **Wire or client-only = client-only**（本地审批/沙箱路径约定）。

### 4) Approval / permission hooks / multi-agent terminal events

- 新 `tools/approvals.rs`：permission hook 可在 strict auto-review 下 Allow/Deny；guardian/user 共用 rejection 文本与 telemetry source。
- `request_permissions.rs` 仅注释：`strict_auto_review` 可被 permission hook 解析。
- multi-agent 相关 diff 主要是 `TurnCompleteEvent.{error,started_at}` / `TurnAbortedEvent.started_at` 填值，以及子代理 reasoning summary support 测试。
- 这些是 **Codex app-server/TUI event protocol**，不是 OpenAI Responses HTTP body 的 tool 字段。
- **Wire or client-only = client-only**（对 Hub 的 Responses 透传无新 tool 协议字段）。

### 5) MCP client

- `rmcp-client` OAuth refresh lock/transaction：并发 refresh 串行、失败 fail-fast、排除 refresh 时间于 init timeout。
- `rmcp_client.rs` / streamable HTTP retry 小改。
- suite `rmcp_client.rs` 几乎无语义 diff。
- **不改变** Responses `tools[].type=mcp`、`mcp_list_tools` / `mcp_call` / approval item 的公共 wire schema。
- **Wire or client-only = client-only**。

### 6) Unified exec / function_call_output text

- 新增 `output_omitted_bytes` 与 `... N bytes omitted ...` marker。
- 进入 model 的仍是 `function_call_output` / exec tool result 的 **text body**（可能多一行 Warning/omission notice）。
- **Wire shape**：无新 JSON key；**内容语义** 可能变化。
- Hub 若做 same-protocol raw preserve 文本，应原样保留；若做结构化 parse truncation marker，当前无证据需要。

### 7) Web search

- `ext/web-search/src/history.rs` 仅 `set_id(ResponseItemId::...)` 适配。
- 无 web_search tool options / call item schema diff。

### 8) Non-tool but co-located request assembly (out of tool family, note only)

`client.rs` 同 diff：

- 始终 `reasoning: Some(...)`，即使 model 不支持 summary 也发 effort。
- `include` 恒为 `["reasoning.encrypted_content"]`（不再依赖 reasoning Some）。
- `stream_options` 仅在 summary 实际发送时出现。

这些属于 reasoning/stream 范围，**不是 tools/MCP 家族**；若其他 research scope 覆盖 G-reasoning，此处不新建 tool G。

## External references

- Codex commits in range (tool-relevant):
  - `c9d52de5c` Require prefixes for outbound response item IDs (#32312)
  - `6138909d6` Keep unified exec output collection bounded (#32150)
  - `2da1b1282` Let permission hooks resolve strict auto-review requests (#32232)
  - `b66c25c6a` Preserve local path conventions in automatic approvals (#32261)
  - `c8dc8e5fd` Propagate workspace roots to exec-server sandboxes (#32214)
  - `6962a2eca` Serialize MCP OAuth credential refreshes (#32229)
  - `dffe1f02a` / multi-agent suite: summary support for final model (reasoning, not tools)
- Hub protocol matrix after G1–G8: `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` §5/§9
- Hub classification: `llm/openai_responses_classification.go` already lists `tool_search`, `apply_patch`, `mcp`, `function_call_output`, etc.
- Guideline: `.trellis/spec/backend/protocol-transformer-guidelines.md` — Codex Responses 作为 OpenAI Responses usage profile；禁止伪造 MCP/tool ecosystem 桥接。

## Related specs

- `docs/specs/protocols/drafts/batch-tools-mcp.md` — tools/MCP inventory；hosted/tool_search/namespace 分家
- `docs/specs/protocols/openai-responses-protocol.md` — function_call/output 保 `call_id`/`id`/`status`/`namespace`
- G1–G7 closed modules；G8 residual five boundary fields（含 hosted tools inventory evidence）
- 本任务 PRD/design 以 reasoning effort 为主；本文件是额外 Codex delta 范围 B 证据，不改变 effort 切片范围

## Cross-scope G numbering alignment

Concurrent research in this task already labels the same non-tool wire deltas differently:

| Topic | `codex-delta-request-response-*.md` | `codex-delta-streaming-metadata-*.md` | This tools file |
|---|---|---|---|
| Always-on `reasoning` + `include[reasoning.encrypted_content]` | **G9** | **G13** | non-tool; defer to those files |
| Capability-gated `reasoning.summary` / summary `stream_options` | **G10** | **G14** | non-tool; defer |
| Prefixed / unprefixed Responses item `id` filtering | **G11** | **G15** | **same gap** (tools touch because FunctionCall/ToolSearch/WebSearch/AdditionalTools ids use it) |
| Prior architecture helper seams `stream_options` raw nested / raw JSON / lossy recorder / raw capture | historical G9–G12 in archive task | referenced as “既有 G9–G12” | **not** reopened by tools delta |

There is **no pre-existing Hub registry document that freezes G9–G15 for this Codex delta**. Below uses the request-response numbering as primary (**G9–G11**) and notes streaming aliases (**G13–G15**). Tools-only findings that are not wire schema changes are marked **不进入 Hub**.

## Itemized findings → wire vs client-only → Hub G 归类建议

| # | 变更 | 证据 | wire or client-only | 归类建议 |
|---|---|---|---|---|
| B1 | 出站 Responses `input[]` item `id`：未前缀/空 ID 省略；生成 ID 使用类型前缀表（`fc`/`fco`/`tsc`/`tso`/`ctc`/`ctco`/`ws`/`at`/`msg`/`rs`…） | `c9d52de5c`; `client.rs` `prepare_response_items_for_request`; `response_item_id.rs`; `models.rs` `id_prefix`; client/WS tests | **wire**（字段存在性/取值约定；类型仍是 string） | **G11（= streaming 文件的 G15）**。Hub same-protocol 应透传已有 id 字符串；允许缺 id；不要把客户端 id 强制改成 Hub `generateItemID()` 的 `item_*`。跨协议不要求合成 Codex 前缀表。 |
| B2 | `FunctionCall` / `FunctionCallOutput` / `ToolSearch*` / `WebSearchCall` / `AdditionalTools` 等 `id` 类型换为 transparent `ResponseItemId` | `protocol/src/models.rs` | **wire-neutral**（serde 仍是 string） | **并入 G11/G15**；不为类型换壳单独开 G。 |
| B3 | unified exec / shell 输出 truncation 增加 `... N bytes omitted ...` 与 Warning 行，经 tool result 文本回灌模型 | `6138909d6`; `tools/context.rs` `truncated_output`; unified_exec handler | **wire content-only**（`function_call_output.output` 文本；无新 JSON key） | **不进入 Hub 实现 G**；文档-only：既有 `function_call_output` raw preserve 原文透传，勿把 omission marker 提升为协议字段。 |
| B4 | apply_patch 审批 action 保留 `PathUri` + `environment_id`；本地路径约定在 automatic approval 中保留 | `b66c25c6a`; `apply_patch.rs`; `approvals.rs` | **client-only** | **不进入 Hub**。 |
| B5 | approval 中心化：permission hook 可在 strict auto-review 下先 Allow/Deny；guardian/user rejection 文案 | `2da1b1282`; `tools/approvals.rs`; `orchestrator.rs`; `request_permissions.rs` comment | **client-only**（Codex 本地权限协议，非 Responses body） | **不进入 Hub**。与 OpenAI Responses MCP `require_approval` / `mcp_approval_*` **不是同一协议**，禁止伪桥。 |
| B6 | exec-server sandbox 传入 workspace roots | `c8dc8e5fd`; `sandboxing.rs` | **client-only** | **不进入 Hub**。 |
| B7 | MCP OAuth refresh 串行、持久化失败 fail-fast | `6962a2eca`; `rmcp-client/src/oauth/**` | **client-only** | **不进入 Hub**。不改变 Responses MCP tool/item schema。 |
| B8 | multi-agent 终端事件增加 `error`/`started_at`；子代理 reasoning summary support 测试 | `protocol.rs` TurnComplete/Aborted; multi_agents tests; subagent_notifications | **client-only**（app event / multi-agent control plane） | **不进入 Hub** 作为 Responses tool gap。 |
| B9 | web-search history 仅适配 `ResponseItemId` | `ext/web-search/src/history.rs` | **client-only** / wire-neutral | **不进入 Hub**。 |
| B10 | tool_search / defer_loading / namespace / apply_patch tool schema / web_search tool schema / MCP tool declaration | tools crate + handlers **无生产 diff** | **no delta** | **本区间不新开 G**。继续使用矩阵既有行：`RSP.TOOL.tool_search`、`CODEX.TOOL.namespace`、`RSP.TOOL.hosted`、Anthropic MCP **G6** 隔离、G8 hosted inventory 证据。这些是**既有残差**，不是 `1f0566d..9e552e9d1` 引入的新 wire。 |
| B11 | `client.rs` 始终发 reasoning / 恒 include encrypted_content / stream_options 条件 | co-located client diff | **wire** but **non-tool** | **G9/G10（= streaming G13/G14）**；tools 范围只记录“非 tool family”，不重复立项。 |

### Tools residual backlog (not created by this delta)

If later implementation work continues after G1–G8, residual tool matrix rows remain fixture/docs work, **not** new Codex-delta G numbers:

- Hosted tools typed options beyond G8 inventory/raw
- tool_search lifecycle (`tool_search_call` / `tool_search_output` / `additional_tools`) deeper fixtures
- Codex namespace / lazy multi-agent tools (P1)
- Responses MCP lifecycle raw preserve fixtures (no Anthropic bridge)
- apply_patch / local_shell / shell same-protocol field-level evidence
- unknown future tool/item variant policy tests

Do **not** reuse historical architecture helper IDs G10–G12 for these.

## Hub impact assessment (tools scope)

1. **Only shared wire gap for tools family in this range**：B1 → **G11 / G15** item-id policy.
2. **Content preserve only**：B3 tool result truncation text.
3. **Do not implement from this delta**：B4–B9 本地审批、沙箱、OAuth、multi-agent 事件。
4. **No new public tool families** from this range：tool_search / MCP / apply_patch / web_search schema 未变。
5. **Do not bridge** Codex approval events 到 OpenAI Responses MCP approval items。

## Caveats / Not Found

- 本范围未完整展开 `codex-api` 序列化层除 client 测试外的每个 endpoint；item id 省略行为以 `client.rs` + suite 断言为准。
- 未发现 `tools[]` 数组元素新增字段（如新的 defer/lazy flag 名称）的 diff。
- 未发现 `tool_choice` 形状变化。
- 未发现 MCP `require_approval` / `mcp_approval_request` 公共 Responses item 形状变化。
- G 编号在本任务三个 research 文件间存在 alias（G9–G11 vs G13–G15）；实现立项前主会话应统一一次编号。
- 本文件只写 research；未改生产代码/测试/任务状态。

## Recommended next actions (for implement agents, not done here)

1. 与 request/stream research 对齐后，若立项 item-id gap：same-protocol fixtures 保留 `msg_`/`fc_`/`fco_`/`tsc_` 等；无 id 的 item 往返仍合法。
2. 文档：在 tools batch / matrix 注记 Codex 出站省略 unprefixed ids 的 usage-profile 行为（不是新 tool type）。
3. 不要因本 delta 重开 MCP OAuth、approval orchestrator 或 multi-agent event 的 Hub 协议工作。
