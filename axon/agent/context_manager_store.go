package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ContextManagerStore interface {
	Load(ctx context.Context) (ContextManagerState, []Message, error)
	Save(ctx context.Context, state ContextManagerState, messages []Message, archivedMessages []Message) error
}

type ContextManagerMemoryStore struct {
	mu    sync.RWMutex
	state ContextManagerState
	msgs  []Message
}

func NewContextManagerMemoryStore() *ContextManagerMemoryStore {
	return &ContextManagerMemoryStore{state: emptyContextState()}
}

func (s *ContextManagerMemoryStore) Load(_ context.Context) (ContextManagerState, []Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return copyContextState(s.state), cloneMessages(s.msgs), nil
}

func (s *ContextManagerMemoryStore) Save(_ context.Context, state ContextManagerState, messages []Message, _ []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = copyContextState(state)
	s.msgs = cloneMessages(messages)
	return nil
}

type ContextManagerFileStore struct {
	dir string
	mu  sync.Mutex
}

type contextManagerMessagesFile struct {
	State    ContextManagerState `json:"state"`
	Messages []Message           `json:"messages"`
}

type contextManagerArchiveItem struct {
	File         string    `json:"file"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type contextManagerIndexFile struct {
	Archives  []contextManagerArchiveItem `json:"archives"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

func NewContextManagerFileStore(dir string) *ContextManagerFileStore {
	return &ContextManagerFileStore{dir: dir}
}

func (s *ContextManagerFileStore) Load(_ context.Context) (ContextManagerState, []Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.messagesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return emptyContextState(), nil, nil
		}
		return ContextManagerState{}, nil, fmt.Errorf("read context manager messages: %w", err)
	}

	var persisted contextManagerMessagesFile
	if err := json.Unmarshal(data, &persisted); err != nil {
		return ContextManagerState{}, nil, fmt.Errorf("unmarshal context manager messages: %w", err)
	}

	return copyContextState(persisted.State), cloneMessages(persisted.Messages), nil
}

func (s *ContextManagerFileStore) Save(_ context.Context, state ContextManagerState, messages []Message, archivedMessages []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create context manager directory: %w", err)
	}
	if err := os.MkdirAll(s.archivesDir(), 0o755); err != nil {
		return fmt.Errorf("create context manager archives directory: %w", err)
	}

	now := time.Now().UTC()

	if len(archivedMessages) > 0 {
		archiveFile := fmt.Sprintf("compaction-%d.json", now.UnixNano())
		if err := s.writeJSONAtomic(filepath.Join(s.archivesDir(), archiveFile), cloneMessages(archivedMessages)); err != nil {
			return fmt.Errorf("write context manager archive: %w", err)
		}

		index, err := s.readIndex()
		if err != nil {
			return err
		}
		index.Archives = append(index.Archives, contextManagerArchiveItem{
			File:         filepath.ToSlash(filepath.Join("archives", archiveFile)),
			MessageCount: len(archivedMessages),
			CreatedAt:    now,
		})
		index.UpdatedAt = now
		if err := s.writeJSONAtomic(s.indexPath(), index); err != nil {
			return fmt.Errorf("write context manager index: %w", err)
		}
	} else {
		index, err := s.readIndex()
		if err != nil {
			return err
		}
		index.UpdatedAt = now
		if err := s.writeJSONAtomic(s.indexPath(), index); err != nil {
			return fmt.Errorf("write context manager index: %w", err)
		}
	}

	persisted := contextManagerMessagesFile{
		State:    copyContextState(state),
		Messages: cloneMessages(messages),
	}
	if err := s.writeJSONAtomic(s.messagesPath(), persisted); err != nil {
		return fmt.Errorf("write context manager messages: %w", err)
	}

	return nil
}

func (s *ContextManagerFileStore) messagesPath() string {
	return filepath.Join(s.dir, "messages.json")
}

func (s *ContextManagerFileStore) indexPath() string {
	return filepath.Join(s.dir, "index.json")
}

func (s *ContextManagerFileStore) archivesDir() string {
	return filepath.Join(s.dir, "archives")
}

func (s *ContextManagerFileStore) readIndex() (contextManagerIndexFile, error) {
	var index contextManagerIndexFile
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return contextManagerIndexFile{}, nil
		}
		return contextManagerIndexFile{}, fmt.Errorf("read context manager index: %w", err)
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return contextManagerIndexFile{}, fmt.Errorf("unmarshal context manager index: %w", err)
	}
	return index, nil
}

func (s *ContextManagerFileStore) writeJSONAtomic(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
