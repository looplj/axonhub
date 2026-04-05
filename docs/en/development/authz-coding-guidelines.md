# Authz Package Usage Guidelines

This document defines the usage guidelines for the `internal/authz` package, which implements the Ent Privacy permission governance solution.

## Background

According to the [Ent Privacy Governance Plan](./ent-privacy-governance-plan.md), we introduced the `internal/authz` package to provide a controlled permission bypass mechanism, replacing the direct use of `privacy.DecisionContext(ctx, privacy.Allow)`.

## Core Principles

1. **Single Principal**: Each request can have only one authorization principal
2. **Controlled Bypass**: All bypass operations must use the API provided by the `authz` package
3. **Scope Isolation**: Bypass should be limited to the minimum operation scope
4. **Auditable**: All bypass operations are recorded in audit logs

## API Overview

### Principal (Authorization Subject)

```go
// Principal types
PrincipalTypeSystem  // System principal (background tasks)
PrincipalTypeUser    // User principal
PrincipalTypeAPIKey  // API Key principal

// Create principal context
ctx = authz.NewSystemContext(ctx)           // System principal
ctx = authz.NewUserContext(ctx, userID)     // User principal
ctx = authz.NewAPIKeyContext(ctx, apiKeyID, projectID) // API Key principal

// Set principal (with conflict detection)
ctx, err := authz.WithPrincipal(ctx, principal)

// Get principal
p, ok := authz.GetPrincipal(ctx)
p := authz.MustGetPrincipal(ctx)  // panics if not exists
```

### Bypass (Controlled Bypass)

```go
// Method 1: Closure pattern (recommended)
// Limit bypass to the minimum operation scope
result, err := authz.RunWithBypass(ctx, "quota-check", func(bypassCtx context.Context) (T, error) {
    return client.Request.Query().Where(...).Count(bypassCtx)
})

// Method 2: Explicitly create bypass context
// Use only when necessary, variable must be named bypassCtx
bypassCtx := authz.WithBypassPrivacy(ctx, "auth-lookup")
result, err := client.User.Get(bypassCtx, id)
```

### Scope Authorization Decision (Scope-Gated Decision)

```go
// Method 1: Closure pattern (recommended)
// Limit scope decision to the minimum operation scope
result, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadDashboard, func(ctx context.Context) (*DashboardOverview, error) {
    return buildDashboardOverview(ctx, client)
})

// Method 2: Explicitly create scope decision context
// Use only when necessary
scopeCtx := authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)
result, err := client.Request.Query().All(scopeCtx)

// Pure check function (does not inject DecisionContext, used for business branch logic)
if authz.HasScope(ctx, scopes.ScopeAdmin) {
    // Execute admin operations
}
```

## Coding Guidelines

### 1. Prohibit Direct Use of privacy.Allow

**Prohibited** (unless in allowlist):
```go
// Wrong! Direct use outside authz package is prohibited
ctx = privacy.DecisionContext(ctx, privacy.Allow)
```

**Correct**:
```go
// Use the controlled bypass mechanism provided by the authz package
result, err := authz.RunWithBypass(ctx, "reason", func(bypassCtx context.Context) (T, error) {
    // operations
})
```

### 2. Prefer RunWithBypass Closure Pattern

**Recommended**:
```go
// Bypass only covers necessary queries
count, err := authz.RunWithBypass(ctx, "quota-request-count", func(ctx context.Context) (int, error) {
    return client.Request.Query().Where(...).Count(ctx)
})
```

**Not Recommended**:
```go
// Bypass context passed to the entire function, easy to spread
bypassCtx := authz.WithBypassPrivacy(ctx, "quota-check")
result := s.doSomethingComplex(bypassCtx)
```

### 3. Variable Naming Convention

When `WithBypassPrivacy` must be used, the variable **must** be named `bypassCtx`, and assigning to `ctx` is prohibited:

```go
// Correct
bypassCtx := authz.WithBypassPrivacy(ctx, "reason")

// Wrong! Will cause bypass to spread along the call chain
ctx = authz.WithBypassPrivacy(ctx, "reason")
```

### 4. Provide Stable Reason

All bypass usage must carry a concise and stable reason (used for audit aggregation):

```go
// Good reasons: stable, aggregatable, semantic
authz.RunWithBypass(ctx, "quota-check", ...)
authz.RunWithBypass(ctx, "auth-lookup", ...)
authz.RunWithBypass(ctx, "permission-check", ...)

// Avoid reasons: containing dynamic values
authz.RunWithBypass(ctx, fmt.Sprintf("quota-check-%d", userID), ...) // Don't do this
```

### 5. Background Tasks Must Declare System Principal

```go
// Before (direct bypass, no principal declaration)
ctx = ent.NewContext(ctx, svc.db)
ctx = privacy.DecisionContext(ctx, privacy.Allow)

// After (explicit System principal + controlled bypass)
ctx = ent.NewContext(ctx, svc.db)
ctx = authz.NewSystemContext(ctx)
ctx = authz.WithBypassPrivacy(ctx, "provider-quota-check")
```

### 6. Principal Conflict Detection

Middleware must ensure Principal uniqueness through `authz.WithPrincipal`:

```go
// Set principal in middleware
principal := authz.Principal{
    Type:   authz.PrincipalTypeUser,
    UserID: &user.ID,
}
ctx, err := authz.WithPrincipal(ctx, principal)
if err != nil {
    // Principal conflict, reject request
    return nil, fmt.Errorf("principal conflict: %w", err)
}
```

### 7. File Naming Convention

- Prefer calling `authz.WithBypassPrivacy` / `authz.RunWithBypass` in `*_internal.go` files
- `*_internal.go` files are used to carry orchestration logic for "system internal use cases/background jobs/internal validation"
- For files like `system.go` with complex scenarios and dense bypass calls, direct use of `authz` primitives is allowed, managed through lint allowlist

### 8. Difference Between Bypass and Scope Decision

| Dimension | `WithBypassPrivacy` | `WithScopeDecision` |
|-----------|---------------------|---------------------|
| Semantics | Bypass permission rules | Authorization decision based on scope |
| Condition | Unconditional Allow | Has scope → Allow, no scope → Deny |
| Applicable Scenarios | System internal operations, internal queries | Business authorization in request chain |
| Principal Requirements | Usually requires System principal | Supports User / APIKey / System |
| Audit Identifier | reason string | scope slug |
| Scope Isolation | `RunWithBypass` closure | `RunWithScopeDecision` closure |

## Common Scenario Examples

### Scenario 1: Internal Statistics in Quota Check

```go
func (s *QuotaService) CheckQuota(ctx context.Context, apiKeyID int) error {
    // Get current request principal
    p, ok := authz.GetPrincipal(ctx)
    if !ok {
        return fmt.Errorf("no principal")
    }

    // Verify apiKeyID matches request principal
    if p.IsAPIKey() && p.APIKeyID != nil && *p.APIKeyID != apiKeyID {
        return fmt.Errorf("api key mismatch")
    }

    // Use RunWithBypass for internal statistics query
    count, err := authz.RunWithBypass(ctx, "quota-request-count", func(bypassCtx context.Context) (int, error) {
        return s.client.Request.Query().
            Where(request.APIKeyID(apiKeyID)).
            Count(bypassCtx)
    })
    if err != nil {
        return err
    }

    // Continue quota check logic...
}
```

### Scenario 2: Permission Check Auxiliary Query

```go
func (v *PermissionValidator) HasPermission(ctx context.Context, userID int, permission string) (bool, error) {
    // Use RunWithBypass to query permission data
    roles, err := authz.RunWithBypass(ctx, "permission-check", func(bypassCtx context.Context) ([]*ent.Role, error) {
        return v.client.User.Query().
            Where(user.ID(userID)).
            QueryRoles().
            All(bypassCtx)
    })
    if err != nil {
        return false, err
    }

    // Validate permissions...
}
```

### Scenario 3: Background GC Task

```go
func (gc *GC) Run(ctx context.Context) error {
    // Explicitly declare System principal
    ctx = authz.NewSystemContext(ctx)

    // Use RunWithBypass to execute cleanup
    deleted, err := authz.RunWithBypass(ctx, "gc-old-requests", func(bypassCtx context.Context) (int, error) {
        return gc.client.Request.Delete().
            Where(request.CreatedAtLT(cutoff)).
            Exec(bypassCtx)
    })
    if err != nil {
        return err
    }

    log.Info(ctx, "GC completed", log.Int("deleted", deleted))
    return nil
}
```

### Scenario 4: Dashboard Scope Authorization

```go
func (r *dashboardResolver) Overview(ctx context.Context) (*DashboardOverview, error) {
    // Use RunWithScopeDecision to limit scope decision scope
    result, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadDashboard, func(ctx context.Context) (*DashboardOverview, error) {
        return buildDashboardOverview(ctx, r.client)
    })
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

## Testing

### Test Principal Setup

```go
func TestSomething(t *testing.T) {
    ctx := authz.NewSystemContext(context.Background())
    // or
    ctx := authz.NewUserContext(context.Background(), 123)

    // Test code...
}
```

### Test Principal Conflict

```go
func TestPrincipalConflict(t *testing.T) {
    ctx := authz.NewUserContext(context.Background(), 123)

    // Attempting to set a different principal should fail
    apiKeyID := 456
    _, err := authz.WithPrincipal(ctx, authz.Principal{
        Type:     authz.PrincipalTypeAPIKey,
        APIKeyID: &apiKeyID,
    })
    if err == nil {
        t.Error("expected conflict error")
    }
}
```

## Migration Check

Run the following command to check for illegal `privacy.DecisionContext(...Allow)` calls:

```bash
make lint-privacy
```

Or check automatically in CI:

```bash
make lint
```

## Allowlist

The following locations allow direct use of `privacy.DecisionContext(...Allow)`:

1. `internal/authz/*` - Controlled bypass mechanism implementation

The following locations allow calling `authz` bypass API (e.g., `WithBypassPrivacy`, `RunWithBypass`):

1. `internal/authz/*` - Framework implementation
2. `*_internal.go` - Internal use case files
3. `internal/server/gql/*.resolvers.go` - GraphQL Resolver files
4. `internal/server/biz/system_*.go` - System service files
5. `*_test.go` - Test files
6. `internal/ent/migrate/datamigrate/*` - Data migration scripts
7. The following files (special cases can be added to allowlist):
   - `internal/server/orchestrator/orchestrator.go`
   - `internal/server/biz/auth.go`
   - `internal/server/biz/quota.go`
   - `internal/server/biz/permission_validator.go`
   - `internal/server/biz/prompt.go`
   - `internal/server/biz/system.go`

## Related Documentation

- [Ent Privacy Governance Plan](./ent-privacy-governance-plan.md)
- `internal/authz/principal.go` - Principal implementation
- `internal/authz/bypass.go` - Bypass mechanism implementation
- `internal/authz/scope.go` - Scope authorization decision implementation
