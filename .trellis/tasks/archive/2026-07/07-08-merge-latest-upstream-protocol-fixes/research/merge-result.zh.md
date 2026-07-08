# Merge result — latest upstream with protocol transformer fixes

## Summary

本地试合并成功。

- Upstream base: `origin/unstable@35a8e5ba fix(trace): include instruction content in system_instruction span key (#1974)`
- Integration worktree: `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes`
- Integration branch: `codex/merge-latest-upstream-protocol-fixes`
- Source fix worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
- Source fix branch: `codex/responses-top-level-preservation-clean`
- Source fix HEAD: `c62c111f refactor(llm): share raw top-level capture helper`

## Applied commits

25 个 clean protocol transformer 修复提交已按原顺序 cherry-pick 到最新作者底座上。

新分支上的对应提交范围：

```text
e592b97e fix: preserve responses request native fields
b6a1a298 fix: preserve responses response stream fields
901c663b fix: preserve responses mcp stream events
ea56d2b3 fix: preserve responses search stream events
d60d07d0 fix: preserve openai chat request fields
083613ee fix: preserve anthropic native fields
34aa75c0 fix: diagnose responses mcp downgrade to anthropic
65f7ceac fix: diagnose responses native state downgrade to anthropic
1cca8add fix: diagnose chat native downgrade to anthropic
5f42209a fix: diagnose anthropic mcp downgrade to responses
40ba5e19 fix: diagnose anthropic state downgrade to responses
2ce2bc08 fix: diagnose chat native downgrade to responses
faaacbc4 fix: diagnose responses state downgrade to chat
8463ec72 fix: diagnose responses mcp downgrade to chat
dd476509 fix: diagnose anthropic state downgrade to chat
beee8b11 fix: diagnose anthropic mcp downgrade to chat
ead86300 fix: diagnose chat n downgrade
3ff63013 fix: diagnose chat legacy function downgrade
96b37707 fix: diagnose responses raw top level downgrade
6241bc7a fix: address lossy downgrade review findings
4f63593a fix(transformer): move Responses body fields off TransformerMetadata into ProviderExtensions sidecar
c2ffbfcf fix(responses): preserve stream_options raw nested fields
2be14f8f refactor(llm): share raw JSON clone helpers
a0b33997 refactor(llm): centralize lossy downgrade recording
ef56b7c4 refactor(llm): share raw top-level capture helper
```

## Conflict

Only one conflict occurred:

- File: `llm/transformer/openai/inbound_convert.go`
- Commit: `1f9780b6 fix: preserve openai chat request fields`
- Upstream side added: `Thinking.Type == "disabled"` maps to `ReasoningEffort = "none"`.
- Fix side added: `preserveOpenAIChatRequestExtensions(req, r)`.

Resolution:

- Keep both behaviors.
- Run upstream Thinking conversion first.
- Then call `preserveOpenAIChatRequestExtensions(req, r)` before returning the common request.

This is not a semantic conflict; both changes are independent and should coexist.

## Validation

Executed in `/Users/asuan/项目/AI/axonhub-worktrees/merge-latest-upstream-protocol-fixes`:

```text
git status --short
=> clean
```

```text
git diff --check origin/unstable...HEAD
=> PASS, no output
```

```text
cd llm && go test ./... -count=1
=> PASS
```

## Conclusion

Latest upstream merge is viable. The protocol transformer fixes can be carried onto current `origin/unstable` with only one straightforward Chat inbound conflict. No failing `llm` tests after merge.

No GitHub push was performed.
