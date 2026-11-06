package authz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
)

// RunAsHost runs a fixed host operation for a caller that has already been authorized.
//
// fn receives a System bypass context detached from the caller's authority: it keeps
// only client (normally the caller's transaction client), cancellation and deadline.
// fn must return data and must not leak its context. Use it for fixed reads or writes
// the host owns, never to run arbitrary caller-supplied callbacks.
func RunAsHost[T any](ctx context.Context, client *ent.Client, reason string, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	hostCtx, cancel := context.WithCancel(context.Background())
	if deadline, ok := ctx.Deadline(); ok {
		cancel()
		hostCtx, cancel = context.WithDeadline(context.Background(), deadline)
	}
	stop := context.AfterFunc(ctx, cancel)
	defer func() {
		stop()
		cancel()
	}()

	p, _ := GetPrincipal(ctx)
	log.Debug(ctx, "authz: host operation", log.String("principal", p.String()), log.Int("plugin_installation_id", p.PluginInstallationID), log.String("reason", reason))

	return fn(WithSystemBypass(ent.NewContext(hostCtx, client), reason))
}
