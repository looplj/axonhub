# AGENTS.md

This file provides guidance to AI coding assistants when working with code in this repository.

> **Detailed rules are in `.agent/rules/`** — see [Rules Index](#rules-index) below.

## Global Rules

### General

1. Do NOT run lint or build commands unless explicitly requested by the user.
2. Do NOT restart the development server — it's already started and managed.
3. All summary files should be stored in `.agent/summary` directory if available.

### Configuration

- Uses SQLite database (axonhub.db) by default.
- Configuration loaded from `conf/conf.go` with YAML and env var support.
- Backend API: port 8090, Frontend dev server: port 5173 (proxies to backend).
- Go version: 1.25.3+.

### Error Handling

- Always handle errors using the unified error response format from `internal/pkg/errors`.
- Implement proper error wrapping with context.

### Development Commands

#### Backend (Go)

```bash
go run cmd/axonhub/main.go       # Run the main server
make generate                     # Generate GraphQL and Ent code (after schema changes)
go test ./...                     # Run tests
air                               # Hot reload (development)
```

#### Frontend (React)

```bash
cd frontend
pnpm install                      # Install dependencies
pnpm dev                          # Start dev server (port 5173)
pnpm format                       # Format code
pnpm knip                         # Check unused dependencies
pnpm test:e2e                     # E2E tests
```

#### Make Commands

```bash
make generate                     # Generate GraphQL and Ent code
make build                        # Build both frontend and backend
make build-backend                # Build backend only
make build-frontend               # Build frontend only
make cleanup-db                   # Cleanup test database
make e2e-test                     # Run full E2E test suite
make migration-test TAG=v0.1.0    # Test migration from specific tag
make sync-faq                     # Sync FAQ from GitHub issues
make sync-models                  # Sync model developers data
```

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

- **Backend**: Go 1.25.3+ with Gin HTTP framework, Ent ORM, gqlgen GraphQL, FX dependency injection
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
- `internal/llm/` — AI provider transformers and pipeline processing
- `internal/llm/pipeline/` — Pipeline processing architecture
- `internal/contexts/` — Context handling utilities
- `internal/pkg/` — Shared utilities (HTTP client, streams, errors, JSON)
- `internal/scopes/` — Permission system with role-based access control
- `conf/conf.go` — Configuration loading and validation

## Frontend Structure

- `frontend/src/routes/` — TanStack Router file-based routing
- `frontend/src/gql/` — GraphQL API communication
- `frontend/src/features/` — Feature-based component organization
- `frontend/src/components/` — Reusable shared components
- `frontend/src/hooks/` — Custom shared hooks
- `frontend/src/stores/` — Zustand state management
- `frontend/src/locales/` — i18n support (en.json, zh.json)
- `frontend/src/utils/` — Shared utilities

## Rules Index

All detailed rules are in `.agent/rules/`:

| File | Scope | Description |
|------|-------|-------------|
| [backend.md](.agent/rules/backend.md) | `**/*.go` | Go, Ent, GraphQL, Biz service rules |
| [frontend.md](.agent/rules/frontend.md) | `frontend/**/*.ts,tsx` | React, i18n, UI components rules |
| [e2e.md](.agent/rules/e2e.md) | `frontend/tests/**/*.ts` | E2E testing rules |
| [docs.md](.agent/rules/docs.md) | `docs/**/*.md` | Documentation rules |
| [workflows/add-channel.md](.agent/rules/workflows/add-channel.md) | Manual | Workflow for adding a new channel |
