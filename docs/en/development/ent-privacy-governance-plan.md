# Ent Privacy Governance Plan

This document is the English companion for the Ent Privacy governance work referenced by the authorization coding guidelines.

## Purpose

The project uses Ent Privacy as the final guardrail for data access. Over time, direct use of `privacy.DecisionContext(ctx, privacy.Allow)` spread into business paths that should have remained explicitly scoped and auditable.

The governance plan exists to standardize how privileged operations are performed:

- Use a single authorization principal per request
- Replace ad hoc `privacy.Allow` bypasses with controlled `internal/authz` APIs
- Limit bypass scope to the smallest possible operation boundary
- Keep privileged actions auditable and easier to review

## Current Status

The detailed migration inventory and rollout checklist are currently maintained in the Chinese source document:

- [Ent Privacy 权限治理方案（中文）](../../zh/development/ent-privacy-governance-plan.md)

## Related Documentation

- [Authz Package Usage Guidelines](./authz-coding-guidelines.md)
- [Entity Relationship Diagram](./erd.md)
