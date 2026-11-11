---
alwaysApply: false
globs: "internal/server/biz/**/*.go"
---

# Biz Service Rules

1. Do not add nil checks for dependency services that are initialized by the framework.
2. Use `contexts.GetUser(ctx)` to read the current user. Do not read ad hoc values like `ctx.Value(\"user_id\")`.
3. Use `RunInTransaction` for operations that modify multiple tables in one logical change.
4. Check Ent schema delete behavior before manually deleting related rows. If the relation uses `OnDelete(ent.Cascade)`, do not duplicate the cascade in biz code.
5. Authorize once at the boundary (resolver or service entry, via `authz.RequireScope` / `WithScopeDecision`); reads the host does for itself inside that boundary must not depend on the caller. Run them through `authz.RunAsHost`, not `WithSystemBypass` on the caller's context, and never add `identity.IsPlugin` branches to choose between the two. See `internal/authz/README.md`.
6. Ordinary settings getters on `SystemService` are host reads and perform no authorization. `SecretKey` is an internal credential accessor and requires a System principal before any cache read. A resolver that exposes a setting must check its required scope itself; a cache hit skips Ent privacy, so the Ent policy on `System` is not a substitute.
