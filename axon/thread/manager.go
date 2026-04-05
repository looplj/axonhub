package thread

import (
	"sync"

	"github.com/looplj/axonhub/axon/agent"
)

// Manager manages thread lifecycle and provides higher-level operations
// on top of a Store implementation.
type Manager struct {
	store Store
	mu    sync.RWMutex
}

// NewManager creates a new Manager backed by the given Store.
func NewManager(store Store) *Manager {
	return &Manager{
		store: store,
	}
}

// GetOrCreate returns an existing thread or creates a new one with the given ID.
func (m *Manager) GetOrCreate(id string) *Thread {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, err := m.store.Get(id)
	if err == nil {
		return sess
	}

	sess = NewThread(id)
	_ = m.store.Save(sess)
	return sess
}

// AddMessage adds a message to the specified thread, creating the thread if needed.
func (m *Manager) AddMessage(threadID string, msg agent.Message) {
	sess := m.GetOrCreate(threadID)
	sess.AddMessage(msg)

	m.mu.RLock()
	defer m.mu.RUnlock()

	_ = m.store.Save(sess)
}

// GetHistory returns the message history for a thread. Returns nil if the thread
// does not exist.
func (m *Manager) GetHistory(threadID string) []agent.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, err := m.store.Get(threadID)
	if err != nil {
		return nil
	}
	return sess.GetMessages()
}

// GetSummary returns the thread summary. Returns an empty string if the thread
// does not exist.
func (m *Manager) GetSummary(threadID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, err := m.store.Get(threadID)
	if err != nil {
		return ""
	}
	return sess.GetSummary()
}

// SetSummary sets the thread summary, creating the thread if needed.
func (m *Manager) SetSummary(threadID string, summary string) {
	sess := m.GetOrCreate(threadID)
	sess.SetSummary(summary)

	m.mu.RLock()
	defer m.mu.RUnlock()

	_ = m.store.Save(sess)
}

// TruncateHistory truncates the thread messages, keeping only the last n.
func (m *Manager) TruncateHistory(threadID string, keepLast int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, err := m.store.Get(threadID)
	if err != nil {
		return
	}
	sess.TruncateMessages(keepLast)
	_ = m.store.Save(sess)
}

// Delete removes a thread from the store.
func (m *Manager) Delete(threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.store.Delete(threadID)
}
