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
7. **Adding nested object fields**: When adding a nested object field (e.g., `settings.rateLimit`):
   - Define both Input type (e.g., `ChannelRateLimitInput`) and Output type (e.g., `ChannelRateLimit`) in the GraphQL schema
   - Add the field to both the Input type (e.g., `ChannelSettingsInput`) and Output type (e.g., `ChannelSettings`)
   - Add type mappings in `gqlgen.yml` for both Input and Output types to map to the same Go struct
   - Update frontend GraphQL queries to include the new field
   - Run `make generate` to regenerate code
   - Example workflow:
     ```graphql
     # 1. Define types in axonhub.graphql
     input ChannelRateLimitInput {
       rpm: Int
       tpm: Int
     }
     type ChannelRateLimit {
       rpm: Int
       tpm: Int
     }

     # 2. Add to both Input and Output types
     input ChannelSettingsInput {
       # ... existing fields
       rateLimit: ChannelRateLimitInput
     }
     type ChannelSettings {
       # ... existing fields
       rateLimit: ChannelRateLimit
     }
     ```
     ```yaml
     # 3. Add mappings in gqlgen.yml
     ChannelRateLimit:
       model:
         - github.com/looplj/axonhub/internal/objects.ChannelRateLimit
     ChannelRateLimitInput:
       model:
         - github.com/looplj/axonhub/internal/objects.ChannelRateLimit
     ```

## Schema Changes

1. Do not write manual migration SQL files for normal schema changes.
2. Update `internal/ent/schema/*.go`, run `make generate`, and let Ent-managed migrations handle the rest.
