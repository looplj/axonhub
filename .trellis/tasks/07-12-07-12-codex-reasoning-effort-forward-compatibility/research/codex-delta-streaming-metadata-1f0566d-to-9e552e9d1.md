# Research: Codex delta — streaming / metadata / response item IDs (`1f0566d..9e552e9d1`)

- Query: 对比本地 Codex 源码 `1f0566d..9e552e9d1`，审计 **流式 Responses events、response item IDs、client metadata/include、reasoning summary、WebSocket/HTTP 请求参数与 response parsing** 的协议相关变更；区分后端 wire 变化、兼容性修正、纯 telemetry/UI。
- Scope: mixed（Codex 源码 diff + 任务 PRD/design + protocol guideline 对照；Hub 仅做相关性归类，不改代码）
- Date: 2026-07-12
- Range:
  - Base: `1f0566d3f59298d1bb88820a0d35294f1eeb07ea`
  - Head: `9e552e9d15ba52bed7077d5357f3e18e330f8f38`
  - Repo: `/Users/asuan/项目/ai工具/openai-codex`
- Research scope ID: **C**（与其他 agent 范围不重叠）

## Method

- 只读 `git log` / `git diff` / `git show` 与 HEAD 文件内容。
- 重点文件（任务指定）：
  - `codex-rs/core/src/client.rs`
  - `codex-rs/core/src/event_mapping.rs`
  - `codex-rs/codex-api/src/sse/responses.rs`
  - `codex-rs/protocol/src/response_item_id.rs`
  - 对应 tests（`core/tests/suite/client.rs`、`client_websockets.rs`、`protocol/src/response_item_id_tests.rs` 等）
- 扩展到同范围的直接关联文件：`codex-api/src/endpoint/responses_websocket.rs`、`protocol/src/models.rs`、`protocol/src/openai_models.rs`、`stream_events_utils.rs`。
- 不猜测：下列“未变更”项均有 diff 字节数或 log 为空证据。

## Related commits in range (protocol-relevant)

| Commit | Title | Primary impact |
|---|---|---|
| `d2d00b663` | Always send reasoning parameters in Responses requests (#32206) | HTTP/WS 请求体：`reasoning` 恒发；`include` 恒含 `reasoning.encrypted_content`；删除 `supports_reasoning_summaries` 门控 |
| `dffe1f02a` | Respect model support for reasoning summaries (#32290) | 模型元数据新字段；条件省略 `reasoning.summary` 与 summary-delivery `stream_options` |
| `c9d52de5c` | Require prefixes for outbound response item IDs (#32312) | 新 `ResponseItemId`；出站剥离空/无前缀 item id；HTTP+WS 请求 input 受影响 |
| `9993fb838` | Improve Responses WebSocket timing telemetry (#32256) | 仅本地 trace/telemetry，不改请求/事件 wire 语义 |

非本范围主线但触及测试夹具的提交（不作为协议发现）：`5c19155cb`（pagination ordinals 改 client tests 夹具）。

## Files found

| Path | One-line description |
|---|---|
| `codex-rs/core/src/client.rs` | Responses 请求构造：`build_reasoning` / `include` / `stream_options` / `prepare_response_items_for_request` |
| `codex-rs/core/src/event_mapping.rs` | ResponseItem → TurnItem 本地映射；本范围仅适配 `ResponseItemId` |
| `codex-rs/core/src/stream_events_utils.rs` | 非工具 output item 处理日志；本范围仅 `item_id` 日志适配 |
| `codex-rs/codex-api/src/sse/responses.rs` | SSE Responses event 解析；**本范围 0 字节 diff** |
| `codex-rs/codex-api/src/endpoint/responses_websocket.rs` | WS 传输 + timing telemetry + 测试夹具 ID 前缀 |
| `codex-rs/protocol/src/response_item_id.rs` | 新增 item ID 类型（前缀 + UUIDv7 / 宽松反序列化） |
| `codex-rs/protocol/src/response_item_id_tests.rs` | ID 生成/前缀识别/legacy 反序列化测试 |
| `codex-rs/protocol/src/models.rs` | `ResponseItem.id: Option<ResponseItemId>` + `id_prefix()` |
| `codex-rs/protocol/src/openai_models.rs` | 删除 `supports_reasoning_summaries`；新增 `supports_reasoning_summary_parameter` |
| `codex-rs/core/tests/suite/client.rs` | HTTP 请求体断言：summary omit、include 恒在、prefixed ids |
| `codex-rs/core/tests/suite/client_websockets.rs` | WS 路径同样改用 prefixed IDs；telemetry 相关微调 |

## Findings

### F1. SSE / stream event parsing: **无变更**

- **变更**: `codex-rs/codex-api/src/sse/responses.rs` 在 `1f0566d..9e552e9d1` 中 blob 相同（`git diff` 输出 0 字节）；`codex-rs/codex-api/src/sse/**` 无提交。
- **证据**:
  - `git ls-tree` 两端 blob = `70f96cb855005d577c57fd768062d035cc919b12`
  - `git log 1f0566d..9e552e9d1 -- codex-rs/codex-api/src/sse/**` 空
- **wire / client-only**: 无
- **归类**: **不属于 Hub**（无 delta 可跟进）；也 **不是** G9–G15 新缺口。
- **含义**: 本范围**不存在**新的 Responses 流式事件类型、字段解析或 SSE 映射变化。WebSocket 仍复用 `ResponsesStreamEvent` 反序列化路径（`responses_websocket.rs` 引用 `crate::sse::ResponsesStreamEvent`），但解析器本身未改。

---

### F2. Always send `reasoning` + always `include: ["reasoning.encrypted_content"]` — **wire 请求策略变化**

- **变更** (`d2d00b663`):
  1. `ModelClient::build_reasoning` 从 `Option<Reasoning>` 改为恒返回 `Reasoning`；删除 `if model_info.supports_reasoning_summaries { ... } else { None }` 门控。
  2. 请求构造改为：
     - `reasoning: Some(Self::build_reasoning(...))`（恒有）
     - `include = vec!["reasoning.encrypted_content".to_string()]`（恒有，不再依赖 `reasoning.is_some()`）
  3. 从 `ModelInfo` / config 删除 `supports_reasoning_summaries` 与 `model_supports_reasoning_summaries` 覆盖项。
- **证据**:
  - commit message: “Build a reasoning payload for every Responses request and always include `reasoning.encrypted_content`.”
  - HEAD `client.rs:803-820` `build_reasoning` 恒返回 `Reasoning { effort, summary, context }`
  - HEAD `client.rs:864-901`：`let include = vec!["reasoning.encrypted_content".to_string()];`；`reasoning: Some(reasoning)`
  - Base `client.rs`（`1f0566d`）仍为 `Option` + 条件 `include`
  - 测试（经 `dffe` 保留）断言 `include == ["reasoning.encrypted_content"]` 即使 summary 被省略：`client.rs` `model_without_summary_parameter_support_omits_configured_summary`（约 2494–2499 行）
- **wire / client-only**: **后端 wire 请求体形状变化（由 Codex 客户端发出）**。不是服务端 SSE 事件变化；是 Codex 作为 Responses 客户端的出站合同变化。
- **归类**: **新 G（建议 G13）— Always-on Responses `reasoning` + `include[reasoning.encrypted_content]`**
  - 与既有 protocol draft 行相关：`docs/specs/protocols/drafts/batch-reasoning-stream.md` 已记录 `include:["reasoning.encrypted_content"]` same-protocol replay，但**本 delta 证明 Codex 现在几乎总是发送该 include**，不再受 `supports_reasoning_summaries` 开关约束。
  - Hub 含义（相关性，非实现结论）：同协议路径应继续原样保留客户端发来的 `include` / `reasoning`；不要用旧的“仅在 supports_reasoning 时才有 include”假设过滤字段。
- **非结论**: 本条**不**证明 OpenAI 服务端强制要求所有模型都接受 `reasoning` 或 encrypted include；仅证明 Codex 客户端策略变为恒发。

---

### F3. Conditional `reasoning.summary` + summary-delivery `stream_options` — **兼容性修正（模型能力门控）**

- **变更** (`dffe1f02a`，建立在 F2 之上):
  1. `ModelInfo.supports_reasoning_summary_parameter: bool`，`#[serde(default = "default_true")]`，默认 `true`（向后兼容）。
  2. `build_reasoning` 中：
     ```text
     summary: (model_info.supports_reasoning_summary_parameter
               && summary != ReasoningSummaryConfig::None)
              .then_some(summary)
     ```
  3. `stream_options` 仅当：
     - `concurrent_reasoning_summaries_enabled`
     - provider is OpenAI
     - **且** `reasoning.summary.is_some()`
     才发送 `StreamOptions { reasoning_summary_delivery: SequentialCutoff }`。
- **证据**:
  - HEAD `client.rs:812-814` summary 门控
  - HEAD `client.rs:865-870` stream_options 依赖 `reasoning.summary.is_some()`
  - `openai_models.rs` 字段注释: “Whether the model accepts the Responses API `reasoning.summary` parameter.”
  - 测试 `model_without_summary_parameter_support_omits_configured_summary`：
    - 配置 `effort=high` + `summary=detailed` + ConcurrentReasoningSummaries
    - 模型 `supports_reasoning_summary_parameter = false`
    - 断言 body：`reasoning == {"effort":"high"}`（**无 summary**）
    - 断言 `include == ["reasoning.encrypted_content"]` 仍在
    - 断言 `stream_options` 为 `None`
- **wire / client-only**: **条件性 wire 字段省略**（客户端按模型目录决定是否发 `reasoning.summary` / summary stream option）。模型元数据字段本身是 Codex catalog 客户端数据，不是 Responses 公共请求 schema 的一部分。
- **归类**: **新 G（建议 G14）— Model-gated `reasoning.summary` / `stream_options.reasoning_summary_delivery`**
  - 与既有 G9（`stream_options` raw nested preservation）**相关但不等同**：G9 是 Hub 对 raw nested stream_options 保真；G14 是上游 Codex 何时生成 summary 相关 stream_options。
  - Hub 含义：网关应保真客户端实际发送的 summary/stream_options 有无；**不要**根据 Hub 自己的模型表擅自补/删 `reasoning.summary`，除非产品明确要求模拟 Codex catalog 行为（当前任务非目标）。
- **与 F2 的合成 HEAD 行为**:
  | 字段 | HEAD 行为 |
  |---|---|
  | `reasoning` 对象 | 恒发送 |
  | `reasoning.effort` | 配置或模型默认；`ultra→max` 仍在 `reasoning_effort_for_request` |
  | `reasoning.summary` | 模型支持且配置非 `None` 时发送 |
  | `include` | 恒 `["reasoning.encrypted_content"]` |
  | `stream_options.reasoning_summary_delivery` | OpenAI + feature + summary 实际发出时 |

---

### F4. Prefixed response item IDs + outbound strip of empty/unprefixed IDs — **wire 兼容性修正**

- **变更** (`c9d52de5c`):
  1. 新增 `ResponseItemId`（`protocol/src/response_item_id.rs`）：
     - 生成：`{prefix}_{uuid_v7}` 或 `with_suffix(prefix, suffix)`
     - `from_server` / 反序列化：**宽松**，接受任意字符串（legacy rollout）
     - `is_prefixed()`：存在非空 prefix 与非空 suffix（按第一个 `_` 分割）
  2. 所有 `ResponseItem` 变体的 `id: Option<String>` → `Option<ResponseItemId>`；新增 `id_prefix()`：
     - `msg` / `amsg` / `rs` / `fc` / `fco` / `lsh` / `ctc` / `ctco` / `ws` / `ig` / `tsc` / `tso` / `at` / `cmp` 等
  3. `prepare_response_items_for_request` **先**剥离无前缀 ID：
     ```rust
     for item in input.iter_mut() {
         if item.id().is_some_and(|id| !id.is_prefixed()) {
             item.set_id(None);
         }
     }
     // 然后若 !item_ids_enabled && !store，再清空全部 id
     ```
  4. Base 的 `id()` 会 `filter(|id| !id.is_empty())`；HEAD 的 `id()` 直接返回 `Option<&ResponseItemId>`，**空 id 的出站剥离**改由 `is_prefixed()` + `set_id(None)` + serde `skip_serializing_if = Option::is_none` 完成。测试显式加入 empty-id 与 unprefixed 项，断言出站 body **省略** `id` 字段。
- **证据**:
  - 新文件 `response_item_id.rs` 全文与 `response_item_id_tests.rs`
  - HEAD `client.rs:910-924` `prepare_response_items_for_request`
  - 测试更名：`azure_responses_request_includes_store_and_prefixed_item_ids`
  - 断言样例：`rs_reasoning-id` / `msg_message-id` / `ws_web-search-id` / `fc_function-id` / …；empty/unprefixed 对应 `get("id") == None`
  - WS 测试夹具从 `msg-1` 改为 `msg_1` 或 `ResponseItemId::with_suffix("msg", ...)`（`client_websockets.rs`）
  - 本地 ID 生成点：`session/mod.rs` 用 `ResponseItemId::new(prefix)`（客户端生成，非 SSE 解析）
- **wire / client-only**:
  - **Wire（出站）**: 发往 Responses HTTP/WS 的 `input[].id` 仅保留“有前缀”ID，否则字段省略。
  - **Client-only（入站/历史）**: 反序列化仍接受 legacy 无前缀 ID；`event_mapping` / `stream_events_utils` 仅类型适配，不改变事件语义。
- **归类**: **新 G（建议 G15）— Outbound Responses item ID prefix contract**
  - **不是** G9（stream_options raw nested）。
  - Hub 相关性：
    - 若 Hub 透传客户端 input item `id`：应原样保留（包括已有前缀）。
    - 若 Hub **自造** 或 **规范化** item id（例如 history rewrite、store 回放）：需意识到 Codex/上游可能拒绝或忽略无前缀 ID；本 delta 是 **客户端侧** 的防御性过滤，不是 OpenAI 官方公开 schema 全文证据。
    - 当前任务主线（reasoning effort 前向兼容）**不要求**实现 G15，但审计矩阵若覆盖 item id，应单独成行。

---

### F5. `event_mapping.rs` / `stream_events_utils.rs` — **纯客户端适配，无 wire 语义变化**

- **变更**:
  - `event_mapping.rs`: `Option<&String>` → `Option<&str>` / `id.as_deref()`，因 `ResponseItemId` 实现 `Deref<Target=str>`。
  - `stream_events_utils.rs`: debug log 从 `item.id()` 改为 `item.id().map(ResponseItemId::as_str)`。
- **证据**: `git show c9d52de5c` 对应 hunks；无新增事件分支、无字段映射变化。
- **wire / client-only**: **client-only**
- **归类**: **不属于 Hub**

---

### F6. WebSocket timing telemetry — **纯 telemetry**

- **变更** (`9993fb838`):
  - 识别 stream event kind `responsesapi.websocket_timing`
  - 用 TRACE 目标 `codex_api::responses_websocket_timing` 记录 model/session/thread/turn/traceparent/`previous_response_id`/request_start_ms/warmup/connection_reused + payload
  - 注释明确：默认排除 always-on sinks；需 `RUST_LOG=...=trace` 才启用
  - 从已有 `client_metadata` **读取** key（`session_id`/`thread_id`/`turn_id`/traceparent/`x-codex-ws-stream-request-start-ms`），**未**新增这些 key 的发送逻辑
- **证据**: `responses_websocket.rs` 新增 `emit_responses_websocket_timing_event`；commit message “excluding them from diagnostic uploads and persisted logs by default”
- **wire / client-only**: **client-only telemetry**；不改变 create 请求体 schema，不改变业务 stream 事件处理结果
- **归类**: **不属于 Hub**

---

### F7. client_metadata / include / HTTP headers — **本范围无新 metadata key 协议**

- **变更**:
  - `include` 行为见 F2（恒含 encrypted_content）
  - `client_metadata` 构造路径在 `client.rs` 仍调用 `responses_metadata.client_metadata()` / `build_ws_client_metadata`；本范围 **无** `responses_metadata*` 或 `codex-api` common 请求结构的协议字段 diff
  - F6 仅**消费**已有 metadata key 做日志
- **证据**: `git diff --name-only 1f0566d..9e552e9d1 -- 'codex-rs/core/src/responses_metadata*'` 空；`codex-api` 除 websocket/files/models/search 外无 request schema 文件改动
- **归类**: include 恒发 → **G13**；其余 metadata → **不属于 Hub**（无新协议）

---

### F8. Response parsing beyond ID type — **无业务解析变化**

- SSE parser 未变（F1）
- `ResponseItem` serde 仍是透明字符串 ID；宽松反序列化保持 legacy 可读
- 测试：`"id":""` 的 item 可反序列化，但出站前被剥离（F4）
- **归类**: 解析宽松性 → **兼容性修正（client history）**；出站过滤 → **G15**；**无新 stream event 解析 G**

## Code patterns (file:line at HEAD `9e552e9d1`)

| Pattern | Location |
|---|---|
| `ultra → max` request normalize | `codex-rs/core/src/client.rs:174-179` |
| Always-build reasoning | `codex-rs/core/src/client.rs:803-820` |
| summary gated by `supports_reasoning_summary_parameter` | `codex-rs/core/src/client.rs:812-814` |
| stream_options only if summary present + OpenAI + feature | `codex-rs/core/src/client.rs:865-870` |
| include always encrypted_content | `codex-rs/core/src/client.rs:871` |
| reasoning always `Some` in request | `codex-rs/core/src/client.rs:897` |
| strip unprefixed item IDs before send | `codex-rs/core/src/client.rs:910-924` |
| `ResponseItemId` type | `codex-rs/protocol/src/response_item_id.rs:1-70` |
| `id_prefix()` map | `codex-rs/protocol/src/models.rs:1216-1233` |
| Model catalog flag | `codex-rs/protocol/src/openai_models.rs` (`supports_reasoning_summary_parameter`) |

## Gap classification map (G9–G15 / 新 G / 不属于 Hub)

> 说明：既有审计切片 G9–G12 属于上一任务（stream_options raw nested / raw JSON helper / lossy recorder / raw capture）。本 scope C **不复用其实现缝**，但在归类时标注关系。

| ID | 主题 | 本 delta 证据 | wire? | Hub 行动建议 |
|---|---|---|---|---|
| G9（既有） | stream_options raw nested preservation | 无直接代码变更；F3 改变 Codex **何时生成** summary-delivery option | 间接 | 保持 G9 保真语义；勿把 Codex feature flag 逻辑搬进 Hub |
| G10–G12（既有） | raw/lossy helper seams | 本范围未触及 | 否 | 无 |
| **G13（新）** | Always-on `reasoning` + `include: reasoning.encrypted_content` | F2 `d2d00b663` | **是（请求）** | same-protocol 保真 include/reasoning；更新矩阵“Codex 客户端常见请求形态” |
| **G14（新）** | Model-gated `reasoning.summary` / summary stream_options | F3 `dffe1f02a` | **是（条件请求字段）** | 保真有/无；不伪造 summary；catalog 能力判断属 Codex 客户端 |
| **G15（新）** | Prefixed outbound item IDs / strip unprefixed | F4 `c9d52de5c` | **是（请求 input[].id）** | 透传时勿丢前缀；自造 id 需知前缀约定；非本任务 effort 主线 |
| 不属于 Hub | SSE parser 无变更 | F1 | 否 | 无 |
| 不属于 Hub | event_mapping / stream log 类型适配 | F5 | 否 | 无 |
| 不属于 Hub | WS timing telemetry | F6 | 否 | 无 |
| 不属于 Hub | client_metadata 无新协议 key | F7 | 否 | 无 |

## External / related specs

| Ref | Use |
|---|---|
| `.trellis/spec/backend/protocol-transformer-guidelines.md` | same-protocol raw-preserve 优先；stream option ≠ stream event；Codex usage-profile 标 P1 |
| `docs/specs/protocols/drafts/batch-reasoning-stream.md` | `include` / `reasoning.summary` / stream_options / encrypted content 行 |
| `docs/specs/protocols/openai-responses-protocol.md` | Responses 官方基准（本文件未重审全文，仅作路由） |
| `.agent/research/codex-reasoning-effort-latest-2026-07-12.md` | effort 值域与 `ultra→max`（effort 主线；本 scope C 不重复展开） |

## Caveats / Not Found

1. **`codex-rs/codex-api/src/sse/responses.rs` 在本范围内零变更** — 不能声称有新的流式事件解析修复。
2. 未在本范围发现 **新的** Responses stream event type、新的 `ResponseEvent` 变体、或新的 HTTP header 作为业务协议要求。
3. `ResponseItemId` 前缀约定来自 **Codex 客户端源码**；本文件**不**声称其已写入 OpenAI 公开 API 文档。
4. `supports_reasoning_summary_parameter` 默认 `true` 是客户端 catalog 兼容策略，不是服务端 capability discovery 协议。
5. WebSocket 与 HTTP 共享同一 `build_responses_request` / `prepare_response_items_for_request` 策略（F2–F4）；WS 独有变化在本范围主要是 telemetry（F6）与测试夹具。
6. 未审计 `files.rs` 大 diff（blob upload diagnostics）— 超出本 scope C 的 streaming/metadata/item-id 焦点。
7. 未运行 Codex 测试；结论仅基于源码与已存在的测试断言文本。
8. 本文件不修改生产代码；G13–G15 仅为研究归类，是否进入实现切片由主会话决定。

## Summary for implementers

Scope C 在 `1f0566d..9e552e9d1` 的**可证实协议相关 delta**只有三类：

1. **G13**：Codex 恒发 `reasoning` + `include=["reasoning.encrypted_content"]`。
2. **G14**：按模型元数据条件省略 `reasoning.summary` 及对应 `stream_options`。
3. **G15**：出站 item `id` 必须带前缀，否则省略；入站/历史仍宽松。

**流式事件解析无变化。** telemetry / TurnItem 映射适配 **不属于 Hub**。
