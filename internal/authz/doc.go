// Package authz implements the Ent Privacy governance mechanism,
// providing controlled privacy bypass and a single-principal authorization model.
//
// Core concepts:
//
//   - Principal: A single authorization identity per request (System/User/APIKey).
//     Set via NewSystemContext, NewUserContext, NewAPIKeyContext, or WithPrincipal.
//
//   - Bypass: Controlled privacy bypass via RunWithBypass (closure, preferred)
//     or WithBypassPrivacy (explicit context). All bypass operations are audited.
//
//   - Scope Decision: Scope-aware authorization via WithScopeDecision or
//     RunWithScopeDecision, supporting all principal types. Use HasScope for
//     pure scope checks without injecting privacy decisions.
//
// Usage rules:
//
//  1. Never use privacy.DecisionContext directly outside this package.
//  2. Prefer RunWithBypass / RunWithScopeDecision closures to limit scope.
//  3. When using WithBypassPrivacy, assign to bypassCtx, never ctx.
//  4. All bypass reasons must be stable strings for audit aggregation.
//  5. Background tasks must declare System principal via NewSystemContext.
package authz
