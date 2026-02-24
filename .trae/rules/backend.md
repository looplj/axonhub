---
alwaysApply: false
globs: **/*.go
---
# Backend Rules

1. The server in development is managed by air, it will rebuild and start when code changes, so DO NOT restart manually.

2. Use `make build-backend` to build the server to make sure the server is built successfully.

# Multi-Module Structure

This project uses Go workspace with multiple modules:

| Module | Path | Description |
|--------|------|-------------|
| Main | `/` | Main server with ent, graphql, api handlers |
| Axon | `/axon` | Agent framework with LLM providers, tools, memory |
| LLM | `/llm` | LLM related utilities (replaced by main module) |
| CLI | `/cmd/axoncli` | Terminal UI CLI tool |

## Running Tests

### Main Module (Root)
```bash
go test ./...                    # Run all tests
go test ./internal/... -v        # Run internal package tests with verbose
go test -run TestName ./...      # Run specific test
```

### Axon Module
```bash
cd axon && go test ./...         # Run all axon tests
cd axon && go test ./provider/anthropic/... -v  # Run anthropic provider tests
```

### CLI Module
```bash
cd cmd/axoncli && go test ./...  # Run CLI tests
```

# Golang Rules

1. DO NOT run `go build` or `make build-backend` unless explicitly asked. The server is managed by air in development and will auto-rebuild on code changes.

2. USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.

3. DO NOT RUN golangci-lint run, I will run manually.

# Ent Rules

1. Change any ent schema or graphql schema, need to run `make generate` to regenerate models and resolvers.

2. Add or update struct in the objects, should update the mapping in the gqlgen.yml

3. Use `enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")` to create a new client for test.

4. DO NOT EDIT ent.graphql directly, add graphql in other graphql file.

# Biz Service Rules

1. Ensure the dependency service not be nil, the logic code should not check the service is nil.
2. Dependency services are guaranteed initialized; business logic must not add nil checks.
