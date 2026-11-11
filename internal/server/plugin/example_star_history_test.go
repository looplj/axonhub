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

func TestExampleStarHistoryPackage(t *testing.T) {
	root := filepath.Join("..", "..", "..", "plugins", "examples", "star-history")
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

	html := string(files["ui/stars/index.html"])
	require.Contains(t, html, `src="../../_axonhub/bridge.js"`)
	require.Contains(t, html, `crossorigin="anonymous"`)
	require.NotContains(t, html, `src="/`)
	require.NotContains(t, html, `href="/`)
	app := string(files["ui/stars/app.js"])
	require.Contains(t, app, `from "./format.js"`)
	require.Contains(t, app, "axonhub.fetch")
	assertNoDirectNetwork(t, app)
	assertNoDirectNetwork(t, string(files["ui/stars/format.js"]))
	require.NotContains(t, app, "Authorization")
	format := string(files["ui/stars/format.js"])
	require.Contains(t, format, "https://api.star-history.com/chart")
	require.Contains(t, format, "data:image/svg+xml")
	require.NotContains(t, app, "/stargazers")
	require.NotContains(t, app, "starred_at")
	require.NotContains(t, app, "innerHTML")
	require.Contains(t, html, `id="chart"`)
	require.NotContains(t, html, "api.star-history.com")
	require.NotContains(t, files, "plugin.js")

	raw := zipExampleFiles(t, files)
	packed := packExampleDirectory(t, root)
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

	assertExampleStarHistoryPackage(t, raw)
	assertExampleStarHistoryPackage(t, packed)
}

func assertExampleStarHistoryPackage(t *testing.T, raw []byte) {
	t.Helper()
	pkg, err := ParsePackage(raw)
	if err != nil && networkPending(err) {
		t.Fatalf("waiting on backend: manifest.network is not enabled yet: %v", err)
	}
	require.NoError(t, err)
	require.Equal(t, "example.star-history", pkg.Manifest.ID)
	require.Empty(t, pkg.Manifest.Permissions)
	require.Empty(t, pkg.Manifest.Events)
	require.Nil(t, pkg.Manifest.Hooks)
	require.Equal(t, []string{
		"https://api.github.com/repos/looplj/axonhub",
		"https://api.star-history.com/chart",
	}, pkg.Manifest.NetworkGrants())
	require.Equal(t, "https://api.github.com/repos/looplj/axonhub", pkg.Manifest.Network[0].Exact)
	require.Empty(t, pkg.Manifest.Network[0].Regex)
	require.Equal(t, "https://api.star-history.com/chart", pkg.Manifest.Network[1].Exact)
	require.Empty(t, pkg.Manifest.Network[1].Regex)
	require.Equal(t, "stars", pkg.Manifest.UI.Pages[0].Key)
	require.Equal(t, "ui/stars/index.html", pkg.Manifest.UI.Pages[0].Entry)
	title, err := json.Marshal(pkg.Manifest.UI.Pages[0].Title)
	require.NoError(t, err)
	require.JSONEq(t, `{"en":"Star history","zh-CN":"Star 历史"}`, string(title))
	require.Equal(t, []string{
		"manifest.json",
		"ui/stars/app.js",
		"ui/stars/format.js",
		"ui/stars/index.html",
		"ui/stars/style.css",
	}, pkg.Files)
}

func assertNoDirectNetwork(t *testing.T, source string) {
	t.Helper()
	stripped := strings.ReplaceAll(source, "axonhub.fetch", "")
	compact := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\t', '\r':
			return -1
		default:
			return r
		}
	}, stripped)
	require.NotContains(t, compact, "fetch(")
	require.NotContains(t, source, "XMLHttpRequest")
	require.NotContains(t, source, "WebSocket")
}

func networkPending(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "network is not enabled") || strings.Contains(msg, "unknown field network")
}
