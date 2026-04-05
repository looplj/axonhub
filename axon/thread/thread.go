package thread

import (
	"sync"
	"time"

	"github.com/looplj/axonhub/axon/agent"
)

// Thread represents a conversation thread with message history.
type Thread struct {
	ID        string            `json:"id"`
	Messages  []agent.Message   `json:"messages"`
	Summary   string            `json:"summary,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	mu        sync.RWMutex
}

// NewThread creates a new thread with the given ID.
func NewThread(id string) *Thread {
	now := time.Now()
	return &Thread{
		ID:        id,
		Messages:  make([]agent.Message, 0),
		Metadata:  make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage appends a message to the thread. If the message has no ID,
// a new UUID is generated. The thread's UpdatedAt timestamp is refreshed.
func (s *Thread) AddMessage(msg agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetMessages returns a copy of all messages in the thread.
func (s *Thread) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]agent.Message, len(s.Messages))
	copy(out, s.Messages)
	return out
}

// SetSummary sets the conversation summary.
func (s *Thread) SetSummary(summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Summary = summary
	s.UpdatedAt = time.Now()
}

// GetSummary returns the conversation summary.
func (s *Thread) GetSummary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Summary
}

// TruncateMessages keeps only the last n messages, removing older ones.
func (s *Thread) TruncateMessages(keepLast int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if keepLast < 0 {
		keepLast = 0
	}
	if keepLast >= len(s.Messages) {
		return
	}

	s.Messages = append([]agent.Message(nil), s.Messages[len(s.Messages)-keepLast:]...)
	s.UpdatedAt = time.Now()
}

// MessageCount returns the number of messages in the thread.
func (s *Thread) MessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Messages)
}
