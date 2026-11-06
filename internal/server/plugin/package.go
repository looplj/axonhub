package plugin

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/objects"
)

const (
	// MaxPackageBytes is the maximum size of an uploaded ZIP archive.
	MaxPackageBytes      = 5 << 20
	MaxUncompressedBytes = 20 << 20
	MaxPackageFiles      = 200
	MaxPackageFileBytes  = 5 << 20
	// MaxPluginJSBytes is the maximum size of the root plugin.js entry.
	// Other files, including ui/plugin.js, keep MaxPackageFileBytes.
	MaxPluginJSBytes = 1 << 20
)

var windowsDrivePattern = regexp.MustCompile(`^[A-Za-z]:`)

// Package is a validated .axplugin archive. ParsePackage does not write the database.
type Package struct {
	Manifest objects.PluginManifest
	SHA256   string
	Files    []string
}

// ParsePackage reads a ZIP, validates manifest.json, and returns the sha256 of
// the original bytes. When root plugin.js is present it compiles, links, and
// runs that module, then checks named exports. It does not write the database.
func ParsePackage(data []byte) (*Package, error) {
	contents, err := readPackage(data)
	if err != nil {
		return nil, err
	}
	manifestBytes, ok := contents.sources["manifest.json"]
	if !ok {
		return nil, errors.New("manifest.json is required")
	}
	manifest, err := ParseManifest(manifestBytes, build.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin manifest: %w", err)
	}

	var prog *pluginProgram
	if contents.hasPluginJS() {
		prog, err = compilePluginSources(contents)
		if err != nil {
			return nil, err
		}
	}
	if err := rejectDisallowed(contents, prog.closure()); err != nil {
		return nil, err
	}
	fileSet := make(map[string]struct{}, len(contents.files))
	for _, name := range contents.files {
		fileSet[name] = struct{}{}
	}
	if manifest.UI != nil {
		for _, page := range manifest.UI.Pages {
			if _, ok := fileSet[page.Entry]; !ok {
				return nil, fmt.Errorf("ui page entry %q does not exist in the package", page.Entry)
			}
		}
	}
	if prog != nil {
		exports, err := prog.namedExports()
		if err != nil {
			return nil, err
		}
		if err := ValidateExports(manifest, exports); err != nil {
			return nil, err
		}
	} else if err := ValidateExports(manifest, nil); err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)
	names := slices.Clone(contents.files)
	slices.Sort(names)
	return &Package{
		Manifest: *manifest,
		SHA256:   hex.EncodeToString(sum[:]),
		Files:    names,
	}, nil
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

	limited := io.LimitReader(reader, MaxPackageFileBytes+1)
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

// ExtractPackageUI streams ui/ files from a package into dest.
// Each file is copied with io.Copy, fsynced, and closed. File bytes are not retained.
// Root plugin.js is checked as the backend entry and is not written.
// ui/plugin.js is a UI asset and is written. The same path and size checks as ParsePackage apply.
func ExtractPackageUI(data []byte, dest string) ([]string, error) {
	contents, err := readPackage(data)
	if err != nil {
		return nil, err
	}
	if _, err := compileContents(contents); err != nil {
		return nil, err
	}
	return writePreparedUI(data, dest, contents.uiFiles, contents.dirs)
}

// packageContents is one validated ZIP before allow-list and manifest checks.
// sources keeps non-UI file bytes. UI bytes are read only when plugin.js imports them.
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

// lookupSource returns the bytes of a package file. UI files are read from the
// ZIP only when plugin.js imports them, and are cached for this compile.
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
	if len(data) > MaxPackageBytes {
		return nil, errors.New("plugin package exceeds 5 MiB")
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("plugin package is not a valid zip: %w", err)
	}
	if len(reader.File) > MaxPackageFiles {
		return nil, fmt.Errorf("plugin package exceeds %d files", MaxPackageFiles)
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
		if file.UncompressedSize64 > MaxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if cleaned == pluginEntry && file.UncompressedSize64 > uint64(MaxPluginJSBytes) {
			return nil, errors.New("plugin.js exceeds 1 MiB")
		}
		if file.UncompressedSize64 > 0 && uncompressed > MaxUncompressedBytes-file.UncompressedSize64 {
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
		if readBytes > MaxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if cleaned == pluginEntry && readBytes > MaxPluginJSBytes {
			return nil, errors.New("plugin.js exceeds 1 MiB")
		}
		if uncompressed > MaxUncompressedBytes-uint64(readBytes) {
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

// rejectDisallowed allows manifest.json, ui/**, and the static import closure.
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

func writePreparedUI(data []byte, dest string, uiFiles, dirs []string) ([]string, error) {
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}
	if err := os.Chmod(dest, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("plugin package is not a valid zip: %w", err)
	}
	byName := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		cleaned, isDir, err := inspectZipEntry(file)
		if err != nil {
			return nil, err
		}
		if isDir {
			continue
		}
		byName[cleaned] = file
	}
	for _, dir := range dirs {
		if dir != "ui" && !strings.HasPrefix(dir, "ui/") {
			continue
		}
		target, err := safeJoin(dest, dir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(target, 0o700); err != nil {
			return nil, err
		}
		if err := os.Chmod(target, 0o700); err != nil {
			return nil, err
		}
	}
	written := make([]string, 0, len(uiFiles))
	for _, name := range uiFiles {
		file := byName[name]
		if file == nil {
			return nil, fmt.Errorf("file %q is not in the plugin package", name)
		}
		if _, err := writeZipFile(dest, file, name); err != nil {
			return nil, err
		}
		written = append(written, name)
	}
	if err := syncDir(dest); err != nil {
		return nil, err
	}
	slices.Sort(written)
	return written, nil
}

func writeZipFile(dest string, file *zip.File, cleaned string) (int, error) {
	target, err := safeJoin(dest, cleaned)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, fmt.Errorf("create directory for %q: %w", cleaned, err)
	}
	if err := os.Chmod(filepath.Dir(target), 0o700); err != nil {
		return 0, fmt.Errorf("create directory for %q: %w", cleaned, err)
	}

	reader, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("read file %q: %w", file.Name, err)
	}
	defer reader.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("write file %q: %w", cleaned, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
	}()

	n, err := io.Copy(out, io.LimitReader(reader, MaxPackageFileBytes+1))
	if err != nil {
		return int(n), fmt.Errorf("write file %q: %w", cleaned, err)
	}
	if err := out.Sync(); err != nil {
		return int(n), fmt.Errorf("write file %q: %w", cleaned, err)
	}
	if err := out.Close(); err != nil {
		return int(n), fmt.Errorf("write file %q: %w", cleaned, err)
	}
	closed = true
	if err := os.Chmod(target, 0o600); err != nil {
		return int(n), fmt.Errorf("write file %q: %w", cleaned, err)
	}
	return int(n), nil
}

func safeJoin(root, name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("file path %q is not allowed", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("file path %q escapes the archive", name)
	}
	target := filepath.Join(root, filepath.FromSlash(cleaned))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("file path %q escapes the archive", name)
	}
	return target, nil
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
