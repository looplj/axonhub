# G6 Slice Ledger — Anthropic MCP connector

## Workflow
TDD per ~5-minute slice; self-review each slice; real module sub-agent review after all slices.

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| S1 red fixtures | prove current drop of mcp_servers + mcp_toolset | anthropic-mcp-connector.request.json + tests | pass | completed |
| S2 mcp_servers preserve | top-level raw same-protocol replay | MessageRequest.MCPServers + TransformerMetadataKeyMCPServers | pass | completed |
| S3 mcp_toolset preserve | tools[] union raw fragment ordered merge | Tool.Raw + TransformerMetadataKeyRawTools + appendAnthropicRawTools | pass | completed |
| S4 no fake map | Responses/Chat do not invent Anthropic connector | TestAnthropicMCPConnectorNotSynthesizedForResponsesOrChat | pass | completed |
| S5 modern tools regression | function tool coexists; package tools still work | round-trip asserts lookup function + package tests | pass | completed |
| S6 package verification | anthropic/openai green | go test ./transformer/anthropic ./transformer/openai -count=1 | pass | completed |

## Implementation notes
- `mcp_servers`: Anthropic-native json.RawMessage + namespaced TransformerMetadata (like container/inference_geo).
- `tools[].type=mcp_toolset`: Tool.Raw opaque fragment; not common llm.Tool; outbound append after function/web_search conversion.
- No bridge to OpenAI Responses `mcp` tool.

## Module review gate
- review: pending real sub-agent
- report: research/reviews/
- commit: pending
