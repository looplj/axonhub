package contexts

import "context"

// ForkRequest inherits authentication, tenant and tracing identity without sharing
// mutable per-request bookkeeping. Cancellation and other context values remain
// inherited. Internal requests must not replace the outer HTTP request's record.
func ForkRequest(ctx context.Context) context.Context {
	parent := getContainer(ctx)
	parent.mu.RLock()
	child := &contextContainer{
		ProjectID: parent.ProjectID,
		TraceID:   parent.TraceID,
		APIKey:    parent.APIKey,
		User:      parent.User,
		Source:    parent.Source,
		Thread:    parent.Thread,
		Trace:     parent.Trace,
	}
	parent.mu.RUnlock()
	ctx = context.WithValue(ctx, containerContextKey, child)
	return WithRequestRecordObserver(ctx, func(int) {})
}
