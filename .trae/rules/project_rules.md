The rules for AxonHub


# Backend

1. The server in development is managed by air, will rebuild when code changes, so no need to restart manually.

2. Change any ent schema or graphql schema, need to run `go generate` in server directory.

# Frontend

1. We use the pnpm as the package manager, can run `pnpm dev` to start the development server
