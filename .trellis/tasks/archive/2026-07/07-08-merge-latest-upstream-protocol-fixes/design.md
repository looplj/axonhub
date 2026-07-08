# Design — latest upstream merge strategy

## Strategy

Use upstream-first clean integration:

```text
origin/unstable latest
  ↓ create clean local worktree/branch
apply clean protocol transformer commits in original order
  ↓ resolve conflicts if needed
validate llm module
```

## Branch/worktree

- New worktree path: `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes`
- New branch: `codex/merge-latest-upstream-protocol-fixes`
- Base: `origin/unstable`

## Source of protocol fixes

Source branch/worktree:

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
- `codex/responses-top-level-preservation-clean`

Commit chain to apply is `origin/unstable@old-base..c62c111f` from the clean implementation worktree, preserving original order.

## Safety boundaries

- Do not use current polluted main repo branch as base.
- Do not stage unrelated dirty files in `/Users/asuan/项目/AI/axonhub`.
- Do not push.
- If conflicts appear, inspect and resolve only in the new worktree.
