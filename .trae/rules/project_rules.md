The rules for AxonHub

# Rules

1. All SUMMARY FILE SHOULD STORE IN .trae/summary directory if have.

# Backend rules

1. The server in development is managed by air, will rebuild when code changes, so no need to restart manually.

2. Change any ent schema or graphql schema, need to run `go generate` in server directory.

3. DO NOT ADD ANY NEW METHOD/STRUCTURE/FUNCTION/VARIABLE IN *.resolvers.go

4. USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.

# Frontend rules

1. We use the pnpm as the package manager, can run `pnpm dev` to start the development server
