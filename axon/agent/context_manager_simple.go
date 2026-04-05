package agent

import (
	"context"
	"sync"
)

// SimpleContextManager keeps in-memory history and preserves current behavior.
type SimpleContextManager struct {
	mu       sync.RWMutex
	messages []Message
}

func NewSimpleContextManager(initial []Message) *SimpleContextManager {
	m := &SimpleContextManager{}
	if len(initial) > 0 {
		m.messages = cloneMessages(initial)
	}
	return m
}

func (m *SimpleContextManager) AddMessages(ctx context.Context, msgs ...Message) {
	if len(msgs) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs...)
}

func (m *SimpleContextManager) SetMessages(ctx context.Context, msgs []Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = cloneMessages(msgs)
}

func (m *SimpleContextManager) Messages(ctx context.Context) []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneMessages(m.messages)
}

func (m *SimpleContextManager) ClearMessages(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

func (m *SimpleContextManager) BuildMessages(ctx context.Context) []Message {
	return m.Messages(ctx)
}

func (m *SimpleContextManager) Snapshot() ContextManagerState {
	return EmptyContextState()
}

var _ ContextManager = (*SimpleContextManager)(nil)
