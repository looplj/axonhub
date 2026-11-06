package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
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
	var stdout bytes.Buffer
	if err := run([]string{root}, &stdout); err != nil {
		t.Fatal(err)
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

	if err := os.WriteFile(filepath.Join(root, "unused.js"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-o", output, root}, &stdout); err == nil {
		t.Fatal("expected stray file to fail")
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
