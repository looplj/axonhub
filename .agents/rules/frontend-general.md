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
11. For any field used by a create/edit form, update both write operations and read operations in the same change:
    mutations must send the field, and the queries used for edit echo, backfill, list refresh, or detail refresh must also return it when the UI depends on that value.
12. 非必需情况下不要使用 `crypto.randomUUID()`；普通随机标识（组件实例、会话、记录键等）优先使用已有的 `nanoid`。前端需要支持 HTTP 局域网访问，不能假定处于 HTTPS 或 localhost 安全上下文。确需 UUID 格式时，使用支持非安全上下文的实现，不要直接依赖 `crypto.randomUUID()`。
