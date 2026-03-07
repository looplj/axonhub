package grant

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Scope string

const (
	ScopeOnce      Scope = "once"
	ScopeThread    Scope = "thread"
	ScopeWorkspace Scope = "workspace"
	ScopeGlobal    Scope = "global"
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
	ResourceSkill   ResourceType = "skill"
	ResourceDir     ResourceType = "dir"
)

type Resource struct {
	Type ResourceType

	Path             string
	WorkspaceRel     string
	OutsideWorkspace bool

	Domain  string
	Command string
	Skill   string
}

type Entry struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Scope     Scope     `json:"scope"`
	ThreadID  string    `json:"thread_id,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	ToolName  string    `json:"tool_name"`
	Key       string    `json:"key"`
}

// Store persists approval grants and answers whether a request should be allowed
// without prompting the user.
//
// Matching is done against a derived key from (tool name, resources),
// with additional scoping rules:
//   - once: keyed by ToolCallID only; it is consumed on first match. For the same
//     ToolCallID, the store records it only once, and the next permission check
//     for that ToolCallID will pass immediately (and the one-time grant is removed).
//   - thread: keyed by (ThreadID, key)
//   - workspace: keyed by (Workspace, key) and persisted via SaveWorkspace/LoadWorkspace
//   - global: keyed by key only, applies to all workspaces, persisted via SaveGlobal/LoadGlobal
type Store interface {
	Add(req Request, scope Scope, resources []Resource)
	Match(req Request, resources []Resource) bool
	LoadWorkspace(workspace string) error
	SaveWorkspace(workspace string) error
	LoadGlobal() error
	SaveGlobal() error
}

type MemoryStore struct {
	mu sync.RWMutex

	once      map[string]struct{}
	thread    map[string]map[string]struct{}
	workspace map[string]map[string]struct{}
	global    map[string]struct{}

	fileStore FileStore
}

func NewMemoryStore(fileStore FileStore) *MemoryStore {
	return &MemoryStore{
		once:      make(map[string]struct{}),
		thread:    make(map[string]map[string]struct{}),
		workspace: make(map[string]map[string]struct{}),
		global:    make(map[string]struct{}),
		fileStore: fileStore,
	}
}

func (s *MemoryStore) Match(req Request, resources []Resource) bool {
	s.mu.Lock()
	if _, ok := s.once[req.ToolCallID]; ok {
		delete(s.once, req.ToolCallID)
		s.mu.Unlock()
		return true
	}
	s.mu.Unlock()

	keys := buildHierarchicalKeys(req, resources)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range keys {
		if req.ThreadID != "" {
			if m := s.thread[req.ThreadID]; m != nil {
				if _, ok := m[key]; ok {
					return true
				}
			}
		}
		if req.Workspace != "" {
			if m := s.workspace[req.Workspace]; m != nil {
				if _, ok := m[key]; ok {
					return true
				}
			}
		}
		if _, ok := s.global[key]; ok {
			return true
		}
	}
	return false
}

func (s *MemoryStore) Add(req Request, scope Scope, resources []Resource) {
	key := BuildKey(req, resources)

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
	case ScopeGlobal:
		s.global[key] = struct{}{}
	}
}

func (s *MemoryStore) LoadWorkspace(workspace string) error {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	keys, err := s.fileStore.LoadWorkspace(workspace)
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
	return s.fileStore.SaveWorkspace(ws, keys)
}

func (s *MemoryStore) LoadGlobal() error {
	keys, err := s.fileStore.LoadGlobal()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.global = keys
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) SaveGlobal() error {
	s.mu.RLock()
	keys := s.global
	s.mu.RUnlock()
	if keys == nil {
		keys = make(map[string]struct{})
	}
	return s.fileStore.SaveGlobal(keys)
}

func BuildKey(req Request, resources []Resource) string {
	return hashParts(buildParts(req, resources))
}

// buildParts returns the string components used to derive a grant key.
func buildParts(req Request, resources []Resource) []string {
	tool := strings.ToLower(req.ToolName)

	var tokens []string

	for _, r := range resources {
		switch r.Type {
		case ResourcePath:
			if r.WorkspaceRel != "" {
				tokens = append(tokens, "dir:"+filepath.Dir(filepath.Clean(r.WorkspaceRel)))
			} else {
				tokens = append(tokens, "dir_abs:"+filepath.Dir(filepath.Clean(r.Path)))
			}
		case ResourceDir:
			if r.WorkspaceRel != "" {
				tokens = append(tokens, "dir:"+filepath.Clean(r.WorkspaceRel))
			} else {
				tokens = append(tokens, "dir_abs:"+filepath.Clean(r.Path))
			}
		case ResourceDomain:
			if r.Domain != "" {
				tokens = append(tokens, "domain:"+strings.ToLower(r.Domain))
			}
		case ResourceCommand:
			if r.Command != "" {
				for _, t := range commandGrantTokens(r.Command) {
					tokens = append(tokens, "cmd:"+t)
				}
			}
		case ResourceSkill:
			if r.Skill != "" {
				tokens = append(tokens, "skill:"+strings.ToLower(r.Skill))
			}
		}
	}

	sort.Strings(tokens)
	tokens = compactSorted(tokens)

	parts := make([]string, 0, 1+len(tokens))
	parts = append(parts, tool)
	parts = append(parts, tokens...)
	return parts
}

func hashParts(parts []string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func compactSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := in[:0]

	prev := ""
	for _, s := range in {
		if s == "" || s == prev {
			prev = s
			continue
		}

		out = append(out, s)
		prev = s
	}

	return out
}

// buildHierarchicalKeys returns grant keys that should be considered matches
// for the given request/resources.
//
// It includes:
//   - directory ancestor keys (a grant on a parent dir covers children)
//   - resource-subset keys (a grant created for a subset of resources can match
//     a request that includes additional resources)
func buildHierarchicalKeys(req Request, resources []Resource) []string {
	baseParts := buildParts(req, resources)
	if len(baseParts) == 0 {
		return nil
	}

	keys := []string{hashParts(baseParts)}
	seen := map[string]struct{}{keys[0]: {}}

	appendKey := func(parts []string) {
		k := hashParts(parts)
		if _, ok := seen[k]; ok {
			return
		}

		seen[k] = struct{}{}
		keys = append(keys, k)
	}

	tool := baseParts[0]
	baseTokens := append([]string(nil), baseParts[1:]...)

	if len(baseTokens) == 0 {
		return keys
	}

	appendWithDirHierarchy := func(tokens []string) {
		// Keys are based on sorted/compacted tokens so subset generation is stable.
		cp := append([]string(nil), tokens...)
		sort.Strings(cp)

		cp = compactSorted(cp)
		if len(cp) == 0 {
			return
		}

		parts := make([]string, 0, 1+len(cp))
		parts = append(parts, tool)
		parts = append(parts, cp...)
		appendKey(parts)

		// Group directory-like tokens by their path so replacements stay aligned.
		type dirMember struct {
			idx    int
			prefix string
			path   string
		}

		groups := make(map[string][]dirMember)

		for i, t := range cp {
			switch {
			case strings.HasPrefix(t, "dir_abs:"):
				p := strings.TrimPrefix(t, "dir_abs:")
				groups[p] = append(groups[p], dirMember{idx: i, prefix: "dir_abs:", path: p})
			case strings.HasPrefix(t, "dir:"):
				p := strings.TrimPrefix(t, "dir:")
				groups[p] = append(groups[p], dirMember{idx: i, prefix: "dir:", path: p})
			}
		}

		if len(groups) == 0 {
			return
		}

		paths := make([]string, 0, len(groups))
		for p := range groups {
			paths = append(paths, p)
		}

		ancestorOptions := make([][]string, 0, len(paths))
		for _, p := range paths {
			var opts []string

			cur := p
			for {
				opts = append(opts, cur)

				parent := filepath.Dir(cur)
				if parent == cur {
					break
				}

				cur = parent
				if cur == "." || cur == "/" {
					opts = append(opts, cur)
					break
				}
			}

			ancestorOptions = append(ancestorOptions, opts)
		}

		var walk func(int, []string)

		walk = func(i int, curTokens []string) {
			if i == len(paths) {
				parts := make([]string, 0, 1+len(curTokens))
				parts = append(parts, tool)
				parts = append(parts, curTokens...)
				appendKey(parts)

				return
			}

			path := paths[i]
			members := groups[path]

			for _, anc := range ancestorOptions[i] {
				next := append([]string(nil), curTokens...)
				for _, m := range members {
					next[m.idx] = m.prefix + anc
				}

				walk(i+1, next)
			}
		}

		walk(0, tokens)
	}

	const subsetAllThreshold = 10

	emitSubset := func(sub []string) {
		appendWithDirHierarchy(sub)
	}

	// Generate keys for non-empty subsets so a grant created with a subset of
	// selected resources can match a request that includes additional resources.
	if len(baseTokens) > subsetAllThreshold {
		emitSubset(baseTokens)

		for _, t := range baseTokens {
			emitSubset([]string{t})
		}

		return keys
	}

	n := len(baseTokens)
	for mask := 1; mask < (1 << n); mask++ {
		sub := make([]string, 0, n)
		for i := range n {
			if mask&(1<<i) != 0 {
				sub = append(sub, baseTokens[i])
			}
		}

		emitSubset(sub)
	}

	return keys
}

func commandSummary(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func commandSubcommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return ""
	}
	// Find the first non-flag token after the program name.
	for i := 1; i < len(fields); i++ {
		f := strings.TrimSpace(fields[i])
		if f == "" {
			continue
		}

		if strings.HasPrefix(f, "-") {
			continue
		}

		return f
	}

	return ""
}

// commandGrantTokens returns the command tokens used for grant keying.
//
// It emits a program token and, when present, a program+subcommand token:
//   - "go"
//   - "go test"
//
// This enables hierarchical matching via subset matching: a stored "go" grant
// covers "go test" requests, while a stored "go test" grant does not cover "go".
func commandGrantTokens(cmd string) []string {
	prog := commandSummary(cmd)
	if prog == "" {
		return nil
	}

	out := []string{prog}
	if sub := commandSubcommand(cmd); sub != "" {
		out = append(out, prog+" "+sub)
	}

	return out
}
