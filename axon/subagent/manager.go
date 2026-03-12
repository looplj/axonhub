package subagent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	Name        string
	Hidden      bool
	Model       string
	Color       string
	Tools       map[string]bool
	Description string
}

type definitionFrontMatter struct {
	Hidden bool           `yaml:"hidden"`
	Model  string         `yaml:"model"`
	Color  string         `yaml:"color"`
	Tools  map[string]any `yaml:"tools"`
}

type Manager struct {
	fsys   fs.FS
	agents map[string]*Definition
}

func NewManager(fsys fs.FS) *Manager {
	return &Manager{
		fsys:   fsys,
		agents: make(map[string]*Definition),
	}
}

func NewManagerFromPath(agentDir string) *Manager {
	if agentDir == "" {
		return &Manager{
			fsys:   nil,
			agents: make(map[string]*Definition),
		}
	}

	return NewManager(os.DirFS(agentDir))
}

func (m *Manager) Load() error {
	if m.fsys == nil {
		return nil
	}

	entries, err := fs.ReadDir(m.fsys, ".")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to read agent directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		def, err := m.parseAgentFile(name)
		if err != nil {
			return fmt.Errorf("failed to parse agent file %s: %w", name, err)
		}

		if def == nil {
			continue
		}

		agentName := strings.TrimSuffix(name, filepath.Ext(name))
		def.Name = agentName
		m.agents[agentName] = def
	}

	return nil
}

func (m *Manager) parseAgentFile(name string) (*Definition, error) {
	content, err := fs.ReadFile(m.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return parseAgentContent(string(content))
}

func (m *Manager) Get(name string) (*Definition, bool) {
	def, ok := m.agents[name]
	return def, ok
}

func (m *Manager) List() []*Definition {
	result := make([]*Definition, 0, len(m.agents))
	for _, def := range m.agents {
		result = append(result, def)
	}

	return result
}

func (m *Manager) ListVisible() []*Definition {
	result := make([]*Definition, 0, len(m.agents))
	for _, def := range m.agents {
		if !def.Hidden {
			result = append(result, def)
		}
	}

	return result
}

func parseAgentContent(content string) (*Definition, error) {
	content = strings.TrimPrefix(content, "\ufeff")

	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return nil, nil
	}

	content = trimmed

	if len(content) < 4 {
		return nil, nil
	}

	afterStart := content[4:]
	if len(afterStart) == 0 {
		return nil, nil
	}

	var (
		endIdx    int
		endMarker string
	)

	if strings.HasPrefix(afterStart, "---") || strings.HasPrefix(afterStart, "...") {
		endIdx = 0
		endMarker = afterStart[:3]
	} else {
		endMarker = "\n---"

		endIdx = strings.Index(afterStart, endMarker)
		if endIdx == -1 {
			endMarker = "\n..."

			endIdx = strings.Index(afterStart, endMarker)
			if endIdx == -1 {
				return nil, nil
			}
		}
	}

	frontMatterStr := strings.TrimSpace(afterStart[:endIdx])

	bodyStart := endIdx + len(endMarker)
	for bodyStart < len(afterStart) && (afterStart[bodyStart] == '\n' || afterStart[bodyStart] == '\r') {
		bodyStart++
	}

	description := ""
	if bodyStart < len(afterStart) {
		description = strings.TrimSpace(afterStart[bodyStart:])
	}

	var fm definitionFrontMatter
	if err := yaml.Unmarshal([]byte(frontMatterStr), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	var tools map[string]bool
	if fm.Tools != nil {
		tools = make(map[string]bool, len(fm.Tools))

		for toolName, value := range fm.Tools {
			switch v := value.(type) {
			case bool:
				tools[toolName] = v
			case string:
				tools[toolName] = strings.ToLower(v) == "true"
			default:
				tools[toolName] = false
			}
		}
	}

	return &Definition{
		Hidden:      fm.Hidden,
		Model:       fm.Model,
		Color:       fm.Color,
		Tools:       tools,
		Description: description,
	}, nil
}
