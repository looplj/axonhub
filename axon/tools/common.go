package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/samber/lo"
)

func validatePath(path, workspace string, restrict bool) (string, error) {
	path = normalizePathInput(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(workspace, path)
	}

	path = filepath.Clean(path)

	if restrict && !strings.HasPrefix(path, filepath.Clean(workspace)+string(filepath.Separator)) && path != filepath.Clean(workspace) {
		return "", fmt.Errorf("path %q is outside the workspace %q", path, workspace)
	}

	return path, nil
}

func toFSPath(path, workspace string) (string, error) {
	path = filepath.Clean(path)
	workspace = filepath.Clean(workspace)

	if path == workspace {
		return ".", nil
	}

	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return "", err
	}

	return rel, nil
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
