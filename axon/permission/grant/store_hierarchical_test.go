package grant

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestBuildHierarchicalKeys_NoDir(t *testing.T) {
	req := Request{ToolName: "Fetch"}
	resources := []Resource{
		{Type: ResourceDomain, Domain: "example.com"},
	}

	keys := buildHierarchicalKeys(req, resources)

	require.Len(t, keys, 1)
	require.Equal(t, BuildKey(req, resources), keys[0])
}

func TestBuildHierarchicalKeys_SingleLevelDir(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}

	keys := buildHierarchicalKeys(req, resources)

	require.Equal(t, BuildKey(req, resources), keys[0])

	parentRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "."},
	}
	require.Contains(t, keys, BuildKey(req, parentRes))
}

func TestBuildHierarchicalKeys_NestedDir(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg/util"},
	}

	keys := buildHierarchicalKeys(req, resources)

	require.Equal(t, BuildKey(req, resources), keys[0])

	expected := []string{"src/pkg/util", "src/pkg", "src", "."}
	for _, exp := range expected {
		expRes := []Resource{{Type: ResourceDir, WorkspaceRel: exp}}
		require.Contains(t, keys, BuildKey(req, expRes), "should include key for dir %q", exp)
	}
}

func TestBuildHierarchicalKeys_AbsoluteDir(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourceDir, Path: "/tmp/a/b/c"},
	}

	keys := buildHierarchicalKeys(req, resources)

	expectedDirs := []string{"/tmp/a/b/c", "/tmp/a/b", "/tmp/a", "/tmp", "/"}
	for _, exp := range expectedDirs {
		expRes := []Resource{{Type: ResourceDir, Path: exp}}
		require.Contains(t, keys, BuildKey(req, expRes), "should include key for dir %q", exp)
	}
}

func TestBuildHierarchicalKeys_WorkspaceRoot(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "."},
	}

	keys := buildHierarchicalKeys(req, resources)

	require.Equal(t, BuildKey(req, resources), keys[0])
}

func TestBuildHierarchicalKeys_AbsoluteRoot(t *testing.T) {
	req := Request{ToolName: "Read"}
	resources := []Resource{
		{Type: ResourceDir, Path: "/tmp"},
	}

	keys := buildHierarchicalKeys(req, resources)

	require.Equal(t, BuildKey(req, resources), keys[0])
}

func TestBuildHierarchicalKeys_MixedResources(t *testing.T) {
	req := Request{ToolName: "Bash"}
	resources := []Resource{
		{Type: ResourceCommand, Command: "go test"},
		{Type: ResourceDir, WorkspaceRel: "src/pkg"},
	}

	keys := buildHierarchicalKeys(req, resources)

	require.Equal(t, BuildKey(req, resources), keys[0])
	require.Contains(t, keys, BuildKey(req, []Resource{
		{Type: ResourceCommand, Command: "go test"},
		{Type: ResourceDir, WorkspaceRel: "src"},
	}))
	require.Contains(t, keys, BuildKey(req, []Resource{
		{Type: ResourceCommand, Command: "go test"},
		{Type: ResourceDir, WorkspaceRel: "."},
	}))
}

func TestBuildKey_DirResource_WorkspaceRel(t *testing.T) {
	req := Request{ToolName: "Read"}
	r1 := []Resource{{Type: ResourceDir, WorkspaceRel: "src"}}
	r2 := []Resource{{Type: ResourceDir, WorkspaceRel: "src/pkg"}}

	k1 := BuildKey(req, r1)
	k2 := BuildKey(req, r2)

	require.NotEqual(t, k1, k2)
}

func TestBuildKey_DirResource_AbsPath(t *testing.T) {
	req := Request{ToolName: "Read"}
	r1 := []Resource{{Type: ResourceDir, Path: "/tmp/a"}}
	r2 := []Resource{{Type: ResourceDir, Path: "/tmp/b"}}

	k1 := BuildKey(req, r1)
	k2 := BuildKey(req, r2)

	require.NotEqual(t, k1, k2)
}

func TestBuildKey_DirResource_PathNormalization(t *testing.T) {
	req := Request{ToolName: "Read"}
	r1 := []Resource{{Type: ResourceDir, WorkspaceRel: "src/pkg/../pkg"}}
	r2 := []Resource{{Type: ResourceDir, WorkspaceRel: "src/pkg"}}

	require.Equal(t, BuildKey(req, r1), BuildKey(req, r2))
}

func TestMatch_HierarchicalDir_ParentGrantCoversChild_Thread(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	parentResources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}
	store.Add(req, ScopeThread, parentResources)

	childReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	childResources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg/util"},
	}

	require.True(t, store.Match(childReq, childResources))
}

func TestMatch_HierarchicalDir_ParentGrantCoversChild_Workspace(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		Workspace:  "/workspace",
		ToolName:   "Read",
	}
	parentResources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}
	store.Add(req, ScopeWorkspace, parentResources)

	childReq := Request{
		ToolCallID: "call-2",
		Workspace:  "/workspace",
		ToolName:   "Read",
	}
	childResources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg"},
	}

	require.True(t, store.Match(childReq, childResources))
}

func TestMatch_HierarchicalDir_ParentGrantCoversChild_Global(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ToolName:   "Read",
	}
	parentResources := []Resource{
		{Type: ResourceDir, Path: "/tmp/data"},
	}
	store.Add(req, ScopeGlobal, parentResources)

	childReq := Request{
		ToolCallID: "call-2",
		ToolName:   "Read",
	}
	childResources := []Resource{
		{Type: ResourceDir, Path: "/tmp/data/sub/deep"},
	}

	require.True(t, store.Match(childReq, childResources))
}

func TestMatch_HierarchicalDir_ChildDoesNotCoverParent(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	childResources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg"},
	}
	store.Add(req, ScopeThread, childResources)

	parentReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	parentResources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}

	require.False(t, store.Match(parentReq, parentResources))
}

func TestMatch_HierarchicalDir_SiblingNotCovered(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	res := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg"},
	}
	store.Add(req, ScopeThread, res)

	siblingReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	siblingRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/cmd"},
	}

	require.False(t, store.Match(siblingReq, siblingRes))
}

func TestMatch_HierarchicalDir_DifferentToolNotCovered(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	res := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}
	store.Add(req, ScopeThread, res)

	childReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Write",
	}
	childRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg"},
	}

	require.False(t, store.Match(childReq, childRes))
}

func TestMatch_HierarchicalDir_WorkspaceRootCoversAll(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	rootRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "."},
	}
	store.Add(req, ScopeThread, rootRes)

	deepReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	deepRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "a/b/c/d"},
	}

	require.True(t, store.Match(deepReq, deepRes))
}

func TestMatch_HierarchicalDir_ExactDirMatchPreferred(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	exactRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg/util"},
	}
	store.Add(req, ScopeThread, exactRes)

	matchReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}

	require.True(t, store.Match(matchReq, exactRes))
}

func TestMatch_HierarchicalDir_PersistenceRoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()
	fileStore := NewFileStoreWithFS("/base", fs)
	store := NewMemoryStore(fileStore)

	req := Request{
		ToolCallID: "call-1",
		Workspace:  "/workspace",
		ToolName:   "Read",
	}
	parentRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}
	store.Add(req, ScopeWorkspace, parentRes)
	err := store.SaveWorkspace("/workspace")
	require.NoError(t, err)

	store2 := NewMemoryStore(fileStore)
	err = store2.LoadWorkspace("/workspace")
	require.NoError(t, err)

	childReq := Request{
		ToolCallID: "call-2",
		Workspace:  "/workspace",
		ToolName:   "Read",
	}
	childRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg/util"},
	}
	require.True(t, store2.Match(childReq, childRes))
}

func TestMatch_HierarchicalDir_AbsolutePathWalk(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	parentRes := []Resource{
		{Type: ResourceDir, Path: "/home/user"},
	}
	store.Add(req, ScopeThread, parentRes)

	childReq := Request{
		ToolCallID: "call-2",
		ThreadID:   "thread-1",
		ToolName:   "Read",
	}
	childRes := []Resource{
		{Type: ResourceDir, Path: "/home/user/projects/myapp"},
	}
	require.True(t, store.Match(childReq, childRes))

	uncoveredRes := []Resource{
		{Type: ResourceDir, Path: "/home/other/projects"},
	}
	require.False(t, store.Match(childReq, uncoveredRes))
}

func TestMatch_HierarchicalDir_OnceScope_NoHierarchy(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ToolName:   "Read",
	}
	res := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}
	store.Add(req, ScopeOnce, res)

	require.True(t, store.Match(req, res))
	require.False(t, store.Match(req, res))
}

func TestMatch_HierarchicalDir_CommandResourceNoHierarchy(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Bash",
	}
	res := []Resource{
		{Type: ResourceCommand, Command: "npm install"},
	}
	store.Add(req, ScopeThread, res)

	require.True(t, store.Match(req, res))

	differentCmd := []Resource{
		{Type: ResourceCommand, Command: "npm run build"},
	}
	require.False(t, store.Match(req, differentCmd))
}

func TestMatch_HierarchicalDir_ReadDirGrantCoversFileReadsInDir(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		Workspace:  "/workspace",
		ToolName:   "Read",
	}
	// Grant on directory-only resource (typical when reading a directory path).
	store.Add(req, ScopeWorkspace, []Resource{
		{Type: ResourceDir, WorkspaceRel: "cmd/axonclaw"},
	})

	matchReq := Request{
		ToolCallID: "call-2",
		Workspace:  "/workspace",
		ToolName:   "Read",
	}
	// File read extracts both path and dir resources.
	matchResources := []Resource{
		{Type: ResourcePath, WorkspaceRel: "cmd/axonclaw/main.go"},
		{Type: ResourceDir, WorkspaceRel: "cmd/axonclaw"},
	}
	require.True(t, store.Match(matchReq, matchResources))
}

func TestMatch_HierarchicalDir_WriteDirGrantCoversFileWritesInDir(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		Workspace:  "/workspace",
		ToolName:   "Write",
	}
	store.Add(req, ScopeWorkspace, []Resource{
		{Type: ResourceDir, WorkspaceRel: "cmd/axonclaw"},
	})

	matchReq := Request{
		ToolCallID: "call-2",
		Workspace:  "/workspace",
		ToolName:   "Write",
	}
	matchResources := []Resource{
		{Type: ResourcePath, WorkspaceRel: "cmd/axonclaw/main.go"},
		{Type: ResourceDir, WorkspaceRel: "cmd/axonclaw"},
	}
	require.True(t, store.Match(matchReq, matchResources))
}

func TestMatch_HierarchicalDir_DomainResourceNoHierarchy(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "WebFetch",
	}
	res := []Resource{
		{Type: ResourceDomain, Domain: "example.com"},
	}
	store.Add(req, ScopeThread, res)

	require.True(t, store.Match(req, res))

	otherDomain := []Resource{
		{Type: ResourceDomain, Domain: "other.com"},
	}
	require.False(t, store.Match(req, otherDomain))
}

func TestMatch_HierarchicalDir_SkillResourceNoHierarchy(t *testing.T) {
	store := NewMemoryStore(NewFileStoreWithFS("/tmp", afero.NewMemMapFs()))
	req := Request{
		ToolCallID: "call-1",
		ThreadID:   "thread-1",
		ToolName:   "Skill",
	}
	res := []Resource{
		{Type: ResourceSkill, Skill: "deploy"},
	}
	store.Add(req, ScopeThread, res)

	require.True(t, store.Match(req, res))

	otherSkill := []Resource{
		{Type: ResourceSkill, Skill: "review"},
	}
	require.False(t, store.Match(req, otherSkill))
}
