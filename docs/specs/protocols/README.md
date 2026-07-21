# Protocol Baselines

- Regenerated: 2026-07-06 (wire facts)
- Doc authority refreshed: 2026-07-22
- Scope: local protocol baselines for AxonHub protocol conversion/audit work.
- Source-of-truth for this regenerated set: `docs/specs/vendor/protocol-canonical-2026-07-06/`.
- Important: older generated protocol docs are not source-of-truth for this set. Regenerate from canonical sources when protocol facts are disputed.

## Document authority (read this first)

| Priority | Document | Role | Trust for "done"? |
|---|---|---|---|
| **P0 wire** | This directory's three `*-protocol.md` baselines + `vendor/protocol-canonical-2026-07-06/` | Official field existence and meaning | Never claims Hub completion |
| **P0 architecture** | `docs/adr/0001-*.md`, `docs/adr/0002-*.md`, `.trellis/spec/backend/protocol-transformer-guidelines.md` | How Hub owns and converts fields | Design rules, not completion |
| **P1 audit ledger** | `protocol-conversion-strict-verification-matrix.md` | Field-ID status (`CONFIRMED` / `PARTIAL` / …) | Only rows marked `CONFIRMED` under §1.2 |
| **P2 navigation** | `hub-protocol-field-matrix.md`, `drafts/batch-*.md`, `.agent/summary/*` | Index / history | Must not override P0/P1 |
| **P3 implementation** | Current `llm/**` + targeted tests | Actual preserve/bridge/diagnose behavior | Highest for "what Hub does now" |

**Completion rule:** cross-protocol conversion is never "perfect." Same-protocol near-lossless is the only mode that can approach lossless. Any `UNCHECKED` / `PARTIAL` row in the strict matrix means "not finished," not "protocol docs are wrong."

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

## Field classes and conversion modes (author architecture)

Stable vocabulary (must match `.trellis/spec/backend/protocol-transformer-guidelines.md`):

| Class | Owner in author architecture | Conversion mode |
|---|---|---|
| `common-abstraction` | `llm.Request` / `llm.Response` / `Message` / `Tool` | `direct` / `rename` / `structural_transform` / `value_map` when target has equivalent semantics |
| `native-field` | Protocol package request/response structs | Same-protocol typed preserve |
| `adapter-specific` | Provider/channel extension | Not in common model |
| `raw-preserve` | Named sidecar / `RawRequest` fragments | Same-protocol re-emit only |
| `lossy-conversion` | `ProviderExtensions.Diagnostics.LossyDowngrades` | Cross-protocol drop **with** diagnostic (never silent) |
| `unsupported/absent` | — | No fake map |
| `deprecated-compat` | Separate path from modern fields | Preserve identity; explicit precedence tests |

**Near-lossless** applies only to **same-protocol** routes (Responses→Responses, Chat→Chat, Anthropic→Anthropic): typed native + raw sidecar + independent item identities. Cross-protocol always has a residual loss set (see drop types below).

### Drop / diagnose types (must not be silent)

| Drop type | When | Required handling |
|---|---|---|
| `same_protocol_must_not_drop` | Native/raw field on A→A | Bug if dropped |
| `cross_protocol_lossy` | Target cannot express source semantics | `LossyDowngrade` or documented deliberate unsupported |
| `cross_protocol_no_synth` | Name-similar but different ecosystems (MCP, reasoning/thinking, web search shapes) | No rename bridge |
| `deprecated_compat_only` | Legacy fields | Keep on same protocol; do not invent modern equivalents |
| `codex_usage_profile` | Codex-only behavior without public P0 | P1 profile; not public baseline completion |

## Use rules

1. Treat each baseline as a wire-protocol reference, not AxonHub's canonical internal model.
2. Same-protocol round trip should preserve fields/items that the common abstraction does not understand.
3. Cross-protocol conversion must either implement an explicit bridge or emit explicit lossy diagnostics.
4. Do not silently map protocol-specific tool/state features by name similarity alone.
5. Keep Codex-only compatibility fields separate from public OpenAI/Anthropic protocol baselines unless a canonical source confirms them.
6. Prefer the strict verification matrix Field IDs for completion claims; summaries and old handoffs are not completion evidence.

## Regeneration notes

This README and the three protocol baseline documents were regenerated from the canonical source set. During regeneration, disputed fields were checked against the canonical sources instead of copied from earlier generated docs.

Examples of source-based corrections:

- Chat Completions keeps deprecated `function_call`, `functions`, `max_tokens`, `seed`, and `user` only because the new canonical create page/API definition still show them as deprecated fields.
- Chat Completions includes current `web_search_options`, `prompt_cache_key`, `prompt_cache_retention`, `reasoning_effort`, and `verbosity` because they appear in the canonical create page/API definition.
- Responses `client_metadata` is not treated as public OpenAI Responses baseline because it is not confirmed by the canonical public OpenAI sources here.
- Anthropic `mcp_servers` is documented as an MCP connector companion/extension field, not as a base field extracted from `messages-api.official-raw.md`.

## 2026-07-22 doc hygiene

- Obsolete root handoff notes moved to `docs/archive/protocol-handoffs-2026-07/`.
- Hub matrix / Round-5 "implementation candidates" wording must be read against current code for same-protocol evidence (Chat custom tools, Chat raw preserve fields, Anthropic MCP/container/geo, Responses raw/native sidecars including response `RawOutputItems`).
- Drafts under `drafts/` may contain stale P2 notes; when they conflict with baselines or current `llm/**` tests, baselines + code win.
