package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// PackDirectory archives original plugin files after compiling and linking their
// static imports with the same moejs parser used during installation. It does
// not execute plugin code; runtime and manifest compatibility checks happen at
// installation. README.md at the root and .DS_Store files are excluded.
//
// cmd/pack-plugin copies this walk together with readPackage, compilePluginSources,
// and rejectDisallowed. Keep those copies aligned. ParsePackage remains the
// install-time check.
func PackDirectory(root string) ([]byte, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	defer writer.Close()
	count, total := 0, 0
	// Keep traversal and reads rooted even if a path is replaced with a symlink
	// between inspecting the directory entry and opening its contents.
	err = fs.WalkDir(dir.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("file %q is a symlink", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("file %q is not a regular file", name)
		}
		if name == "README.md" || entry.Name() == ".DS_Store" {
			return nil
		}
		count++
		if count > MaxPackageFiles {
			return fmt.Errorf("plugin package exceeds %d files", MaxPackageFiles)
		}
		file, err := dir.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, MaxPackageFileBytes+1))
		if err != nil {
			return err
		}
		if len(content) > MaxPackageFileBytes {
			return fmt.Errorf("file %q exceeds 5 MiB", name)
		}
		if name == pluginEntry && len(content) > MaxPluginJSBytes {
			return errors.New("plugin.js exceeds 1 MiB")
		}
		total += len(content)
		if total > MaxUncompressedBytes {
			return errors.New("plugin package uncompressed size exceeds 20 MiB")
		}
		out, err := writer.Create(name)
		if err != nil {
			return err
		}
		_, err = out.Write(content)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	contents, err := readPackage(buf.Bytes())
	if err != nil {
		return nil, err
	}
	if err := checkPackManifest(contents); err != nil {
		return nil, err
	}
	if _, err := compileContents(contents); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Packing checks the manifest shape and UI entries without coupling the archive
// to the packer's AxonHub version or executing its JavaScript exports.
func checkPackManifest(contents *packageContents) error {
	raw, ok := contents.sources["manifest.json"]
	if !ok {
		return errors.New("manifest.json is required")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return errors.New("invalid plugin manifest: manifest.json must be an object")
	}
	var manifest struct {
		UI *struct {
			Pages []struct {
				Entry string `json:"entry"`
			} `json:"pages"`
			Actions []struct {
				Effect struct {
					Entry string `json:"entry"`
				} `json:"effect"`
			} `json:"actions"`
		} `json:"ui"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("invalid plugin manifest: %w", err)
	}
	if manifest.UI != nil {
		for _, action := range manifest.UI.Actions {
			if !slices.Contains(contents.files, action.Effect.Entry) {
				return fmt.Errorf("ui action entry %q does not exist in the package", action.Effect.Entry)
			}
		}
		for _, page := range manifest.UI.Pages {
			if !slices.Contains(contents.files, page.Entry) {
				return fmt.Errorf("ui page entry %q does not exist in the package", page.Entry)
			}
		}
	}
	return nil
}
