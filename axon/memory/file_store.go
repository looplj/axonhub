package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/pkg/grep"
)

// FileStore implements memory storage based on file system
type FileStore struct {
	dir      string         // storage directory
	mu       sync.RWMutex   // read-write lock
	searcher *grep.Searcher // searcher
}

// NewFileStore creates a new file store instance
func NewFileStore(dir string) *FileStore {
	return &FileStore{
		dir:      dir,
		searcher: grep.NewSearcher(dir),
	}
}

// filePath returns the file path for the given path
func (s *FileStore) filePath(path string) string {
	return filepath.Join(s.dir, path+".jsonl")
}

// generateID generates a unique identifier
func generateID() string {
	return uuid.New().String()[:8]
}

// Add adds a new entry to the store
func (s *FileStore) Add(_ context.Context, path, content, source string) error {
	filePath := s.filePath(path)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := Entry{
		ID:        generateID(),
		Path:      path,
		Content:   content,
		Source:    source,
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open memory file: %w", err)
	}
	defer f.Close()

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write memory: %w", err)
	}

	return nil
}

// Get retrieves entry content by path
func (s *FileStore) Get(_ context.Context, path string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, err := os.Open(s.filePath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("memory not found: %s", path)
		}
		return "", fmt.Errorf("failed to open memory file: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(entry.Content)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read memory: %w", err)
	}

	return sb.String(), nil
}

// Search searches entries by query
func (s *FileStore) Search(ctx context.Context, query string, limit int) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, err := s.searcher.Search(ctx, grep.Options{
		Pattern:    query,
		OutputMode: "content",
		HeadLimit:  limit,
		Glob:       "*.jsonl",
		MaxLineLen: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search memory: %w", err)
	}

	if result.Text == "No results found." {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSuffix(result.Text, "\n"), "\n")
	var entries []Entry
	seen := make(map[string]bool)

	for _, line := range lines {
		if strings.HasPrefix(line, "--") || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		jsonContent := parts[2]
		var entry Entry
		if err := json.Unmarshal([]byte(jsonContent), &entry); err != nil {
			continue
		}

		if !seen[entry.ID] {
			seen[entry.ID] = true
			entries = append(entries, entry)
			if limit > 0 && len(entries) >= limit {
				return entries, nil
			}
		}
	}

	return entries, nil
}

// List lists all entries with a limit
func (s *FileStore) List(_ context.Context, limit int) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var allEntries []Entry
	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		entriesFromFile, err := s.readAllEntries(path)
		if err != nil {
			return nil
		}
		allEntries = append(allEntries, entriesFromFile...)
		if limit > 0 && len(allEntries) >= limit {
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read memory directory: %w", err)
	}

	return allEntries, nil
}

// readAllEntries reads all entries from a file
func (s *FileStore) readAllEntries(filePath string) ([]Entry, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// Delete deletes an entry by path
func (s *FileStore) Delete(_ context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.filePath(path)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(filePath)
}
