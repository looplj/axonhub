# OpenAI Codex source snapshot

Source repository: https://github.com/openai/codex
Snapshot commit: `be33f80bc65159c094ecd06bf155afa3061ce23d`

This directory stores only the Codex source files needed to verify the Responses wire shape used by Codex clients. It is a local evidence snapshot for AxonHub protocol compatibility work.

Selected files:

- `codex-rs/codex-api/src/common.rs` — `ResponsesApiRequest` wire request struct.
- `codex-rs/core/src/client.rs` — Codex request builder for `/v1/responses`.
- `codex-rs/tools/src/tool_spec.rs` — Codex `ToolSpec` variants serialized into `tools`.
- `codex-rs/tools/src/responses_api.rs` — function / namespace / freeform tool structs.
- `codex-rs/tools/src/tool_search.rs` — deferred tool search metadata handling.
- `codex-rs/tools/src/tool_discovery.rs` — `tool_search` name/default constants.
- `codex-rs/protocol/src/models.rs` — `ResponseItem` wire item enum including function/tool-search/custom outputs.
- `codex-rs/core/src/mcp_tool_exposure.rs` — direct vs deferred MCP exposure decision.
- `codex-rs/core/src/tool_search_spec.rs` — Codex client-executed `tool_search` spec.
- `codex-rs/core/src/tool_search_handler.rs` — Codex handler payload expectations.
