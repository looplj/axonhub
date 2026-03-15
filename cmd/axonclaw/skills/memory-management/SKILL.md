---
name: memory-management
description: Use this skill when you need to manage AxonClaw memory from the CLI, including reading MEMORY.md, appending daily notes, searching memory, rewriting long-term memory, or deleting memory files.
metadata:
  short-description: Manage AxonClaw memory files
---

# Memory Management

Use the `axonclaw memory` CLI for persistent local memory stored under `.axonclaw/`.

## When To Use

- Read long-term memory from `.axonclaw/MEMORY.md`
- Append a note to today's memory log
- Search memory entries by keyword
- Rewrite long-term memory after consolidating notes
- Delete a daily or long-term memory file

## Commands

```bash
axonclaw memory add "Finished migration for billing retries"
axonclaw memory add --longterm "User prefers concise status updates"
axonclaw memory get
axonclaw memory get --longterm
axonclaw memory get --date 2026-03-15
axonclaw memory search "quota exceeded" --limit 20
axonclaw memory rewrite --longterm --content "Consolidated long-term memory"
axonclaw memory delete --date 2026-03-15
```

## Guidance

- Use daily memory for append-only working notes.
- Use `--longterm` only for durable preferences, decisions, or facts.
- Prefer `search` before rewriting to avoid losing useful context.
- `rewrite --longterm` archives the previous `MEMORY.md` into today's daily memory file.
- `delete` is destructive. Confirm the target first.
