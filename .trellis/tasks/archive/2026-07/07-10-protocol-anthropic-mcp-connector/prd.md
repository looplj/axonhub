# PRD — Anthropic MCP Connector

## Goal

实现 Anthropic Messages `mcp_servers` 与 `tools[].type="mcp_toolset"` 的 same-protocol adapter-specific preservation，不将其错误等价为 OpenAI Responses `mcp` tool。

## Required Behavior

1. Anthropic -> Anthropic 保留 `mcp_servers` 与 `mcp_toolset` 原始 JSON / nested fields。
2. connector config、auth/server identity、tool lifecycle 不被压缩成 common `llm.Tool`。
3. Anthropic -> Responses/Chat 和反向不存在已证明 bridge 时 explicit unsupported/lossy。
4. 不破坏既有 Anthropic client tool、server tool、tool_use/tool_result 处理。

## Acceptance Criteria

- `mcp_servers` fixture 与 `mcp_toolset` fixture 各一份，含未知 nested field。
- same-protocol outbound preserve。
- Responses MCP tool 不触发 Anthropic connector synthetic map。
- existing tools/server-tool tests 回归通过。
- targeted Anthropic tests 和 diff check 通过。

