---
alwaysApply: false
globs: **/*.go
---
# Backend Rules

1. The server in development is managed by air, it will rebuild and start when code changes, so DO NOT restart manually.

2. Use `make build-backend` to build the server to make sure the server is built successfully.

# Golang Rules

1. USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.

2. DO NOT RUN golangci-lint run, I will run manually.

# Ent Rules

1. Change any ent schema or graphql schema, need to run `make generate` to regenerate models and resolvers.

2. Add or update struct in the objects, should update the mapping in the gqlgen.yml

3. Use `enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")` to create a new client for test.

4. DO NOT EDIT ent.graphql directly, add graphql in other graphql file.

# Biz Service Rules

1. Ensure the dependency service not be nil, the logic code should not check the service is nil.
2. Dependency services are guaranteed initialized; business logic must not add nil checks.
