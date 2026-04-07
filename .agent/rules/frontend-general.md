---
alwaysApply: false
globs: "frontend/**/*.ts, frontend/**/*.tsx"
---

# Frontend General Rules

1. Do not restart the frontend development server; it is already managed.
2. Use `pnpm` as the package manager.
3. Do not run frontend lint or build commands unless the user explicitly asks.
4. Prefer GraphQL input filters over client-side filtering when data should be filtered by the API.
5. When adding fields used by the UI, update the relevant GraphQL query and schema together.
6. Search filters should use debounce to avoid excessive requests.
7. When adding a new feature page, also add the corresponding route and sidebar entry if the feature should be navigable.
8. Use `extractNumberID` from `frontend/src/lib/utils.ts` to extract integer IDs from GUID values.
9. Respect page scoping semantics:
   Project-level pages must explicitly pass project context such as `projectId` or `X-Project-ID`.
   Admin-level pages must not implicitly inherit the current project unless the feature is intentionally project-scoped.
10. The app is client-side only; SSR compatibility is not required unless the code already depends on it.
