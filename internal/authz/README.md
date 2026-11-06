# Authz Package

This package implements the core mechanism of the Ent Privacy governance solution, providing controlled permission bypass functionality and a single-principal model.

## Design Goals

1. **Single Principal Model**: Only one authorization principal exists per request (System/User/APIKey/Plugin)
2. **Controlled Bypass Mechanism**: Business layer is prohibited from directly using `privacy.Allow`; all bypasses must go through controlled entry points
3. **Bypass Scope Isolation**: Bypass context cannot spread indefinitely along the call chain
4. **Auditable and Regression-testable**: All bypass actions are traceable, with anti-regression tests and static checks

## Core Components

### Principal (Authorization Principal)

Defined in [principal.go](./principal.go):

- `PrincipalTypeSystem` - System principal (background tasks, internal operations)
- `PrincipalTypeUser` - User principal
- `PrincipalTypeAPIKey` - API Key principal
- `PrincipalTypePlugin` - a user delegation bound to one installation and an immutable grant snapshot

Key Features:
- `WithPrincipal` is set-once. Constructors preserve existing internal non-plugin transitions, but reject replacement of a Plugin principal.
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
- Plugin decisions intersect installation grants with user scopes, then behave like a direct user: Allow on success, Deny on failure. Use scope decisions only for controlled internal reads after the check; never hand the Allow context to plugin-supplied callbacks.

## Plugin delegation

`DelegatePlugin(ctx, installationID, grants)` requires an authenticated direct User and rejects inherited privacy Allow/bypass or mixed API-key identity. The host resolves installation/grants and forks mutable request state. Shared identity storage lives in `identity/`; effective business permissions remain in `authz` and `scopes`.

The delegated context carries two pieces of state with separate jobs:

- **Plugin principal** (`identity`): who is calling — installation ID, real user ID and an immutable grant snapshot. Use it for explicit decisions: `RequireDirectUser`, `scopes.IsDirectOwner`, audit.
- **Delegated user view** (`scopes.DelegatedUserView`): the user entity seen through `contexts.GetUser`. Scopes are intersected with grants, `IsOwner` is false on the user and every membership, and roles are removed. The view is pinned in the delegation snapshot, so replacing the container user later cannot restore the real user. Code that reads `IsOwner` or roles directly therefore fails closed for plugins.

Principal constructors (`NewUserContext`, `NewAPIKeyContext`, `NewSystemContext`, `NewTestContext`) panic when they would replace a Plugin principal: that is a host programming invariant violation.

Generic system bypass fails closed for a Plugin principal: the refusal is audited, `RunWithSystemBypass*` returns an error without calling the closure, and `WithSystemBypass` returns a context that keeps the Plugin principal and denies every Ent operation. Ordinary permission failures remain errors; `WithPrincipal` and `WithBypassPrivacy` retain their existing error contracts.

### Host operations

`RunAsHost(ctx, client, reason, fn)` runs a fixed host operation after the caller has been authorized. `fn` receives a System bypass context detached from the caller: it keeps only the Ent client (normally the caller's transaction), cancellation and deadline. Return data only; never return the context or run caller-supplied callbacks. Plugin KV, API key quota usage, request storage lookup, the project profile read for available channels and system settings reads use it, the same way for every caller. Ordinary settings reads are host reads for every caller; settings resolvers authorize at the GraphQL boundary. `SystemService.SecretKey` is an internal credential accessor: it requires a System principal before reading either the cache or the database. User, Owner, API-key and Plugin callers cannot invoke it directly, even with a privacy Allow decision.

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
11. Owner checks reachable from plugin requests use `scopes.IsDirectOwner(ctx)` or `scopes.Effective*Scope`, not `user.IsOwner` / `membership.IsOwner` / roles directly
12. A host operation that must run with System authority on behalf of an authorized plugin uses `authz.RunAsHost`; do not add `IsPlugin` branches around `WithSystemBypass`

## Related Documentation

- [Privacy Governance Solution](../../docs/zh/development/ent-privacy-governance-plan.md)
- [Usage Guidelines](../../docs/zh/development/authz-coding-guidelines.md)

### Personal and owner operations

Personal account operations require `RequireDirectUser` at their business entry point. Private templates use Ent ownership policies, which do not grant plugins implicit access through the delegating user's ID. Owner-only operations require a direct owner; plugin user-management operations cannot change ownership fields. Scoped management operations still check target permissions and every scope or role being granted. GraphQL does not maintain a mutation-name denylist; rejection occurs during field execution, so earlier external side effects in the same operation are not automatically undone.

System bypass denial is identified by `ErrPluginSystemBypass`. The context-returning helper also preserves `privacy.Deny`, and GraphQL maps either bypass form to `FORBIDDEN` with a plugin-specific message. Fixed host maintenance uses the caller's transaction client; commit hooks must be registered on the original context outside `RunAsHost`.
