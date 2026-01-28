---
trigger: glob
globs: *.go
---

# Backend rules

1. The server in development is managed by air, it will rebuild and start when code changes, so DO NOT restart manually.

2. Use `make build-backend` to build the server to make sure the server is built successfully.

3. Change any ent schema or graphql schema, need to run `make generate` to regenerate models and resolvers.

4. Add or update struct in the objects, should update the mapping in the gqlgen.yml

5. Use `enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")` to create a new client for test.

6. DO NOT EDIT ent.graphql directly, add graphql in other graphql file.

#  Golang rules

1. USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.

# Biz Service Rules

1. Ensure the dependency service not be nil, the logic code should not check the service is nil.