# Design — Anthropic MCP Connector

## Classification

```text
mcp_servers / mcp_toolset = Anthropic adapter-specific native/raw connector config
Responses mcp tool = OpenAI Responses adapter-specific native/raw tool
```

名字相同不构成 common abstraction。

## Seam

Anthropic `MessageRequest` top-level raw fields与 `Tool` raw/native variant preservation。若 `mcp_toolset` 位于 `tools[]` union，必须保留 discriminator 和原始 nested JSON。

## Tests

1. `mcp_servers` same-protocol round-trip。
2. `tools[].type=mcp_toolset` same-protocol round-trip。
3. unknown nested config survives。
4. no bridge between OpenAI Responses MCP and Anthropic connector。

