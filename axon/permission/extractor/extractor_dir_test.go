package extractor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/permission/policy"
)

func TestExtract_Read_ProducesPathAndDirResources(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, "src/main.go", resources[0].WorkspaceRel)
	assert.False(t, resources[0].OutsideWorkspace)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
	assert.Equal(t, "src", resources[1].WorkspaceRel)
	assert.False(t, resources[1].OutsideWorkspace)
}

func TestExtract_Write_ProducesPathAndDirResources(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Write", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
	assert.Equal(t, "src", resources[1].WorkspaceRel)
}

func TestExtract_Edit_ProducesPathAndDirResources(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Edit", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
}

func TestExtract_Read_FileAtWorkspaceRoot(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/README.md"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/README.md", resources[0].Path)
	assert.Equal(t, "README.md", resources[0].WorkspaceRel)
	assert.False(t, resources[0].OutsideWorkspace)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace", resources[1].Path)
	assert.Equal(t, ".", resources[1].WorkspaceRel)
	assert.False(t, resources[1].OutsideWorkspace)
}

func TestExtract_Read_OutsideWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/etc/config.json"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/etc/config.json", resources[0].Path)
	assert.True(t, resources[0].OutsideWorkspace)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/etc", resources[1].Path)
	assert.True(t, resources[1].OutsideWorkspace)
	assert.Empty(t, resources[1].WorkspaceRel)
}

func TestExtract_Read_OutsideWorkspace_Dir(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/etc/"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/etc", resources[0].Path)
	assert.True(t, resources[0].OutsideWorkspace)
}

func TestExtract_Read_QuotedPathWithSpaces(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"\"/workspace/Claw 001/no01/IDENTITY.md\""}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/Claw 001/no01/IDENTITY.md", resources[0].Path)
	assert.Equal(t, filepath.ToSlash("Claw 001/no01/IDENTITY.md"), filepath.ToSlash(resources[0].WorkspaceRel))
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/Claw 001/no01", resources[1].Path)
}

func TestExtract_Read_OutsideWorkspace_ExtensionlessFile(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/etc/passwd"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/etc/passwd", resources[0].Path)
	assert.True(t, resources[0].OutsideWorkspace)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/etc", resources[1].Path)
	assert.True(t, resources[1].OutsideWorkspace)
}

func TestExtract_Read_NestedPath(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/pkg/util/helper.go"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/pkg/util/helper.go", resources[0].Path)
	assert.Equal(t, "src/pkg/util/helper.go", resources[0].WorkspaceRel)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src/pkg/util", resources[1].Path)
	assert.Equal(t, "src/pkg/util", resources[1].WorkspaceRel)
}

func TestExtract_Read_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Read", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Write_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Write", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Edit_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Edit", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Glob_ProducesDirResource(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src"}`)

	resources, err := ext.Extract("/workspace", "Glob", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/src", resources[0].Path)
	assert.Equal(t, "src", resources[0].WorkspaceRel)
	assert.False(t, resources[0].OutsideWorkspace)
}

func TestExtract_Glob_EmptyPathDefaultsToWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{}`)

	resources, err := ext.Extract("/workspace", "Glob", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace", resources[0].Path)
	assert.Equal(t, ".", resources[0].WorkspaceRel)
}

func TestExtract_Glob_WhitespacePathDefaultsToWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"   "}`)

	resources, err := ext.Extract("/workspace", "Glob", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "/workspace", resources[0].Path)
}

func TestExtract_Glob_OutsideWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/tmp/data"}`)

	resources, err := ext.Extract("/workspace", "Glob", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.True(t, resources[0].OutsideWorkspace)
}

func TestExtract_Glob_QuotedPathWithSpaces(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"\"/workspace/Claw 001/no01\""}`)

	resources, err := ext.Extract("/workspace", "Glob", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/Claw 001/no01", resources[0].Path)
	assert.Equal(t, filepath.ToSlash("Claw 001/no01"), filepath.ToSlash(resources[0].WorkspaceRel))
}

func TestExtract_Glob_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Glob", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Grep_ProducesDirResource(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src"}`)

	resources, err := ext.Extract("/workspace", "Grep", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/src", resources[0].Path)
	assert.Equal(t, "src", resources[0].WorkspaceRel)
}

func TestExtract_Grep_EmptyPathDefaultsToWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{}`)

	resources, err := ext.Extract("/workspace", "Grep", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "/workspace", resources[0].Path)
}

func TestExtract_Grep_OutsideWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/other/dir"}`)

	resources, err := ext.Extract("/workspace", "Grep", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.True(t, resources[0].OutsideWorkspace)
}

func TestExtract_Grep_QuotedPathWithSpaces(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"\"/workspace/Claw 001/no01\""}`)

	resources, err := ext.Extract("/workspace", "Grep", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/Claw 001/no01", resources[0].Path)
	assert.Equal(t, filepath.ToSlash("Claw 001/no01"), filepath.ToSlash(resources[0].WorkspaceRel))
}

func TestExtract_Grep_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Grep", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Bash_ProducesDirResourceForCwd(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"command":"ls -la","cwd":"/workspace/src"}`)

	resources, err := ext.Extract("/workspace", "Bash", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourceCommand, resources[0].Type)
	assert.Equal(t, "ls -la", resources[0].Command)
	assert.Equal(t, "/workspace/src", resources[0].Cwd)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
	assert.Equal(t, "src", resources[1].WorkspaceRel)
}

func TestExtract_Bash_EmptyCwdDefaultsToWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"command":"pwd"}`)

	resources, err := ext.Extract("/workspace", "Bash", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace", resources[1].Path)
}

func TestExtract_Bash_CwdOutsideWorkspace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"command":"ls","cwd":"/tmp"}`)

	resources, err := ext.Extract("/workspace", "Bash", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.True(t, resources[1].OutsideWorkspace)
}

func TestExtract_Bash_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Bash", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_UnknownTool_ReturnsNil(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/file.txt"}`)

	resources, err := ext.Extract("/workspace", "UnknownTool", input)

	assert.NoError(t, err)
	assert.Nil(t, resources)
}

func TestExtract_LS_NotSupported(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"directory_path":"/workspace/src"}`)

	resources, err := ext.Extract("/workspace", "LS", input)

	assert.NoError(t, err)
	assert.Nil(t, resources)
}

func TestExtract_Read_RelativePath(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, "src/main.go", resources[0].WorkspaceRel)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
	assert.Equal(t, "src", resources[1].WorkspaceRel)
}

func TestExtract_Read_PathNormalization(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/../src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
}

func TestExtract_Bash_QuotedCwdWithSpaces(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"command":"pwd","cwd":"\"/workspace/Claw 001/no01\""}`)

	resources, err := ext.Extract("/workspace", "Bash", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourceCommand, resources[0].Type)
	assert.Equal(t, "/workspace/Claw 001/no01", resources[0].Cwd)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/Claw 001/no01", resources[1].Path)
	assert.Equal(t, filepath.ToSlash("Claw 001/no01"), filepath.ToSlash(resources[1].WorkspaceRel))
}

func TestExtract_WebFetch(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"query":"https://example.com/page?q=test"}`)

	resources, err := ext.Extract("/workspace", "WebFetch", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourceURL, resources[0].Type)
	assert.Equal(t, "https://example.com/page", resources[0].URL)
	assert.Equal(t, "example.com", resources[0].Domain)
	assert.Equal(t, "https", resources[0].Scheme)
	assert.Equal(t, policy.ResourceDomain, resources[1].Type)
	assert.Equal(t, "example.com", resources[1].Domain)
}

func TestExtract_WebFetch_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "WebFetch", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_WebSearch_Domains(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"allowed_domains":["a.com"],"blocked_domains":["b.com"]}`)

	resources, err := ext.Extract("/workspace", "WebSearch", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, "a.com", resources[0].Domain)
	assert.Equal(t, "b.com", resources[1].Domain)
}

func TestExtract_WebSearch_EmptyDomains(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{}`)

	resources, err := ext.Extract("/workspace", "WebSearch", input)

	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestExtract_WebSearch_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "WebSearch", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Glob_NestedDir(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/a/b/c"}`)

	resources, err := ext.Extract("/workspace", "Glob", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "a/b/c", resources[0].WorkspaceRel)
}

func TestExtract_Grep_NestedDir(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/a/b/c"}`)

	resources, err := ext.Extract("/workspace", "Grep", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "a/b/c", resources[0].WorkspaceRel)
}

func TestExtract_Read_PathEndingWithSlash_IsDir(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/src", resources[0].Path)
	assert.Equal(t, "src", resources[0].WorkspaceRel)
}

func TestExtract_Write_PathEndingWithSlash_IsDir(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/"}`)

	resources, err := ext.Extract("/workspace", "Write", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/src", resources[0].Path)
}

func TestExtract_Edit_PathEndingWithSlash_IsDir(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/"}`)

	resources, err := ext.Extract("/workspace", "Edit", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, "/workspace/src", resources[0].Path)
}

func TestExtract_Read_PathWithoutExtension_Unknown_IsFile(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/mydir"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/mydir", resources[0].Path)
	assert.Equal(t, "src/mydir", resources[0].WorkspaceRel)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
	assert.Equal(t, "src", resources[1].WorkspaceRel)
}

func TestExtract_Read_PathWithExtension_IsFile(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"/workspace/src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
}

func TestExtract_Read_RelativePathWithoutExtension_Unknown_IsFile(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"src/mydir"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/mydir", resources[0].Path)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
}

func TestExtract_Read_RelativePathWithExtension_IsFile(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"path":"src/main.go"}`)

	resources, err := ext.Extract("/workspace", "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, policy.ResourcePath, resources[0].Type)
	assert.Equal(t, "/workspace/src/main.go", resources[0].Path)
	assert.Equal(t, policy.ResourceDir, resources[1].Type)
	assert.Equal(t, "/workspace/src", resources[1].Path)
}

func TestExtract_Read_PathWithoutExtension_ExistingDir_IsDir(t *testing.T) {
	ext := DefaultExtractor{}
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "src", "mydir"), 0o755))

	input := json.RawMessage(`{"path":"src/mydir"}`)

	resources, err := ext.Extract(workspace, "Read", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceDir, resources[0].Type)
	assert.Equal(t, filepath.Join(workspace, "src", "mydir"), resources[0].Path)
	assert.Equal(t, filepath.ToSlash("src/mydir"), filepath.ToSlash(resources[0].WorkspaceRel))
}
