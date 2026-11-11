package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleTemperatureCapPackage(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugins", "examples", "temperature-cap")
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
		if rel != "manifest.json" && rel != "plugin.js" {
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

	source := string(files["plugin.js"])
	require.Contains(t, source, "export function onRequest")
	require.NotContains(t, source, "onAttempt")
	require.NotContains(t, source, "onEvent")
	require.NotContains(t, source, "stream")
	require.NotContains(t, source, "import ")
	for line := range strings.SplitSeq(source, "\n") {
		if strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "let ") || strings.HasPrefix(line, "var ") {
			t.Errorf("module-level binding %q", line)
		}
	}

	raw := zipExampleFiles(t, files)
	assertExampleTemperatureCapPackage(t, raw)

	packed := packExampleDirectory(t, root)
	assertExampleTemperatureCapPackage(t, packed)
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

func assertExampleTemperatureCapPackage(t *testing.T, raw []byte) {
	t.Helper()
	pkg, err := ParsePackage(raw)
	require.NoError(t, err)
	require.Equal(t, "example.temperature-cap", pkg.Manifest.ID)
	require.Equal(t, "Temperature Cap", pkg.Manifest.Name)
	require.Equal(t, "1.0.0", pkg.Manifest.Version)
	require.Equal(t, "^1.0.0", pkg.Manifest.AxonhubVersion)
	require.Equal(t, 1, pkg.Manifest.PluginAPIVersion)
	require.Empty(t, pkg.Manifest.Permissions)
	require.Empty(t, pkg.Manifest.Events)
	require.Empty(t, pkg.Manifest.Network)
	require.NotNil(t, pkg.Manifest.Hooks)
	require.True(t, pkg.Manifest.Hooks.Request.IsEnabled())
	require.False(t, pkg.Manifest.Hooks.Attempt.IsEnabled())
	require.False(t, pkg.Manifest.Hooks.ResponsePresent)
	require.False(t, pkg.Manifest.Hooks.StreamChunkPresent)
	require.Nil(t, pkg.Manifest.UI)
	require.NotNil(t, pkg.Manifest.ConfigSchema)
	prop, ok := pkg.Manifest.ConfigSchema.Properties["maxTemperature"]
	require.True(t, ok)
	require.Equal(t, "number", prop.Type)
	require.Equal(t, "Max temperature", prop.Title)
	require.Equal(t, "Temperature written on every request. The hook uses 0.2 until this value is saved.", prop.Description)
	require.NotNil(t, prop.Minimum)
	require.Equal(t, 0.0, *prop.Minimum)
	require.NotNil(t, prop.Maximum)
	require.Equal(t, 2.0, *prop.Maximum)
	require.Equal(t, json.Number("0.2"), prop.Default)
	require.Equal(t, []string{"manifest.json", "plugin.js"}, pkg.Files)
}
