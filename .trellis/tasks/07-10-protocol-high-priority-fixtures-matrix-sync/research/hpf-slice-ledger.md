# High-Priority Fixtures / Matrix Sync Ledger

## Scope
No new protocol features. Close high-priority fixture/test evidence for fields already implemented in G1–G7, synchronize matrix/spec language, and mark intentional lossy/unsupported with evidence.

## Implemented modules evidence (code + tests)

| Module | Fields | Evidence |
|---|---|---|
| G1 | Chat `n` | `llm/transformer/openai/chat_n.go` + chat_n_test |
| G2 | Chat `prompt_cache_retention` | chat_n raw preserve + tests |
| G3 | Anthropic `container`/`inference_geo` | `container_inference_geo_test.go` |
| G4 | Chat `audio`/`prediction`/`moderation` | chat_n_test output controls |
| G5a | Chat `web_search_options` | chat_n_test + anthropic lossy |
| G5b | Chat deprecated `functions`/`function_call`/`message.function_call` | `chat_deprecated_functions_test.go` |
| G6 | Anthropic `mcp_servers`/`mcp_toolset` | `mcp_connector_test.go` |
| G7 | Responses reasoning context/generate_summary/content/stream/unknown nested | `reasoning_*_test.go` + stream/aggregator |

## Slices
| Slice | Outcome | Status |
|---|---|---|
| S1 inventory high-priority rows mapped to G1–G7 | this ledger | completed |
| S2 matrix row updates for implemented fields | protocol-conversion-strict-verification-matrix.md §9 | completed |
| S3 Trellis guidelines evidence pointers | protocol-transformer-guidelines.md Field Evidence Index | completed |
| S4 residual fixture-only / intentional lossy notes | residual-gaps.md | completed |
| S5 module review | research/reviews/ | in_progress |

## Residual (explicitly not completed as features)
- Broad token-limit precedence matrix across all three protocols (fixture-only expansion beyond existing token tests)
- Full Responses namespace/Codex P1 behavior catalog
- Full SSE event family cross-protocol equivalence claims
- Anthropic thinking/redacted multi-block ordering beyond existing coverage
