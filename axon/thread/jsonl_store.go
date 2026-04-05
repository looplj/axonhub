package thread

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/samber/lo"
)

// jsonlEntry represents a single line in the JSONL thread file.
type jsonlEntry struct {
	Type    string         `json:"type"`
	Message *agent.Message `json:"message,omitempty"`
	Summary *string        `json:"summary,omitempty"`
}

// JSONLStore is a thread-safe file-system based Store that uses append-only JSONL files.
// Each thread is stored as thread-{id}.jsonl in the base directory.
type JSONLStore struct {
	dir string
	mu  sync.RWMutex
}

// NewJSONLStore creates a new JSONL-based thread store, auto-creating the directory if needed.
func NewJSONLStore(dir string) (*JSONLStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}
	return &JSONLStore{dir: dir}, nil
}

func (s *JSONLStore) threadPath(id string) string {
	return filepath.Join(s.dir, "thread-"+id+".jsonl")
}

// Get retrieves a thread by replaying the JSONL file.
func (s *JSONLStore) Get(id string) (*Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readThread(id)
}

// Save persists a thread by rewriting the full JSONL file.
// For normal append operations, use AppendMessage or SetSummary instead.
func (s *JSONLStore) Save(thread *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeFullThread(thread)
}

// Delete removes a thread JSONL file.
func (s *JSONLStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.threadPath(id))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete thread %s: %w", id, err)
	}
	return nil
}

// List returns all threads by reading all JSONL files in the directory.
func (s *JSONLStore) List() ([]*Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read store directory: %w", err)
	}

	jsonlEntries := lo.Filter(entries, func(e os.DirEntry, _ int) bool {
		return !e.IsDir() && strings.HasPrefix(e.Name(), "thread-") && strings.HasSuffix(e.Name(), ".jsonl")
	})

	threads := make([]*Thread, 0, len(jsonlEntries))
	for _, entry := range jsonlEntries {
		id := strings.TrimPrefix(strings.TrimSuffix(entry.Name(), ".jsonl"), "thread-")
		sess, err := s.readThread(id)
		if err != nil {
			return nil, fmt.Errorf("failed to load thread %s: %w", id, err)
		}
		threads = append(threads, sess)
	}

	return threads, nil
}

// AppendMessage appends a single message entry to the JSONL file (append-only).
func (s *JSONLStore) AppendMessage(id string, msg agent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := jsonlEntry{
		Type:    "message",
		Message: &msg,
	}

	return s.appendEntry(id, entry)
}

// AppendSummary appends a summary entry to the JSONL file.
func (s *JSONLStore) AppendSummary(id string, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := jsonlEntry{
		Type:    "summary",
		Summary: &summary,
	}

	return s.appendEntry(id, entry)
}

func (s *JSONLStore) appendEntry(id string, entry jsonlEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry for thread %s: %w", id, err)
	}

	f, err := os.OpenFile(s.threadPath(id), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open thread file %s: %w", id, err)
	}
	defer f.Close()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to append to thread %s: %w", id, err)
	}

	return nil
}

func (s *JSONLStore) readThread(id string) (*Thread, error) {
	path := s.threadPath(id)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("thread not found: %s", id)
		}
		return nil, fmt.Errorf("failed to open thread %s: %w", id, err)
	}
	defer f.Close()

	sess := NewThread(id)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry jsonlEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to parse entry in thread %s: %w", id, err)
		}

		switch entry.Type {
		case "message":
			if entry.Message != nil {
				sess.Messages = append(sess.Messages, *entry.Message)
			}
		case "summary":
			if entry.Summary != nil {
				sess.Summary = *entry.Summary
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read thread %s: %w", id, err)
	}

	return sess, nil
}

func (s *JSONLStore) writeFullThread(thread *Thread) error {
	f, err := os.Create(s.threadPath(thread.ID))
	if err != nil {
		return fmt.Errorf("failed to create thread file %s: %w", thread.ID, err)
	}
	defer f.Close()

	if thread.Summary != "" {
		entry := jsonlEntry{Type: "summary", Summary: &thread.Summary}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}

	for i := range thread.Messages {
		entry := jsonlEntry{Type: "message", Message: &thread.Messages[i]}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}

	return nil
}
