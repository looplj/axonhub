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
	"path"
	"slices"
	"strings"

	"github.com/MoeForge/moejs"
)

// PackDirectory archives original plugin files after compiling and linking their
// static imports with the same moejs parser used during installation. It does
// not execute plugin code; runtime and manifest compatibility checks happen at
// installation. Only manifest.json, files under ui/, plugin.js, and files that
// plugin.js reaches through static relative imports are packed. Every other
// path is left out and returned in skipped, collapsed to its highest directory
// that holds no packed file. .DS_Store files are left out without being listed.
//
// cmd/pack-plugin copies this walk together with readPackage, compilePluginSources,
// and rejectDisallowed. Keep those copies aligned. ParsePackage remains the
// install-time check.
func PackDirectory(root string) ([]byte, []string, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	defer dir.Close()
	closure, err := packImportClosure(dir)
	if err != nil {
		return nil, nil, err
	}
	tree, err := scanPackTree(dir, closure)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range tree.irregular {
		if packedByLayout(name) {
			return nil, nil, fmt.Errorf("file %q %s", name, tree.reasons[name])
		}
	}
	var packed, skipped []string
	for _, name := range tree.files {
		if _, ok := closure[name]; ok || packedByLayout(name) {
			packed = append(packed, name)
		} else {
			skipped = append(skipped, name)
		}
	}
	skipped = append(skipped, tree.irregular...)
	skipped = append(skipped, tree.skippedDirs...)
	if len(packed) > MaxPackageFiles {
		return nil, nil, fmt.Errorf("plugin package exceeds %d files", MaxPackageFiles)
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	defer writer.Close()
	total := 0
	for _, name := range packed {
		content, err := readPackFile(dir, name)
		if err != nil {
			return nil, nil, err
		}
		total += len(content)
		if total > MaxUncompressedBytes {
			return nil, nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}
		out, err := writer.Create(name)
		if err != nil {
			return nil, nil, err
		}
		if _, err := out.Write(content); err != nil {
			return nil, nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}
	contents, err := readPackage(buf.Bytes())
	if err != nil {
		return nil, nil, err
	}
	if err := checkPackManifest(contents); err != nil {
		return nil, nil, err
	}
	if _, err := compileContents(contents); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), collapseSkipped(packed, skipped), nil
}

type packTree struct {
	files       []string
	skippedDirs []string
	irregular   []string
	reasons     map[string]string
}

func scanPackTree(dir *os.Root, closure map[string]struct{}) (*packTree, error) {
	tree := &packTree{reasons: map[string]string{}}
	// Only descend into layout directories and ancestors of static imports.
	// Unused development trees are reported once without reading their contents.
	importDirs := map[string]struct{}{}
	for name := range closure {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			importDirs[parent] = struct{}{}
		}
	}
	err := fs.WalkDir(dir.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		switch {
		case name == ".":
		case entry.Type()&os.ModeSymlink != 0:
			tree.irregular = append(tree.irregular, name)
			tree.reasons[name] = "is a symlink"
		case entry.IsDir():
			if _, needed := importDirs[name]; !needed && !packedByLayout(name) {
				tree.skippedDirs = append(tree.skippedDirs, name+"/")
				return fs.SkipDir
			}
		case !entry.Type().IsRegular():
			tree.irregular = append(tree.irregular, name)
			tree.reasons[name] = "is not a regular file"
		case entry.Name() == ".DS_Store":
		default:
			tree.files = append(tree.files, name)
		}
		return nil
	})
	return tree, err
}

func packedByLayout(name string) bool {
	return name == "manifest.json" || name == pluginEntry || name == "ui" || strings.HasPrefix(name, "ui/")
}

// packImportClosure follows static imports from plugin.js on disk so that
// unrelated files are never read or packed.
func packImportClosure(dir *os.Root) (map[string]struct{}, error) {
	closure := map[string]struct{}{}
	if ok, err := regularPackImport(dir, pluginEntry); err != nil {
		return nil, err
	} else if !ok {
		return closure, nil
	}
	var visit func(name string) error
	visit = func(name string) error {
		closure[name] = struct{}{}
		source, err := readPackFile(dir, name)
		if err != nil {
			return err
		}
		mod, err := moejs.Compile(name, string(source))
		if err != nil {
			return err
		}
		for _, specifier := range mod.Requests() {
			resolved, err := resolveImport(name, specifier)
			if err != nil {
				return err
			}
			if _, seen := closure[resolved]; seen {
				continue
			}
			if ok, err := regularPackImport(dir, resolved); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("import %q does not exist in the package", specifier)
			}
			if err := visit(resolved); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(pluginEntry); err != nil {
		return nil, err
	}
	return closure, nil
}

// regularPackImport inspects just the imported path, checking each ancestor
// before accessing descendants so symlinks cannot hide inside an unscanned tree.
func regularPackImport(dir *os.Root, name string) (bool, error) {
	parts := strings.Split(name, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		info, err := dir.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("file %q is a symlink", current)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return false, fmt.Errorf("file %q is not a regular file", current)
		}
		if i == len(parts)-1 {
			return info.Mode().IsRegular() && parts[i] != ".DS_Store", nil
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	return false, nil
}

func readPackFile(dir *os.Root, name string) ([]byte, error) {
	file, err := dir.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxPackageFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > MaxPackageFileBytes {
		return nil, fmt.Errorf("file %q exceeds 5 MiB", name)
	}
	if name == pluginEntry && len(content) > MaxPluginJSBytes {
		return nil, errors.New("plugin.js exceeds 1 MiB")
	}
	return content, nil
}

// collapseSkipped reports a skipped path by its highest ancestor directory that
// contains no packed file, so .git or node_modules appear as one line.
func collapseSkipped(packed, skipped []string) []string {
	packedDirs := map[string]struct{}{}
	for _, name := range packed {
		for current := path.Dir(name); current != "."; current = path.Dir(current) {
			packedDirs[current] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(skipped))
	for _, name := range skipped {
		report := name
		parts := strings.Split(name, "/")
		for i := 1; i < len(parts); i++ {
			ancestor := strings.Join(parts[:i], "/")
			if _, ok := packedDirs[ancestor]; !ok {
				report = ancestor + "/"
				break
			}
		}
		if _, dup := seen[report]; dup {
			continue
		}
		seen[report] = struct{}{}
		out = append(out, report)
	}
	slices.Sort(out)
	return out
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
