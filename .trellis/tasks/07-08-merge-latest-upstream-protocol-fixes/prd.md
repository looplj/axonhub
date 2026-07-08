# PRD — Merge latest upstream with protocol transformer fixes

## Goal

把作者仓库 `origin/unstable` 的最新代码作为底座，安全合入 clean implementation worktree 中已经通过审查的协议转换修复，先本地试合并并确认冲突/测试结果，不 push GitHub。

## Confirmed facts

- 作者远端是 `origin = https://github.com/looplj/axonhub.git`。
- 作者默认分支是 `origin/unstable`。
- 最新作者底座：`origin/unstable = 35a8e5ba fix(trace): include instruction content in system_instruction span key (#1974)`。
- clean implementation worktree：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`。
- clean implementation branch：`codex/responses-top-level-preservation-clean`。
- clean implementation HEAD：`c62c111f refactor(llm): share raw top-level capture helper`。
- clean implementation worktree 状态干净，之前 `cd llm && go test ./... -count=1` 和 `git diff --check` 已通过。
- 当前主仓库分支 `codex-transformer-field-fixes` 有旧污染改动，不作为合并底座。

## Requirements

1. 不在当前污染主仓库分支上直接合并业务代码。
2. 从最新 `origin/unstable` 创建新的干净本地分支/worktree。
3. 将 clean implementation 的协议转换修复提交按顺序丢入新分支。
4. 如果出现冲突，记录冲突文件、冲突原因和推荐解决方向。
5. 如果合并成功，至少验证 `llm` 模块测试和 diff whitespace。
6. 不 push GitHub。

## Acceptance criteria

- AC1：有一个基于 `origin/unstable@35a8e5ba` 的干净本地合并 worktree/branch。
- AC2：clean implementation 修复提交已尝试按顺序应用。
- AC3：合并结果、冲突或成功状态被明确记录。
- AC4：若无冲突或冲突已解决，`cd llm && go test ./... -count=1` 通过。
- AC5：`git diff --check` 通过。
- AC6：没有 GitHub push。

## Out of scope

- 不处理当前主仓库旧污染文件。
- 不推送远端、不开 PR。
- 不改前端/UI。
- 不把 LossyDowngrade 展示接入 UI/trace。
