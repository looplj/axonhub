package contexts

import "context"

type requestRecordObserverKey struct{}

// WithRequestRecordObserver associates the HTTP handler with a persisted request.
func WithRequestRecordObserver(ctx context.Context, observer func(int)) context.Context {
	return context.WithValue(ctx, requestRecordObserverKey{}, observer)
}

func NotifyRequestRecord(ctx context.Context, id int) {
	if observer, ok := ctx.Value(requestRecordObserverKey{}).(func(int)); ok {
		observer(id)
	}
}
