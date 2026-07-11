# HPF Module Review — high-priority fixtures / matrix sync

| Field | Value |
|---|---|
| **Conclusion** | **FAIL** |
| **Agent** | high-priority fixture/matrix sync module review sub-agent |
| **Commit** | `c8718b9a91f3cf341d260b075ec6514873ce585e` (`docs(protocols): sync matrix and specs for G1-G7 evidence`) |
| **Branch** | `codex-transformer-field-fixes` |
| **Scope** | docs/evidence consistency only; no code changes |
| **Date** | 2026-07-12 |

## Design intent check

| Intent | Result | Note |
|---|---|---|
| 1. No new protocol feature | **PASS** | Commit is docs/task-only; no transformer code. |
| 2. Matrix/spec evidence for G1–G7 | **PARTIAL** | §9 + guidelines Field Evidence Index present and largely accurate; primary §5 rows / §0 / batch drafts not reconciled. |
| 3. Residual fixture-only explicit, not faked complete | **PASS** | `residual-gaps.md` + matrix §9.3; whole matrix remains **INCOMPLETE**. |
| 4. Parent final audit input usable | **PASS with caveats** | `parent-final-audit-input.md` exists and is structured; must be read with majors below. |

## Smoke (optional)

```text
cd llm && go test ./transformer/openai ./transformer/anthropic ./transformer/openai/responses \
  -count=1 -run 'TestOpenAIChatRequest|TestAnthropicMCP|TestResponsesReasoning|TestAnthropicContainer'
```

Result: **all three packages ok**. Listed tests matching G1–G7 claims exist (e.g. `TestOpenAIChatRequestN*`, `TestAnthropicContainer*`, `TestAnthropicMCP*`, `TestResponsesReasoning*`).

Referenced implementation commits resolve:

- G5b: `628e659d`, `97686bd6`
- G6: `610a3426`, `5c03dc48`
- G7: `7a1d1cfe`, `e6fe1a78`

## Findings

### Blockers

None.

Does **not** invent protocol features; does **not** mark the whole matrix `CONFIRMED`; residual gaps are labeled fixture-only / intentional non-goals.

### Major

1. **Primary matrix §5 rows still `UNCHECKED` while §9 claims evidence elevation**  
   - G1–G7 rows in §5 still show `UNCHECKED` for owner/code/test columns (examples: `CHAT.TOP.n` L473, `CHAT.TOP.prompt_cache_retention` L478, `CHAT.TOP.audio` L465, `CHAT.TOP.moderation` L472, `CHAT.TOP.prediction` L475, `CHAT.TOP.web_search_options` L493, `CHAT.TOP.function_call` L494, `CHAT.TOP.functions` L495, `ANT.TOP.container` L507, `ANT.TOP.inference_geo` L508, `ANT.TOP.mcp_servers` L521; nested/stream rows for reasoning also lack §5 elevation).  
   - §9.2 only adds a *reader overlay* (“interpret §5 as PARTIAL using §9.1 paths”) instead of updating the authoritative row cells.  
   - Violates task PRD/implement expectation that matrix rows themselves are updated with traceable evidence. Parent auditors cannot trust §5 alone.

2. **§0 “当前结论” is stale vs G1–G7 + §9**  
   - §0 still claims the only closed loop is Codex Responses `tools[].type = "namespace"`.  
   - §9 and residual-gaps document many additional same-protocol closed seams.  
   - Document-level status narrative is inconsistent; risk of under/over-reading completion.

3. **Batch drafts not synced despite PRD / implement.jsonl scope**  
   - PRD: 同步主矩阵、**batch drafts**、Trellis spec、本地 evidence.  
   - `implement.jsonl` lists all five `docs/specs/protocols/drafts/batch-*.md`.  
   - Commit touches **none** of the drafts (last mtime remains 2026-07-09).  
   - Spec surface is only partially synchronized.

4. **Task metadata claims completion while pointers incomplete**  
   - `task.json`: `"status": "completed"` but `"commit": "pending"` even though this commit exists.  
   - `hpf-slice-ledger.md` S5 review still `in_progress` at commit time (expected if review runs after commit, but task should not mark fully completed with `commit: pending`).

### Minor

1. **Guidelines Field Evidence Index is coarser than matrix §9.1**  
   - Index compresses G1/G2/G4/G5a into one “Chat top-level raw preserve” row; acceptable as index, but weaker for row-ID audit than §9.1.

2. **§9.1 test globs are approximate**  
   - e.g. `TestOpenAIChatRequestN*` / “output-controls” / “stream tests” — real names exist and smoke passes; naming is slightly loose, not false.

3. **Parent input asserts module multi-agent reviews “done (G1–G7)”** without citing review paths in this commit; relies on archives. Non-blocking if archives hold PASS reviews (not re-audited here).

4. **Matrix file is introduced whole (890 lines) in this commit** rather than a delta on a pre-existing branch file. Acceptable if first import; still means most content is Round-4 baseline + §9 append, not a full G1–G7 re-score of every cell.

## Evidence that is good

- Scope discipline: docs only; residual explicitly non-feature.  
- §9.1 field→module→code→test mapping is substantially correct for G1–G7:  
  - Chat `n` / cache / output controls / web_search → `chat_n.go` + `chat_n_test.go`  
  - Deprecated functions family → `chat_deprecated_functions_test.go`  
  - Anthropic container/geo → `container_inference_geo_test.go`  
  - Anthropic MCP → `mcp_connector_test.go`  
  - Responses reasoning → `reasoning_context_test.go`, `reasoning_g7_test.go`  
- Cross-protocol policy language (no-synth / LossyDowngrade / not fake MCP bridge) matches design intent and residual-gaps.  
- `parent-final-audit-input.md` checklist is usable as parent entrypoint once majors are fixed or accepted.  
- Smoke tests for claimed suites pass on current tree.

## Verdict rationale

**FAIL** because PRD acceptance requires matrix/spec consistency for parent final check, but the commit leaves **triple-source status**:

1. §5 cells still `UNCHECKED`  
2. §9 overlay claims same-protocol PARTIAL + tests  
3. §0 still “only namespace closed”

Plus **batch drafts untouched** while listed as sync targets.

This is documentation inconsistency, not implementation fraud. Fix is docs-only:

1. Update §5 (and relevant nested rows) Code/Test/Status cells for G1–G7 set to match §9.1, **or** demote §9 to pure appendix and stop claiming matrix row elevation.  
2. Rewrite §0 to list G1–G7 same-protocol closes + residual non-claims + overall still INCOMPLETE.  
3. Either sync batch drafts with the same evidence pointers, or narrow PRD/implement.jsonl and residual note “drafts deferred”.  
4. Set `task.json.commit` to `c8718b9a` (or successor fix commit); keep status accurate.

After those, re-review can pass without code changes.

## Residual (accepted as non-blocking if remaining open)

As documented in `research/residual-gaps.md`:

- token-limit multi-direction fixture expansion  
- Responses namespace / Codex P1 catalog  
- full SSE tool/audio family parity  
- Anthropic thinking multi-block ordering edge fixtures  
- Chat custom tool source gap  

These are correctly **not** presented as G1–G7 feature completion.

## Recommendation to parent

Do **not** treat `c8718b9a` alone as “matrix fully synced for final audit”. Use it as:

- honest residual list  
- test path index for G1–G7  

Require a follow-up docs commit for §0/§5 (and draft policy) before parent architecture review closes the fixtures/matrix gate.
