package plugin

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/authz/identity"
	"github.com/looplj/axonhub/internal/contexts"
)

type callContextKey struct{}

// Call is the plugin identity fixed for one page GraphQL request.
// InstallationID is plugins.id from the row resolved at the start of the request.
type Call struct {
	ManifestID            string
	InstallationID        int
	KVWritable            bool // The caller had write_plugins before delegation.
	CanReadCatalogSummary bool // The site-scoped reader role before plugin delegation.
}

// WithCall stores the plugin call on ctx.
func WithCall(ctx context.Context, call Call) context.Context {
	return context.WithValue(ctx, callContextKey{}, call)
}

// CallFromContext returns the plugin call stored by the page GraphQL endpoint.
func CallFromContext(ctx context.Context) (Call, bool) {
	call, ok := ctx.Value(callContextKey{}).(Call)
	return call, ok
}

func (call Call) matchesPrincipal(ctx context.Context) bool {
	p, _ := authz.GetPrincipal(ctx)
	user, ok := contexts.GetUser(ctx)
	_, hasAPIKey := contexts.GetAPIKey(ctx)
	return !hasAPIKey && ok && user != nil && identity.ValidPlugin(ctx, user.ID) && call.InstallationID > 0 && p.PluginInstallationID == call.InstallationID
}
