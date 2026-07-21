# Implement: S1–S7 ordered slices

## Preconditions

- [x] User locked 10 decisions + Goal exit standard in `prd.md`
- [x] S0 doc freeze landed (ADR/CONTEXT/guidelines/matrix notes)
- [ ] User approves planning → `task.py start` → implement S1 only first

## Global per-slice checklist

1. Write/adjust **red** tests for slice contract only  
2. Implement + **delete** dual path  
3. `cd llm && go test` on touched packages (orchestrator: from repo module as needed)  
4. Reviewer sub-agent short review  
5. Commit `slice(SN): ...`  
6. Stop — do not start SN+1 until short review pass  

## S1 — Responses PE single owner

**Do:** Remove metadata dual-write/read for Responses body natives (`include`, `background`, `max_tool_calls`, `prompt_cache_retention`, `truncation`, `prompt` raw, etc.). PE only. Fix tests that expect metadata body.  
**Don't:** Chat PE, custom tools, pass-through.  
**Validate:** `cd llm && go test ./transformer/openai/responses/ -count=1` (narrower if needed).  
**Done when:** No production write of those fields to TransformerMetadata; g13a + related green.

## S2 — Diagnostics convergence

**Do:** Single formal path for Lossy for fields this stack already diagnoses; remove redundant triple records where safe.  
**Don't:** New bridges.  
**Validate:** responses + anthropic + openai outbound tests for Lossy.  
**Done when:** Same loss event not recorded by three independent policies without shared owner.

## S3 — CustomToolLifecycle in llm

**Do:** Extract preserve/bridge/drop/rehydrate; wire orch + Chat + Anthropic to it; delete shadow policies.  
**Don't:** Chat raw PE (S4).  
**Validate:** freeform bridge tests + openai/anthropic custom tool tests.  
**Done when:** Orchestrator has no full custom→function conversion implementation body (thin call only).

## S4 — Chat PE owner

**Do:** Introduce Chat PE; migrate chat_n preserve fields; update lossy/bridge readers.  
**Don't:** Anthropic PE migration beyond what's required for compile.  
**Validate:** `go test ./transformer/openai/ -count=1` (chat_n + outbound).  
**Done when:** Same-protocol Chat natives not owned solely by full Body reparse.

## S5 — Anthropic PE body natives

**Do:** container/geo/mcp_servers primary PE; delete metadata primary writes after green.  
**Validate:** anthropic container/mcp tests.  
**Done when:** Primary owner is PE for those fields.

## S6 — Opaque reasoning strip in llm

**Do:** Shared strip helper; orch calls when only.  
**Validate:** encrypted_reasoning tests + responses identity tests.  
**Done when:** Orch does not encode Responses item-type strip details inline.

## S7 — Pass-through explicit mode

**Do:** Clear Convert vs PassThrough seam; keep channel/global flags.  
**Validate:** pass_through tests + recovery interaction.  
**Done when:** Mode is explicit; not only post-hoc body dump without contract.

## After S7 (still this parent, before Goal exit)

1. Phase reviews P1/P2/P3 if not already done at phase boundaries  
2. Re-run union of targeted tests  
3. Open package PRs to `origin/unstable` (no merge)  
4. Tick **Goal exit standard** in `prd.md`  
5. `task.py finish` only when exit standard complete  

## Follow-up parent (not this task)

- S8 bridges + Codex table expansion  
- S9 PersistenceState  
- S10 merge campaign / matrix full CONFIRMED drive  

## Goal exit (copy of hard gate)

See `prd.md` § Goal exit standard A–F. **All must be true.** Incomplete exit = blocked notes, not success.
