---
alwaysApply: false
globs: "internal/server/biz/**/*.go"
---

# Biz Service Rules

1. Do not add nil checks for dependency services that are initialized by the framework.
2. Use `contexts.GetUser(ctx)` to read the current user. Do not read ad hoc values like `ctx.Value(\"user_id\")`.
3. Use `RunInTransaction` for operations that modify multiple tables in one logical change.
4. Check Ent schema delete behavior before manually deleting related rows. If the relation uses `OnDelete(ent.Cascade)`, do not duplicate the cascade in biz code.
