# 本地 Hub / upstream 源码目录清点

日期：2026-07-06

## 结论

当前不删除任何目录。先把作者最新版源码固定为当前仓库的 git worktree：

```text
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
```

这个 worktree 指向 `origin/unstable`，当前提交：

```text
97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)
```

后续源码审计、MCP 索引、当前分支与作者最新版对比，都以这个 worktree 为准，不再使用 `/tmp` 目录。

## 目录清单

### 1. 当前工作仓库

```text
/Users/asuan/项目/AI/axonhub
```

用途：当前任务工作区。

状态：

```text
branch: codex-transformer-field-fixes
HEAD: c798c6e9
remote origin: https://github.com/looplj/axonhub.git
remote fork: https://github.com/asuan-dev/axonhub.git
working tree: dirty
```

判断：保留。这里有当前任务的规划文档、旧实现污染和后续修复工作。

### 2. 新建作者最新版 worktree

```text
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
```

用途：作者最新版 upstream 源码，只读审计基线。

状态：

```text
HEAD: 97c9351a
source ref: origin/unstable
worktree mode: detached HEAD
```

判断：保留。后续 MCP 项目名：

```text
Users-asuan-AI-axonhub-worktrees-upstream-unstable
```

### 3. 旧 upstream clone

```text
/Users/asuan/项目/AI/axonhub-upstream
```

状态：

```text
branch: unstable
HEAD: ea7edb3
remote origin: https://github.com/looplj/axonhub.git
working tree: clean
```

判断：疑似旧的独立 upstream clone，落后于当前 `origin/unstable`。可以作为历史目录保留；如果要清理，建议先确认没有本地配置、索引或引用，再删除。

### 4. 9router external 里的 axonhub

```text
/Users/asuan/项目/AI/9router/external/axonhub
```

状态：

```text
branch: unstable
HEAD: 94394eb
remote origin: https://github.com/looplj/axonhub.git
working tree: has untracked .codebase-memory/
```

判断：更旧，可能是 9router 项目的 external 依赖/历史引用。不能直接删。若要删除或更新，必须先检查 9router 是否引用这个路径。

## 推荐后续处理

1. 当前任务审计只使用 `/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable`。
2. 暂时不删除 `/Users/asuan/项目/AI/axonhub-upstream` 和 `/Users/asuan/项目/AI/9router/external/axonhub`。
3. 等本轮协议转换修复完成后，再单独做一次“旧目录清理”任务：检查引用、确认无依赖、给删除清单，再执行。
