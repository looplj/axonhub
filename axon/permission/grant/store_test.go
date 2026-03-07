package grant

import (
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStore(t *testing.T) {
	fs := afero.NewMemMapFs()
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", fs))

	require.NotNil(t, store)
	require.NotNil(t, store.once)
	require.NotNil(t, store.thread)
	require.NotNil(t, store.workspace)
	require.NotNil(t, store.global)
}

func TestMemoryStore_Add_ScopeOnce(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeOnce, resources)

	require.Contains(t, store.once, "call-123")
}

func TestMemoryStore_Add_ScopeOnce_Overwrite(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeOnce, resources)
	store.Add(req, ScopeOnce, resources)

	require.Contains(t, store.once, "call-123")
	require.Len(t, store.once, 1)
}

func TestMemoryStore_Add_ScopeThread(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeThread, resources)

	require.NotNil(t, store.thread["thread-456"])
	key := BuildKey(req, resources)
	require.Contains(t, store.thread["thread-456"], key)
}

func TestMemoryStore_Add_ScopeThread_EmptyThreadID(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeThread, resources)

	require.Nil(t, store.thread[""])
}

func TestMemoryStore_Add_ScopeThread_MultipleKeys(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req1 := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-123",
		ToolName:   "Read",
	}
	req2 := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-123",
		ToolName:   "Write",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req1, ScopeThread, resources)
	store.Add(req2, ScopeThread, resources)

	require.NotNil(t, store.thread["thread-123"])
	require.Len(t, store.thread["thread-123"], 2)
}

func TestMemoryStore_Add_ScopeWorkspace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/workspace/test/file.txt"},
	}

	store.Add(req, ScopeWorkspace, resources)

	key := BuildKey(req, resources)
	require.NotNil(t, store.workspace["/workspace/test"])
	require.Contains(t, store.workspace["/workspace/test"], key)
}

func TestMemoryStore_Add_ScopeWorkspace_EmptyWorkspace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		Workspace:  "",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeWorkspace, resources)

	require.Nil(t, store.workspace[""])
}

func TestMemoryStore_Add_ScopeGlobal(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeGlobal, resources)

	key := BuildKey(req, resources)
	require.Contains(t, store.global, key)
}

func TestMemoryStore_Match_ScopeOnce(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeOnce, resources)

	require.True(t, store.Match(req, resources))
	require.NotContains(t, store.once, "call-123")
}

func TestMemoryStore_Match_ScopeOnce_ConsumedOnce(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeOnce, resources)

	require.True(t, store.Match(req, resources))
	require.False(t, store.Match(req, resources))
}

func TestMemoryStore_Match_ScopeThread(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeThread, resources)

	require.True(t, store.Match(req, resources))
	require.True(t, store.Match(req, resources))
}

func TestMemoryStore_Match_ScopeThread_DifferentThread(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeThread, resources)

	otherReq := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-789",
		ToolName:   "Read",
	}
	require.False(t, store.Match(otherReq, resources))
}

func TestMemoryStore_Match_ScopeThread_EmptyThreadID(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeThread, resources)

	emptyThreadReq := Request{
		ToolCallID: "call-123",
		ThreadID:   "",
		ToolName:   "Read",
	}
	require.False(t, store.Match(emptyThreadReq, resources))
}

func TestMemoryStore_Match_ScopeWorkspace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/workspace/test/file.txt"},
	}

	store.Add(req, ScopeWorkspace, resources)

	require.True(t, store.Match(req, resources))
	require.True(t, store.Match(req, resources))
}

func TestMemoryStore_Match_ScopeWorkspace_DifferentWorkspace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/workspace/test/file.txt"},
	}

	store.Add(req, ScopeWorkspace, resources)

	otherReq := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/other",
		ToolName:   "Read",
	}
	require.False(t, store.Match(otherReq, resources))
}

func TestMemoryStore_Match_ScopeWorkspace_EmptyWorkspace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/workspace/test/file.txt"},
	}

	store.Add(req, ScopeWorkspace, resources)

	emptyWsReq := Request{
		ToolCallID: "call-123",
		Workspace:  "",
		ToolName:   "Read",
	}
	require.False(t, store.Match(emptyWsReq, resources))
}

func TestMemoryStore_Match_ScopeGlobal(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeGlobal, resources)

	require.True(t, store.Match(req, resources))
	require.True(t, store.Match(req, resources))
}

func TestMemoryStore_Match_GlobalMatchesAll(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeGlobal, resources)

	reqWithThread := Request{
		ToolCallID: "call-456",
		ThreadID:   "thread-789",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	require.True(t, store.Match(reqWithThread, resources))
}

func TestMemoryStore_Match_NoMatch(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	require.False(t, store.Match(req, resources))
}

func TestMemoryStore_Match_DifferentResources(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	resources1 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test1/file.txt"},
	}
	resources2 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test2/file.txt"},
	}

	store.Add(req, ScopeGlobal, resources1)

	require.True(t, store.Match(req, resources1))
	require.False(t, store.Match(req, resources2))
}

func TestMemoryStore_Match_DifferentToolName(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req1 := Request{
		ToolCallID: "call-123",
		ToolName:   "Read",
	}
	req2 := Request{
		ToolCallID: "call-456",
		ToolName:   "Write",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req1, ScopeGlobal, resources)

	require.True(t, store.Match(req1, resources))
	require.False(t, store.Match(req2, resources))
}

func TestMemoryStore_Match_PriorityOrder(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeOnce, resources)
	store.Add(req, ScopeThread, resources)
	store.Add(req, ScopeWorkspace, resources)
	store.Add(req, ScopeGlobal, resources)

	require.True(t, store.Match(req, resources))

	require.NotContains(t, store.once, "call-123")
	require.Contains(t, store.thread["thread-456"], BuildKey(req, resources))
}

func TestMemoryStore_LoadWorkspace(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)

	keys := map[string]struct{}{
		"key1": {},
		"key2": {},
	}
	err := fileStore.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	store := NewMemoryStore(fileStore)
	err = store.LoadWorkspace("/workspace/test")

	require.NoError(t, err)
	require.Contains(t, store.workspace, "/workspace/test")
	require.Len(t, store.workspace["/workspace/test"], 2)
}

func TestMemoryStore_LoadWorkspace_Empty(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	err := store.LoadWorkspace("")

	require.NoError(t, err)
}

func TestMemoryStore_LoadWorkspace_Whitespace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	err := store.LoadWorkspace("   ")

	require.NoError(t, err)
}

func TestMemoryStore_LoadWorkspace_CleansPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)

	keys := map[string]struct{}{
		"key1": {},
	}
	err := fileStore.SaveWorkspace("/workspace/test", keys)
	require.NoError(t, err)

	store := NewMemoryStore(fileStore)
	err = store.LoadWorkspace("/workspace/test/../test")

	require.NoError(t, err)
	require.Contains(t, store.workspace, "/workspace/test")
}

func TestMemoryStore_SaveWorkspace(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

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

	store2 := NewMemoryStore(fileStore)
	err = store2.LoadWorkspace("/workspace/test")
	require.NoError(t, err)
	require.True(t, store2.Match(req, resources))
}

func TestMemoryStore_SaveWorkspace_Empty(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	err := store.SaveWorkspace("")

	require.NoError(t, err)
}

func TestMemoryStore_SaveWorkspace_NilKeys(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

	err := store.SaveWorkspace("/workspace/test")

	require.NoError(t, err)
}

func TestMemoryStore_LoadGlobal(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)

	keys := map[string]struct{}{
		"key1": {},
		"key2": {},
	}
	err := fileStore.SaveGlobal(keys)
	require.NoError(t, err)

	store := NewMemoryStore(fileStore)
	err = store.LoadGlobal()

	require.NoError(t, err)
	require.Len(t, store.global, 2)
}

func TestMemoryStore_SaveGlobal(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

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

	store2 := NewMemoryStore(fileStore)
	err = store2.LoadGlobal()
	require.NoError(t, err)
	require.True(t, store2.Match(req, resources))
}

func TestMemoryStore_SaveGlobal_NilKeys(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

	err := store.SaveGlobal()

	require.NoError(t, err)
}

func TestBuildKey_ToolName(t *testing.T) {
	req1 := Request{ToolName: "Read"}
	req2 := Request{ToolName: "Write"}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	key1 := BuildKey(req1, resources)
	key2 := BuildKey(req2, resources)

	require.NotEqual(t, key1, key2)
}

func TestBuildKey_ToolNameCaseInsensitive(t *testing.T) {
	req1 := Request{ToolName: "Read"}
	req2 := Request{ToolName: "READ"}
	req3 := Request{ToolName: "read"}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	key1 := BuildKey(req1, resources)
	key2 := BuildKey(req2, resources)
	key3 := BuildKey(req3, resources)

	require.Equal(t, key1, key2)
	require.Equal(t, key1, key3)
}

func TestBuildKey_ResourcePath(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
	require.Len(t, key, 64)
}

func TestBuildKey_ResourcePath_WorkspaceRel(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourcePath, WorkspaceRel: "src/main.go"},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_ResourcePath_WithWorkspaceRel(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources1 := []Resource{
		{Type: ResourcePath, Path: "/workspace/test/src/main.go", WorkspaceRel: "src/main.go"},
	}
	resources2 := []Resource{
		{Type: ResourcePath, Path: "/workspace/test/src/main.go"},
	}

	key1 := BuildKey(req, resources1)
	key2 := BuildKey(req, resources2)

	require.NotEqual(t, key1, key2)
}

func TestBuildKey_ResourcePath_Directory(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources1 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test/file.txt"},
	}
	resources2 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test/other.txt"},
	}

	key1 := BuildKey(req, resources1)
	key2 := BuildKey(req, resources2)

	require.Equal(t, key1, key2)
}

func TestBuildKey_ResourceDomain(t *testing.T) {
	req := Request{ToolName: "Fetch"}
	resources := []Resource{
		{Type: ResourceDomain, Domain: "example.com"},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_ResourceDomain_CaseInsensitive(t *testing.T) {
	req := Request{ToolName: "Fetch"}
	resources1 := []Resource{
		{Type: ResourceDomain, Domain: "Example.COM"},
	}
	resources2 := []Resource{
		{Type: ResourceDomain, Domain: "example.com"},
	}

	key1 := BuildKey(req, resources1)
	key2 := BuildKey(req, resources2)

	require.Equal(t, key1, key2)
}

func TestBuildKey_ResourceDomain_Empty(t *testing.T) {
	req := Request{ToolName: "Fetch"}
	resources := []Resource{
		{Type: ResourceDomain, Domain: ""},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_ResourceCommand(t *testing.T) {
	req := Request{ToolName: "Execute"}
	resources := []Resource{
		{Type: ResourceCommand, Command: "npm install"},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_ResourceCommand_OnlyFirstWord(t *testing.T) {
	req := Request{ToolName: "Execute"}
	resources1 := []Resource{
		{Type: ResourceCommand, Command: "npm install"},
	}
	resources2 := []Resource{
		{Type: ResourceCommand, Command: "npm run build"},
	}

	key1 := BuildKey(req, resources1)
	key2 := BuildKey(req, resources2)

	require.NotEqual(t, key1, key2)
}

func TestBuildKey_ResourceCommand_Empty(t *testing.T) {
	req := Request{ToolName: "Execute"}
	resources := []Resource{
		{Type: ResourceCommand, Command: ""},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_ResourceCommand_Whitespace(t *testing.T) {
	req := Request{ToolName: "Execute"}
	resources := []Resource{
		{Type: ResourceCommand, Command: "   npm   install   "},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_MultipleResources(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
		{Type: ResourceDomain, Domain: "example.com"},
		{Type: ResourceCommand, Command: "cat file.txt"},
	}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_EmptyResources(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{}

	key := BuildKey(req, resources)

	require.NotEmpty(t, key)
}

func TestBuildKey_ConsistentHash(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	key1 := BuildKey(req, resources)
	key2 := BuildKey(req, resources)

	require.Equal(t, key1, key2)
}

func TestBuildKey_DifferentResourceTypes(t *testing.T) {
	req := Request{ToolName: "Tool"}
	resources1 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}
	resources2 := []Resource{
		{Type: ResourceDomain, Domain: "example.com"},
	}
	resources3 := []Resource{
		{Type: ResourceCommand, Command: "npm install"},
	}

	key1 := BuildKey(req, resources1)
	key2 := BuildKey(req, resources2)
	key3 := BuildKey(req, resources3)

	require.NotEqual(t, key1, key2)
	require.NotEqual(t, key2, key3)
	require.NotEqual(t, key1, key3)
}

func TestCommandSummary(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected string
	}{
		{
			name:     "simple command",
			cmd:      "npm install",
			expected: "npm",
		},
		{
			name:     "command with path",
			cmd:      "/usr/bin/git commit",
			expected: "/usr/bin/git",
		},
		{
			name:     "single word",
			cmd:      "ls",
			expected: "ls",
		},
		{
			name:     "empty string",
			cmd:      "",
			expected: "",
		},
		{
			name:     "whitespace only",
			cmd:      "   ",
			expected: "",
		},
		{
			name:     "leading whitespace",
			cmd:      "   npm install",
			expected: "npm",
		},
		{
			name:     "trailing whitespace",
			cmd:      "npm install   ",
			expected: "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := commandSummary(tt.cmd)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCommandSubcommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected string
	}{
		{
			name:     "program only",
			cmd:      "go",
			expected: "",
		},
		{
			name:     "program with subcommand",
			cmd:      "go test ./...",
			expected: "test",
		},
		{
			name:     "program with leading flags",
			cmd:      "go -v test ./...",
			expected: "test",
		},
		{
			name:     "program with only flags",
			cmd:      "go -v -x",
			expected: "",
		},
		{
			name:     "whitespace",
			cmd:      "   go   test   ./... ",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := commandSubcommand(tt.cmd)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMatch_CommandSubcommand_ProgramGrantCoversSubcommand(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Bash",
	}
	// User granted "go" at program-level.
	store.Add(req, ScopeThread, []Resource{
		{Type: ResourceCommand, Command: "go"},
	})

	matchReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Bash",
	}
	require.True(t, store.Match(matchReq, []Resource{
		{Type: ResourceCommand, Command: "go test ./..."},
	}))
}

func TestMatch_CommandSubcommand_SubcommandGrantDoesNotCoverProgram(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Bash",
	}
	store.Add(req, ScopeThread, []Resource{
		{Type: ResourceCommand, Command: "go test ./..."},
	})

	matchReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Bash",
	}
	require.False(t, store.Match(matchReq, []Resource{
		{Type: ResourceCommand, Command: "go"},
	}))
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			req := Request{
				ToolCallID: "call-" + string(rune('0'+id)),
				ThreadID:   "thread-123",
				Workspace:  "/workspace/test",
				ToolName:   "Read",
			}
			resources := []Resource{
				{Type: ResourcePath, Path: "/tmp/test.txt"},
			}

			store.Add(req, ScopeThread, resources)
			store.Match(req, resources)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMemoryStore_ConcurrentMatchAndAdd(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			req := Request{
				ToolCallID: "call-once",
				ToolName:   "Read",
			}
			resources := []Resource{
				{Type: ResourcePath, Path: "/tmp/test.txt"},
			}
			store.Add(req, ScopeOnce, resources)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			req := Request{
				ToolCallID: "call-once",
				ToolName:   "Read",
			}
			resources := []Resource{
				{Type: ResourcePath, Path: "/tmp/test.txt"},
			}
			store.Match(req, resources)
		}
	}()

	wg.Wait()
}

func TestScope_Constants(t *testing.T) {
	require.Equal(t, Scope("once"), ScopeOnce)
	require.Equal(t, Scope("thread"), ScopeThread)
	require.Equal(t, Scope("workspace"), ScopeWorkspace)
	require.Equal(t, Scope("global"), ScopeGlobal)
}

func TestResourceType_Constants(t *testing.T) {
	require.Equal(t, ResourceType("path"), ResourcePath)
	require.Equal(t, ResourceType("domain"), ResourceDomain)
	require.Equal(t, ResourceType("command"), ResourceCommand)
	require.Equal(t, ResourceType("skill"), ResourceSkill)
}

func TestMemoryStore_Interface(t *testing.T) {
	var _ Store = NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
}

func TestEntry_Fields(t *testing.T) {
	entry := Entry{
		ID:        "entry-123",
		CreatedAt: mustParseTime("2024-01-01T00:00:00Z"),
		Scope:     ScopeThread,
		ThreadID:  "thread-456",
		Workspace: "/workspace/test",
		ToolName:  "Read",
		Key:       "abc123",
	}

	require.Equal(t, "entry-123", entry.ID)
	require.Equal(t, ScopeThread, entry.Scope)
	require.Equal(t, "thread-456", entry.ThreadID)
	require.Equal(t, "/workspace/test", entry.Workspace)
	require.Equal(t, "Read", entry.ToolName)
	require.Equal(t, "abc123", entry.Key)
}

func TestRequest_Fields(t *testing.T) {
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}

	require.Equal(t, "call-123", req.ToolCallID)
	require.Equal(t, "thread-456", req.ThreadID)
	require.Equal(t, "/workspace/test", req.Workspace)
	require.Equal(t, "Read", req.ToolName)
}

func TestResource_Fields(t *testing.T) {
	res := Resource{
		Type:             ResourcePath,
		Path:             "/tmp/test.txt",
		WorkspaceRel:     "test.txt",
		OutsideWorkspace: true,
		Domain:           "example.com",
		Command:          "npm install",
	}

	require.Equal(t, ResourcePath, res.Type)
	require.Equal(t, "/tmp/test.txt", res.Path)
	require.Equal(t, "test.txt", res.WorkspaceRel)
	require.True(t, res.OutsideWorkspace)
	require.Equal(t, "example.com", res.Domain)
	require.Equal(t, "npm install", res.Command)
}

func TestBuildKey_Skill(t *testing.T) {
	req := Request{ToolName: "Skill"}
	resources := []Resource{
		{Type: ResourceSkill, Skill: "code-review"},
	}

	key := BuildKey(req, resources)
	require.NotEmpty(t, key)

	// Same skill should produce same key
	key2 := BuildKey(req, resources)
	require.Equal(t, key, key2)

	// Different skill should produce different key
	resources2 := []Resource{
		{Type: ResourceSkill, Skill: "walkthrough"},
	}
	key3 := BuildKey(req, resources2)
	require.NotEqual(t, key, key3)
}

func TestBuildKey_Skill_CaseSensitive(t *testing.T) {
	req := Request{ToolName: "Skill"}
	r1 := []Resource{{Type: ResourceSkill, Skill: "Code-Review"}}
	r2 := []Resource{{Type: ResourceSkill, Skill: "code-review"}}

	// BuildKey lowercases skill, so they should be equal
	require.Equal(t, BuildKey(req, r1), BuildKey(req, r2))
}

func TestMemoryStore_SkillGrant_ThreadScope(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Skill",
	}
	resources := []Resource{
		{Type: ResourceSkill, Skill: "code-review"},
	}

	store.Add(req, ScopeThread, resources)

	require.True(t, store.Match(req, resources))

	// Different thread should not match
	otherReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-2",
		ToolName:   "Skill",
	}
	require.False(t, store.Match(otherReq, resources))
}

func TestBuildKey_PathNormalization(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources1 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test/../test/file.txt"},
	}
	resources2 := []Resource{
		{Type: ResourcePath, Path: "/tmp/test/file.txt"},
	}

	key1 := BuildKey(req, resources1)
	key2 := BuildKey(req, resources2)

	require.Equal(t, key1, key2)
}

func TestMemoryStore_Match_OncePriorityOverOthers(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-123",
		ThreadID:   "thread-456",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}

	store.Add(req, ScopeThread, resources)
	store.Add(req, ScopeOnce, resources)

	require.True(t, store.Match(req, resources))
	require.True(t, store.Match(req, resources))
}

func TestMemoryStore_SaveWorkspace_CleansPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

	req := Request{
		ToolCallID: "call-123",
		Workspace:  "/workspace/test",
		ToolName:   "Read",
	}
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/test.txt"},
	}
	store.Add(req, ScopeWorkspace, resources)

	err := store.SaveWorkspace("/workspace/test/../test")

	require.NoError(t, err)
}

func TestMemoryStore_LoadSave_RoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

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

	store2 := NewMemoryStore(fileStore)
	err = store2.LoadWorkspace("/workspace/test")
	require.NoError(t, err)

	require.True(t, store2.Match(req, resources))
}

func TestMemoryStore_LoadSaveGlobal_RoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

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

	store2 := NewMemoryStore(fileStore)
	err = store2.LoadGlobal()
	require.NoError(t, err)

	require.True(t, store2.Match(req, resources))
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
