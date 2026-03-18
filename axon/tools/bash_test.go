package tools

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipWithoutRtk(t *testing.T) string {
	t.Helper()

	path, err := exec.LookPath("rtk")
	if err != nil {
		t.Skip("rtk not installed, skipping")
	}

	return path
}

func TestDetectRtk(t *testing.T) {
	skipWithoutRtk(t)

	path := detectRtk()
	require.NotEmpty(t, path, "detectRtk should find rtk")
}

func TestNewBashToolRtkAware(t *testing.T) {
	skipWithoutRtk(t)

	t.Run("enabled", func(t *testing.T) {
		tool := NewBashTool(t.TempDir(), false, true)
		assert.NotEmpty(t, tool.rtkPath)
	})

	t.Run("disabled", func(t *testing.T) {
		tool := NewBashTool(t.TempDir(), false, false)
		assert.Empty(t, tool.rtkPath)
	})
}

func TestRtkRewrite(t *testing.T) {
	rtkPath := skipWithoutRtk(t)

	tool := &BashTool{
		workingDir: t.TempDir(),
		rtkPath:    rtkPath,
	}

	t.Run("rewrites git status", func(t *testing.T) {
		got := tool.rtkRewrite("git status")
		assert.Equal(t, "rtk git status", got)
	})

	t.Run("rewrites ls", func(t *testing.T) {
		got := tool.rtkRewrite("ls -la")
		assert.Contains(t, got, "rtk")
	})

	t.Run("rewrites cargo test", func(t *testing.T) {
		got := tool.rtkRewrite("cargo test")
		assert.Equal(t, "rtk cargo test", got)
	})

	t.Run("skips non-rewritable command", func(t *testing.T) {
		got := tool.rtkRewrite("echo hello")
		assert.Equal(t, "echo hello", got)
	})

	t.Run("skips mkdir", func(t *testing.T) {
		got := tool.rtkRewrite("mkdir -p /tmp/foo")
		assert.Equal(t, "mkdir -p /tmp/foo", got)
	})

	t.Run("skips already rewritten command", func(t *testing.T) {
		got := tool.rtkRewrite("rtk git status")
		assert.Equal(t, "rtk git status", got)
	})

	t.Run("skips empty command", func(t *testing.T) {
		got := tool.rtkRewrite("")
		assert.Equal(t, "", got)
	})
}

func TestRtkRewriteDisabledWhenNoRtk(t *testing.T) {
	tool := &BashTool{
		workingDir: t.TempDir(),
		rtkPath:    "",
	}

	got := tool.rtkRewrite("git status")
	assert.Equal(t, "git status", got)
}

func TestBashExecuteRtkComparison(t *testing.T) {
	rtkPath := skipWithoutRtk(t)

	dir := t.TempDir()

	// Initialize a git repo so git commands work.
	for _, args := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "test"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}

	withRtk := &BashTool{workingDir: dir, rtkPath: rtkPath}
	withoutRtk := &BashTool{workingDir: dir, rtkPath: ""}

	// Create a commit so git log works.
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run())

	tests := []struct {
		name    string
		command string
	}{
		{"git status", "git status"},
		{"git log", "git log --oneline -5"},
		{"ls", "ls -la"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			rtkResult := withRtk.Execute(ctx, bashInput{Command: tt.command})
			rawResult := withoutRtk.Execute(ctx, bashInput{Command: tt.command})

			require.NoError(t, rtkResult.Error)
			require.NoError(t, rawResult.Error)
			require.NotNil(t, rtkResult.Content.Text)
			require.NotNil(t, rawResult.Content.Text)

			rtkOut := *rtkResult.Content.Text
			rawOut := *rawResult.Content.Text

			t.Logf("=== rtk output (%d bytes) ===\n%s", len(rtkOut), rtkOut)
			t.Logf("=== raw output (%d bytes) ===\n%s", len(rawOut), rawOut)

			// rtk output should be shorter or equal (compressed).
			assert.LessOrEqual(t, len(rtkOut), len(rawOut),
				"rtk output should not be longer than raw output")
		})
	}
}
