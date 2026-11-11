package plugin

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleFailoverGuardPackage(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugins", "examples", "failover-guard")
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
		if rel != "manifest.json" && rel != "plugin.js" && !strings.HasPrefix(rel, "lib/") {
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
	require.Contains(t, string(files["plugin.js"]), `from "./lib/key.js"`)
	require.Contains(t, string(files["plugin.js"]), "export function onEvent")
	require.NotContains(t, files, "ui/page.html")

	raw := zipExampleFiles(t, files)
	assertExampleFailoverGuardPackage(t, raw)

	packed := packExampleDirectory(t, root)
	assertExampleFailoverGuardPackage(t, packed)
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

func assertExampleFailoverGuardPackage(t *testing.T, raw []byte) {
	t.Helper()
	pkg, err := ParsePackage(raw)
	require.NoError(t, err)
	require.Equal(t, "example.failover-guard", pkg.Manifest.ID)
	require.Equal(t, []string{"write_channels", "webhook"}, pkg.Manifest.Permissions)
	require.Equal(t, []string{"request.failover"}, pkg.Manifest.Events)
	require.Nil(t, pkg.Manifest.Hooks)
	require.Nil(t, pkg.Manifest.UI)
	require.Equal(t, []string{"lib/key.js", "manifest.json", "plugin.js"}, pkg.Files)
}
