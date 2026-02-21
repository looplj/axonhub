package extractor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

type ResourceType string

const (
	ResourcePath    ResourceType = "path"
	ResourceCommand ResourceType = "command"
	ResourceURL     ResourceType = "url"
	ResourceDomain  ResourceType = "domain"
)

type Resource struct {
	Type ResourceType

	// Path
	Path             string
	WorkspaceRel     string
	OutsideWorkspace bool

	// Command
	Command string
	Cwd     string

	// Network
	URL    string
	Domain string
	Scheme string
}

type Extractor interface {
	Capabilities(toolName string) []string
	Extract(workspace, toolName string, input json.RawMessage) ([]Resource, error)
}

type DefaultExtractor struct{}

func (e DefaultExtractor) Capabilities(toolName string) []string {
	return CapabilityForTool(toolName)
}

func (e DefaultExtractor) Extract(workspace, toolName string, input json.RawMessage) ([]Resource, error) {
	switch toolName {
	case "Read", "Write", "Edit":
		var v struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		return []Resource{pathResource(workspace, v.Path)}, nil
	case "Glob":
		var v struct {
			Path string `json:"path,omitempty"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		p := v.Path
		if strings.TrimSpace(p) == "" {
			p = workspace
		}
		return []Resource{pathResource(workspace, p)}, nil
	case "Grep":
		var v struct {
			Path string `json:"path,omitempty"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		p := v.Path
		if strings.TrimSpace(p) == "" {
			p = workspace
		}
		return []Resource{pathResource(workspace, p)}, nil
	case "Bash":
		var v struct {
			Command string `json:"command"`
			Cwd     string `json:"cwd,omitempty"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		cwd := strings.TrimSpace(v.Cwd)
		if cwd == "" {
			cwd = workspace
		}
		return []Resource{
			{
				Type:    ResourceCommand,
				Command: strings.TrimSpace(v.Command),
				Cwd:     cleanPath(workspace, cwd),
			},
			pathResource(workspace, cwd),
		}, nil
	case "WebFetch":
		var v struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		raw := strings.TrimSpace(v.Query)
		u, err := url.Parse(raw)
		if err != nil {
			return []Resource{{Type: ResourceURL, URL: redactURL(raw)}}, nil
		}
		return []Resource{
			{
				Type:   ResourceURL,
				URL:    redactURL(u.String()),
				Domain: strings.ToLower(u.Hostname()),
				Scheme: strings.ToLower(u.Scheme),
			},
			{
				Type:   ResourceDomain,
				Domain: strings.ToLower(u.Hostname()),
			},
		}, nil
	case "WebSearch":
		var v struct {
			AllowedDomains []string `json:"allowed_domains,omitempty"`
			BlockedDomains []string `json:"blocked_domains,omitempty"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		var out []Resource
		for _, d := range v.AllowedDomains {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" {
				out = append(out, Resource{Type: ResourceDomain, Domain: d})
			}
		}
		for _, d := range v.BlockedDomains {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" {
				out = append(out, Resource{Type: ResourceDomain, Domain: d})
			}
		}
		return out, nil
	default:
		return nil, nil
	}
}

// CapabilityForTool maps built-in tool names to stable capabilities.
func CapabilityForTool(toolName string) []string {
	switch toolName {
	case "Read", "Glob", "Grep":
		return []string{"fs.read"}
	case "Write":
		return []string{"fs.write"}
	case "Edit":
		return []string{"fs.edit"}
	case "Bash":
		return []string{"proc.exec"}
	case "WebFetch":
		return []string{"net.fetch"}
	case "WebSearch":
		return []string{"net.search"}
	case "Skill":
		return []string{"skill.run"}
	default:
		if strings.HasPrefix(toolName, "Memory") {
			switch toolName {
			case "MemoryAdd", "MemoryDelete":
				return []string{"memory.write"}
			default:
				return []string{"memory.read"}
			}
		}
		return nil
	}
}

func cleanPath(workspace, p string) string {
	if strings.TrimSpace(p) == "" {
		return filepath.Clean(workspace)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(workspace, p)
	}
	return filepath.Clean(p)
}

func pathResource(workspace, p string) Resource {
	abs := cleanPath(workspace, p)
	ws := filepath.Clean(workspace)
	outside := abs != ws && !strings.HasPrefix(abs, ws+string(filepath.Separator))
	rel := ""
	if !outside {
		if r, err := filepath.Rel(ws, abs); err == nil {
			rel = r
		}
	}
	return Resource{
		Type:             ResourcePath,
		Path:             abs,
		WorkspaceRel:     rel,
		OutsideWorkspace: outside,
	}
}

func redactURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
