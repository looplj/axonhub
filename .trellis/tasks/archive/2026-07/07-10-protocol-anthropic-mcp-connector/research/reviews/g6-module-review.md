# G6 Module Review: Anthropic MCP connector (`mcp_servers` / `mcp_toolset`)

| Field | Value |
|---|---|
| **结论** | **PASS** |
| **Agent ID** | `g6-module-review` |
| **Commits** | `610a3426` fix(anthropic): preserve MCP connector mcp_servers and mcp_toolset<br>`5c03dc48` fix(anthropic): preserve mcp_toolset original tools index order |
| **Branch** | `codex-transformer-field-fixes` |
| **Repo** | `/Users/asuan/项目/AI/axonhub` |
| **Review date** | 2026-07-12 |
| **Scope** | anthropic inbound/outbound tool+top-level MCP preserve; not OpenAI Responses MCP bridge |

---

## 1. 设计意图对照

| # | 意图 | 判定 | 证据 |
|---|---|---|---|
| 1 | Anthropic same-protocol 保留 `mcp_servers` 与 `tools[].type=mcp_toolset` 原始 JSON / unknown nested | **满足** | `MessageRequest.MCPServers json.RawMessage` + `TransformerMetadataKeyMCPServers`；`Tool.Raw` + `UnmarshalJSON` 在 `type=="mcp_toolset"` 时整包保留；fixture 含 `future_nested`/`configs` 且 round-trip `JSONEq`/`Contains` |
| 2 | 不压缩成 common `llm.Tool`；不 bridge 成 OpenAI Responses `mcp` | **满足** | inbound 对 `len(tool.Raw)>0` 只进 metadata，不调用 `convertToolToLLM`；Responses/Chat outbound 测试断言无 `mcp_servers` / `mcp_toolset` / `"type":"mcp"` |
| 3 | `mcp_toolset` 按原始 `tools[]` index 保序，不能简单 append | **满足**（第二笔修复） | `anthropicRawToolFragment{OriginalIndex,Raw}`；`appendAnthropicRawTools` 按 index 重插；`TestAnthropicMCPToolsetOrderPreservedWhenFirst` |
| 4 | function/web_search 现代工具路径不回归 | **满足**（包级） | function 与 mcp 共存 round-trip；`go test ./transformer/anthropic ./transformer/openai -count=1` 全绿；既有 web_search inbound/outbound 测试仍在包内 |
| 5 | Responses/Chat 不合成 Anthropic connector 字段 | **满足** | `TestAnthropicMCPConnectorNotSynthesizedForResponsesOrChat` |
| 6 | 遵循 container / raw-tool 框架 | **基本满足** | top-level opaque JSON 与 `container`/`inference_geo` 同型；tools 侧 index-addressable raw fragment 与 Responses `OpenAIResponsesRawFragment`/`mergeRawOnlyTools` 同思想，实现更轻（无 signature guard） |

---

## 2. 实现摘要（两笔提交合读）

### 2.1 `610a3426` — 首次保留

- `model.go`：`MCPServers json.RawMessage`；metadata keys `anthropic_mcp_servers` / `anthropic_raw_tools`；`Tool.Raw json.RawMessage \`json:"-"\``。
- `tools.go`：`Tool.UnmarshalJSON` 对 `mcp_toolset` 存整包 Raw；`MarshalJSON` Raw 优先。
- `inbound_convert.go`：`mcp_servers` → metadata；tools 循环里 Raw 工具旁路 common 转换。
- `outbound_convert.go`：`buildBaseRequest` 恢复 `MCPServers`；`appendAnthropicRawTools` **最初为 append-only**（function/web_search 后追加）。
- 测试 + fixtures：`anthropic-mcp-connector.request.json`、`anthropic-mcp-toolset-only.request.json`。

### 2.2 `5c03dc48` — 保序修复

- inbound：`rawTools []anthropicRawToolFragment{OriginalIndex:i, Raw:...}`。
- outbound：`appendAnthropicRawTools` 改为按 `OriginalIndex` 与 common tools 交错合并；兼容读取 `[]anthropicRawToolFragment` / `[]json.RawMessage` / `[]any`。
- 测试：`mcp_toolset` 在前的 order 用例；原 round-trip 期望 index=1。

当前有效路径：

```
inbound tools[]:
  Raw 非空 → anthropicRawToolFragment{OriginalIndex, Raw} → TransformerMetadata[anthropic_raw_tools]
  否则 convertToolToLLM (function / web_search)

outbound:
  convertToolsAnthropic(chatReq.Tools) → common []Tool
  appendAnthropicRawTools(common, chatReq) → 按 OriginalIndex 重插 Raw
  mcp_servers ← asJSONRawMessage(metadata[anthropic_mcp_servers])
```

---

## 3. 审查轴

### 3.1 协议正确性 — PASS

- same-protocol 下 `mcp_servers` 不丢、unknown nested 不剥。
- `mcp_toolset` 不进 `llm.Tool`，outbound 用 Raw 原文发射。
- 不把 Anthropic connector 伪装成 Responses `type=mcp`。
- 工具顺序：function 在前 / mcp 在前 两种 fixture 均覆盖。

### 3.2 代码 bug — 无 blocker

| 级别 | 项 | 说明 |
|---|---|---|
| **minor** | legacy `[]json.RawMessage` 注释与行为不一致 | 注释写 “unindexed fragments append after common tools”，实现却 `OriginalIndex: i`（从 0 起），会把 fragment 插到前面。当前 inbound 只写 `[]anthropicRawToolFragment`，生产路径不依赖该 legacy 分支。 |
| **minor** | 中间 common 工具被 drop 时的绝对 index | 若某非-raw tool 被 `convertToolToLLM`/`supportsAnthropicNativeTools` 过滤，common 槽位变少，合并会跳过空洞、压缩相对顺序；same-protocol 直连 Anthropic 下通常不触发。比 Responses `mergeRawOnlyTools` 宽松（无 signature 校验）。 |
| **minor** | `append` 时硬编码 `Type: "mcp_toolset"` | 实际发射走 `MarshalJSON`→Raw，Type 字段仅辅助；若未来扩展其它 Raw 类型，应改读 Raw 内 type 或只设 Raw。 |

未发现：mcp 被压成 function、auth token 被结构化剥落、Cross-protocol 合成 `mcp_servers` 等协议级错误。

### 3.3 架构 / 屎山 — 可接受

- 与既有 Anthropic opaque top-level（`container` / `inference_geo` / `context_management`）一致。
- tools 侧 index fragment 合理；`anthropicRawToolFragment` 放在 `outbound_convert.go` 稍怪（inbound 也依赖），但包内可见、无循环依赖。
- 未污染 `llm.Request` 公共字段；metadata key 命名空间 `anthropic_*` 正确。
- 未引入与 Responses MCP 的错误等价映射。

### 3.4 测试充分性 / false green

| 覆盖 | 状态 |
|---|---|
| same-protocol mcp_servers + mcp_toolset + unknown nested | 有 |
| toolset-only（无 function） | 有 |
| mcp 在 tools[0] 保序 | 有 |
| function 仍 common 化 | 有 |
| Responses/Chat 不合成 connector | 有 |
| 多枚 `mcp_toolset` 交错 + 中间 drop | **无**（探针手动验证通过，未入库） |
| web_search + mcp_toolset 混排 | **无**（包级 web_search 回归有，本 slice 无组合） |
| metadata JSON 再水合 | **无**（临时探针：`[]any` map 与 `mcp_servers` 再水合可工作） |

**false-green 点（minor）**  
`TestAnthropicMCPConnectorSameProtocolRoundTrip` 中：

```go
_, hasMCPServersField := any(llmReq).(interface{ GetMCPServers() })
_ = hasMCPServersField
```

未断言任何结果，对 “不 widen llm.Request” **零约束**。真实保障来自：未给 `llm.Request` 加字段 + metadata key 使用 + Chat/Responses 不读该 key。

---

## 4. Findings 汇总

### Blocker
无。

### Major
无。

### Minor

1. **Legacy raw-tools 分支注释错误 / 行为非 append-after**  
   文件：`llm/transformer/anthropic/outbound_convert.go` → `anthropicRawToolFragments`  
   建议：要么改实现为 `OriginalIndex: len(commonTools)+i`（真 append-after），要么改注释为 “treat as wire index starting at 0”；并加单测钉死选择。

2. **“不 widen llm.Request” 断言是死代码**  
   文件：`mcp_connector_test.go`  
   建议：删除假断言；或 `require.NotContains` 对 marshaled `llm.Request` JSON 的 top-level keys / 明确 metadata-only。

3. **缺组合/交错回归**  
   建议补：  
   - tools = `[mcp, function, mcp]`  
   - tools = `[function, web_search, mcp]`（Direct platform）  
   - 可选：metadata 经 `json.Marshal/Unmarshal` 后再 outbound。

4. **与 Responses raw-merge 严格度不对齐（文档级）**  
   Anthropic 合并允许 common 数量与“空洞”不匹配时静默压缩；若未来依赖 index 语义做更强保真，可考虑 signature/count 校验或失败降级策略（非本 slice 必须）。

---

## 5. 已验证证据

### 5.1 代码（当前 HEAD 合读两 commit）

- `llm/transformer/anthropic/model.go`：`MCPServers`、`TransformerMetadataKeyMCPServers`、`TransformerMetadataKeyRawTools`、`Tool.Raw`
- `llm/transformer/anthropic/tools.go`：`UnmarshalJSON`/`MarshalJSON` 对 `mcp_toolset`
- `llm/transformer/anthropic/inbound_convert.go`：metadata 写入 + index fragment 收集
- `llm/transformer/anthropic/outbound_convert.go`：`buildBaseRequest` 恢复 mcp_servers；`appendAnthropicRawTools` index merge
- fixtures：`testdata/anthropic-mcp-connector.request.json`、`anthropic-mcp-toolset-only.request.json`
- task docs：`prd.md` / `design.md` / `research/g6-slice-ledger.md` 与实现一致（ledger 中 S3 早期 “append after” 已被 `5c03dc48` 纠正）

### 5.2 测试命令与结果

```bash
cd /Users/asuan/项目/AI/axonhub/llm
go test ./transformer/anthropic -count=1 -run TestAnthropicMCP
# ok  github.com/looplj/axonhub/llm/transformer/anthropic

go test ./transformer/anthropic ./transformer/openai -count=1
# ok  github.com/looplj/axonhub/llm/transformer/anthropic
# ok  github.com/looplj/axonhub/llm/transformer/openai
```

临时探针（已删除文件，不入库）验证：

- 多 raw 交错 index 0/2 + 两 common → `[mcp,a,mcp,b]`
- raw at OriginalIndex=2 + 一 common → `[lookup,mcp]`（空洞不插入空位）
- metadata JSON 再水合后 order 与 `mcp_servers` 仍可恢复

### 5.3 Graph / 索引

- 已 incremental index project `Users-asuan-AI-axonhub-llm`
- 命中：`appendAnthropicRawTools`、`convertToolsAnthropic`、`Tool.UnmarshalJSON`、四个 `TestAnthropicMCP*`

---

## 6. 若不通过时的修复建议

本审查 **PASS**，无需强制修复。若消化 minor：

1. 修正 `anthropicRawToolFragments` 对 `[]json.RawMessage` 的注释或实现，并加 1 个单元测试。  
2. 去掉/改写 dead “GetMCPServers” 断言。  
3. 增加多 toolset + web_search 混排 round-trip。  
4. （可选）当 `convertToolsAnthropic` 因 platform 丢弃 native tool 时，记录诊断或按相对顺序重编号，避免 silent 压缩——仅在有真实路由场景时再做。

---

## 7. 结论

两笔提交合起来完成了 G6 设计意图：Anthropic same-protocol 对 `mcp_servers` / `mcp_toolset` 做 **opaque preserve + original index 保序**，不进入 common `llm.Tool`，不 bridge Responses MCP，function 路径与 openai/anthropic 包测试保持绿色。

**结论：PASS**（0 blocker / 0 major / 4 minor 改进项，均不阻塞合入本 slice）。
