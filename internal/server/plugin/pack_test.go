package plugin

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackDirectoryStaticImportClosure(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{
  "schemaVersion": 1,
  "id": "example.pack-probe",
  "name": "Pack Probe",
  "version": "1.0.0",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1
}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "lib", "a.js"), []byte("export const a = 1\n"), 0o644))
	pluginJS := `
import { a } from "./lib/\u0061.js";
const re = /import\s+["']\.\/lib\/fake\.js["']/;
const nested = ` + "`keep ${`import { b } from \"./lib/fake.js\"`}`" + `;
export function onEvent() {
  return from("./lib/fake.js") + import("./lib/dyn.js") + a + re + nested;
}

`
	require.NoError(t, os.WriteFile(filepath.Join(root, "plugin.js"), []byte(pluginJS), 0o644))

	packed := packExampleDirectory(t, root)
	pkg, err := ParsePackage(packed)
	require.NoError(t, err)
	require.Equal(t, []string{"lib/a.js", "manifest.json", "plugin.js"}, pkg.Files)
}

func TestPackDirectoryMoejsDependencies(t *testing.T) {
	for name, source := range map[string]string{
		"local export without semicolon": "const a = 1; export { a }\nimport './lib/a.js';",
		"nested template expression":     "const value = `${(() => { return 1 })() + `/*`}`;\nimport './lib/a.js';",
		"re-export":                      "export { a } from './lib/a.js';",
		"no top-level execution":         "import './lib/a.js'; throw new Error('must not execute');",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(root, "lib"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "plugin.js"), []byte(source), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "lib/a.js"), []byte("export const a = 1;"), 0o644))
			packed, _, err := PackDirectory(root)
			require.NoError(t, err)
			contents, err := readPackage(packed)
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"manifest.json", "plugin.js", "lib/a.js"}, contents.files)
			require.Equal(t, source, string(contents.sources["plugin.js"]))
		})
	}
}

func TestPackDirectoryRejectsInvalidPackages(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
	}{
		{"missing import", "import './missing.js';", "does not exist"},
		{"escaping import", "import '../outside.js';", "escapes the archive"},
		{"bare import", "import 'node:fs';", "not a package-relative path"},
		{"invalid syntax", "export function {", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "plugin.js"), []byte(tc.source), 0o644))
			_, _, err := PackDirectory(root)
			require.Error(t, err)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestPackDirectorySkipsUnusedFiles(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".git", ".claude/commands", "node_modules/left-pad", "lib", "ui", "docs"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	for name, content := range map[string]string{
		"manifest.json":                  `{"ui":{"pages":[{"entry":"ui/page.html"}]}}`,
		"plugin.js":                      "import { a } from './lib/a.js';\nexport function load() { return import('./dynamic.js') + a; }\n",
		"lib/a.js":                       "export const a = 1;\n",
		"lib/notes.md":                   "notes\n",
		"dynamic.js":                     "export const b = 2;\n",
		"ui/page.html":                   "<html></html>",
		"ui/fixture.test.js":             "keep\n",
		"ui/.DS_Store":                   "junk",
		".DS_Store":                      "junk",
		".git/config":                    "git\n",
		".gitignore":                     "*.axplugin\n",
		".claude/commands/review.md":     "review\n",
		"node_modules/left-pad/index.js": "module.exports = 1\n",
		"docs/guide.md":                  "guide\n",
		"README.md":                      "readme\n",
		"package.json":                   "{}\n",
		"format.test.mjs":                "test\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	require.NoError(t, os.Symlink(filepath.Join(root, "README.md"), filepath.Join(root, ".claude", "link.md")))
	require.NoError(t, os.Symlink(filepath.Join(root, "docs"), filepath.Join(root, "docs-link")))

	packed, skipped, err := PackDirectory(root)
	require.NoError(t, err)
	contents, err := readPackage(packed)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"manifest.json", "plugin.js", "lib/a.js", "ui/page.html", "ui/fixture.test.js"}, contents.files)
	require.Equal(t, []string{
		".claude/", ".git/", ".gitignore", "README.md", "docs-link", "docs/",
		"dynamic.js", "format.test.mjs", "lib/notes.md", "node_modules/", "package.json",
	}, skipped)
}

func TestPackDirectoryRejectsSymlinks(t *testing.T) {
	for name, setup := range map[string]func(t *testing.T, root string){
		"plugin.js": func(t *testing.T, root string) {
			require.NoError(t, os.WriteFile(filepath.Join(root, "real.js"), []byte("export const a = 1;"), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(root, "real.js"), filepath.Join(root, "plugin.js")))
		},
		"ui file": func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "ui"), 0o755))
			require.NoError(t, os.Symlink(filepath.Join(root, "manifest.json"), filepath.Join(root, "ui", "page.html")))
		},
		"ui directory": func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "pages"), 0o755))
			require.NoError(t, os.Symlink(filepath.Join(root, "pages"), filepath.Join(root, "ui")))
		},
		"imported file": func(t *testing.T, root string) {
			require.NoError(t, os.WriteFile(filepath.Join(root, "real.js"), []byte("export const a = 1;"), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(root, "real.js"), filepath.Join(root, "a.js")))
			require.NoError(t, os.WriteFile(filepath.Join(root, "plugin.js"), []byte("import './a.js';"), 0o644))
		},
		"imported directory": func(t *testing.T, root string) {
			require.NoError(t, os.MkdirAll(filepath.Join(root, "real"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "real", "a.js"), []byte("export const a = 1;"), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "lib")))
			require.NoError(t, os.WriteFile(filepath.Join(root, "plugin.js"), []byte("import './lib/a.js';"), 0o644))
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644))
			setup(t, root)
			_, _, err := PackDirectory(root)
			require.ErrorContains(t, err, "symlink")
		})
	}
}

// Unused subtrees must not be opened, even when a sibling contains an import.
func TestPackDirectoryDoesNotWalkUnusedDirectories(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifest.json", `{}`)
	write("plugin.js", "import './node_modules/used/index.js';")
	write("node_modules/used/index.js", "import './nested.js';")
	write("node_modules/used/nested.js", "import './index.js';")
	for _, name := range []string{".git", "docs", "node_modules/unused", "node_modules/used/tests"} {
		write(name+"/unreadable/file", "unused")
		blocked := filepath.Join(root, name, "unreadable")
		if err := os.Chmod(blocked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
	}
	packed, skipped, err := PackDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".git/", "docs/", "node_modules/unused/", "node_modules/used/tests/"}
	if !slices.Equal(skipped, want) {
		t.Fatalf("skipped = %v, want %v", skipped, want)
	}
	contents, err := readPackage(packed)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"manifest.json", "node_modules/used/index.js", "node_modules/used/nested.js", "plugin.js"}
	if !slices.Equal(contents.files, want) {
		t.Fatalf("files = %v, want %v", contents.files, want)
	}
}

func TestPackDirectoryImportsFromHiddenDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"manifest.json":      `{}`,
		"plugin.js":          "import './.git/lib/helper.js';",
		".git/lib/helper.js": "export const value = 1;",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packed, _, err := PackDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := readPackage(packed)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(contents.files, ".git/lib/helper.js") {
		t.Fatalf("missing imported file: %v", contents.files)
	}
	if err := os.Symlink("lib", filepath.Join(root, ".git", "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.js"), []byte("import './.git/linked/helper.js';"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PackDirectory(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected imported ancestor symlink rejection, got %v", err)
	}
}
