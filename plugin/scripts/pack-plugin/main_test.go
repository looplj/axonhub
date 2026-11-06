package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "example")
	require.NoError(t, os.Mkdir(root, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("not packaged"), 0o644))
	var stdout bytes.Buffer
	require.NoError(t, run([]string{root}, &stdout))
	packed, err := os.ReadFile(filepath.Join(parent, "example.axplugin"))
	require.NoError(t, err)
	reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
	require.Equal(t, "manifest.json", reader.File[0].Name)

	output := filepath.Join(parent, "out", "custom.axplugin")
	require.NoError(t, run([]string{root, "-o", output}, &stdout))
	require.FileExists(t, output)
	require.ErrorContains(t, run([]string{root, "--output", filepath.Join(root, "bad.axplugin")}, &stdout), "outside")

	// A rejected package must not replace an existing archive.
	require.NoError(t, os.WriteFile(filepath.Join(root, "unused.js"), []byte(""), 0o644))
	require.Error(t, run([]string{"-o", output, root}, &stdout))
	after, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, packed, after)
}
