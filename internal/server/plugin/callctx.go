package plugin

import "context"

type callContextKey struct{}

// Call is the plugin identity fixed for one page GraphQL request.
// InstallationID is plugins.id from the row resolved at the start of the request.
type Call struct {
	ManifestID     string
	InstallationID int
	KVWritable     bool
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
