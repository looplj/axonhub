package plugin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePackage(t *testing.T) {
	t.Run("dotdot", func(t *testing.T) {
		for _, name := range []string{"../secret.txt", "ui/../../secret.txt"} {
			raw := mustZip(t, map[string]string{name: "nope"})
			_, err := ParsePackage(raw)
			require.Error(t, err, name)
			require.ErrorContains(t, err, "escapes the archive")
			require.ErrorContains(t, err, "..")
		}
	})

	t.Run("oversize zip", func(t *testing.T) {
		_, err := ParsePackage(bytes.Repeat([]byte{1}, MaxPackageBytes+1))
		require.ErrorContains(t, err, "5 MiB")
	})

	t.Run("oversize file", func(t *testing.T) {
		raw := mustZipBytes(t, map[string][]byte{
			"ui/big.html": bytes.Repeat([]byte{0}, MaxPackageFileBytes+1),
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "5 MiB")
	})

	t.Run("oversize uncompressed", func(t *testing.T) {
		files := make(map[string][]byte, 5)
		for i := range 4 {
			files[fmt.Sprintf("ui/part%d.html", i)] = bytes.Repeat([]byte{0}, MaxPackageFileBytes)
		}
		files["ui/extra.html"] = []byte{0}
		raw := mustZipBytes(t, files)
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "20 MiB")
	})

	t.Run("too many files", func(t *testing.T) {
		files := make(map[string]string, MaxPackageFiles+1)
		for i := range MaxPackageFiles + 1 {
			files[fmt.Sprintf("ui/f%d.html", i)] = "x"
		}
		_, err := ParsePackage(mustZip(t, files))
		require.ErrorContains(t, err, "200 files")
	})

	t.Run("illegal manifest", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": `{"schemaVersion":2}`,
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "schemaVersion must be 1")
	})

	t.Run("plugin.js", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
			"plugin.js":     "export {}",
			"ui/page.html":  "<html></html>",
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		require.Equal(t, []string{"manifest.json", "plugin.js", "ui/page.html"}, pkg.Files)
	})

	t.Run("plugin.js under ui", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
			"ui/plugin.js":  "export {}",
			"ui/page.html":  "<html></html>",
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		require.Equal(t, []string{"manifest.json", "ui/page.html", "ui/plugin.js"}, pkg.Files)
	})

	t.Run("ui plugin.js is not the backend entry", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
			"ui/plugin.js":  "this is not javascript {{{",
			"ui/page.html":  "<html></html>",
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		require.Contains(t, pkg.Files, "ui/plugin.js")
	})

	t.Run("invalid plugin.js", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "export function (",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "SyntaxError")
		require.ErrorContains(t, err, "plugin.js")
	})

	t.Run("bare import", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "import lodash from \"lodash\"\nexport {}",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "lodash")
		require.ErrorContains(t, err, "package-relative")
	})

	t.Run("node import", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "import fs from \"node:fs\"\nexport {}",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "node:fs")
		require.ErrorContains(t, err, "package-relative")
	})

	t.Run("import escapes the archive", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "import secret from \"../secret.js\"\nexport {}",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "escapes the archive")
		require.ErrorContains(t, err, "../secret.js")
	})

	t.Run("absolute import", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "import secret from \"/etc/passwd\"\nexport {}",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "absolute")
		require.ErrorContains(t, err, "/etc/passwd")
	})

	t.Run("import a ui module", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
			"ui/page.html":  "<html></html>",
			"ui/helper.js":  "export const label = \"ui\"\n",
			"plugin.js":     "import { label } from \"./ui/helper.js\"\nexport const value = label\n",
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		require.Contains(t, pkg.Files, "plugin.js")
		require.Contains(t, pkg.Files, "ui/helper.js")

		dir := t.TempDir()
		files, err := ExtractPackageUI(raw, dir)
		require.NoError(t, err)
		require.Contains(t, files, "ui/helper.js")
		require.NotContains(t, files, "plugin.js")
		body, err := os.ReadFile(filepath.Join(dir, "ui", "helper.js"))
		require.NoError(t, err)
		require.Contains(t, string(body), "label")
		_, err = os.Stat(filepath.Join(dir, "plugin.js"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("relative import", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "import { label } from \"./lib/label.js\"\nexport function onEvent() { return label }\n",
			"lib/label.js":  "export const label = \"ok\"\n",
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		require.Equal(t, []string{"lib/label.js", "manifest.json", "plugin.js"}, pkg.Files)
	})

	t.Run("file outside the import closure", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "export {}",
			"notes.txt":     "hello",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "notes.txt")
		require.ErrorContains(t, err, "not allowed")
	})

	t.Run("plugin.js exceeds 1 MiB", func(t *testing.T) {
		raw := mustZipBytes(t, map[string][]byte{
			"manifest.json": []byte(manifestWithoutUI(t)),
			"plugin.js":     bytes.Repeat([]byte{' '}, MaxPluginJSBytes+1),
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "1 MiB")
	})

	t.Run("dynamic code is disabled", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": manifestWithoutUI(t),
			"plugin.js":     "eval(\"1\")\nexport {}\n",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "EvalError")
	})

	t.Run("stray file", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
			"README.md":     "hello",
			"ui/page.html":  "<html></html>",
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "README.md")
		require.ErrorContains(t, err, "not allowed")
	})

	t.Run("valid ui package", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
			"ui/page.html":  "<html></html>",
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		sum := sha256.Sum256(raw)
		require.Equal(t, hex.EncodeToString(sum[:]), pkg.SHA256)
		require.Equal(t, "acme.failover-guard", pkg.Manifest.ID)
		require.Equal(t, []string{"manifest.json", "ui/page.html"}, pkg.Files)
	})

	t.Run("manifest only", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": string(mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
				delete(manifest, "ui")
			})),
		})
		pkg, err := ParsePackage(raw)
		require.NoError(t, err)
		require.Nil(t, pkg.Manifest.UI)
	})

	t.Run("missing page", func(t *testing.T) {
		raw := mustZip(t, map[string]string{
			"manifest.json": validManifestJSON,
		})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, `ui page entry "ui/page.html" does not exist`)
	})

	t.Run("symlink", func(t *testing.T) {
		var buf bytes.Buffer
		writer := zip.NewWriter(&buf)
		header := &zip.FileHeader{Name: "ui/link.html"}
		header.SetMode(os.ModeSymlink | 0o777)
		entry, err := writer.CreateHeader(header)
		require.NoError(t, err)
		_, err = entry.Write([]byte("ui/page.html"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		_, err = ParsePackage(buf.Bytes())
		require.ErrorContains(t, err, "symlink")
	})

	t.Run("absolute path", func(t *testing.T) {
		raw := mustZip(t, map[string]string{"/etc/passwd": "root"})
		_, err := ParsePackage(raw)
		require.ErrorContains(t, err, "absolute")
	})
}

func manifestWithoutUI(t *testing.T) string {
	t.Helper()
	return string(mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
		delete(manifest, "ui")
	}))
}

func mustZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	payloads := make(map[string][]byte, len(files))
	for name, content := range files {
		payloads[name] = []byte(content)
	}
	return mustZipBytes(t, payloads)
}

func mustZipBytes(t *testing.T, files map[string][]byte) []byte {
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
