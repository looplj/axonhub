package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncateToolOutputLines(t *testing.T) {
	t.Run("does not truncate exact line limit", func(t *testing.T) {
		text := "line 1\nline 2\n"

		require.Equal(t, text, truncateToolOutputLines(text, 2, "hint"))
	})

	t.Run("keeps a single newline before truncation suffix", func(t *testing.T) {
		text := "line 1\nline 2\nline 3\n"

		require.Equal(t, "line 1\nline 2\n... (truncated) hint\n", truncateToolOutputLines(text, 2, "hint"))
	})
}

func TestReadToolExecute_TruncatesLargeOutput(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "large.txt")

	var sb strings.Builder
	for i := range 500 {
		fmt.Fprintf(&sb, "line %03d %s\n", i, strings.Repeat("x", 80))
	}

	require.NoError(t, os.WriteFile(path, []byte(sb.String()), 0o644))

	tool := NewReadTool(workspace, false)
	result := tool.Execute(context.Background(), readInput{Path: path})

	require.NoError(t, result.Error)
	require.NotNil(t, result.Content.Text)
	require.Contains(t, *result.Content.Text, "... (truncated)")
	require.Contains(t, *result.Content.Text, readTruncationHint)
	require.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(*result.Content.Text, "\n"), "\n")), defaultReadMaxLines+2)
	require.LessOrEqual(t, len([]rune(*result.Content.Text)), defaultToolOutputMaxChars)
}

func TestBashToolExecute_TruncatesLargeOutput(t *testing.T) {
	tool := NewBashTool(t.TempDir(), false, false)

	command := "i=0; while [ $i -lt 4000 ]; do echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; i=$((i+1)); done"
	if runtime.GOOS == "windows" {
		command = `1..4000 | ForEach-Object { Write-Output ('x' * 30) }`
	}

	result := tool.Execute(context.Background(), bashInput{Command: command})

	require.NoError(t, result.Error)
	require.NotNil(t, result.Content.Text)
	require.Contains(t, *result.Content.Text, "... (truncated)")
	require.Contains(t, *result.Content.Text, bashTruncationHint)
	require.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(*result.Content.Text, "\n"), "\n")), bashOutputMaxLines+2)
	require.LessOrEqual(t, len([]rune(*result.Content.Text)), defaultToolOutputMaxChars)
}

func TestGrepToolExecute_TruncatesLargeOutput(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "matches.txt")

	var sb strings.Builder
	for i := range 64 {
		fmt.Fprintf(&sb, "needle %03d %s\n", i, strings.Repeat("y", 900))
	}

	require.NoError(t, os.WriteFile(path, []byte(sb.String()), 0o644))

	tool := NewGrepTool(workspace, false)
	result := tool.Execute(context.Background(), grepInput{
		Pattern:    "needle",
		Path:       path,
		OutputMode: "content",
	})

	require.NoError(t, result.Error)
	require.NotNil(t, result.Content.Text)
	require.Contains(t, *result.Content.Text, "... (truncated)")
	require.Contains(t, *result.Content.Text, grepTruncationHint)
	require.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(*result.Content.Text, "\n"), "\n")), grepOutputMaxLines+2)
	require.LessOrEqual(t, len([]rune(*result.Content.Text)), defaultToolOutputMaxChars)
}

func TestGlobToolExecute_PreservesInternalTruncationMarkerWithoutToolLevelLineTruncation(t *testing.T) {
	workspace := t.TempDir()

	for i := range 210 {
		name := fmt.Sprintf("f%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(workspace, name), []byte("x"), 0o644))
	}

	tool := NewGlobTool(workspace, false)
	result := tool.Execute(context.Background(), globInput{Pattern: "*.txt"})

	require.NoError(t, result.Error)
	require.NotNil(t, result.Content.Text)
	require.Contains(t, *result.Content.Text, fmt.Sprintf("... (showing first %d results)", 200))
	require.NotContains(t, *result.Content.Text, globTruncationHint)
	require.NotContains(t, *result.Content.Text, "... (truncated)")
}

func TestGlobToolExecute_TruncatesLargeOutput(t *testing.T) {
	workspace := t.TempDir()

	for i := range 200 {
		name := fmt.Sprintf("file_%03d_%s.txt", i, strings.Repeat("z", 64))
		require.NoError(t, os.WriteFile(filepath.Join(workspace, name), []byte("x"), 0o644))
	}

	tool := NewGlobTool(workspace, false)
	result := tool.Execute(context.Background(), globInput{Pattern: "*.txt"})

	require.NoError(t, result.Error)
	require.NotNil(t, result.Content.Text)
	require.LessOrEqual(t, len(strings.Split(strings.TrimSuffix(*result.Content.Text, "\n"), "\n")), globOutputMaxLines+1)
	require.LessOrEqual(t, len([]rune(*result.Content.Text)), defaultToolOutputMaxChars)
}
