# Implement

Worktree: `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes`

1. Read official Anthropic thinking/effort docs and the revised capability-policy design.
2. Add red tests for a centralized capability resolver and config override; do not add model-name branches to `buildBaseRequest`.
3. Add red serialized-request tests for:
   - Opus 4.8 + `high` regression;
   - `minimal -> low` controlled downgrade;
   - DeepSeek independent effort behavior;
   - manual invalid budget / too-small `max_tokens` explicit error.
4. Replace the current allowlist helper with centralized Anthropic capability policy data plus optional config override.
5. Make outbound conversion consume capability enum only.
6. Make manual thinking reject impossible budgets rather than emitting illegal requests.
7. Revert unrelated empty-signature fixture churn; keep only budget changes that follow the final legal manual-budget policy.
8. Run targeted Anthropic tests and `git diff --check`.
9. Reuse the three Slice 0 reviewers for delta review. Do not advance until all pass.
10. Only then isolate a local commit and record code-only verification evidence. Do not build, start, restart, initialize, log in to, or replay requests against a runtime service.
