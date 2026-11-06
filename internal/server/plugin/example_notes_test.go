package plugin

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleNotesPackage(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugin", "examples", "notes")
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "README.md" {
			return nil
		}
		if rel != "manifest.json" && !strings.HasPrefix(rel, "ui/") {
			t.Errorf("unexpected example file %s", rel)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	require.NoError(t, err)

	html := string(files["ui/notes/index.html"])
	require.Contains(t, html, `src="../../_axonhub/bridge.js"`)
	require.Contains(t, html, `crossorigin="anonymous"`)
	require.NotContains(t, html, `src="/`)
	require.Contains(t, string(files["ui/notes/app.js"]), `from "./format.js"`)
	require.NotContains(t, files, "plugin.js")

	raw := zipExampleFiles(t, files)
	assertExampleNotesPackage(t, raw)

	packed := packExampleWithScript(t, root)
	assertExampleNotesPackage(t, packed)
	reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	require.NoError(t, err)
	require.Len(t, reader.File, len(files))
	for _, file := range reader.File {
		content, err := file.Open()
		require.NoError(t, err)
		got, err := io.ReadAll(content)
		require.NoError(t, content.Close())
		require.NoError(t, err)
		require.Equal(t, files[file.Name], got, file.Name)
	}
}

func assertExampleNotesPackage(t *testing.T, raw []byte) {
	t.Helper()
	pkg, err := ParsePackage(raw)
	require.NoError(t, err)
	require.Equal(t, "example.notes", pkg.Manifest.ID)
	require.Equal(t, []string{"read_channels"}, pkg.Manifest.Permissions)
	require.Empty(t, pkg.Manifest.Events)
	require.Nil(t, pkg.Manifest.Hooks)
	require.Equal(t, "ui/notes/index.html", pkg.Manifest.UI.Pages[0].Entry)
	require.Equal(t, []string{
		"manifest.json",
		"ui/assets/mark.svg",
		"ui/notes/app.js",
		"ui/notes/format.js",
		"ui/notes/index.html",
		"ui/notes/style.css",
	}, pkg.Files)
}

func packExampleWithScript(t *testing.T, root string) []byte {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	script := filepath.Join("..", "..", "..", "plugin", "scripts", "pack-plugin.py")
	out := filepath.Join(t.TempDir(), "notes.axplugin")
	cmd := exec.Command("python3", script, root, "-o", out)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	packed, err := os.ReadFile(out)
	require.NoError(t, err)
	return packed
}

func zipExampleFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}
