# Authz Package

This package implements the core mechanism of the Ent Privacy governance solution, providing controlled permission bypass functionality and a single-principal model.

## Design Goals

1. **Single Principal Model**: Only one authorization principal exists per request (System/User/APIKey)
2. **Controlled Bypass Mechanism**: Business layer is prohibited from directly using `privacy.Allow`; all bypasses must go through controlled entry points
3. **Bypass Scope Isolation**: Bypass context cannot spread indefinitely along the call chain
4. **Auditable and Regression-testable**: All bypass actions are traceable, with anti-regression tests and static checks

## Core Components

### Principal (Authorization Principal)

Defined in [principal.go](./principal.go):

- `PrincipalTypeSystem` - System principal (background tasks, internal operations)
- `PrincipalTypeUser` - User principal
- `PrincipalTypeAPIKey` - API Key principal

Key Features:
- Set-once semantics: Each context can only set Principal once
- Conflict detection: Attempting to set different Principals returns an error
- Independent context key: Does not rely on mutable contextContainer

### Bypass (Controlled Bypass)

Defined in [bypass.go](./bypass.go):

- `WithBypassPrivacy(ctx, reason)` - Create bypass context (variable must be named `bypassCtx`)
- `RunWithBypass(ctx, reason, fn)` - Closure mode, recommended usage
- Automatic audit logging: Every bypass is logged for audit

### Scope Decision (Scope Authorization Decision)

Defined in [scope.go](./scope.go):

- `WithScopeDecision(ctx, scope)` - Inject Allow/Deny decision based on principal scope
- `RunWithScopeDecision(ctx, scope, fn)` - Closure mode, recommended usage
- `HasScope(ctx, scope)` - Pure scope check without injecting DecisionContext
- `RequireScope(ctx, scope)` - Require principal to have specified scope, otherwise return error
- Unified support for three principal types: System (always Allow), User, APIKey

## Usage Examples

### Basic Usage

```go
// Create System principal context (background tasks)
ctx = authz.NewSystemContext(ctx)

// Use closure mode to execute bypass operation
result, err := authz.RunWithBypass(ctx, "quota-check", func(bypassCtx context.Context) (T, error) {
    return client.Request.Query().Count(bypassCtx)
})
```

### Setting Principal in Middleware

```go
principal := authz.Principal{
    Type:   authz.PrincipalTypeUser,
    UserID: &user.ID,
}
ctx, err := authz.WithPrincipal(ctx, principal)
if err != nil {
    return nil, fmt.Errorf("principal conflict: %w", err)
}
```

### Scope Authorization Decision

```go
// Closure mode (recommended): scope decision only covers necessary queries
result, err := authz.RunWithScopeDecision(ctx, scopes.ScopeReadDashboard, func(scopeCtx context.Context) (T, error) {
    return client.Request.Query().All(scopeCtx)
})

// Explicit mode
ctx = authz.WithScopeDecision(ctx, scopes.ScopeReadDashboard)

// Pure scope check (without injecting privacy decision)
if authz.HasScope(ctx, scopes.ScopeWriteChannels) {
    // ...
}

// Require principal to have scope, otherwise return error
if err := authz.RequireScope(ctx, scopes.ScopeWriteSettings); err != nil {
    return err
}
```

## Coding Standards

1. **Prohibited** from directly using `privacy.DecisionContext(ctx, privacy.Allow)` outside of `internal/authz`
2. All unconditional bypasses must go through `authz.WithBypassPrivacy` or `authz.RunWithBypass`
3. All scope authorization decisions must go through `authz.WithScopeDecision` or `authz.RunWithScopeDecision`
4. Pure scope checks (without injecting DecisionContext) use `authz.HasScope`
5. **Prefer closure mode** (`RunWithBypass` / `RunWithScopeDecision`), limiting Allow/Deny to the smallest operation scope
6. When `WithBypassPrivacy` must be used, return value variable must be named `bypassCtx`, never assign to `ctx`
7. All bypass usage must carry concise and stable reasons (for audit aggregation)
8. Middleware must ensure Principal uniqueness via `authz.WithPrincipal`
9. Background tasks must explicitly declare System principal via `authz.NewSystemContext`
10. **Prohibited** from adding new calls to `scopes.WithUserScopeDecision`; new code should uniformly use `authz.WithScopeDecision`

## Related Documentation

- [Privacy Governance Solution](../../docs/zh/development/ent-privacy-governance-plan.md)
- [Usage Guidelines](../../docs/zh/development/authz-coding-guidelines.md)
