---
alwaysApply: false
globs: "**/*.go"
---

# Cache Compatibility Rules

1. Treat cache value shape changes as compatibility-sensitive. Changing cache type parameters or wrapped JSON structure can break existing Redis or multi-level cache data after upgrade.
2. When changing the serialized shape of cached data, either version the cache key or add a stale-entry guard on reads and fall back to the database.
3. Do not assume old cached JSON can deserialize safely into a new wrapper type.
