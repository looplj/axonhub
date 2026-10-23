package lifecycle

import (
	"context"
	"errors"
	"sync"
)

type contextKey struct{}

// Manager owns cancellation that is independent from net/http connection
// lifetime. It is attached to request contexts by the server and canceled
// before HTTP shutdown starts.
type Manager struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	once   sync.Once
}

func New() *Manager {
	ctx, cancel := context.WithCancelCause(context.Background())
	return &Manager{ctx: ctx, cancel: cancel}
}

func (m *Manager) Context() context.Context {
	if m == nil {
		return context.Background()
	}
	return m.ctx
}

func (m *Manager) Attach(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, m)
}

func FromContext(ctx context.Context) *Manager {
	manager, _ := ctx.Value(contextKey{}).(*Manager)
	return manager
}

func (m *Manager) Shutdown(cause error) {
	if m == nil {
		return
	}
	if cause == nil {
		cause = errors.New("server shutdown")
	}
	m.once.Do(func() { m.cancel(cause) })
}
