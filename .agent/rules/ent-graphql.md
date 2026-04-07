---
alwaysApply: false
globs: "internal/ent/schema/**/*.go, internal/server/gql/**/*.go, internal/server/gql/**/*.graphql, gqlgen.yml"
---

# Ent And GraphQL Rules

## Ent

1. If you change any Ent schema or GraphQL schema, run `make generate`.
2. If you add or update a struct used by GraphQL objects, update the mapping in `gqlgen.yml`.
3. Use `enttest.NewEntClient(t, \"sqlite3\", \"file:ent?mode=memory&_fk=0\")` for Ent tests.
4. Do not edit `ent.graphql` directly; add GraphQL schema in the appropriate non-generated schema file.

## GraphQL

1. Add or change fields by editing `*.graphql` schema files first, then regenerate code with `make generate`.
2. In generated resolver files, only edit generated method bodies. Do not add custom helpers, types, or methods outside those bodies.
3. After running `make generate`, implement any newly required resolvers under `internal/server/gql/`.
4. Prefer GraphQL-side filtering inputs instead of moving filtering logic to the frontend.
5. If a Go field uses a string enum type, prefer a GraphQL `enum` plus `gqlgen.yml` mapping instead of GraphQL `String` with manual conversions.
6. For object or input fields backed by the same Go struct, prefer schema and `gqlgen.yml` mappings that let gqlgen bind directly before adding manual field resolvers.

## Schema Changes

1. Do not write manual migration SQL files for normal schema changes.
2. Update `internal/ent/schema/*.go`, run `make generate`, and let Ent-managed migrations handle the rest.
