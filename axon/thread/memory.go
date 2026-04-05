package thread

import (
	"fmt"
	"sync"

	"github.com/samber/lo"
)

// MemoryStore is a thread-safe in-memory implementation of Store.
type MemoryStore struct {
	threads map[string]*Thread
	mu      sync.RWMutex
}

// NewMemoryStore creates a new empty in-memory thread store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		threads: make(map[string]*Thread),
	}
}

// Get retrieves a thread by ID. Returns an error if the thread does not exist.
func (s *MemoryStore) Get(id string) (*Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.threads[id]
	if !ok {
		return nil, fmt.Errorf("thread not found: %s", id)
	}
	return sess, nil
}

// Save persists a thread to the in-memory store.
func (s *MemoryStore) Save(thread *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.threads[thread.ID] = thread
	return nil
}

// Delete removes a thread from the in-memory store.
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.threads, id)
	return nil
}

// List returns all threads in the store.
func (s *MemoryStore) List() ([]*Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return lo.Values(s.threads), nil
}
