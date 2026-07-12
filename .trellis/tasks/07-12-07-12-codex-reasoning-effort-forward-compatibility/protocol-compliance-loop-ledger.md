# Protocol compliance loop ledger — latest Codex delta

## Source baseline

- Codex delta: `1f0566d..9e552e9d1`.
- Research reports:
  - `research/codex-delta-request-response-1f0566d-to-9e552e9d1.md`
  - `research/codex-delta-streaming-metadata-1f0566d-to-9e552e9d1.md`
  - `research/codex-delta-tools-mcp-1f0566d-to-9e552e9d1.md`

## Numbering resolution

| Number | Status | Meaning |
|---|---|---|
| G9 | historical, complete | Responses `stream_options` raw nested preservation |
| G10 | historical, complete | raw JSON clone helper |
| G11 | historical, complete | lossy downgrade recorder |
| G12 | historical, complete | raw top-level capture helper |
| G13 | complete | Responses request `reasoning` + encrypted reasoning include preservation |
| G14 | complete | Responses reasoning summary + summary stream-options preservation, without Codex model gating |
| G15 | complete | Responses request `input[]` item id identity/presence preservation (message / tool / reasoning) |

## Delta classification table

| Delta | Classification | Why | Next action |
|---|---|---|---|
| Always `reasoning` + `include: reasoning.encrypted_content` | G13 | existing Responses fields, new Codex outbound policy | audit current Hub preservation only |
| Capability-gated `reasoning.summary` + summary delivery option | G14 | existing Responses fields, new Codex emission gate | audit preservation/absence, do not duplicate gate |
| Strip empty/unprefixed outbound `input[].id` | G15 | existing Responses item identity behavior | audit preservation/absence, do not import prefix policy |
| `ReasoningEffort` named values + custom support; `ultra -> max` | G13/G14 note | value-domain/client policy, not a new wire field | retain same-family open-string proof; no generic mapping |
| tool_search/defer_loading/namespace/apply_patch/web-search/MCP declaration | existing domains, no delta | no production wire shape change in range | no G13–G15 work |
| approval/MCP OAuth/sandbox/multi-agent events/telemetry | client-only | not Responses body | document exclusion only |

## Slice state

| G / slice | Seam | State | Evidence | Next route |
|---|---|---|---|---|
| Pre-G13 unknown effort same-family | Chat/Responses inbound → canonical → outbound | PASS, auxiliary evidence | `future-effort` fixtures (`reasoning_effort_forward_compat_test.go`, Chat inbound tests); no production change | retain as G13/G14 value-domain proof |
| G13a | Responses body → canonical → Responses body | PASS | `g13a_reasoning_include_test.go`; `g13a-*.request.json`; preserve + default omission; module review PASS | G14 |
| G14a/G14b | Responses body → canonical → Responses body | PASS | `g14a_summary_stream_options_test.go`, `g14b_stream_options_sidecar_test.go`; dedicated `RawStreamOptions` clone; module re-review PASS | G15 |
| G15a | request `input[]` message / function / function_output item id | PASS | `g15a_input_item_identity_test.go`; `g15a-*.request.json`; preserve + no synthetic ids | G15b |
| G15b | custom tool + reasoning-following tool item id | PASS | `g15b_input_item_identity_test.go`; `g15b-*.request.json`; custom_tool_call(_output) + reasoning→tool path | G15c |
| G15c | reasoning item id / presence | PASS | `g15c_reasoning_item_identity_test.go`; fixtures cover standalone, pure standalone, summary-only, reasoning→tool, and no cross-protocol invent | parent review |
| Parent review (G13–G15) | integration | PENDING | — | required before commit |
| Final integration docs/commit | matrices + guidelines + scoped commit | IN PROGRESS (docs sync) | this docs-only sync | parent review → commit |

## Checkpoint

G13, G14, G15a/b/c are complete with public-seam fixtures and independent module reviews. Residual-coverage language is retired: custom-tool, summary-only, pure standalone, and reasoning→tool identities now have dedicated G15b/G15c fixtures.

Still open (do not mark the task complete):

1. Independent parent review across G13–G15.
2. Final scoped commit after parent review PASS.

Rules retained: no Codex default injection; no model-capability gate; no item-id synthesis/fallback; no cross-protocol Codex id invention.
