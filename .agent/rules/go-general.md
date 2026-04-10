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
2. Do not add handwritten pointer helper functions such as `stringPtr`; use `lo.ToPtr(...)` or `new(T)` when appropriate.
3. It is acceptable to use `lo.ToPtr(...)` for constants or literals that cannot be addressed directly.
4. Follow the existing FX dependency injection patterns.
5. Use structured logging with zap.
6. Propagate `context.Context` correctly through request and service boundaries.
7. Handle errors with the unified helpers in `internal/pkg/xerrors` and wrap them with useful context.
