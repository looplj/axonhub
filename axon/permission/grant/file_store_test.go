package grant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileStore(t *testing.T) {
	fs := NewFileStore("/tmp")

	assert.Equal(t, "/tmp", fs.BaseDir)
	assert.NotNil(t, fs.fsys)
}

func TestNewFileStoreWithFS(t *testing.T) {
	memFs := afero.NewMemMapFs()
	fs := NewFileStoreWithFS("/base", memFs)

	assert.Equal(t, "/base", fs.BaseDir)
	assert.NotNil(t, fs.fsys)
}

func TestFileStore_LoadWorkspace_NotExist(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys, err := fs.LoadWorkspace("/workspace/test")

	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestFileStore_SaveAndLoadWorkspace(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{
		"key1": {},
		"key2": {},
		"key3": {},
	}

	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Len(t, loaded, 3)
	assert.Contains(t, loaded, "key1")
	assert.Contains(t, loaded, "key2")
	assert.Contains(t, loaded, "key3")
}

func TestFileStore_SaveWorkspace_EmptyKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{}

	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Empty(t, loaded)
}

func TestFileStore_SaveWorkspace_NilKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	err := fs.SaveWorkspace("/workspace/test", nil)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Empty(t, loaded)
}

func TestFileStore_LoadGlobal_NotExist(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys, err := fs.LoadGlobal()

	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestFileStore_SaveAndLoadGlobal(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{
		"global-key1": {},
		"global-key2": {},
	}

	err := fs.SaveGlobal(keys)
	require.NoError(t, err)

	loaded, err := fs.LoadGlobal()
	require.NoError(t, err)

	assert.Len(t, loaded, 2)
	assert.Contains(t, loaded, "global-key1")
	assert.Contains(t, loaded, "global-key2")
}

func TestFileStore_SaveGlobal_EmptyKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{}

	err := fs.SaveGlobal(keys)
	require.NoError(t, err)

	loaded, err := fs.LoadGlobal()
	require.NoError(t, err)

	assert.Empty(t, loaded)
}

func TestFileStore_SaveGlobal_NilKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	err := fs.SaveGlobal(nil)
	require.NoError(t, err)

	loaded, err := fs.LoadGlobal()
	require.NoError(t, err)

	assert.Empty(t, loaded)
}

func TestFileStore_WorkspacePath(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	path := fs.workspacePath("/workspace/test")

	assert.Contains(t, path, "/base/workspaces/")
	assert.Contains(t, path, ".json")
}

func TestFileStore_WorkspacePath_Consistent(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	path1 := fs.workspacePath("/workspace/test")
	path2 := fs.workspacePath("/workspace/test")

	assert.Equal(t, path1, path2)
}

func TestFileStore_WorkspacePath_Different(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	path1 := fs.workspacePath("/workspace/test1")
	path2 := fs.workspacePath("/workspace/test2")

	assert.NotEqual(t, path1, path2)
}

func TestFileStore_WorkspacePath_PathCleaning(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	path1 := fs.workspacePath("/workspace/test/../test")
	path2 := fs.workspacePath("/workspace/test")

	assert.Equal(t, path1, path2)
}

func TestFileStore_GlobalPath(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	path := fs.globalPath()

	assert.Equal(t, "/base/global.json", path)
}

func TestWorkspaceHash(t *testing.T) {
	tests := []struct {
		name string
		ws   string
	}{
		{"simple path", "/workspace/test"},
		{"complex path", "/workspace/test/sub/dir"},
		{"empty string", ""},
		{"relative path", "workspace/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := workspaceHash(tt.ws)

			assert.Len(t, hash, 32)
		})
	}
}

func TestWorkspaceHash_Consistent(t *testing.T) {
	ws := "/workspace/test"

	hash1 := workspaceHash(ws)
	hash2 := workspaceHash(ws)

	assert.Equal(t, hash1, hash2)
}

func TestWorkspaceHash_Different(t *testing.T) {
	hash1 := workspaceHash("/workspace/test1")
	hash2 := workspaceHash("/workspace/test2")

	assert.NotEqual(t, hash1, hash2)
}

func TestFileStore_FileFormat(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{
		"key1": {},
		"key2": {},
	}

	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	path := fs.workspacePath("/workspace/test")
	data, err := afero.ReadFile(fs.fsys, path)
	require.NoError(t, err)

	var f fileFormat
	err = json.Unmarshal(data, &f)
	require.NoError(t, err)

	assert.Equal(t, 1, f.Version)
	assert.NotZero(t, f.UpdatedAt)
	assert.Len(t, f.Keys, 2)
	assert.Contains(t, f.Keys, "key1")
	assert.Contains(t, f.Keys, "key2")
}

func TestFileStore_Overwrite(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys1 := map[string]struct{}{
		"key1": {},
		"key2": {},
	}
	err := fs.SaveWorkspace("/workspace/test", keys1)
	require.NoError(t, err)

	keys2 := map[string]struct{}{
		"key3": {},
		"key4": {},
		"key5": {},
	}
	err = fs.SaveWorkspace("/workspace/test", keys2)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Len(t, loaded, 3)
	assert.Contains(t, loaded, "key3")
	assert.Contains(t, loaded, "key4")
	assert.Contains(t, loaded, "key5")
	assert.NotContains(t, loaded, "key1")
	assert.NotContains(t, loaded, "key2")
}

func TestFileStore_MultipleWorkspaces(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys1 := map[string]struct{}{"ws1-key": {}}
	keys2 := map[string]struct{}{"ws2-key": {}}
	keys3 := map[string]struct{}{"ws3-key": {}}

	err := fs.SaveWorkspace("/workspace/test1", keys1)
	require.NoError(t, err)

	err = fs.SaveWorkspace("/workspace/test2", keys2)
	require.NoError(t, err)

	err = fs.SaveWorkspace("/workspace/test3", keys3)
	require.NoError(t, err)

	loaded1, err := fs.LoadWorkspace("/workspace/test1")
	require.NoError(t, err)
	assert.Contains(t, loaded1, "ws1-key")

	loaded2, err := fs.LoadWorkspace("/workspace/test2")
	require.NoError(t, err)
	assert.Contains(t, loaded2, "ws2-key")

	loaded3, err := fs.LoadWorkspace("/workspace/test3")
	require.NoError(t, err)
	assert.Contains(t, loaded3, "ws3-key")
}

func TestFileStore_GlobalAndWorkspaceIndependent(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	wsKeys := map[string]struct{}{"ws-key": {}}
	globalKeys := map[string]struct{}{"global-key": {}}

	err := fs.SaveWorkspace("/workspace/test", wsKeys)
	require.NoError(t, err)

	err = fs.SaveGlobal(globalKeys)
	require.NoError(t, err)

	loadedWs, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)
	assert.Contains(t, loadedWs, "ws-key")
	assert.NotContains(t, loadedWs, "global-key")

	loadedGlobal, err := fs.LoadGlobal()
	require.NoError(t, err)
	assert.Contains(t, loadedGlobal, "global-key")
	assert.NotContains(t, loadedGlobal, "ws-key")
}

func TestFileStore_InvalidJSON(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	path := fs.workspacePath("/workspace/test")
	invalidJSON := []byte("{invalid json}")
	err := afero.WriteFile(fs.fsys, path, invalidJSON, 0o600)
	require.NoError(t, err)

	_, err = fs.LoadWorkspace("/workspace/test")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestFileStore_TmpFileRemoved(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{"key1": {}}
	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	path := fs.workspacePath("/workspace/test")
	tmpPath := path + ".tmp"

	_, err = fs.fsys.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err))

	_, err = fs.fsys.Stat(path)
	require.NoError(t, err)
}

func TestFileStore_LargeKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	largeKeys := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		largeKeys[string(rune(i))] = struct{}{}
	}

	err := fs.SaveWorkspace("/workspace/test", largeKeys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Len(t, loaded, 1000)
}

func TestFileStore_SpecialCharactersInKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{
		"key-with-dash":      {},
		"key_with_underscore": {},
		"key.with.dot":       {},
		"key/with/slash":     {},
		"key with space":     {},
		"key:with:colon":     {},
	}

	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Len(t, loaded, 6)
	for k := range keys {
		assert.Contains(t, loaded, k)
	}
}

func TestFileStore_UnicodeKeys(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())

	keys := map[string]struct{}{
		"键1":   {},
		"キー2":  {},
		"ключ3": {},
		"🔑":    {},
	}

	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.Len(t, loaded, 4)
	for k := range keys {
		assert.Contains(t, loaded, k)
	}
}

func TestFileStore_BaseDirWithTrailingSlash(t *testing.T) {
	fs := NewFileStoreWithFS("/base/", afero.NewMemMapFs())

	keys := map[string]struct{}{"key1": {}}
	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)
	assert.Contains(t, loaded, "key1")
}

func TestFileStore_RelativeBaseDir(t *testing.T) {
	fs := NewFileStoreWithFS("relative/path", afero.NewMemMapFs())

	keys := map[string]struct{}{"key1": {}}
	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	loaded, err := fs.LoadWorkspace("/workspace/test")
	require.NoError(t, err)
	assert.Contains(t, loaded, "key1")
}

func TestFileStore_IntegrationWithMemoryStore(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())
	store := NewMemoryStore(fs)

	req := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeWorkspace, resources)

	err := store.SaveWorkspace("/workspace/test")
	require.NoError(t, err)

	store2 := NewMemoryStore(fs)
	err = store2.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	assert.True(t, store2.Match(req, resources))
}

func TestFileStore_IntegrationWithGlobal(t *testing.T) {
	fs := NewFileStoreWithFS("/base", afero.NewMemMapFs())
	store := NewMemoryStore(fs)

	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeGlobal, resources)

	err := store.SaveGlobal()
	require.NoError(t, err)

	store2 := NewMemoryStore(fs)
	err = store2.LoadGlobal()
	require.NoError(t, err)

	assert.True(t, store2.Match(req, resources))
}

func TestFileStore_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	realFs := afero.NewBasePathFs(afero.NewOsFs(), tmpDir)
	fs := NewFileStoreWithFS("/base", realFs)

	keys := map[string]struct{}{"key1": {}}
	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	path := filepath.Join(tmpDir, fs.workspacePath("/workspace/test"))
	info, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestFileStore_DirectoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	realFs := afero.NewBasePathFs(afero.NewOsFs(), tmpDir)
	fs := NewFileStoreWithFS("/base", realFs)

	keys := map[string]struct{}{"key1": {}}
	err := fs.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	dirPath := filepath.Join(tmpDir, "/base/workspaces")
	info, err := os.Stat(dirPath)
	require.NoError(t, err)

	assert.True(t, info.IsDir())
}
