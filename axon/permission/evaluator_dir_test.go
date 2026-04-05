package permission

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/permission/grant"
	"github.com/looplj/axonhub/axon/permission/policy"
)

func TestToGrantResources_Dir(t *testing.T) {
	in := []policy.Resource{
		{
			Type:             policy.ResourceDir,
			Path:             "/workspace/src",
			WorkspaceRel:     "src",
			OutsideWorkspace: false,
		},
	}

	out := toGrantResources(in)

	require.Len(t, out, 1)
	require.Equal(t, grant.ResourceDir, out[0].Type)
	require.Equal(t, "/workspace/src", out[0].Path)
	require.Equal(t, "src", out[0].WorkspaceRel)
	require.False(t, out[0].OutsideWorkspace)
}

func TestToGrantResources_AllTypes(t *testing.T) {
	in := []policy.Resource{
		{Type: policy.ResourceDir, Path: "/workspace/src", WorkspaceRel: "src"},
		{Type: policy.ResourcePath, Path: "/workspace/file.txt", WorkspaceRel: "file.txt"},
		{Type: policy.ResourceCommand, Command: "ls"},
		{Type: policy.ResourceURL, URL: "https://example.com", Domain: "example.com", Scheme: "https"},
		{Type: policy.ResourceDomain, Domain: "example.com"},
		{Type: policy.ResourceSkill, Skill: "deploy"},
	}

	out := toGrantResources(in)
	require.Len(t, out, 5, "url resources are ignored by grant store")
}

func TestToGrantResources_SkipsUnknownTypes(t *testing.T) {
	in := []policy.Resource{
		{Type: policy.ResourceDir, Path: "/workspace/src", WorkspaceRel: "src"},
		{Type: policy.ResourceURL, URL: "https://example.com"},
	}

	out := toGrantResources(in)

	require.Len(t, out, 1)
	require.Equal(t, grant.ResourceDir, out[0].Type)
}

func TestToGrantResources_MixedWithDir(t *testing.T) {
	in := []policy.Resource{
		{Type: policy.ResourceDir, Path: "/workspace/src", WorkspaceRel: "src"},
		{Type: policy.ResourceCommand, Command: "go test"},
		{Type: policy.ResourceDomain, Domain: "example.com"},
		{Type: policy.ResourceSkill, Skill: "deploy"},
	}

	out := toGrantResources(in)

	require.Len(t, out, 4)
	require.Equal(t, grant.ResourceDir, out[0].Type)
	require.Equal(t, grant.ResourceCommand, out[1].Type)
	require.Equal(t, grant.ResourceDomain, out[2].Type)
	require.Equal(t, grant.ResourceSkill, out[3].Type)
}
