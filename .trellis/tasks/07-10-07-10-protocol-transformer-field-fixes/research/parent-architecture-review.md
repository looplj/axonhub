# Parent Architecture Review — Protocol Transformer Field Fixes (G1–G7)

| Field | Value |
|---|---|
| **结论** | **PASS** |
| **Agent ID** | `parent-architecture-review` |
| **Branch** | `codex-transformer-field-fixes` |
| **HEAD** | `c8718b9a91f3cf341d260b075ec6514873ce585e` |
| **Commits range (G1–G7 + matrix sync)** | `d2d2f5a9` … `c8718b9a` (inclusive; G1 start through matrix/docs sync) |
| **Repo** | `/Users/asuan/项目/AI/axonhub` |
| **Review date** | 2026-07-12 |
| **Mode** | architecture-only review; no production code changes |
| **Allow goal complete?** | **YES** — for the goal-scoped claim: *应实现协议转换缺口（本 goal 识别的模块）已修复* |

---

## 1. Scope of claim (non-overclaim)

This review **does not** claim:

- full 101-row matrix is `CONFIRMED`;
- all cross-protocol semantic equivalence is complete;
- Codex/Responses namespace P1 catalog is public P0;
- full SSE tool/audio family parity.

This review **does** claim:

- modules G1–G7 identified by the parent PRD/design are implemented with targeted tests;
- author transformer architecture (typed native / common / raw sidecar / LossyDowngrade / stream fidelity separation) is retained;
- residual gaps are explicitly non-blocking / fixture-only / intentional lossy;
- no open blocker-class findings remain for the goal-scoped modules.

Primary evidence sources:

- git history on `codex-transformer-field-fixes`
- `.trellis/tasks/archive/2026-07/07-10-protocol-*`
- `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` §9
- `.trellis/spec/backend/protocol-transformer-guidelines.md` Field Evidence Index
- `.trellis/tasks/07-10-protocol-high-priority-fixtures-matrix-sync/research/{residual-gaps,parent-final-audit-input,hpf-slice-ledger}.md`
- packages: `llm/transformer/openai`, `llm/transformer/anthropic`, `llm/transformer/openai/responses`

---

## 2. Exit-criteria table

| # | Exit criterion (from parent PRD / audit input) | Verdict | Evidence |
|---:|---|---|---|
| 1 | Should-implement conversion gaps fixed for identified modules | **PASS** | G1–G7 code commits + §9.1 row map |
| 2 | High-priority fixtures/tests or N/A reasons | **PASS** | per-module tests + `residual-gaps.md` |
| 3 | Slice self-reviews / ledgers | **PASS** | archive ledgers G4–G7 + hpf ledger S1–S4 |
| 4 | Module multi-agent reviews | **PASS** | G2–G7 formal PASS (G5b/G7 via re-review); G1 small slice verified by code+tests+shared chat raw path |
| 5 | Parent architecture review | **PASS** | this document |
| 6 | Protocol fields aligned to baseline docs | **PASS** | matrix §9 + guidelines Field Evidence Index (`c8718b9a`) |
| 7 | No known blockers | **PASS** | residual majors closed in re-reviews; residual list is non-blocking |
| 8 | Specs updated + local commits | **PASS** | docs/trellis archives + matrix sync commit |
| 9 | No forced lossy conversion / no fake maps | **PASS** | LossyDowngrade / no-synth / MCP connector isolation retained |
| 10 | Goal wording: identified should-implement modules fixed | **PASS** | see §8 declaration |

---

## 3. Architecture fidelity (author transformer model)

Expected chain:

```text
source protocol adapter
  -> llm common model + provider extensions / raw sidecar / bridge metadata
  -> target protocol adapter
```

Field classes observed in G1–G7:

| Class | G1–G7 usage | Architecture compliance |
|---|---|---|
| `common-abstraction` | modern tools / messages remain on common model; not widened for Chat-only top-level fields | **OK** |
| `native-field` | Anthropic `container`/`inference_geo`; Responses reasoning native fields | **OK** |
| `raw-preserve` | Chat `openAIChatRawPreserveFields` + `marshalOpenAIChatRequest`; Anthropic `mcp_toolset` `Tool.Raw` | **OK** |
| `adapter-specific` | Anthropic `MCPServers` + metadata key; MCP connector not common tools | **OK** |
| `deprecated-compat` | G5b origin-gated `message.function_call` reverse path; G7 `generate_summary` distinct wire identity | **OK** |
| `lossy / unsupported` | Anthropic outbound Chat native lossy diagnostics; Responses/Chat no-synth of Anthropic MCP | **OK** |
| `stream fidelity` | G7 `reasoning_text.*` gated prefer-text in stream/aggregator; default summary path retained | **OK** |

### 3.1 Separation checks

1. **Typed native vs raw sidecar**  
   - Chat top-level preserve is raw sidecar merge at outbound (`chat_n.go`), not promotion onto shared `openai.Request` with custom `MarshalJSON`.  
   - Aligns with guidelines: Chat raw top-level replay via explicit outbound helper only.

2. **Common model not used as protocol body dump**  
   - G1/G2/G4/G5a/G5b request top-level fields stay out of common request structs.  
   - G6 `mcp_toolset` stays raw fragment; function tools still convert to common tools.

3. **TransformerMetadata as bridge/staging, not body trash**  
   - Used for MCP servers opaque bytes, reasoning context/prefer-text/text-content staging, deprecated origin flags.  
   - No evidence of stuffing full protocol matrices into metadata as a second schema.

4. **LossyDowngrade / no-fake-map**  
   - Cross-protocol incompatible fields diagnosed or explicitly no-synth.  
   - G6 tests assert Responses/Chat do **not** synthesize `mcp_servers` / `mcp_toolset` / Responses `"type":"mcp"`.

5. **Stream fidelity separated**  
   - G7 stream path lives in `outbound_stream` / `inbound_stream` / `aggregator`, not request-body model.  
   - Prefer-text is production-written; default summary path not unconditionally replaced.

**Architecture verdict:** G1–G7 follow author architecture; no rewrite toward universal native AST.

---

## 4. Per-module architecture notes (anti-shitpile check)

| Module | Commits | Review | Architecture shape | Shitpile / false-bridge? |
|---|---|---|---|---|
| **G1** Chat `n` | `d2d2f5a9` | code+test (no separate multi-agent file found) | raw preserve list entry | no |
| **G2** `prompt_cache_retention` | `e1c332ae` | Boole PASS | same raw preserve path | no; naming lag minor (`chat_n.go`) |
| **G3** `container`/`inference_geo` | `ef149bea` | Carson PASS | opaque JSON + metadata | no; OpenAI direction no-synth minor residual |
| **G4** audio/prediction/moderation | `9a2692ed`, `7cd64f9f` | Boyle PASS after fail→fix | raw preserve; scope tightened | no; overreach fixed |
| **G5a** `web_search_options` | `6525bb82` | Laplace PASS | raw preserve; not hosted-tool confusion | no |
| **G5b** deprecated functions | `628e659d`, `97686bd6` | re-review PASS (majors closed) | raw request + origin-gated response bridge | no false modern rewrite; modern path isolated |
| **G6** MCP connector | `610a3426`, `5c03dc48` | G6 PASS | MCPServers + Tool.Raw + index order | **explicit no-fake-map to Responses MCP** |
| **G7** reasoning stream | `7a1d1cfe`, `e6fe1a78` | re-review PASS (M1/M2 closed) | native+sidecar + stream gate | no; summary vs text identities kept |
| **HPF matrix sync** | `c8718b9a` | docs | §9 + residual honesty | no new features |

**False-bridge search:** none found for OpenAI Responses MCP ↔ Anthropic MCP connector, Chat hosted search ↔ Anthropic tools, or deprecated `functions` auto-rewrite into modern tools without origin gate.

---

## 5. Cross-protocol MCP / tooling no-fake-map

Guidelines rule:

> Do not fake-map incompatible tool ecosystems: OpenAI Responses `mcp` / `file_search` / `code_interpreter` and Anthropic `mcp_servers` / `mcp_toolset` need explicit tested semantics before any bridge.

Verified:

| Check | Result |
|---|---|
| Anthropic same-protocol preserves `mcp_servers` / `mcp_toolset` | PASS (`mcp_connector_test.go`) |
| `mcp_toolset` not converted into common function tools | PASS (comment + convert path) |
| Responses outbound does not invent Anthropic connector fields | PASS (`TestAnthropicMCPConnectorNotSynthesizedForResponsesOrChat`) |
| Chat outbound does not invent connector fields | PASS (same) |
| Responses `"type":"mcp"` not fabricated from Anthropic connector | PASS (asserted not contains) |
| Deprecated Chat functions not synthesized for Responses | PASS (`RequestDeprecatedFunctionsNotSynthesizedForResponses`) |

**Verdict:** cross-protocol MCP/tooling remains **explicit no-fake-map**.

---

## 6. Findings

### Blocker
**None.**

### Major
**None open.**

Closed majors (historical, for audit trail):

| Module | Closed major | Closed by |
|---|---|---|
| G5b | stream/history reverse path for legacy `function_call` | `97686bd6` + re-review |
| G7 | outbound stream/aggregator `reasoning_text`; production prefer-text | `e6fe1a78` + re-review |
| G4 | scope overreach + fixture granularity | `7cd64f9f` |

### Minor (non-blocking residual architecture notes)

1. **`chat_n.go` naming lag** — file owns multiple raw-preserve fields beyond `n` (G2/G4/G5a/G5b). Cosmetic ownership debt; not wrong abstraction.
2. **Some no-synth paths without LossyDowngrade** — G2/G3/G5a reviews note Responses/OpenAI directions may be documented no-synth rather than diagnostic emit. Policy acceptable when intentional; consistency can improve later.
3. **G5b mixed multi-tool_calls origin edge** — first-tool_call rewrite when mixed origins; legacy single `function_call` shape OK for current scope.
4. **G7 aggregator edge** — `OutputItemDone` weak if only final content[] without delta; real upstream usually streams deltas.
5. **G7 stream golden metadata compare relaxed** — protocol payload still compared; sidecar metadata not fully golden-locked.
6. **G1 formal multi-agent review artifact thin** — G1 is minimal raw-preserve slice; risk low because subsequent modules reuse same helper and tests cover the field set.
7. **HPF ledger S5 previously `in_progress`** — parent architecture review is the remaining gate; this document closes it from parent-goal perspective.

---

## 7. Residual gaps honesty

Source: `residual-gaps.md` + matrix §9.3.

Closed by G1–G7 (implementation + tests): listed fields for n, cache retention, container/geo, output controls, web_search_options, deprecated functions, MCP connector, reasoning context/generate_summary/content/stream/unknown nested.

Still residual / intentional / fixture-only (not reopened as G1–G7 features):

1. Token-limit precedence multi-direction table expansion  
2. Responses namespace / Codex P1 sub-agent/codex_app catalog  
3. Full Responses SSE tool/audio event family parity  
4. Anthropic thinking multi-block ordering edge fixtures  
5. Chat custom tool source gap (do not invent support)

**Honesty verdict:** residual gaps are correctly labeled; no silent claim of full-matrix completion.

---

## 8. Tests executed this review

```bash
cd /Users/asuan/项目/AI/axonhub/llm
go test ./transformer/openai ./transformer/anthropic ./transformer/openai/responses -count=1
```

Result:

```text
ok  github.com/looplj/axonhub/llm/transformer/openai
ok  github.com/looplj/axonhub/llm/transformer/anthropic
ok  github.com/looplj/axonhub/llm/transformer/openai/responses
```

---

## 9. Commits range (implementation map)

| Module | Implementation commits |
|---|---|
| G1 | `d2d2f5a9` |
| G2 | `e1c332ae` |
| G3 | `ef149bea` |
| G4 | `9a2692ed`, `7cd64f9f` |
| G5a | `6525bb82` |
| G5b | `628e659d`, `97686bd6` |
| G6 | `610a3426`, `5c03dc48` |
| G7 | `7a1d1cfe`, `e6fe1a78` |
| Docs/matrix | `c8718b9a` (+ per-module trellis archive commits) |

**Range for parent gate:** `d2d2f5a9` … `c8718b9a` on `codex-transformer-field-fixes`.

Note: earlier branch work (pre-G1 adaptive thinking / upstream merges / broader transformer fixes) is **out of this G1–G7 goal declaration**, though architecturally compatible.

---

## 10. Goal-complete declaration

### Allowed wording

> **应实现协议转换缺口（本 goal 识别的模块 G1–G7）已修复。**

Meaning:

- same-protocol preserve or explicit deprecated/stream identity for those fields is implemented and tested;
- cross-protocol fake maps were not introduced;
- intentional lossy/unsupported residuals remain explicitly documented.

### Disallowed overclaim

> 三协议全部字段已完整等价转换 / 101 行矩阵全部 CONFIRMED / Codex P1 已作为公共 P0。

---

## 11. Final decision

| Question | Answer |
|---|---|
| Still follows author transformer architecture? | **Yes** |
| G1–G7 introduce shitpile / wrong abstraction / false bridges? | **No blocker-level; only minor residual notes** |
| Cross-protocol MCP/tooling still no-fake-map? | **Yes** |
| Blocker-level open review issues? | **None** |
| Residual gaps honestly labeled? | **Yes** |
| May declare goal-scoped modules fixed? | **Yes** |
| **PASS/FAIL** | **PASS** |
| **Allow goal complete** | **YES** |

---

## 12. Recommended next non-goal work (optional, not blocking)

1. Rename or split `chat_n.go` ownership for multi-field raw preserve.  
2. Unify no-synth vs LossyDowngrade diagnostic policy for Chat-native top-level fields on Anthropic/Responses outbound.  
3. Expand fixture-only tables listed in residual-gaps without reopening feature seams.  
4. Optional stronger G7 golden metadata + rare aggregator content-only done path.

