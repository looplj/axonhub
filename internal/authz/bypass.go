package authz

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
)

// bypassKey is an unexported key type to prevent external forgery.
type bypassKey struct{}

// bypassInfo stores bypass metadata.
type bypassInfo struct {
	Reason    string
	Timestamp time.Time
	Principal Principal
}

// WithBypassPrivacy creates a local bypass context.
// Only Principal=System or authenticated internal operations are allowed to call.
// reason must be a stable audit identifier (e.g., "quota-check", "auth-lookup").
func WithBypassPrivacy(ctx context.Context, reason string) (context.Context, error) {
	p, ok := GetPrincipal(ctx)
	if !ok {
		return nil, fmt.Errorf("authz: WithBypassPrivacy requires a principal in context")
	}

	if !p.IsSystem() && !p.IsTest() {
		return nil, fmt.Errorf("authz: WithBypassPrivacy requires system or test principal, got %s", p.String())
	}

	info := bypassInfo{
		Reason:    reason,
		Timestamp: time.Now(),
		Principal: p,
	}

	// Record audit log
	recordBypassAudit(ctx, info)

	ctx = context.WithValue(ctx, bypassKey{}, info)
	// Only here convert capability to Ent-recognizable allow context
	return privacy.DecisionContext(ctx, privacy.Allow), nil
}

// RunWithBypass executes bypass operation within a closure, limiting bypass scope.
// Recommended to use this method to prevent bypass context from spreading along the call chain.
//
// Example usage:
//
//	count, err := authz.RunWithBypass(ctx, "quota-request-count", func(ctx context.Context) (int, error) {
//	    return client.Request.Query().Where(...).Count(ctx)
//	})
func RunWithBypass[T any](ctx context.Context, reason string, fn func(ctx context.Context) (T, error)) (T, error) {
	bypassCtx, err := WithBypassPrivacy(ctx, reason)
	if err != nil {
		var zero T
		return zero, err
	}

	return fn(bypassCtx)
}

// GetBypassInfo retrieves current bypass information.
// Used for audit and debugging.
func GetBypassInfo(ctx context.Context) (bypassInfo, bool) {
	info, ok := ctx.Value(bypassKey{}).(bypassInfo)
	return info, ok
}

// IsBypassActive checks if current context is in bypass state.
func IsBypassActive(ctx context.Context) bool {
	_, ok := ctx.Value(bypassKey{}).(bypassInfo)
	return ok
}

// bypassAuditRecord represents a bypass audit record.
type bypassAuditRecord struct {
	Timestamp   time.Time
	Principal   string
	Reason      string
	Operation   string
	Entity      string
	Description string
}

// auditLogger is the bypass audit logger.
// Can be customized via SetAuditLogger.
var auditLogger func(ctx context.Context, record bypassAuditRecord)

// SetAuditLogger sets a custom audit logger.
// If not set, default standard log output is used.
func SetAuditLogger(fn func(ctx context.Context, record bypassAuditRecord)) {
	auditLogger = fn
}

// recordBypassAudit records bypass audit log.
func recordBypassAudit(ctx context.Context, info bypassInfo) {
	record := bypassAuditRecord{
		Timestamp:   info.Timestamp,
		Principal:   info.Principal.String(),
		Reason:      info.Reason,
		Operation:   "bypass",
		Entity:      "privacy",
		Description: fmt.Sprintf("Privacy bypass triggered: reason=%s, principal=%s", info.Reason, info.Principal.String()),
	}

	if auditLogger != nil {
		auditLogger(ctx, record)
	} else {
		// Default uses standard log
		log.Debug(ctx, "authz: privacy bypass",
			log.String("principal", record.Principal),
			log.String("reason", record.Reason),
			log.String("operation", record.Operation),
		)
	}
}

// RequirePrincipal checks if a principal exists, otherwise returns error.
func RequirePrincipal(ctx context.Context) error {
	_, ok := GetPrincipal(ctx)
	if !ok {
		return fmt.Errorf("authz: no principal in context")
	}

	return nil
}
