---
alwaysApply: false
globs: "**/*.go"
---

# Go General Rules

## Development Workflow

1. The development server is managed by `air`; do not restart it manually.
2. Do not run `go build`, `make build-backend`, `make build`, or `golangci-lint run` unless the user explicitly asks.
3. Use the owning Go module for commands. In particular, run Go commands for `llm/` from the `llm/` directory.

## Coding Conventions

1. Prefer `github.com/samber/lo` for collection, slice, map, and pointer helpers.
2. Use `lo.ToPtr(...)` instead of handwritten pointer helper functions such as `stringPtr`.
3. Follow the existing FX dependency injection patterns.
4. Use structured logging with zap.
5. Propagate `context.Context` correctly through request and service boundaries.
6. Handle errors with the unified helpers in `internal/pkg/xerrors` and wrap them with useful context.
