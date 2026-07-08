# Protocol Baselines

- Regenerated: 2026-07-06
- Scope: local protocol baselines for AxonHub protocol conversion/audit work.
- Source-of-truth for this regenerated set: `docs/specs/vendor/protocol-canonical-2026-07-06/`.
- Important: older generated protocol docs are not source-of-truth for this set. Regenerate from canonical sources when protocol facts are disputed.

## Baseline documents

| Protocol | Baseline document | Canonical source directory | Primary endpoint |
|---|---|---|---|
| OpenAI Responses | `openai-responses-protocol.md` | `docs/specs/vendor/protocol-canonical-2026-07-06/` | `POST /responses` |
| OpenAI Chat Completions | `openai-chat-completions-protocol.md` | `docs/specs/vendor/protocol-canonical-2026-07-06/` | `POST /chat/completions` |
| Anthropic Claude Messages | `anthropic-claude-messages-protocol.md` | `docs/specs/vendor/protocol-canonical-2026-07-06/` | `POST /v1/messages` |

## Canonical files to read first

| Protocol | Files |
|---|---|
| OpenAI Responses | `openai-api-definition.fetch.yaml`, `openai-responses-reference.exa.md`, `openai-responses-create.platform-snapshot.md` |
| OpenAI Chat Completions | `openai-api-definition.fetch.yaml`, `openai-chat-completions-create.developers-snapshot.md`, `openai-chat-completions-create.developers-snapshot.html` |
| Anthropic Claude Messages | `anthropic-messages-api.official-raw.md`, `anthropic-messages-streaming.official-raw.md`, `anthropic-mcp-connector.official-raw.md` |

See `docs/specs/vendor/protocol-canonical-2026-07-06/SOURCES.md` for search evidence, source selection, and failed/insufficient candidate notes.

## Use rules

1. Treat each baseline as a wire-protocol reference, not AxonHub's canonical internal model.
2. Same-protocol round trip should preserve fields/items that the common abstraction does not understand.
3. Cross-protocol conversion must either implement an explicit bridge or emit explicit lossy diagnostics.
4. Do not silently map protocol-specific tool/state features by name similarity alone.
5. Keep Codex-only compatibility fields separate from public OpenAI/Anthropic protocol baselines unless a canonical source confirms them.

## Regeneration notes

This README and the three protocol baseline documents were regenerated from the canonical source set. During regeneration, disputed fields were checked against the canonical sources instead of copied from earlier generated docs.

Examples of source-based corrections:

- Chat Completions keeps deprecated `function_call`, `functions`, `max_tokens`, `seed`, and `user` only because the new canonical create page/API definition still show them as deprecated fields.
- Chat Completions includes current `web_search_options`, `prompt_cache_key`, `prompt_cache_retention`, `reasoning_effort`, and `verbosity` because they appear in the canonical create page/API definition.
- Responses `client_metadata` is not treated as public OpenAI Responses baseline because it is not confirmed by the canonical public OpenAI sources here.
- Anthropic `mcp_servers` is documented as an MCP connector companion/extension field, not as a base field extracted from `messages-api.official-raw.md`.
