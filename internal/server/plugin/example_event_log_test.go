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

func TestExampleEventLogPackage(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugins", "examples", "event-log")
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
		switch {
		case rel == "manifest.json" || rel == "plugin.js":
		case strings.HasPrefix(rel, "lib/") || strings.HasPrefix(rel, "ui/"):
		default:
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

	html := string(files["ui/log/index.html"])
	require.Contains(t, html, `src="../../_axonhub/bridge.js"`)
	require.Contains(t, html, `crossorigin="anonymous"`)
	require.NotContains(t, html, `src="/`)
	require.Contains(t, string(files["ui/log/app.js"]), `from "./format.js"`)
	require.Contains(t, string(files["plugin.js"]), `from "./lib/record.js"`)
	require.Contains(t, string(files["plugin.js"]), "export function onEvent")

	raw := zipExampleFiles(t, files)
	assertExampleEventLogPackage(t, raw)

	packed := packExampleDirectory(t, root)
	assertExampleEventLogPackage(t, packed)
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

func assertExampleEventLogPackage(t *testing.T, raw []byte) {
	t.Helper()
	pkg, err := ParsePackage(raw)
	require.NoError(t, err)
	require.Equal(t, "example.event-log", pkg.Manifest.ID)
	require.Empty(t, pkg.Manifest.Permissions)
	require.Equal(t, []string{
		"channel.credential_disabled",
		"channel.credential_enabled",
		"request.completed",
	}, pkg.Manifest.Events)
	require.Nil(t, pkg.Manifest.Hooks)
	require.Equal(t, "ui/log/index.html", pkg.Manifest.UI.Pages[0].Entry)
	require.Equal(t, []string{
		"lib/record.js",
		"manifest.json",
		"plugin.js",
		"ui/log/app.js",
		"ui/log/format.js",
		"ui/log/index.html",
		"ui/log/style.css",
	}, pkg.Files)
}
