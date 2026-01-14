# Repository Guidelines

This guide helps contributors navigate and work effectively in this repository.

## Project Structure

- `cmd/axonhub/` — main backend entrypoint (Go)
- `internal/` — backend modules (Ent ORM, server APIs, business logic, pkg utils)
- `frontend/` — React 19 TypeScript SPA using TanStack Router and Vite
- `integration_test/` — backend API integration tests (Go)
- `scripts/` — test runners and automation
- `deploy/` — service files and install scripts
- `docs/` — markdown docs and architecture diagrams
- `config.yml` (local dev), `config.example.yml`, `deploy/` samples — configs
- `go.mod`, `go.sum` — Go module with replace directives for forks
- `Makefile` — generate, build, and clean helpers (no test target currently)
- `.github/workflows/` — CI: lint, test, release, docker-publish

## Build, Test, and Development Commands

### Backend (Go)

- `make generate` — runs `go generate` in `internal/server/gql` (GraphQL + Ent)
- `make build-backend` — builds `axonhub` binary via `go build -o axonhub ./cmd/axonhub`
- `golangci-lint run -v` — lint with `.golangci.yml` config (concurrency 4, timeout 10m)
- `air` — optional hot-reload for development if installed

### Frontend (TypeScript/React)

- `cd frontend && pnpm install`
- `pnpm dev` — start dev server at `http://localhost:5173`
- `pnpm build` — build to `frontend/dist/`
- `pnpm lint` — ESLint
- `pnpm format` — Prettier write
- `pnpm format:check` — Prettier check
- `pnpm knip` — find unused code

### End-to-End Tests

- `pnpm test:e2e` (frontend) — launches backend (port 8099), runs Playwright, cleans up
- `pnpm test:e2e:ui` — Playwright UI mode
- `pnpm test:e2e:headed` — headed browser
- `./scripts/e2e/e2e-test.sh` — script entrypoint; see `scripts/README.md`
- `./scripts/e2e/e2e-backend.sh {start|stop|restart|clean}` — manage e2e backend

### Database Migrations And Tests

- `./scripts/migration/migration-test.sh <tag>` — download release binary, init DB, migrate to current, run e2e (see `scripts/migration/MIGRATION_TEST.md`)
- `./scripts/migration/migration-test-all.sh` — batch migration tests across recent tags

## Coding Style & Naming

### Go

- Imports: `goimports` grouping (standard, default, custom imports)
- Line length: 180 (soft limit per golangci-lint config)
- Use `golangci-lint` with `.golangci.yml`; scrutinize concurrency-safety
- Use Ent patterns for DB; tests use `testify`

### TypeScript/React

- ESLint + Prettier; no semicolons, single quotes preferred
- File-based routing via TanStack Router under `frontend/src/routes/`
- hooks/tests colocated when reasonable
- Use Zustand for global state
- Use Tailwind with merge utilities and defaults set in components

## Testing Guidelines

- Backend: use `*testing.T` and `testify`. 914 test functions present as of now.
- Frontend: Playwright e2e tests; no Jest in package.json
- API coverage in `integration_test/` targeting OpenAI API compatibility
- Run `pnpm test:e2e` locally to verify full flows; CI runs workflows
- Cleanup: `make cleanup-db` removes playwright test users/projects/keys if needed

## Commit & Pull Requests

We follow Conventional Commits and squash-merge PRs:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation changes
- `style:` formatting changes
- `refactor:` code refactors
- `test:` test changes
- `chore:` auxiliary tooling/build

### PR Requirements

- PR descriptions should explain what and why
- Reference related issues
- Screenshots for UI changes preferred
- CI checks (lint/test) must pass
- E2E optional for quick feedback but required before merge
- Keep PRs focused: backend + frontend as needed; migrations if required

## Security & Configuration Tips

- Rotate secrets in `config.yml`; do not commit with real keys
- For convenience, use `config.example.yml` to copy new defaults
- Run `./axonhub` from repo root to serve local dev on 8090; frontends on 5173
- Add channels via UI; export `config.yml` as reference for deployments

## Architecture Highlights

AxonHub is a bidirectional data transformation proxy unified under OpenAI-compatible APIs. The request pipeline:

Client → Inbound Transformer → Unified Request Router → Outbound Transformer → Provider

This ensures:
- Zero learning curve for OpenAI SDK users
- Auto failover and load balancing across channels
- Real-time tracing and per-project usage logs
- Support for multiple API formats (OpenAI, Anthropic, Gemini, and custom variants)
