---
trigger: glob
globs: *.go
---

# Backend rules

1. The server in development is managed by air, it will rebuild and start when code changes, so DO NOT restart manually.

2. Use `make build-backend` to build the server to make sure the server is built successfully.

3. Change any ent schema or graphql schema, need to run `make generate` to regenerate models and resolvers.

4. Use `make generate` command to generate GraphQL and Ent code, which will automatically enter the gql directory and run go generate.

5. DO NOT ADD ANY NEW METHOD/STRUCTURE/FUNCTION/VARIABLE IN *.resolvers.go

6. Use `enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")` to create a new client for test.


#  Golang rules

1. USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.