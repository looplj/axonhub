package plugin

import (
	"os"
	"path/filepath"
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
			packed, err := PackDirectory(root)
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
		name, source, extra, want string
	}{
		{"missing import", "import './missing.js';", "", "does not exist"},
		{"escaping import", "import '../outside.js';", "", "escapes the archive"},
		{"bare import", "import 'node:fs';", "", "not a package-relative path"},
		{"invalid syntax", "export function {", "", ""},
		{"unreferenced file", "export const a = 1;", "unused.js", "not allowed"},
		{"dynamic import only", "export function load() { return import('./unused.js'); }", "unused.js", "not allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "plugin.js"), []byte(tc.source), 0o644))
			if tc.extra != "" {
				require.NoError(t, os.WriteFile(filepath.Join(root, tc.extra), []byte("export const a = 1;"), 0o644))
			}
			_, err := PackDirectory(root)
			require.Error(t, err)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			}
		})
	}
}

func TestPackDirectoryRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(root, "manifest.json"), filepath.Join(root, "README.md")))
	_, err := PackDirectory(root)
	require.ErrorContains(t, err, "symlink")
}
