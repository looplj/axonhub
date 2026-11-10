# AGENTS.md

This file provides guidance to AI coding assistants when working with code in this repository.

> **Detailed rules are split into focused files under `.agents/rules/`**. See [Rules Index](#rules-index) below.

## Global Rules

1. Do NOT run lint or build commands unless explicitly requested by the user.
2. Do NOT restart the development server — it's already started and managed.
3. All summary files should be stored in `.agents/summary` directory if available.

## 文件头

新增 Go、TypeScript、TSX 源文件时，在 `package` 子句或 `import` 之前写文件头。Go 文件已有 package 文档注释时，文件头放在文档注释之前，不要替换它。

文件头包含：

- `// 职责:`：一句话说明这个文件做什么。
- `// 关联文件:`：列出 1–5 个直接依赖或协同的业务文件，每行一个，使用仓库相对路径。
- `// 遵循的 Rules:`：可选。只列出 `.agents/rules/` 下的规则文件，不要写 `AGENTS.md`。
- `// 注意事项:`：存在非显而易见的约束、兼容行为或易回归点时必须写。没有这种约束就省略。

```ts
// 职责: 判断未携带凭证或正在完成登录时，401 是否应该离开当前页。
// 关联文件:
//   frontend/src/gql/graphql.ts
//   frontend/src/main.tsx
// 遵循的 Rules:
//   .agents/rules/frontend-general.md
// 注意事项:
//   未携带凭证的 401 不是过期会话。
//   /oauth/ 上的 401 不能跳转 /sign-in，否则会取消 /oauth/oidc/exchange。
```

包含用户可见文案的前端文件，还要在 `// 关联文件:` 中列出 `frontend/src/locales/en.json` 和 `frontend/src/locales/zh.json`。这两条不计入 1–5 个业务关联文件的上限。

修改已有文件时，只在职责偏离原注释、关联文件变化或注意事项变化时更新文件头。不要为无关修改重写文件头。

以下文件不适用：

- Markdown，包括 `docs/` 和 `.agents/` 中的规则与说明。
- 首部标明 `Code generated` 的文件、`frontend/src/routeTree.gen.ts`，以及 `internal/ent/` 下 `schema` 以外的生成代码。

注意事项只写稳定的技术约束和兼容原因。不要写“某版本尚未发布”“当前还没上线”这类会随发布失效的状态，也不要留下临时排查记录。关联文件只列直接协作者，不要把传递依赖、测试夹具或整棵目录抄进文件头。同一约束已经写在文件头时，函数旁只保留读代码时仍然需要的那一句，不要把文件头再复述一遍。

## Configuration

- Backend API: port 8090, Frontend dev server: port 5173 (proxies to backend).
- Configuration: `conf/conf.go` (YAML + env var), SQLite by default.

## Project Overview

AxonHub is an all-in-one AI development platform that serves as a unified API gateway for multiple AI providers. It provides OpenAI and Anthropic-compatible API interfaces with automatic request transformation, enabling seamless communication between clients and various AI providers through a sophisticated bidirectional data transformation pipeline.

## Technology Stack

- **Backend**: Go 1.26.0+ with Gin, Ent ORM, gqlgen, FX
- **Frontend**: React 19 + TypeScript, TanStack Router/Query, Zustand, Tailwind CSS

## Backend Structure

- `cmd/axonhub/main.go` — Application entry point
- `cmd/axonhub-desktop/` — Desktop app (separate Go module)
- `internal/server/` — HTTP server and route handling with Gin
- `internal/server/biz/` — Core business logic and services
- `internal/server/api/` — REST and GraphQL API handlers
- `internal/server/gql/` — GraphQL schema and resolvers
- `internal/ent/` — Ent ORM for database operations
- `internal/ent/schema/` — Database schema definitions
- `internal/contexts/` — Context handling utilities
- `internal/pkg/` — Shared utilities (xerrors, xjson, xcache, xfile, xcontext, etc.)
- `internal/scopes/` — Permission system with role-based access control
- `llm/` — LLM utilities, transformers, and pipeline processing (separate Go module)
- `llm/pipeline/` — Pipeline processing architecture
- `conf/conf.go` — Configuration loading and validation

## Go Modules

- The repository root (`/`) is the main Go module: `github.com/looplj/axonhub`.
- `llm/` is a separate Go module: `github.com/looplj/axonhub/llm`.
- `cmd/axonhub-desktop/` is a separate Go module: `github.com/looplj/axonhub/cmd/axonhub-desktop`. Build it with `make build-desktop`.

### `llm/` Module Notes

- `llm/` is an independent module. Always run Go commands from the `llm/` directory (e.g., `cd llm && go test ./...`).
- Running `go test ./llm/...` from repo root will fail with module boundary errors.

### `cmd/axonhub-desktop/` Module Notes

- Run Go commands from `cmd/axonhub-desktop/` (e.g., `cd cmd/axonhub-desktop && go test ./...`).
- `make build-desktop` packages the app. Extra mygo flags go in `MYGO_FLAGS`.

## Frontend Structure

- `frontend/src/routes/` — TanStack Router file-based routing
- `frontend/src/gql/` — GraphQL API communication
- `frontend/src/features/` — Feature-based component organization
- `frontend/src/components/` — Reusable shared components
- `frontend/src/hooks/` — Custom shared hooks
- `frontend/src/stores/` — Zustand state management
- `frontend/src/locales/` — i18n support (en.json, zh.json)
- `frontend/src/lib/` — Core utilities (API client, i18n, permissions, utils)
- `frontend/src/utils/` — Domain-specific utilities (date, format, error handling)
- `frontend/src/config/` — App configuration
- `frontend/src/context/` — React context providers

## Rules Index

All detailed rules are in `.agents/rules/`:

| File | Scope | Description |
|------|-------|-------------|
| [go-general.md](.agents/rules/go-general.md) | `**/*.go` | Go 通用约定、错误处理、依赖注入、开发命令约束 |
| [ent-graphql.md](.agents/rules/ent-graphql.md) | `internal/ent/schema/**/*.go`, `internal/server/gql/**/*.go`, `internal/server/gql/**/*.graphql`, `gqlgen.yml` | Ent、GraphQL、代码生成、schema 变更规则 |
| [database-indexes.md](.agents/rules/database-indexes.md) | `internal/ent/schema/**/*.go` | 数据库索引设计、命名、跨方言兼容与迁移验证规则 |
| [biz-services.md](.agents/rules/biz-services.md) | `internal/server/biz/**/*.go` | Biz service、上下文取值、事务与级联删除规则 |
| [cache-compat.md](.agents/rules/cache-compat.md) | `**/*.go` | 缓存结构兼容性与升级安全规则 |
| [frontend-general.md](.agents/rules/frontend-general.md) | `frontend/**/*.ts`, `frontend/**/*.tsx` | 前端通用开发约定、GraphQL 数据约束、页面作用域 |
| [frontend-i18n.md](.agents/rules/frontend-i18n.md) | `frontend/src/**/*.ts`, `frontend/src/**/*.tsx`, `frontend/src/locales/*.json` | i18n 与货币格式规则 |
| [frontend-ui.md](.agents/rules/frontend-ui.md) | `frontend/**/*.tsx` | 前端 UI 组件使用规则 |
| [e2e.md](.agents/rules/e2e.md) | `frontend/tests/**/*.ts` | E2E testing rules |
| [docs.md](.agents/rules/docs.md) | `docs/**/*.md` | Documentation rules |
| [workflows/add-channel.md](.agents/rules/workflows/add-channel.md) | Manual | Workflow for adding a new channel |
