# Implement plan

1. Fetch `origin` and confirm latest `origin/unstable`.
2. Create or reset clean integration worktree at `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes` from `origin/unstable`.
3. Determine clean fix commit sequence from `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`.
4. Apply commits with `git cherry-pick` in order.
5. On conflict:
   - capture status and conflicted files;
   - resolve only if straightforward and inside `llm/` protocol transformer scope;
   - otherwise stop and report.
6. If applied cleanly or conflicts resolved, run:
   - `cd llm && go test ./... -count=1`
   - `git diff --check origin/unstable...HEAD`
7. Record result in task research notes.
8. Do not push GitHub.
