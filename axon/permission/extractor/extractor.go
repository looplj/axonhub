package extractor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/looplj/axonhub/axon/permission/policy"
)

// validSkillName matches the Agent Skills specification:
// lowercase alphanumeric + hyphens, no leading/trailing/consecutive hyphens, max 64 chars.
var validSkillName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

type Extractor interface {
	Extract(workspace, toolName string, input json.RawMessage) ([]policy.Resource, error)
}

type DefaultExtractor struct{}

func (e DefaultExtractor) Extract(workspace, toolName string, input json.RawMessage) ([]policy.Resource, error) {
	switch toolName {
	case "Read", "Write", "Edit":
		var v struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		abs := cleanPath(workspace, v.Path)
		if isDirPath(v.Path) {
			return []policy.Resource{dirResource(workspace, abs)}, nil
		}
		// If the path exists and is a directory, treat it as a directory even
		// without a trailing slash. This avoids misclassifying extension-less
		// files (e.g. /etc/passwd, Makefile) as directories.
		if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
			return []policy.Resource{dirResource(workspace, abs)}, nil
		}

		return []policy.Resource{
			pathResource(workspace, abs),
			dirResource(workspace, filepath.Dir(abs)),
		}, nil
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

		return []policy.Resource{dirResource(workspace, p)}, nil
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

		return []policy.Resource{dirResource(workspace, p)}, nil
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

		return []policy.Resource{
			{
				Type:    policy.ResourceCommand,
				Command: strings.TrimSpace(v.Command),
				Cwd:     cleanPath(workspace, cwd),
			},
			dirResource(workspace, cwd),
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
		//nolint:nilerr // We want to ignore the error here.
		if err != nil {
			return []policy.Resource{{Type: policy.ResourceURL, URL: redactURL(raw)}}, nil
		}

		return []policy.Resource{
			{
				Type:   policy.ResourceURL,
				URL:    redactURL(u.String()),
				Domain: strings.ToLower(u.Hostname()),
				Scheme: strings.ToLower(u.Scheme),
			},
			{
				Type:   policy.ResourceDomain,
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

		var out []policy.Resource
		for _, d := range v.AllowedDomains {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" {
				out = append(out, policy.Resource{Type: policy.ResourceDomain, Domain: d})
			}
		}
		for _, d := range v.BlockedDomains {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" {
				out = append(out, policy.Resource{Type: policy.ResourceDomain, Domain: d})
			}
		}
		return out, nil
	case "Skill":
		var v struct {
			Skill string `json:"skill"`
		}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, fmt.Errorf("extract %s: invalid json: %w", toolName, err)
		}
		name := strings.TrimSpace(v.Skill)
		if name == "" {
			return nil, fmt.Errorf("extract %s: empty skill name", toolName)
		}
		// Strip namespace prefix (e.g. "namespace:skill-name" → "skill-name").
		if parts := strings.SplitN(name, ":", 2); len(parts) == 2 {
			name = strings.TrimSpace(parts[1])
		}
		if name == "" {
			return nil, fmt.Errorf("extract %s: empty skill name after namespace strip", toolName)
		}
		name = strings.ToLower(name)
		if len(name) > 64 || !validSkillName.MatchString(name) || strings.Contains(name, "--") {
			return nil, fmt.Errorf("extract %s: invalid skill name %q: must be 1-64 lowercase alphanumeric chars and hyphens, no leading/trailing/consecutive hyphens", toolName, name)
		}

		return []policy.Resource{{Type: policy.ResourceSkill, Skill: name}}, nil
	default:
		return nil, nil
	}
}

func cleanPath(workspace, p string) string {
	p = normalizePathInput(p)
	if p == "" {
		return filepath.Clean(workspace)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(workspace, p)
	}
	return filepath.Clean(p)
}

func normalizePathInput(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 2 {
		if (path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'') {
			path = strings.TrimSpace(path[1 : len(path)-1])
		}
	}

	return path
}

func isDirPath(p string) bool {
	p = normalizePathInput(p)
	if p == "." || p == ".." {
		return true
	}

	if strings.HasSuffix(p, "/") || strings.HasSuffix(p, `\`) {
		return true
	}

	return false
}

func pathResource(workspace, p string) policy.Resource {
	abs := cleanPath(workspace, p)
	ws := filepath.Clean(workspace)
	outside := abs != ws && !strings.HasPrefix(abs, ws+string(filepath.Separator))
	rel := ""
	if !outside {
		if r, err := filepath.Rel(ws, abs); err == nil {
			rel = r
		}
	}

	return policy.Resource{
		Type:             policy.ResourcePath,
		Path:             abs,
		WorkspaceRel:     rel,
		OutsideWorkspace: outside,
	}
}

func dirResource(workspace, dir string) policy.Resource {
	abs := cleanPath(workspace, dir)
	ws := filepath.Clean(workspace)
	outside := abs != ws && !strings.HasPrefix(abs, ws+string(filepath.Separator))
	rel := ""
	if !outside {
		if r, err := filepath.Rel(ws, abs); err == nil {
			rel = r
		}
	}

	return policy.Resource{
		Type:             policy.ResourceDir,
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
