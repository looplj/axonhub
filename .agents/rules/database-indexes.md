---
alwaysApply: false
globs: "internal/ent/schema/**/*.go"
---

# Database Index Rules

## Source of Truth

1. Define indexes in the owning Ent schema's `Indexes()` method. Do not maintain index inventories in the ERD or duplicate them in general documentation.
2. Do not write manual migration SQL for normal index changes. Run `make generate` and let Ent-managed migrations produce the database definition.
3. Treat `internal/ent/schema/` as the authored source of truth and `internal/ent/migrate/schema.go` as generated output.

## Design

1. Add an index only for a concrete query, uniqueness constraint, foreign-key access path, or measured performance problem. Identify the query shape before choosing fields.
2. Check existing primary, unique, single-column, and composite indexes first. Do not add an index whose useful left prefix is already covered unless the new ordering or selectivity has a demonstrated benefit.
3. For composite indexes:
   - Put required equality filters first.
   - Order equality fields by selectivity and useful left-prefix reuse.
   - Put range or ordering fields after equality fields.
   - Avoid low-cardinality fields such as booleans or status values as the leading field unless the data distribution or a partial-index design justifies it.
4. Keep indexes narrow. Do not add response bodies, request bodies, JSON, text payloads, or fields used only as residual filters.
5. Tenant or project predicates must remain in the query even when another indexed identifier logically implies the tenant. Include tenant fields in the index when they materially reduce the scan; an index never replaces authorization predicates.
6. Prefer one index that matches the complete hot-path query over several speculative indexes. Account for write amplification on high-volume tables.

## String and Dialect Safety

1. Index definitions must work with every supported dialect: PostgreSQL, MySQL/TiDB, and SQLite, unless the feature is explicitly dialect-specific.
2. Give indexed string fields a justified `MaxLen` when the domain is bounded. Do not rely on Ent's default string width without checking it.
3. Before adding a composite string index, calculate its worst-case encoded size. For MySQL `utf8mb4`, budget up to four bytes per character and keep the full key within the engine limit. Also avoid PostgreSQL B-tree entries that can approach the per-entry page limit.
4. Do not index unbounded text or JSON with a normal B-tree. Use a bounded identifier, a purpose-built hash/generated field, or a dialect-appropriate index only when the query requires it.
5. MySQL prefix indexes are a last resort for non-unique lookup indexes. Document the selectivity tradeoff, retain the full predicate for exact filtering, and ensure other dialects still have an effective access path.

## Naming and Stability

1. Use an explicit `StorageKey` for application-owned indexes.
2. Use lower snake case and the established form `<table>_by_<field_or_purpose>`, for example `requests_by_api_key_id_created_at`.
3. Keep names at or below 63 bytes so they are valid in both PostgreSQL and MySQL. Use a short purpose name instead of concatenating every field when necessary.
4. Keep index names stable. Renaming an index normally causes a drop and recreate during auto-migration.

## Unique Indexes

1. A unique index must represent a business invariant, not only a query optimization.
2. For soft-deleted entities, follow the existing `deleted_at` uniqueness convention and verify reuse-after-delete behavior.
3. Check `NULL` uniqueness semantics across supported dialects before relying on nullable fields in a unique index.

## Migration and Verification

1. Consider lock duration, disk usage, and write amplification before adding, replacing, or renaming an index on a large table.
2. After changing an Ent index, run `make generate` and inspect the generated migration schema for the exact name, field order, uniqueness, and dialect annotations.
3. Run the relevant database and business-query tests. For a performance-sensitive query, capture `EXPLAIN` or `EXPLAIN ANALYZE` on representative data when an appropriate database is available.
4. Verify that upgrades handle both installations where the old index exists and installations where a previous migration attempt failed before creating it.
