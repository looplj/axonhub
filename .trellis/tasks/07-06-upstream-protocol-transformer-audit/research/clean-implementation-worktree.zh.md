# Clean implementation worktree

日期：2026-07-07

## 结论

后续业务实现只在这个 clean worktree 上进行：

```text
/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
```

分支：

```text
codex/responses-top-level-preservation-clean
```

基线：

```text
origin/unstable
97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)
```

状态：

```text
git status --short => clean
```

codebase-memory 项目：

```text
Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean
nodes: 37149
edges: 205064
status: indexed
```

## 禁止事项

不要在当前污染分支继续业务实现：

```text
/Users/asuan/项目/AI/axonhub
branch: codex-transformer-field-fixes
```

该分支只作为研究、旧方案反例、可摘取思路来源。

## 下一步入口

进入 P1a：OpenAI Responses request top-level raw fallback。

目标文件限定：

```text
llm/provider_extensions.go
llm/transformer/openai/responses/request_extensions.go
llm/transformer/openai/responses/outbound_test.go
llm/transformer/openai/responses/inbound_test.go
```

P1a 不修改：

```text
Chat
Anthropic
Gemini
OpenRouter
stream
shared lossy downgrade
```
