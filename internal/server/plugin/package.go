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
)

var windowsDrivePattern = regexp.MustCompile(`^[A-Za-z]:`)

// Package is a validated .axplugin archive. ParsePackage does not write the database.
type Package struct {
	Manifest objects.PluginManifest
	SHA256   string
	Files    []string
}

// ParsePackage reads a ZIP, validates manifest.json, and returns the sha256 of
// the original bytes. JS plugins are rejected until that runtime exists.
func ParsePackage(data []byte) (*Package, error) {
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
	fileSet := make(map[string]struct{})
	var manifestBytes []byte
	var sawManifest bool
	var uncompressed uint64
	for _, file := range reader.File {
		cleaned, isDir, err := inspectZipEntry(file)
		if err != nil {
			return nil, err
		}
		if isPluginJS(cleaned) {
			return nil, errors.New("JS plugins are not enabled yet")
		}
		if !isAllowedPackagePath(cleaned, isDir) {
			return nil, fmt.Errorf("file %q is not allowed in a plugin package", cleaned)
		}
		if _, dup := seen[cleaned]; dup {
			return nil, fmt.Errorf("duplicate file %q", cleaned)
		}
		seen[cleaned] = struct{}{}
		if isDir {
			continue
		}
		if file.UncompressedSize64 > MaxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if file.UncompressedSize64 > 0 && uncompressed > MaxUncompressedBytes-file.UncompressedSize64 {
			return nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}

		content, readBytes, err := readZipEntry(file, cleaned == "manifest.json")
		if err != nil {
			return nil, err
		}
		if readBytes < 0 {
			return nil, errors.New("plugin package entry has a negative size")
		}
		if readBytes > MaxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if uncompressed > MaxUncompressedBytes-uint64(readBytes) {
			return nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}
		uncompressed += uint64(readBytes)
		fileSet[cleaned] = struct{}{}
		if cleaned == "manifest.json" {
			sawManifest = true
			manifestBytes = content
		}
	}

	if !sawManifest {
		return nil, errors.New("manifest.json is required")
	}
	manifest, err := ParseManifest(manifestBytes, build.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin manifest: %w", err)
	}
	if manifest.UI != nil {
		for _, page := range manifest.UI.Pages {
			if _, ok := fileSet[page.Entry]; !ok {
				return nil, fmt.Errorf("ui page entry %q does not exist in the package", page.Entry)
			}
		}
	}

	sum := sha256.Sum256(data)
	names := make([]string, 0, len(fileSet))
	for name := range fileSet {
		names = append(names, name)
	}
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

func isPluginJS(cleaned string) bool {
	return cleaned == "plugin.js" || strings.HasSuffix(cleaned, "/plugin.js")
}

func isAllowedPackagePath(cleaned string, isDir bool) bool {
	if cleaned == "manifest.json" {
		return !isDir
	}
	if cleaned == "ui" {
		return isDir
	}
	return strings.HasPrefix(cleaned, "ui/")
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
// Manifest bytes are discarded. The same path and size checks as ParsePackage apply.
func ExtractPackageUI(data []byte, dest string) ([]string, error) {
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
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}
	if err := os.Chmod(dest, 0o700); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}

	seen := make(map[string]struct{}, len(reader.File))
	files := make([]string, 0)
	var uncompressed uint64
	for _, file := range reader.File {
		cleaned, isDir, err := inspectZipEntry(file)
		if err != nil {
			return nil, err
		}
		if isPluginJS(cleaned) {
			return nil, errors.New("JS plugins are not enabled yet")
		}
		if !isAllowedPackagePath(cleaned, isDir) {
			return nil, fmt.Errorf("file %q is not allowed in a plugin package", cleaned)
		}
		if _, dup := seen[cleaned]; dup {
			return nil, fmt.Errorf("duplicate file %q", cleaned)
		}
		seen[cleaned] = struct{}{}

		if file.UncompressedSize64 > MaxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if file.UncompressedSize64 > 0 && uncompressed > MaxUncompressedBytes-file.UncompressedSize64 {
			return nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}

		if isDir {
			if cleaned == "ui" || strings.HasPrefix(cleaned, "ui/") {
				target, err := safeJoin(dest, cleaned)
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
			continue
		}

		var readBytes int
		if strings.HasPrefix(cleaned, "ui/") {
			readBytes, err = writeZipFile(dest, file, cleaned)
			if err != nil {
				return nil, err
			}
			files = append(files, cleaned)
		} else {
			_, readBytes, err = readZipEntry(file, false)
			if err != nil {
				return nil, err
			}
		}
		if readBytes < 0 {
			return nil, errors.New("plugin package entry has a negative size")
		}
		if readBytes > MaxPackageFileBytes {
			return nil, fmt.Errorf("file %q exceeds 5 MiB", cleaned)
		}
		if uncompressed > MaxUncompressedBytes-uint64(readBytes) {
			return nil, errors.New("plugin package uncompressed size exceeds 20 MiB")
		}
		uncompressed += uint64(readBytes)
	}

	if err := syncDir(dest); err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
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
