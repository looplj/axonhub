# AGENTS.md

This file provides guidance to AI coding assistants when working with code in this repository.

> **Detailed rules are in `.agent/rules/`** — see [Rules Index](#rules-index) below.

## Global Rules

1. Do NOT run lint or build commands unless explicitly requested by the user.
2. Do NOT restart the development server — it's already started and managed.
3. All summary files should be stored in `.agent/summary` directory if available.

## Configuration

- Uses SQLite database (axonhub.db) by default.
- Configuration loaded from `conf/conf.go` with YAML and env var support.
- Backend API: port 8090, Frontend dev server: port 5173 (proxies to backend).
- Go version: 1.26.0+.

## Project Overview

AxonHub is an all-in-one AI development platform that serves as a unified API gateway for multiple AI providers. It provides OpenAI and Anthropic-compatible API interfaces with automatic request transformation, enabling seamless communication between clients and various AI providers through a sophisticated bidirectional data transformation pipeline.

### Core Architecture

- **Transformation Pipeline**: Bidirectional data transformation between clients and AI providers
- **Unified API Layer**: OpenAI/Anthropic-compatible interfaces with automatic translation
- **Channel Management**: Multi-provider support with configurable channels
- **Thread-aware Tracing**: Request tracing with thread linking capabilities
- **Permission System**: RBAC with fine-grained access control
- **System Management**: Web-based configuration interface

## Technology Stack

- **Backend**: Go 1.26.0+ with Gin HTTP framework, Ent ORM, gqlgen GraphQL, FX dependency injection
- **Frontend**: React 19 with TypeScript, TanStack Router, TanStack Query, Zustand, Tailwind CSS
- **Database**: SQLite (development), PostgreSQL/MySQL/TiDB (production)
- **Authentication**: JWT with role-based access control

## Backend Structure

- `cmd/axonhub/main.go` — Application entry point
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

### `llm/` Module Notes

- Do not assume root-level Go commands can see packages under `llm/...`.
- When working on files under `llm/`, run Go commands from the `llm/` directory unless you explicitly know a workspace-level command is appropriate.
- Typical examples:
  - `cd llm && go test ./...`
  - `cd llm && go test ./transformer/openai/responses -run TestName`
  - `cd llm && go list ./...`
- If you run `go test ./llm/...` or similar from the repo root, you may hit module boundary errors like `main module does not contain package ...`.
- Apply the same rule to any other nested Go module: use the module root that owns the package you are testing or inspecting.

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

All detailed rules are in `.agent/rules/`:

| File | Scope | Description |
|------|-------|-------------|
| [backend.md](.agent/rules/backend.md) | `**/*.go` | Go, Ent, GraphQL, Biz service, error handling, dev commands |
| [frontend.md](.agent/rules/frontend.md) | `frontend/**/*.ts,tsx` | React, i18n, UI components, dev commands |
| [e2e.md](.agent/rules/e2e.md) | `frontend/tests/**/*.ts` | E2E testing rules |
| [docs.md](.agent/rules/docs.md) | `docs/**/*.md` | Documentation rules |
| [workflows/add-channel.md](.agent/rules/workflows/add-channel.md) | Manual | Workflow for adding a new channel |
