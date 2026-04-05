package glob

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FileEntry struct {
	Name    string
	Content string
}

type SliceFS []FileEntry

func (s SliceFS) Open(name string) (fs.File, error) {
	if name == "." {
		return &sliceFile{name: ".", isDir: true, fs: s}, nil
	}
	for _, entry := range s {
		if entry.Name == name {
			return &sliceFile{name: name, content: []byte(entry.Content), fs: s}, nil
		}
	}
	prefix := name + "/"
	for _, entry := range s {
		if strings.HasPrefix(entry.Name, prefix) {
			return &sliceFile{name: name, isDir: true, fs: s}, nil
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (s SliceFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		name = ""
	} else {
		name = name + "/"
	}

	seen := make(map[string]bool)
	var entries []fs.DirEntry

	for _, entry := range s {
		if !strings.HasPrefix(entry.Name, name) {
			continue
		}
		rel := strings.TrimPrefix(entry.Name, name)
		if rel == "" {
			continue
		}
		parts := strings.SplitN(rel, "/", 2)
		entryName := parts[0]
		if seen[entryName] {
			continue
		}
		seen[entryName] = true
		isDir := len(parts) > 1
		entries = append(entries, &sliceDirEntry{name: entryName, isDir: isDir})
	}

	if len(entries) == 0 && name != "" {
		trimmed := strings.TrimSuffix(name, "/")
		for _, entry := range s {
			if entry.Name == trimmed {
				return nil, &fs.PathError{Op: "readdir", Path: trimmed, Err: fmt.Errorf("not a directory")}
			}
		}
		return nil, &fs.PathError{Op: "readdir", Path: trimmed, Err: fs.ErrNotExist}
	}

	return entries, nil
}

func (s SliceFS) Stat(name string) (fs.FileInfo, error) {
	if name == "." {
		return &sliceFileInfo{name: ".", isDir: true}, nil
	}
	for _, entry := range s {
		if entry.Name == name {
			return &sliceFileInfo{name: name, size: int64(len(entry.Content))}, nil
		}
	}
	prefix := name + "/"
	for _, entry := range s {
		if strings.HasPrefix(entry.Name, prefix) {
			return &sliceFileInfo{name: name, isDir: true}, nil
		}
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

type sliceFile struct {
	name    string
	content []byte
	offset  int64
	isDir   bool
	fs      SliceFS
}

func (f *sliceFile) Stat() (fs.FileInfo, error) {
	if f.isDir {
		return &sliceFileInfo{name: f.name, isDir: true}, nil
	}
	return &sliceFileInfo{name: f.name, size: int64(len(f.content))}, nil
}

func (f *sliceFile) Read(b []byte) (int, error) {
	if f.isDir {
		return 0, fmt.Errorf("is a directory")
	}
	if f.offset >= int64(len(f.content)) {
		return 0, io.EOF
	}
	n := copy(b, f.content[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *sliceFile) Close() error {
	return nil
}

func (f *sliceFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.isDir {
		return nil, fmt.Errorf("not a directory")
	}
	return f.fs.ReadDir(f.name)
}

type sliceDirEntry struct {
	name  string
	isDir bool
}

func (e *sliceDirEntry) Name() string      { return e.name }
func (e *sliceDirEntry) IsDir() bool       { return e.isDir }
func (e *sliceDirEntry) Type() fs.FileMode { return 0 }
func (e *sliceDirEntry) Info() (fs.FileInfo, error) {
	return &sliceFileInfo{name: e.name, isDir: e.isDir}, nil
}

type sliceFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *sliceFileInfo) Name() string       { return fi.name }
func (fi *sliceFileInfo) Size() int64        { return fi.size }
func (fi *sliceFileInfo) Mode() fs.FileMode  { return 0644 }
func (fi *sliceFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *sliceFileInfo) IsDir() bool        { return fi.isDir }
func (fi *sliceFileInfo) Sys() interface{}   { return nil }

var _ fs.FS = SliceFS{}
var _ fs.StatFS = SliceFS{}
var _ fs.ReadDirFS = SliceFS{}

func NewSliceFS(files map[string]string) SliceFS {
	var entries []FileEntry
	for name, content := range files {
		entries = append(entries, FileEntry{Name: name, Content: content})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func TestGlobber_Glob(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		opts           Options
		wantErr        bool
		wantErrContain string
		wantMatches    []string
		wantTruncated  bool
	}{
		{
			name: "empty pattern should error",
			files: map[string]string{
				"test.go": "content",
			},
			opts: Options{
				Pattern: "",
			},
			wantErr:        true,
			wantErrContain: "pattern is required",
		},
		{
			name: "simple glob pattern",
			files: map[string]string{
				"main.go":   "content",
				"utils.go":  "content",
				"README.md": "content",
			},
			opts: Options{
				Pattern: "*.go",
			},
			wantMatches: []string{"main.go", "utils.go"},
		},
		{
			name: "glob with extension",
			files: map[string]string{
				"main.go":       "content",
				"main_test.go":  "content",
				"utils.go":      "content",
				"utils_test.go": "content",
			},
			opts: Options{
				Pattern: "*_test.go",
			},
			wantMatches: []string{"main_test.go", "utils_test.go"},
		},
		{
			name: "no matches found",
			files: map[string]string{
				"main.go": "content",
			},
			opts: Options{
				Pattern: "*.js",
			},
			wantMatches: nil,
		},
		{
			name: "glob in subdirectory",
			files: map[string]string{
				"src/main.go":  "content",
				"src/utils.go": "content",
				"test/main.go": "content",
				"README.md":    "content",
			},
			opts: Options{
				Pattern: "*.go",
				Path:    "src",
			},
			wantMatches: []string{"src/main.go", "src/utils.go"},
		},
		{
			name: "double star glob",
			files: map[string]string{
				"main.go":          "content",
				"src/main.go":      "content",
				"src/pkg/main.go":  "content",
				"src/pkg/sub/a.go": "content",
			},
			opts: Options{
				Pattern: "**/*.go",
			},
			wantMatches: []string{"main.go", "src/main.go", "src/pkg/main.go", "src/pkg/sub/a.go"},
		},
		{
			name: "non-existent path should error",
			files: map[string]string{
				"main.go": "content",
			},
			opts: Options{
				Pattern: "*.go",
				Path:    "nonexistent",
			},
			wantErr:        true,
			wantErrContain: "cannot access",
		},
		{
			name: "file path instead of directory should error",
			files: map[string]string{
				"main.go": "content",
			},
			opts: Options{
				Pattern: "*.go",
				Path:    "main.go",
			},
			wantErr:        true,
			wantErrContain: "is not a directory",
		},
		{
			name: "skip skipDirs",
			files: map[string]string{
				"main.go":                   "content",
				"node_modules/pkg/index.js": "content",
				".git/config":               "content",
				"__pycache__/module.pyc":    "content",
			},
			opts: Options{
				Pattern: "*",
			},
			wantMatches: []string{"main.go"},
		},
		{
			name: "glob with question mark",
			files: map[string]string{
				"a1.go": "content",
				"a2.go": "content",
				"ab.go": "content",
			},
			opts: Options{
				Pattern: "a?.go",
			},
			wantMatches: []string{"a1.go", "a2.go", "ab.go"},
		},
		{
			name: "glob with character class",
			files: map[string]string{
				"a.go": "content",
				"b.go": "content",
				"c.go": "content",
				"d.go": "content",
			},
			opts: Options{
				Pattern: "[ab].go",
			},
			wantMatches: []string{"a.go", "b.go"},
		},
		{
			name: "glob with negated character class",
			files: map[string]string{
				"a.go": "content",
				"b.go": "content",
				"c.go": "content",
			},
			opts: Options{
				Pattern: "[^ab].go",
			},
			wantMatches: []string{"c.go"},
		},
		{
			name: "double star at end",
			files: map[string]string{
				"src/a.go":     "content",
				"src/b.go":     "content",
				"src/sub/c.go": "content",
			},
			opts: Options{
				Pattern: "src/**",
			},
			wantMatches: []string{"src/a.go", "src/b.go", "src/sub/c.go"},
		},
		{
			name: "double star in middle",
			files: map[string]string{
				"a/b/c/d.go": "content",
				"a/x/y/d.go": "content",
				"a/d.go":     "content",
			},
			opts: Options{
				Pattern: "a/**/d.go",
			},
			wantMatches: []string{"a/b/c/d.go", "a/d.go", "a/x/y/d.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			globber := NewGlobberWithFS(fsys)
			result, err := globber.Glob(context.Background(), tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantMatches, result.Matches)
			assert.Equal(t, tt.wantTruncated, result.Truncated)
		})
	}
}

func TestGlobber_Glob_ContextCancellation(t *testing.T) {
	files := map[string]string{
		"a.go": "content",
		"b.go": "content",
	}
	fsys := NewSliceFS(files)
	globber := NewGlobberWithFS(fsys)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := globber.Glob(ctx, Options{Pattern: "*.go"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGlobber_Glob_Truncation(t *testing.T) {
	files := make(map[string]string)
	for i := 0; i < MaxResults+10; i++ {
		files[fmt.Sprintf("file%d.go", i)] = "content"
	}
	fsys := NewSliceFS(files)
	globber := NewGlobberWithFS(fsys)

	result, err := globber.Glob(context.Background(), Options{Pattern: "*.go"})
	require.NoError(t, err)
	assert.True(t, result.Truncated)
	assert.Len(t, result.Matches, MaxResults)
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "test.go",
			path:    "test.go",
			want:    true,
		},
		{
			name:    "simple star",
			pattern: "*.go",
			path:    "main.go",
			want:    true,
		},
		{
			name:    "star no match",
			pattern: "*.go",
			path:    "main.js",
			want:    false,
		},
		{
			name:    "question mark match",
			pattern: "test?.go",
			path:    "test1.go",
			want:    true,
		},
		{
			name:    "question mark no match",
			pattern: "test?.go",
			path:    "test12.go",
			want:    false,
		},
		{
			name:    "character class match",
			pattern: "[abc].go",
			path:    "a.go",
			want:    true,
		},
		{
			name:    "character class no match",
			pattern: "[abc].go",
			path:    "d.go",
			want:    false,
		},
		{
			name:    "negated character class",
			pattern: "[^abc].go",
			path:    "d.go",
			want:    true,
		},
		{
			name:    "negated character class no match",
			pattern: "[^abc].go",
			path:    "a.go",
			want:    false,
		},
		{
			name:    "double star prefix",
			pattern: "**/*.go",
			path:    "a/b/c/test.go",
			want:    true,
		},
		{
			name:    "double star prefix root file",
			pattern: "**/*.go",
			path:    "test.go",
			want:    true,
		},
		{
			name:    "double star only",
			pattern: "**",
			path:    "a/b/c/test.go",
			want:    true,
		},
		{
			name:    "double star middle",
			pattern: "src/**/*.go",
			path:    "src/a/b/c/test.go",
			want:    true,
		},
		{
			name:    "double star suffix",
			pattern: "src/**",
			path:    "src/a/b/c/test.go",
			want:    true,
		},
		{
			name:    "path with segments",
			pattern: "src/*.go",
			path:    "src/main.go",
			want:    true,
		},
		{
			name:    "path with segments no match",
			pattern: "src/*.go",
			path:    "lib/main.go",
			want:    false,
		},
		{
			name:    "multiple segments",
			pattern: "a/b/*.go",
			path:    "a/b/test.go",
			want:    true,
		},
		{
			name:    "multiple segments no match",
			pattern: "a/b/*.go",
			path:    "a/c/test.go",
			want:    false,
		},
		{
			name:    "empty pattern empty path",
			pattern: "",
			path:    "",
			want:    true,
		},
		{
			name:    "empty pattern non-empty path",
			pattern: "",
			path:    "test.go",
			want:    false,
		},
		{
			name:    "non-empty pattern empty path",
			pattern: "*.go",
			path:    "",
			want:    false,
		},
		{
			name:    "backslash converted to slash",
			pattern: "src/*.go",
			path:    "src\\main.go",
			want:    true,
		},
		{
			name:    "range in character class",
			pattern: "[a-z].go",
			path:    "m.go",
			want:    true,
		},
		{
			name:    "range in character class no match",
			pattern: "[a-z].go",
			path:    "1.go",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.pattern, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilepathRel(t *testing.T) {
	tests := []struct {
		base   string
		target string
		want   string
	}{
		{
			base:   ".",
			target: "test.go",
			want:   "test.go",
		},
		{
			base:   "src",
			target: "src/main.go",
			want:   "main.go",
		},
		{
			base:   "a/b",
			target: "a/b/c/d.go",
			want:   "c/d.go",
		},
		{
			base:   "src",
			target: "other/main.go",
			want:   "other/main.go",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.base, tt.target), func(t *testing.T) {
			got, _ := filepathRel(tt.base, tt.target)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToSlash(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "a\\b\\c",
			want:  "a/b/c",
		},
		{
			input: "a/b/c",
			want:  "a/b/c",
		},
		{
			input: "",
			want:  "",
		},
		{
			input: "\\",
			want:  "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSlash(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitFirst(t *testing.T) {
	tests := []struct {
		input    string
		sep      byte
		wantSeg  string
		wantRest string
	}{
		{
			input:    "a/b/c",
			sep:      '/',
			wantSeg:  "a",
			wantRest: "b/c",
		},
		{
			input:    "a",
			sep:      '/',
			wantSeg:  "a",
			wantRest: "",
		},
		{
			input:    "",
			sep:      '/',
			wantSeg:  "",
			wantRest: "",
		},
		{
			input:    "a/b",
			sep:      '/',
			wantSeg:  "a",
			wantRest: "b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			seg, rest := splitFirst(tt.input, tt.sep)
			assert.Equal(t, tt.wantSeg, seg)
			assert.Equal(t, tt.wantRest, rest)
		})
	}
}

func TestSliceFS(t *testing.T) {
	t.Run("open file", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"test.go": "content",
		})

		f, err := fsys.Open("test.go")
		require.NoError(t, err)
		defer f.Close()

		info, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, "test.go", info.Name())
		assert.Equal(t, int64(7), info.Size())
		assert.False(t, info.IsDir())
	})

	t.Run("open directory", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"dir/file.go": "content",
		})

		f, err := fsys.Open("dir")
		require.NoError(t, err)
		defer f.Close()

		info, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, "dir", info.Name())
		assert.True(t, info.IsDir())
	})

	t.Run("open root", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"file.go": "content",
		})

		f, err := fsys.Open(".")
		require.NoError(t, err)
		defer f.Close()

		info, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, ".", info.Name())
		assert.True(t, info.IsDir())
	})

	t.Run("open non-existent file", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{})

		_, err := fsys.Open("nonexistent.go")
		require.Error(t, err)
	})

	t.Run("stat file", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"test.go": "content",
		})

		info, err := fsys.Stat("test.go")
		require.NoError(t, err)
		assert.Equal(t, "test.go", info.Name())
		assert.Equal(t, int64(7), info.Size())
	})

	t.Run("stat directory", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"dir/file.go": "content",
		})

		info, err := fsys.Stat("dir")
		require.NoError(t, err)
		assert.Equal(t, "dir", info.Name())
		assert.True(t, info.IsDir())
	})

	t.Run("stat non-existent", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{})

		_, err := fsys.Stat("nonexistent")
		require.Error(t, err)
	})

	t.Run("read directory", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"dir/file1.go": "content1",
			"dir/file2.go": "content2",
			"dir/sub/file": "content3",
		})

		entries, err := fsys.ReadDir("dir")
		require.NoError(t, err)

		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		assert.ElementsMatch(t, []string{"file1.go", "file2.go", "sub"}, names)
	})

	t.Run("read root directory", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"file1.go":    "content1",
			"dir/file.go": "content2",
		})

		entries, err := fsys.ReadDir(".")
		require.NoError(t, err)

		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		assert.ElementsMatch(t, []string{"file1.go", "dir"}, names)
	})

	t.Run("read non-existent directory", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{})

		_, err := fsys.ReadDir("nonexistent")
		require.Error(t, err)
	})

	t.Run("read file as directory should error", func(t *testing.T) {
		fsys := NewSliceFS(map[string]string{
			"file.go": "content",
		})

		_, err := fsys.ReadDir("file.go")
		require.Error(t, err)
	})
}

func TestNewGlobber(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\n"), 0644)
	require.NoError(t, err)

	globber := NewGlobber(tmpDir)
	require.NotNil(t, globber)

	result, err := globber.Glob(context.Background(), Options{
		Pattern: "*.go",
	})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 1)
	assert.Contains(t, result.Matches[0], "test.go")
}

func TestNewGlobberWithFS(t *testing.T) {
	fsys := NewSliceFS(map[string]string{
		"main.go": "content",
	})

	globber := NewGlobberWithFS(fsys)
	require.NotNil(t, globber)

	result, err := globber.Glob(context.Background(), Options{
		Pattern: "*.go",
	})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 1)
	assert.Equal(t, "main.go", result.Matches[0])
}

func TestSkipDirs(t *testing.T) {
	assert.True(t, SkipDirs[".git"])
	assert.True(t, SkipDirs["node_modules"])
	assert.True(t, SkipDirs[".hg"])
	assert.True(t, SkipDirs[".svn"])
	assert.True(t, SkipDirs["__pycache__"])
	assert.True(t, SkipDirs[".DS_Store"])
	assert.False(t, SkipDirs["src"])
	assert.False(t, SkipDirs[""])
}

func TestGlobber_DeepNestedDirectories(t *testing.T) {
	files := map[string]string{
		"a/b/c/d/e/file.go": "content",
		"a/b/c/file.go":     "content",
		"a/file.go":         "content",
	}
	fsys := NewSliceFS(files)
	globber := NewGlobberWithFS(fsys)

	result, err := globber.Glob(context.Background(), Options{
		Pattern: "**/*.go",
	})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 3)
}

func TestGlobber_AllSkipDirs(t *testing.T) {
	files := map[string]string{
		"main.go":                   "content",
		".git/config":               "content",
		".hg/store/data":            "content",
		".svn/entries":              "content",
		"__pycache__/module.pyc":    "content",
		"node_modules/pkg/index.js": "content",
	}
	fsys := NewSliceFS(files)
	globber := NewGlobberWithFS(fsys)

	result, err := globber.Glob(context.Background(), Options{
		Pattern: "*",
	})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 1)
	assert.Equal(t, "main.go", result.Matches[0])
}

func TestGlobber_EmptyFileSystem(t *testing.T) {
	fsys := NewSliceFS(map[string]string{})
	globber := NewGlobberWithFS(fsys)

	result, err := globber.Glob(context.Background(), Options{
		Pattern: "*.go",
	})
	require.NoError(t, err)
	assert.Nil(t, result.Matches)
	assert.False(t, result.Truncated)
}

func TestGlobber_WithOSFS(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "utils.go"), []byte("package main\n"), 0644)
	require.NoError(t, err)

	fsys := os.DirFS(tmpDir)
	globber := NewGlobberWithFS(fsys)

	result, err := globber.Glob(context.Background(), Options{
		Pattern: "*.go",
	})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 2)
}

func TestGlobber_ContextTimeout(t *testing.T) {
	files := make(map[string]string)
	for i := 0; i < 100; i++ {
		files[fmt.Sprintf("file%d.go", i)] = "content"
	}
	fsys := NewSliceFS(files)
	globber := NewGlobberWithFS(fsys)

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	result, err := globber.Glob(ctx, Options{Pattern: "*.go"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGlobber_SpecialPatterns(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		pattern     string
		wantCount   int
		wantMatches []string
	}{
		{
			name: "pattern with multiple stars",
			files: map[string]string{
				"test.go":      "content",
				"test_test.go": "content",
				"main.go":      "content",
			},
			pattern:   "*test*.go",
			wantCount: 2,
		},
		{
			name: "pattern matching files in multiple directories",
			files: map[string]string{
				"src/main.go":  "content",
				"lib/main.go":  "content",
				"test/main.go": "content",
			},
			pattern:   "**/main.go",
			wantCount: 3,
		},
		{
			name: "pattern with character range",
			files: map[string]string{
				"a.go": "content",
				"b.go": "content",
				"c.go": "content",
				"d.go": "content",
				"e.go": "content",
			},
			pattern:   "[a-c].go",
			wantCount: 3,
		},
		{
			name: "complex pattern",
			files: map[string]string{
				"src/pkg/v1/main.go": "content",
				"src/pkg/v2/main.go": "content",
				"src/pkg/v3/main.go": "content",
				"lib/pkg/v1/main.go": "content",
			},
			pattern:   "src/**/main.go",
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			globber := NewGlobberWithFS(fsys)

			result, err := globber.Glob(context.Background(), Options{
				Pattern: tt.pattern,
			})
			require.NoError(t, err)
			assert.Len(t, result.Matches, tt.wantCount)

			if tt.wantMatches != nil {
				assert.ElementsMatch(t, tt.wantMatches, result.Matches)
			}
		})
	}
}
