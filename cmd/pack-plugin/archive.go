// Copied from internal/server/plugin so this module does not depend on the
// AxonHub server. Keep the directory walk, import closure, readPackage, and
// moejs link checks aligned with PackDirectory, readPackage,
// compilePluginSources, and rejectDisallowed.
// This copy does not execute plugin.js. Installation ParsePackage still checks
// the manifest, AxonHub version range, and named exports, and can reject a zip
// this command accepts.
package main

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
	"regexp"
	"slices"
	"strings"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/syntax"
)

const (
	maxPackageBytes      = 5 << 20
	maxUncompressedBytes = 20 << 20
	maxPackageFiles      = 200
	maxPackageFileBytes  = 5 << 20
	maxPluginJSBytes     = 1 << 20
	pluginEntry          = "plugin.js"
)

var windowsDrivePattern = regexp.MustCompile(`^[A-Za-z]:`)

func packDirectory(root string) ([]byte, []string, error) {
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
	if len(packed) > maxPackageFiles {
		return nil, nil, fmt.Errorf("plugin package exceeds %d files", maxPackageFiles)
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
		if total > maxUncompressedBytes {
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
	content, err := io.ReadAll(io.LimitReader(file, maxPackageFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxPackageFileBytes {
		return nil, fmt.Errorf("file %q exceeds 5 MiB", name)
	}
	if name == pluginEntry && len(content) > maxPluginJSBytes {
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
		} `json:"ui"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("invalid plugin manifest: %w", err)
	}
	if manifest.UI != nil {
		for _, page := range manifest.UI.Pages {
			if !slices.Contains(contents.files, page.Entry) {
				return fmt.Errorf("ui page entry %q does not exist in the package", page.Entry)
			}
		}
	}
	return nil
}

type packageContents struct {
	data    []byte
	sources map[string][]byte
	files   []string
	uiFiles []string
	dirs    []string
}

func (c *packageContents) hasPluginJS() bool {
	if c == nil {
		return false
	}
	_, ok := c.sources[pluginEntry]
	return ok
}

func (c *packageContents) lookupSource(name string) ([]byte, bool, error) {
	if c == nil {
		return nil, false, nil
	}
	if content, ok := c.sources[name]; ok {
		return content, true, nil
	}
	if c.data == nil || !slices.Contains(c.uiFiles, name) {
		return nil, false, nil
	}
	content, err := readNamedZipEntry(c.data, name)
	if err != nil {
		return nil, false, err
	}
	c.sources[name] = content
	return content, true, nil
}

func readNamedZipEntry(data []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("plugin package is not a valid zip: %w", err)
	}
	for _, file := range reader.File {
		cleaned, isDir, err := inspectZipEntry(file)
		if err != nil {
			return nil, err
		}
		if isDir || cleaned != name {
			continue
		}
		content, _, err := readZipEntry(file, true)
		if err != nil {
			return nil, err
		}
		return content, nil
	}
	return nil, fmt.Errorf("import %q does not exist in the package", name)
}

func readPackage(data []byte) (*packageContents, error) {
	if len(data) == 0 {
		return nil, errors.New("plugin package is empty")
	}
	if len(data) > maxPackageBytes {
		return nil, errors.New("plugin package exceeds 5 MiB")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("plugin package is not a valid zip: %w", err)
	}
	if len(reader.File) > maxPackageFiles {
		return nil, fmt.Errorf("plugin package exceeds %d files", maxPackageFiles)
	}
	seen := make(map[string]struct{}, len(reader.File))
	contents := &packageContents{data: data, sources: map[string][]byte{}}
	var uncompressed uint64
	for _, file := range reader.File {
		cleaned, isDir, err := inspectZipEntry(file)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[cleaned]; dup {
			return nil, fmt.Errorf("duplicate file %q", cleaned)
		}
		seen[cleaned] = struct{}{}
		if isDir {
			contents.dirs = append(contents.dirs, cleaned)
			continue
		}
		if file.UncompressedSize64 > maxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if cleaned == pluginEntry && file.UncompressedSize64 > uint64(maxPluginJSBytes) {
			return nil, errors.New("plugin.js exceeds 1 MiB")
		}
		if file.UncompressedSize64 > 0 && uncompressed > maxUncompressedBytes-file.UncompressedSize64 {
			return nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}
		keep := !strings.HasPrefix(cleaned, "ui/")
		content, readBytes, err := readZipEntry(file, keep)
		if err != nil {
			return nil, err
		}
		if readBytes < 0 {
			return nil, errors.New("plugin package entry has a negative size")
		}
		if readBytes > maxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if cleaned == pluginEntry && readBytes > maxPluginJSBytes {
			return nil, errors.New("plugin.js exceeds 1 MiB")
		}
		if uncompressed > maxUncompressedBytes-uint64(readBytes) {
			return nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}
		uncompressed += uint64(readBytes)
		contents.files = append(contents.files, cleaned)
		if strings.HasPrefix(cleaned, "ui/") {
			contents.uiFiles = append(contents.uiFiles, cleaned)
			continue
		}
		contents.sources[cleaned] = content
	}
	return contents, nil
}

func rejectDisallowed(contents *packageContents, closure map[string]struct{}) error {
	if contents == nil {
		return errors.New("plugin package is empty")
	}
	allowed := make(map[string]struct{}, len(contents.files))
	for _, name := range contents.files {
		if name == "manifest.json" || strings.HasPrefix(name, "ui/") {
			allowed[name] = struct{}{}
			continue
		}
		if _, ok := closure[name]; ok {
			allowed[name] = struct{}{}
			continue
		}
		return fmt.Errorf("file %q is not allowed in a plugin package", name)
	}
	for _, dir := range contents.dirs {
		if dirAllowed(dir, allowed) {
			continue
		}
		return fmt.Errorf("file %q is not allowed in a plugin package", dir)
	}
	return nil
}

func dirAllowed(dir string, allowed map[string]struct{}) bool {
	if dir == "ui" || strings.HasPrefix(dir, "ui/") {
		return true
	}
	prefix := dir + "/"
	for name := range allowed {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func inspectZipEntry(file *zip.File) (string, bool, error) {
	if file.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("file %q is a symlink", file.Name)
	}
	name := file.Name
	if name == "" {
		return "", false, errors.New("plugin package contains an empty file path")
	}
	if strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return "", false, fmt.Errorf("file path %q is not allowed", name)
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) || windowsDrivePattern.MatchString(name) {
		return "", false, fmt.Errorf("file path %q is absolute", name)
	}
	if slices.Contains(strings.Split(name, "/"), "..") {
		return "", false, fmt.Errorf("file path %q escapes the archive", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, fmt.Errorf("file path %q escapes the archive", name)
	}
	isDir := file.FileInfo().IsDir() || strings.HasSuffix(name, "/")
	if cleaned == "manifest.json" && isDir {
		return "", false, errors.New("manifest.json must be a file")
	}
	return cleaned, isDir, nil
}

func readZipEntry(file *zip.File, keep bool) ([]byte, int, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, 0, fmt.Errorf("read file %q: %w", file.Name, err)
	}
	defer reader.Close()
	limited := io.LimitReader(reader, maxPackageFileBytes+1)
	if !keep {
		n, err := io.Copy(io.Discard, limited)
		if err != nil {
			return nil, 0, fmt.Errorf("read file %q: %w", file.Name, err)
		}
		return nil, int(n), nil
	}
	var buf bytes.Buffer
	n, err := io.Copy(&buf, limited)
	if err != nil {
		return nil, 0, fmt.Errorf("read file %q: %w", file.Name, err)
	}
	return append([]byte(nil), buf.Bytes()...), int(n), nil
}

type pluginProgram struct {
	module   *moejs.Module
	compiled map[string]*moejs.Module
}

func (p *pluginProgram) closure() map[string]struct{} {
	if p == nil {
		return nil
	}
	out := make(map[string]struct{}, len(p.compiled))
	for name := range p.compiled {
		out[name] = struct{}{}
	}
	return out
}

func compilePluginSources(contents *packageContents) (*pluginProgram, error) {
	if contents == nil || !contents.hasPluginJS() {
		return nil, errors.New("plugin.js is required")
	}
	prog := &pluginProgram{compiled: map[string]*moejs.Module{}}
	var compileOne func(name string) error
	compileOne = func(name string) error {
		if _, ok := prog.compiled[name]; ok {
			return nil
		}
		source, ok, err := contents.lookupSource(name)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("import %q does not exist in the package", name)
		}
		mod, err := moejs.Compile(name, string(source))
		if err != nil {
			return err
		}
		if err := rejectTopLevelAwait(name, string(source)); err != nil {
			return err
		}
		prog.compiled[name] = mod
		for _, specifier := range mod.Requests() {
			resolved, err := resolveImport(name, specifier)
			if err != nil {
				return err
			}
			if _, ok, err := contents.lookupSource(resolved); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("import %q does not exist in the package", specifier)
			}
			if err := compileOne(resolved); err != nil {
				return err
			}
		}
		return nil
	}
	if err := compileOne(pluginEntry); err != nil {
		return nil, err
	}
	linked, err := moejs.Link(prog.compiled[pluginEntry], prog.resolve)
	if err != nil {
		return nil, err
	}
	prog.module = linked
	return prog, nil
}

func (p *pluginProgram) resolve(referrer moejs.Referrer, specifier string) (*moejs.Module, error) {
	from := pluginEntry
	if referrer != nil && referrer.Name() != "" {
		from = referrer.Name()
	}
	resolved, err := resolveImport(from, specifier)
	if err != nil {
		return nil, err
	}
	mod, ok := p.compiled[resolved]
	if !ok {
		return nil, fmt.Errorf("import %q does not exist in the package", specifier)
	}
	return mod, nil
}

func rejectTopLevelAwait(name, source string) error {
	parsed, err := syntax.ParseModule(name, source, syntax.Options{})
	if err != nil {
		return err
	}
	if !parsed.Async {
		return nil
	}
	return fmt.Errorf("%s: top-level await is not allowed", name)
}

func resolveImport(from, specifier string) (string, error) {
	if specifier == "" || strings.Contains(specifier, "\\") || strings.ContainsRune(specifier, 0) {
		return "", fmt.Errorf("import %q is not a package-relative path", specifier)
	}
	if strings.HasPrefix(specifier, "node:") {
		return "", fmt.Errorf("import %q is not a package-relative path", specifier)
	}
	if strings.HasPrefix(specifier, "/") || path.IsAbs(specifier) || windowsDrivePattern.MatchString(specifier) {
		return "", fmt.Errorf("import %q is absolute", specifier)
	}
	if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
		return "", fmt.Errorf("import %q is not a package-relative path", specifier)
	}
	resolved := path.Clean(path.Join(path.Dir(from), specifier))
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") || strings.HasPrefix(resolved, "/") || windowsDrivePattern.MatchString(resolved) {
		return "", fmt.Errorf("import %q escapes the archive", specifier)
	}
	return resolved, nil
}

func compileContents(contents *packageContents) (*moejs.Module, error) {
	if contents == nil || !contents.hasPluginJS() {
		if err := rejectDisallowed(contents, nil); err != nil {
			return nil, err
		}
		return nil, nil
	}
	prog, err := compilePluginSources(contents)
	if err != nil {
		return nil, err
	}
	if err := rejectDisallowed(contents, prog.closure()); err != nil {
		return nil, err
	}
	return prog.module, nil
}
