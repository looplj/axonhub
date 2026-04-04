package conf

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestRuntimeDirForWorkspaceUsesUserConfigRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := filepath.Join(home, "project")

	dir, err := RuntimeDirForWorkspace(workspace)
	if err != nil {
		t.Fatalf("RuntimeDirForWorkspace() error = %v", err)
	}

	wantBase := filepath.Join(home, ".config", runtimeDirName)

	if runtime.GOOS == "windows" {
		base, err := runtimeBaseDir()
		if err != nil {
			t.Fatalf("runtimeBaseDir() error = %v", err)
		}

		wantBase = base
	}

	want := filepath.Join(wantBase, workspaceHash(workspace))
	if dir != want {
		t.Fatalf("RuntimeDirForWorkspace() = %q, want %q", dir, want)
	}
}
