package grant

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Scope string

const (
	ScopeOnce      Scope = "once"
	ScopeThread    Scope = "thread"
	ScopeWorkspace Scope = "workspace"
)

type Request struct {
	ToolCallID string
	ThreadID   string
	Workspace  string
	ToolName   string
}

type ResourceType string

const (
	ResourcePath    ResourceType = "path"
	ResourceDomain  ResourceType = "domain"
	ResourceCommand ResourceType = "command"
)

type Resource struct {
	Type ResourceType

	Path             string
	WorkspaceRel     string
	OutsideWorkspace bool

	Domain  string
	Command string
}

type Entry struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Scope      Scope     `json:"scope"`
	ThreadID   string    `json:"thread_id,omitempty"`
	Workspace  string    `json:"workspace,omitempty"`
	ToolName   string    `json:"tool_name"`
	Capability string    `json:"capability"`
	Key        string    `json:"key"`
}

// Store persists approval grants and answers whether a request should be allowed
// without prompting the user.
//
// Matching is done against a derived key from (capability, tool name, resources),
// with additional scoping rules:
//   - once: keyed by ToolCallID only; it is consumed on first match. For the same
//     ToolCallID, the store records it only once, and the next permission check
//     for that ToolCallID will pass immediately (and the one-time grant is removed).
//   - thread: keyed by (ThreadID, key)
//   - workspace: keyed by (Workspace, key) and persisted via SaveWorkspace/LoadWorkspace
type Store interface {
	Add(req Request, scope Scope, capability string, resources []Resource)
	Match(req Request, capability string, resources []Resource) bool
	LoadWorkspace(workspace string) error
	SaveWorkspace(workspace string) error
}

type MemoryStore struct {
	mu sync.RWMutex

	once      map[string]struct{}            // tool_call_id
	thread    map[string]map[string]struct{} // threadID -> key
	workspace map[string]map[string]struct{} // workspace -> key

	workspaceFile WorkspaceFileStore
}

type WorkspaceFileStore interface {
	Load(workspace string) (map[string]struct{}, error)
	Save(workspace string, keys map[string]struct{}) error
}

func NewMemoryStore(fileStore WorkspaceFileStore) *MemoryStore {
	return &MemoryStore{
		once:          make(map[string]struct{}),
		thread:        make(map[string]map[string]struct{}),
		workspace:     make(map[string]map[string]struct{}),
		workspaceFile: fileStore,
	}
}

func (s *MemoryStore) Match(req Request, capability string, resources []Resource) bool {
	key := BuildKey(req, capability, resources)

	s.mu.Lock()
	if _, ok := s.once[req.ToolCallID]; ok {
		delete(s.once, req.ToolCallID)
		s.mu.Unlock()
		return true
	}
	s.mu.Unlock()

	s.mu.RLock()
	if req.ThreadID != "" {
		if m := s.thread[req.ThreadID]; m != nil {
			if _, ok := m[key]; ok {
				s.mu.RUnlock()
				return true
			}
		}
	}
	if req.Workspace != "" {
		if m := s.workspace[req.Workspace]; m != nil {
			if _, ok := m[key]; ok {
				s.mu.RUnlock()
				return true
			}
		}
	}
	s.mu.RUnlock()
	return false
}

func (s *MemoryStore) Add(req Request, scope Scope, capability string, resources []Resource) {
	key := BuildKey(req, capability, resources)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch scope {
	case ScopeOnce:
		s.once[req.ToolCallID] = struct{}{}
	case ScopeThread:
		if req.ThreadID == "" {
			return
		}
		if s.thread[req.ThreadID] == nil {
			s.thread[req.ThreadID] = make(map[string]struct{})
		}
		s.thread[req.ThreadID][key] = struct{}{}
	case ScopeWorkspace:
		if req.Workspace == "" {
			return
		}
		if s.workspace[req.Workspace] == nil {
			s.workspace[req.Workspace] = make(map[string]struct{})
		}
		s.workspace[req.Workspace][key] = struct{}{}
	}
}

func (s *MemoryStore) LoadWorkspace(workspace string) error {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	keys, err := s.workspaceFile.Load(workspace)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.workspace[filepath.Clean(workspace)] = keys
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) SaveWorkspace(workspace string) error {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	ws := filepath.Clean(workspace)
	s.mu.RLock()
	keys := s.workspace[ws]
	s.mu.RUnlock()
	if keys == nil {
		keys = make(map[string]struct{})
	}
	return s.workspaceFile.Save(ws, keys)
}

func BuildKey(req Request, capability string, resources []Resource) string {
	var parts []string
	parts = append(parts, capability)
	parts = append(parts, strings.ToLower(req.ToolName))

	// Prefer stable resource keys; avoid embedding full content.
	for _, r := range resources {
		switch r.Type {
		case ResourcePath:
			if capability == "fs.read" {
				// Reduce approval spam: read grants are directory-scoped by default.
				if r.WorkspaceRel != "" {
					parts = append(parts, "path_dir:"+filepath.Dir(filepath.Clean(r.WorkspaceRel)))
				} else {
					parts = append(parts, "path_dir_abs:"+filepath.Dir(filepath.Clean(r.Path)))
				}
				break
			}
			if r.WorkspaceRel != "" {
				parts = append(parts, "path:"+filepath.Clean(r.WorkspaceRel))
			} else {
				parts = append(parts, "path_abs:"+filepath.Clean(r.Path))
			}
		case ResourceDomain:
			if r.Domain != "" {
				parts = append(parts, "domain:"+strings.ToLower(r.Domain))
			}
		case ResourceCommand:
			if r.Command != "" {
				parts = append(parts, "cmd:"+commandSummary(r.Command))
			}
		}
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func commandSummary(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	// Keep only the first token as a stable summary key.
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
