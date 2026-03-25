package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/axon/agent"
)

type fsScope struct {
	root          string
	path          string
	displayPrefix string
}

func validatePath(path, workspace string, restrict bool) (string, error) {
	workspace = normalizeWorkspacePath(workspace)
	path = normalizePathInput(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(workspace, path)
	}

	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}

	path = filepath.Clean(path)

	if restrict && !isWithinWorkspace(path, workspace) {
		return "", fmt.Errorf("path %q is outside the workspace %q", path, workspace)
	}

	return path, nil
}

func toFSPath(path, workspace string) (string, error) {
	path = filepath.Clean(path)
	workspace = normalizeWorkspacePath(workspace)

	if path == workspace {
		return ".", nil
	}

	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return "", err
	}

	return rel, nil
}

func resolveFSScope(path, workspace string) (fsScope, error) {
	path = filepath.Clean(path)
	workspace = normalizeWorkspacePath(workspace)

	if isWithinWorkspace(path, workspace) {
		rel, err := toFSPath(path, workspace)
		if err != nil {
			return fsScope{}, err
		}

		return fsScope{root: workspace, path: rel}, nil
	}

	root, rel := absoluteFSRoot(path)

	return fsScope{
		root:          root,
		path:          rel,
		displayPrefix: root,
	}, nil
}

func normalizeWorkspacePath(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
	}

	if !filepath.IsAbs(workspace) {
		if abs, err := filepath.Abs(workspace); err == nil {
			workspace = abs
		}
	}

	return filepath.Clean(workspace)
}

func isWithinWorkspace(path, workspace string) bool {
	if workspace == "" {
		return false
	}

	rel, err := filepath.Rel(filepath.Clean(workspace), filepath.Clean(path))
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func absoluteFSRoot(path string) (string, string) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return ".", path
	}

	volume := filepath.VolumeName(path)

	root := string(os.PathSeparator)
	if volume != "" {
		root = volume + string(os.PathSeparator)
	}

	rel := strings.TrimPrefix(path, root)
	if rel == "" {
		rel = "."
	}

	return root, rel
}

func displayPath(prefix, path string) string {
	if prefix == "" {
		return path
	}

	return filepath.Clean(filepath.Join(prefix, path))
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

func TextResult(text string) agent.ToolResult {
	return agent.ToolResult{
		Content: agent.Content{Text: lo.ToPtr(text)},
	}
}

func ErrorResult(err error) agent.ToolResult {
	return agent.ToolResult{Error: err}
}
