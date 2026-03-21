package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobToolExecute_QuotedAbsolutePathWithSpaces(t *testing.T) {
	workspace := t.TempDir()
	targetDir := filepath.Join(workspace, "Claw 001", "no01")
	targetFile := filepath.Join(targetDir, "IDENTITY.md")

	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	require.NoError(t, os.WriteFile(targetFile, []byte("identity"), 0o644))

	tool := NewGlobTool(workspace, false)
	result := tool.Execute(context.Background(), globInput{
		Pattern: "*.md",
		Path:    `"` + targetDir + `"`,
	})

	require.NoError(t, result.Error)
	require.NotNil(t, result.Content.Text)
	assert.Equal(t, "Claw 001/no01/IDENTITY.md\n", filepath.ToSlash(*result.Content.Text))
}
