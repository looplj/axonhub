# PR plan (S1–S7) — create only, do not merge

## Push status (2026-07-22)

`git push fork HEAD` rejected by GitHub secret scanning on **pre-existing** history:

- path: `docs/specs/openrouter-openapi.yaml` (OpenRouter API Key markers)
- not introduced by S1–S7 protocol clean commits

Until secret history is rewritten or unblocked, remote PR create from this branch is blocked.
Local commits for S1–S7 + S5 clone fix are present on `codex/grok-chat-custom-tool-compat`.

## Recommended package PR split (base: `origin/unstable`)

After branch is pushable, open **multiple** PRs (or sequential stacked PRs) rather than one mega-PR of ~100 commits:

1. **docs ownership freeze** — ADR/CONTEXT/guidelines/matrix (d1b5dec7 + 228b52d0 + 6bb615f0)
2. **llm PE core** — S1,S2,S4,S5,S5-clone-fix,S6 (`llm/**`)
3. **llm custom tool lifecycle** — S3 (`llm/transformer/shared/custom_tool_lifecycle.go` + orch thin wrappers)
4. **orchestrator pass-through mode** — S7 (`pass_through.go`)

Or squash S1–S7 protocol-only commits onto a clean branch from `origin/unstable` excluding vendor secret blobs.

## Slice commits (this task)

| Slice | Commit |
|---|---|
| S1 | 474a8e87 |
| S2 | 8bf9571c |
| S3 | 5577f6be |
| S4 | 4989103c |
| S5 | 1f3fce95 |
| S5 clone fix | 4562ec06 |
| S6 | 63c5c294 |
| S7 | b710f6d6 |

## Validation commands

```bash
cd llm && go test ./transformer/openai/ ./transformer/openai/responses/ ./transformer/shared/ ./transformer/anthropic/ . -count=1
cd .. && go test ./internal/server/orchestrator/ -count=1
```

## Residuals (out of this parent / follow-up)

- S8 bridges expansion
- S9 PersistenceState split
- Full matrix CONFIRMED drive
- Chat/Anthropic metadata legacy fallbacks remaining for non-migrated keys (cache_control, top_k, etc.)
- Anthropic drop vs Chat preserve still local adapters (S3 review note)
- Branch push unblocked after secret hygiene
