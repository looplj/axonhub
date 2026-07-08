# Module 6 Slice 2: Anthropic MCP connector preservation

日期：2026-07-07
工作树：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`

## 范围

Anthropic MCP connector companion 字段：

- `mcp_servers`
- `tools[].type = "mcp_toolset"`
- `mcp_server_name`
- mcp toolset 上的 companion/raw 配置字段，例如 `allowed_tools`

不包含本轮未处理字段：

- response `container`
- response `stop_details`
- stream event fidelity
- 跨协议 OpenAI Responses MCP <-> Anthropic MCP connector 映射

## TDD seam

公开边界：

- `anthropic.InboundTransformer.TransformRequest`
- `anthropic.OutboundTransformer.TransformRequest`

新增/扩展测试：

- `TestAnthropicSameProtocolRequestPreservesMCPConnectorFields`
- 扩展 `TestAnthropicRequestNativeTopLevelFieldsAreDirectOnly`，覆盖 mcp 字段不泄露到 Anthropic-format adapter。

红测表现：

- `mcp_servers` 在出站 body 中缺失。
- mixed `tools` 中的 `mcp_toolset` 被 `Tool` struct 丢掉未知字段，只剩 `type` / 空 `name`。

## 修复

- inbound 从原始 JSON body 捕获 Anthropic raw top-level fields：`container`、`inference_geo`、`mcp_servers`、`tools`。
- `ProviderExtensions.Anthropic.Request.RawTopLevelFields` 作为 Anthropic native/companion 字段 owner。
- `MessageRequest.MCPServers` 保存 raw `mcp_servers`。
- `Tool.Raw` + `Tool.MarshalJSON` 支持 raw tool element replay。
- outbound direct Anthropic same-protocol：
  - replay `mcp_servers`；
  - structured function tools 保持 common 输出；
  - raw `mcp_toolset` 元素补回 `tools` 数组。
- Anthropic-format adapter（测试用 DeepSeek）不 replay `mcp_servers` / `mcp_toolset`。

## 验证

已执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm

go test ./transformer/anthropic -run 'TestAnthropicSameProtocolRequestPreservesNativeTopLevelFields|TestAnthropicRequestNativeTopLevelFieldsAreDirectOnly|TestAnthropicSameProtocolRequestPreservesMCPConnectorFields' -count=1

go test ./transformer/anthropic -count=1

git diff --check
```

结果：全部通过。

索引：

```bash
codebase-memory-mcp cli --json index_repository '{"repo_path":"/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean","mode":"fast"}'
```

结果：状态 `ready`，nodes 39780，edges 203053。

## 自审结论

通过本切片自审：

- 未把 Anthropic MCP connector 映射成 OpenAI Responses MCP。
- 未扩大 common `llm.Request`。
- 未新增 `TransformerMetadata` magic key。
- same-protocol Direct Anthropic 保真。
- Anthropic-format adapter 负向保护存在。
- `mcp_toolset` 使用 raw element replay，避免丢 `mcp_server_name` / companion 配置字段。
- 当前仍未进入 Module 6 大审查/提交；后续还需 Anthropic response native 字段切片。
