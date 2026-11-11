package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "example")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("not packaged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("git"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "ci.yml"), []byte("name: ci\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.axplugin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "format.test.mjs"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "unused.js"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{root}, &stdout); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Skipped (not used by the plugin):", "  .claude/\n", "  .git/\n", "  .github/\n", "  README.md\n", "  unused.js\n"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout %q does not contain %q", stdout.String(), want)
		}
	}
	packed, err := os.ReadFile(filepath.Join(parent, "example.axplugin"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "manifest.json" {
		t.Fatalf("zip files = %#v", reader.File)
	}

	output := filepath.Join(parent, "out", "custom.axplugin")
	if err := run([]string{root, "-o", output}, &stdout); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{root, "--output", filepath.Join(root, "bad.axplugin")}, &stdout); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("inside output error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "plugin.js"), []byte("import './missing.js';"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-o", output, root}, &stdout); err == nil {
		t.Fatal("expected missing import to fail")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packed, after) {
		t.Fatal("failed pack replaced the existing archive")
	}
}

func TestPackDoesNotExecutePlugin(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "hook")
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"ui":{"pages":[{"entry":"ui/index.html"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib", "a.js"), []byte("export const a = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := "import { a } from './lib/a.js';\nexport function onRequest() { return a; }\nthrow new Error('must not execute');\n"
	if err := os.WriteFile(filepath.Join(root, "plugin.js"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{root}, &stdout); err != nil {
		t.Fatal(err)
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
	packed, skipped, err := packDirectory(root)
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
	packed, _, err := packDirectory(root)
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
	if _, _, err := packDirectory(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected imported ancestor symlink rejection, got %v", err)
	}
}
