---
alwaysApply: false
globs: "{**/go.mod,Dockerfile,.github/workflows/*.yml,docs/**/development.md,AGENTS.md,scripts/bump-go-version.sh}"
---

# Go Version

Upgrade Go by running `scripts/bump-go-version.sh <version>`. Do not edit the pins by hand. The official `golang` image sets `GOTOOLCHAIN=local`, so a `go` directive newer than the image fails the Docker build even when the Dockerfile prefixes `go build` with `GOTOOLCHAIN=auto`.

The script keeps these on the same patch version:

- `go.mod`
- `llm/go.mod`
- `cmd/schema/go.mod`
- `cmd/pack-plugin/go.mod`
- `cmd/axonhub-desktop/go.mod`
- `Dockerfile` (`golang:<version>-alpine3.24`; the Alpine suffix stays)
- `.github/workflows/*.yml` literal `go-version:` values, including `stable`
- `AGENTS.md` technology stack
- `docs/zh/development/development.md` and `docs/en/development/development.md`

It leaves these alone:

- `go-version-file:` entries, because they follow the module file
- commented workflow lines
- `integration_test/**/go.mod` and `examples/**/go.mod`
