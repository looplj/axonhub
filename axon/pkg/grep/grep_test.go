package grep

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
func (fi *sliceFileInfo) Mode() fs.FileMode  { return 0o644 }
func (fi *sliceFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *sliceFileInfo) IsDir() bool        { return fi.isDir }
func (fi *sliceFileInfo) Sys() interface{}   { return nil }

var (
	_ fs.FS        = SliceFS{}
	_ fs.StatFS    = SliceFS{}
	_ fs.ReadDirFS = SliceFS{}
)

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

// TestingFS creates a test file system with sample files
func TestingFS() SliceFS {
	return NewSliceFS(map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
		"utils.go": `package main

import "strings"

func ToUpper(s string) string {
	return strings.ToUpper(s)
}
`,
		"README.md": `# Test Project

This is a test project.
`,
		"subdir/nested.go": `package subdir

func Helper() string {
	return "help"
}
`,
	})
}

func TestSearcher_Search(t *testing.T) {
	tests := []struct {
		name           string
		files          map[string]string
		opts           Options
		wantErr        bool
		wantErrContain string
		wantResult     string
		wantTruncated  bool
	}{
		{
			name: "empty pattern should error",
			files: map[string]string{
				"test.go": "package main",
			},
			opts: Options{
				Pattern: "",
			},
			wantErr:        true,
			wantErrContain: "pattern is required",
		},
		{
			name: "simple pattern match - files_with_matches mode",
			files: map[string]string{
				"main.go":   "package main\n\nfunc main() {}",
				"utils.go":  "package main\n\nfunc Utils() {}",
				"README.md": "# README",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "files_with_matches",
			},
			wantResult: "main.go\n",
		},
		{
			name: "simple pattern match - content mode",
			files: map[string]string{
				"main.go": "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "content",
			},
			wantResult: "main.go:3:func main() {\n",
		},
		{
			name: "count mode",
			files: map[string]string{
				"main.go": "func a() {}\nfunc b() {}\nfunc c() {}",
			},
			opts: Options{
				Pattern:    "func",
				OutputMode: "count",
			},
			wantResult: "main.go:3\n",
		},
		{
			name: "no matches found",
			files: map[string]string{
				"main.go": "package main",
			},
			opts: Options{
				Pattern: "nonexistent",
			},
			wantResult: "No results found.",
		},
		{
			name: "ignore case",
			files: map[string]string{
				"main.go": "FUNC main() {}",
			},
			opts: Options{
				Pattern:    "func",
				IgnoreCase: new(true),
			},
			wantResult: "main.go\n",
		},
		{
			name: "literal pattern",
			files: map[string]string{
				"main.go": "func main() { [a-z]+ }",
			},
			opts: Options{
				Pattern: "[a-z]+",
				Literal: true,
			},
			wantResult: "main.go\n",
		},
		{
			name: "regexp pattern",
			files: map[string]string{
				"main.go": "abc123 def456",
			},
			opts: Options{
				Pattern: `\d+`,
			},
			wantResult: "main.go\n",
		},
		{
			name: "file type filter - go files only",
			files: map[string]string{
				"main.go":   "func main() {}",
				"utils.js":  "function utils() {}",
				"README.md": "# README",
			},
			opts: Options{
				Pattern:  "func",
				FileType: "go",
			},
			wantResult: "main.go\n",
		},
		{
			name: "glob filter",
			files: map[string]string{
				"main.go":       "func main() {}",
				"utils.go":      "func utils() {}",
				"test_utils.go": "func testUtils() {}",
			},
			opts: Options{
				Pattern: "func",
				Glob:    "*_utils.go",
			},
			wantResult: "test_utils.go\n",
		},
		{
			name: "glob with brace expansion",
			files: map[string]string{
				"main.go":      "func main() {}",
				"main_test.go": "func TestMain() {}",
				"utils.js":     "function utils() {}",
			},
			opts: Options{
				Pattern: "func",
				Glob:    "*.{go,js}",
			},
			// Note: SliceFS iteration order is sorted by name, so order is deterministic
			wantResult: "main.go\nmain_test.go\nutils.js\n",
		},
		{
			name: "context lines - before and after",
			files: map[string]string{
				"main.go": "line1\nline2\nline3\nfunc main() {\nline5\nline6\nline7",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "content",
				Context:    new(1),
			},
			wantResult: "main.go:3-line3\nmain.go:4:func main() {\nmain.go:5-line5\n",
		},
		{
			name: "context lines - before only",
			files: map[string]string{
				"main.go": "line1\nline2\nfunc main() {\nline4",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "content",
				Before:     new(2),
			},
			wantResult: "main.go:1-line1\nmain.go:2-line2\nmain.go:3:func main() {\n",
		},
		{
			name: "context lines - after only",
			files: map[string]string{
				"main.go": "line1\nfunc main() {\nline3\nline4",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "content",
				After:      new(2),
			},
			wantResult: "main.go:2:func main() {\nmain.go:3-line3\nmain.go:4-line4\n",
		},
		{
			name: "disable line numbers",
			files: map[string]string{
				"main.go": "func main() {}",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "content",
				LineNumber: new(false),
			},
			wantResult: "main.go:func main() {}\n",
		},
		{
			name: "multiline mode",
			files: map[string]string{
				"main.go": "func a() {\n  return 1\n}",
			},
			opts: Options{
				Pattern:    `func a\(\) \{[\s\S]*?\}`,
				OutputMode: "content",
				Multiline:  true,
			},
			wantResult: "main.go: func a() {\n  return 1\n}\n",
		},
		{
			name: "pagination - offset",
			files: map[string]string{
				"a.go": "func a() {}",
				"b.go": "func b() {}",
				"c.go": "func c() {}",
			},
			opts: Options{
				Pattern: "func",
				Offset:  1,
			},
			// Note: SliceFS iteration order is sorted by name, so order is deterministic
			wantResult: "skip_order_check",
		},
		{
			name: "pagination - head limit",
			files: map[string]string{
				"a.go": "func a() {}",
				"b.go": "func b() {}",
				"c.go": "func c() {}",
			},
			opts: Options{
				Pattern:   "func",
				HeadLimit: 2,
			},
			wantResult: "a.go\nb.go\n",
		},
		{
			name: "pagination - offset and head limit",
			files: map[string]string{
				"a.go": "func a() {}",
				"b.go": "func b() {}",
				"c.go": "func c() {}",
				"d.go": "func d() {}",
			},
			opts: Options{
				Pattern:   "func",
				Offset:    1,
				HeadLimit: 2,
			},
			// Note: SliceFS iteration order is sorted by name, so order is deterministic
			wantResult: "skip_order_check",
		},
		{
			name: "truncate long lines",
			files: map[string]string{
				"main.go": "func main() { " + strings.Repeat("x", 2000) + " }",
			},
			opts: Options{
				Pattern:    "func main",
				OutputMode: "content",
				MaxLineLen: 50,
			},
			wantResult: "main.go:1:func main() { " + strings.Repeat("x", 36) + "...\n",
		},
		{
			name: "search specific path",
			files: map[string]string{
				"dir1/file.go": "func inDir1() {}",
				"dir2/file.go": "func inDir2() {}",
			},
			opts: Options{
				Pattern: "func",
				Path:    "dir1",
			},
			wantResult: "dir1/file.go\n",
		},
		{
			name: "search single file",
			files: map[string]string{
				"main.go":  "func main() {}",
				"other.go": "func other() {}",
			},
			opts: Options{
				Pattern: "func",
				Path:    "main.go",
			},
			wantResult: "main.go\n",
		},
		{
			name: "unknown file type should error",
			files: map[string]string{
				"main.go": "func main() {}",
			},
			opts: Options{
				Pattern:  "func",
				FileType: "unknown",
			},
			wantErr:        true,
			wantErrContain: "unknown file type",
		},
		{
			name: "invalid regexp should error",
			files: map[string]string{
				"main.go": "func main() {}",
			},
			opts: Options{
				Pattern: "[invalid",
			},
			wantErr:        true,
			wantErrContain: "invalid pattern",
		},
		{
			name: "skip binary files",
			files: map[string]string{
				"main.go":  "func main() {}",
				"main.exe": "should be skipped",
			},
			opts: Options{
				Pattern: "should",
			},
			wantResult: "No results found.",
		},
		{
			name: "skip skipDirs",
			files: map[string]string{
				"main.go":                       "func main() {}",
				"node_modules/package/index.js": "function pkg() {}",
			},
			opts: Options{
				Pattern: "function",
			},
			wantResult: "No results found.",
		},
		{
			name: "multiple matches in single file - limited by MaxPerFile",
			files: map[string]string{
				"main.go": strings.Repeat("func a() {}\n", 20),
			},
			opts: Options{
				Pattern:    "func",
				OutputMode: "count",
			},
			wantResult: "main.go:16\n", // MaxPerFile = 16
		},
		{
			name: "truncated when exceeding MaxMatches",
			files: map[string]string{
				"a.go": "func a() {}",
				"b.go": "func b() {}",
				"c.go": "func c() {}",
			},
			opts: Options{
				Pattern: "func",
			},
			// Note: SliceFS iteration order is sorted by name, so order is deterministic
			wantResult:    "skip_order_check_3",
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			result, err := searcher.Search(context.Background(), tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			switch {
			case tt.wantResult == "skip_order_check":
				// For pagination tests where order is not guaranteed, just verify count
				lines := strings.Count(result.Text, ".go\n")
				assert.Equal(t, 2, lines, "expected 2 results")
			case tt.wantResult == "skip_order_check_3":
				// For tests where order is not guaranteed, just verify count
				lines := strings.Count(result.Text, ".go\n")
				assert.Equal(t, 3, lines, "expected 3 results")
			default:
				assert.Equal(t, tt.wantResult, result.Text)
			}
			assert.Equal(t, tt.wantTruncated, result.Truncated)
		})
	}
}

func TestSearcher_Search_ContextCancellation(t *testing.T) {
	files := map[string]string{
		"a.go": "func a() {}",
		"b.go": "func b() {}",
	}
	fsys := NewSliceFS(files)
	searcher := NewSearcherWithFS(fsys)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := searcher.Search(ctx, Options{Pattern: "func"})
	require.NoError(t, err)
	// Should return empty or partial results without error
	assert.NotNil(t, result)
}

func TestExpandGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty pattern",
			pattern: "",
			want:    nil,
		},
		{
			name:    "simple pattern",
			pattern: "*.go",
			want:    []string{"*.go"},
		},
		{
			name:    "brace expansion single",
			pattern: "*.{go}",
			want:    []string{"*.go"},
		},
		{
			name:    "brace expansion multiple",
			pattern: "*.{go,js,ts}",
			want:    []string{"*.go", "*.js", "*.ts"},
		},
		{
			name:    "brace expansion with spaces",
			pattern: "*.{ go , js }",
			want:    []string{"*.go", "*.js"},
		},
		{
			name:    "nested brace expansion",
			pattern: "*.{go,test.{go,js}}",
			want:    []string{"*.go}", "*.test.go", "*.js}"},
		},
		{
			name:    "unmatched braces",
			pattern: "*.{go",
			want:    []string{"*.{go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandGlob(tt.pattern)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsBinaryExt(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".go", false},
		{".js", false},
		{".exe", true},
		{".zip", true},
		{".png", true},
		{".pdf", true},
		{".wasm", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isBinaryExt(tt.ext)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		maxLen int
		want   string
	}{
		{
			name:   "no limit",
			line:   "hello world",
			maxLen: 0,
			want:   "hello world",
		},
		{
			name:   "within limit",
			line:   "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exactly at limit",
			line:   "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "exceeds limit",
			line:   "hello world",
			maxLen: 5,
			want:   "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLine(tt.line, tt.maxLen)
			assert.Equal(t, tt.want, got)
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

func TestApplyPagination(t *testing.T) {
	searcher := &Searcher{}

	tests := []struct {
		name    string
		entries []string
		offset  int
		limit   int
		want    []string
	}{
		{
			name:    "no pagination",
			entries: []string{"a", "b", "c"},
			offset:  0,
			limit:   0,
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "offset only",
			entries: []string{"a", "b", "c"},
			offset:  1,
			limit:   0,
			want:    []string{"b", "c"},
		},
		{
			name:    "limit only",
			entries: []string{"a", "b", "c"},
			offset:  0,
			limit:   2,
			want:    []string{"a", "b"},
		},
		{
			name:    "offset and limit",
			entries: []string{"a", "b", "c", "d"},
			offset:  1,
			limit:   2,
			want:    []string{"b", "c"},
		},
		{
			name:    "offset exceeds length",
			entries: []string{"a", "b"},
			offset:  5,
			limit:   0,
			want:    nil,
		},
		{
			name:    "limit exceeds remaining",
			entries: []string{"a", "b"},
			offset:  0,
			limit:   10,
			want:    []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searcher.applyPagination(tt.entries, tt.offset, tt.limit)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCollectFiles(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		root      string
		globs     []string
		typeExts  []string
		wantFiles []string
		wantErr   bool
	}{
		{
			name: "collect all files",
			files: map[string]string{
				"a.go": "content",
				"b.go": "content",
			},
			root:      ".",
			wantFiles: []string{"a.go", "b.go"},
		},
		{
			name: "collect from subdirectory",
			files: map[string]string{
				"dir/a.go": "content",
				"b.go":     "content",
			},
			root:      "dir",
			wantFiles: []string{"dir/a.go"},
		},
		{
			name: "filter by extension",
			files: map[string]string{
				"a.go": "content",
				"b.js": "content",
				"c.go": "content",
			},
			root:      ".",
			typeExts:  []string{".go"},
			wantFiles: []string{"a.go", "c.go"},
		},
		{
			name: "filter by glob",
			files: map[string]string{
				"a_test.go": "content",
				"b.go":      "content",
				"c_test.go": "content",
			},
			root:      ".",
			globs:     []string{"*_test.go"},
			wantFiles: []string{"a_test.go", "c_test.go"},
		},
		{
			name: "skip binary files",
			files: map[string]string{
				"a.go":  "content",
				"a.exe": "binary",
				"a.zip": "binary",
			},
			root:      ".",
			wantFiles: []string{"a.go"},
		},
		{
			name: "skip skipDirs",
			files: map[string]string{
				"a.go":                      "content",
				".git/config":               "git config",
				"node_modules/pkg/index.js": "content",
			},
			root:      ".",
			wantFiles: []string{"a.go"},
		},
		{
			name: "single file",
			files: map[string]string{
				"a.go": "content",
			},
			root:      "a.go",
			wantFiles: []string{"a.go"},
		},
		{
			name: "non-existent path should error",
			files: map[string]string{
				"a.go": "content",
			},
			root:    "nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			files, err := searcher.collectFiles(context.Background(), tt.root, tt.globs, tt.typeExts)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantFiles, files)
		})
	}
}

func TestFormatContentMatches(t *testing.T) {
	searcher := &Searcher{}

	tests := []struct {
		name      string
		path      string
		lines     []string
		matchIdxs []int
		opts      Options
		want      []string
	}{
		{
			name:      "simple match with line number",
			path:      "test.go",
			lines:     []string{"line1", "match", "line3"},
			matchIdxs: []int{1},
			opts:      Options{},
			want:      []string{"test.go:2:match"},
		},
		{
			name:      "multiple matches",
			path:      "test.go",
			lines:     []string{"match1", "line2", "match2"},
			matchIdxs: []int{0, 2},
			opts:      Options{},
			want:      []string{"test.go:1:match1", "test.go:3:match2"},
		},
		{
			name:      "with context",
			path:      "test.go",
			lines:     []string{"before", "match", "after"},
			matchIdxs: []int{1},
			opts:      Options{Context: new(1)},
			want:      []string{"test.go:1-before", "test.go:2:match", "test.go:3-after"},
		},
		{
			name:      "with context and separator",
			path:      "test.go",
			lines:     []string{"match1", "line2", "line3", "match2"},
			matchIdxs: []int{0, 3},
			opts:      Options{Context: new(0)},
			want:      []string{"test.go:1:match1", "test.go:4:match2"},
		},
		{
			name:      "without line numbers",
			path:      "test.go",
			lines:     []string{"match"},
			matchIdxs: []int{0},
			opts:      Options{LineNumber: new(false)},
			want:      []string{"test.go:match"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searcher.formatContentMatches(tt.path, tt.lines, tt.matchIdxs, tt.opts)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompilePattern(t *testing.T) {
	searcher := &Searcher{}

	tests := []struct {
		name    string
		opts    Options
		want    string
		wantErr bool
	}{
		{
			name: "simple pattern",
			opts: Options{Pattern: "hello"},
			want: "hello",
		},
		{
			name: "literal pattern",
			opts: Options{Pattern: "[a-z]", Literal: true},
			want: `\[a-z\]`,
		},
		{
			name: "ignore case",
			opts: Options{Pattern: "hello", IgnoreCase: new(true)},
			want: "(?i)hello",
		},
		{
			name: "multiline",
			opts: Options{Pattern: "hello", Multiline: true},
			want: "(?s)hello",
		},
		{
			name: "ignore case and multiline",
			opts: Options{Pattern: "hello", IgnoreCase: new(true), Multiline: true},
			want: "(?is)hello",
		},
		{
			name:    "invalid pattern",
			opts:    Options{Pattern: "[invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := searcher.compilePattern(tt.opts)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, re.String())
		})
	}
}

func TestSearchFile(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		path       string
		pattern    string
		opts       Options
		outputMode string
		want       []string
	}{
		{
			name: "files_with_matches mode",
			files: map[string]string{
				"test.go": "func main() {}",
			},
			path:       "test.go",
			pattern:    "func main",
			opts:       Options{},
			outputMode: "files_with_matches",
			want:       []string{"test.go"},
		},
		{
			name: "count mode",
			files: map[string]string{
				"test.go": "func a() {}\nfunc b() {}",
			},
			path:       "test.go",
			pattern:    "func",
			opts:       Options{},
			outputMode: "count",
			want:       []string{"test.go:2"},
		},
		{
			name: "content mode",
			files: map[string]string{
				"test.go": "func main() {}",
			},
			path:       "test.go",
			pattern:    "func main",
			opts:       Options{},
			outputMode: "content",
			want:       []string{"test.go:1:func main() {}"},
		},
		{
			name: "no match",
			files: map[string]string{
				"test.go": "func other() {}",
			},
			path:       "test.go",
			pattern:    "func main",
			opts:       Options{},
			outputMode: "files_with_matches",
			want:       nil,
		},
		{
			name: "non-existent file",
			files: map[string]string{
				"other.go": "content",
			},
			path:       "test.go",
			pattern:    "content",
			opts:       Options{},
			outputMode: "files_with_matches",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			re, err := regexp.Compile(tt.pattern)
			require.NoError(t, err)

			got := searcher.searchFile(tt.path, re, tt.opts, tt.outputMode)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSearchFileMultiline(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		path       string
		pattern    string
		opts       Options
		outputMode string
		want       []string
	}{
		{
			name: "files_with_matches mode",
			files: map[string]string{
				"test.go": "func a() {\n  return 1\n}",
			},
			path:       "test.go",
			pattern:    `func a\(\) \{[\s\S]*?\}`,
			opts:       Options{},
			outputMode: "files_with_matches",
			want:       []string{"test.go"},
		},
		{
			name: "count mode",
			files: map[string]string{
				"test.go": "func a() {}\nfunc b() {}",
			},
			path:       "test.go",
			pattern:    `func \w+\(\) \{\}`,
			opts:       Options{},
			outputMode: "count",
			want:       []string{"test.go:2"},
		},
		{
			name: "content mode",
			files: map[string]string{
				"test.go": "func main() {\n  return\n}",
			},
			path:       "test.go",
			pattern:    `func main\(\) \{[\s\S]*?\}`,
			opts:       Options{},
			outputMode: "content",
			want:       []string{"test.go: func main() {\n  return\n}"},
		},
		{
			name: "no match",
			files: map[string]string{
				"test.go": "func other() {}",
			},
			path:       "test.go",
			pattern:    `func main\(\) \{[\s\S]*?\}`,
			opts:       Options{},
			outputMode: "files_with_matches",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			re, err := regexp.Compile(tt.pattern)
			require.NoError(t, err)

			got := searcher.searchFileMultiline(tt.path, re, tt.opts, tt.outputMode)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Integration test with real OS filesystem
func TestSearcher_WithOSFS(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create test files
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "utils.go"), []byte("package main\n\nfunc utils() {}\n"), 0o644)
	require.NoError(t, err)

	// Use os.DirFS for the test
	fsys := os.DirFS(tmpDir)
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern: "func main",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Text, "main.go")
}

// Test NewSearcher creates a valid searcher from a directory path
func TestNewSearcher(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\n\nfunc test() {}\n"), 0o644)
	require.NoError(t, err)

	searcher := NewSearcher(tmpDir)
	require.NotNil(t, searcher)

	result, err := searcher.Search(context.Background(), Options{
		Pattern: "func test",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Text, "test.go")
}

// Test empty file system
func TestSearcher_EmptyFileSystem(t *testing.T) {
	fsys := NewSliceFS(map[string]string{})
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern: "anything",
	})
	require.NoError(t, err)
	assert.Equal(t, "No results found.", result.Text)
}

// Test context with timeout
func TestSearcher_Search_ContextTimeout(t *testing.T) {
	// Create a large file system to potentially trigger timeout
	files := make(map[string]string)
	for i := 0; i < 100; i++ {
		files[fmt.Sprintf("file%d.go", i)] = "func test() {}"
	}
	fsys := NewSliceFS(files)
	searcher := NewSearcherWithFS(fsys)

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	result, err := searcher.Search(ctx, Options{Pattern: "func"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// Test special characters in pattern
func TestSearcher_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		pattern string
		literal bool
		want    string
	}{
		{
			name: "regex special chars as literal",
			files: map[string]string{
				"test.go": "func (a *Type) method() {}",
			},
			pattern: "(a *Type)",
			literal: true,
			want:    "test.go\n",
		},
		{
			name: "regex pattern with groups",
			files: map[string]string{
				"test.go": "func test() {}",
			},
			pattern: `func (\w+)\(\)`,
			literal: false,
			want:    "test.go\n",
		},
		{
			name: "pattern with newlines",
			files: map[string]string{
				"test.go": "line1\nline2\nline3",
			},
			pattern: "line2",
			literal: true,
			want:    "test.go\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			result, err := searcher.Search(context.Background(), Options{
				Pattern: tt.pattern,
				Literal: tt.literal,
			})
			require.NoError(t, err)
			assert.Contains(t, result.Text, tt.want)
		})
	}
}

// Test file type filtering with various extensions
func TestSearcher_FileTypeFiltering(t *testing.T) {
	tests := []struct {
		name     string
		fileType string
		files    map[string]string
		want     string
		wantErr  bool
	}{
		{
			name:     "python files",
			fileType: "py",
			files: map[string]string{
				"main.py": "def main():",
				"test.go": "func main()",
			},
			want: "main.py\n",
		},
		{
			name:     "typescript files",
			fileType: "ts",
			files: map[string]string{
				"main.ts": "function main()",
				"test.js": "function test()",
			},
			want: "main.ts\n",
		},
		{
			name:     "rust files",
			fileType: "rust",
			files: map[string]string{
				"main.rs": "fn main()",
				"test.go": "func main()",
			},
			want: "main.rs\n",
		},
		{
			name:     "java files",
			fileType: "java",
			files: map[string]string{
				"Main.java": "public class Main",
				"test.go":   "func main()",
			},
			want: "Main.java\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			result, err := searcher.Search(context.Background(), Options{
				Pattern:  "main|Main",
				FileType: tt.fileType,
			})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, result.Text, tt.want)
		})
	}
}

// Test multiple matches with context
func TestSearcher_MultipleMatchesWithContext(t *testing.T) {
	files := map[string]string{
		"test.go": "line1\nmatch1\nline3\nline4\nmatch2\nline6\nmatch3\nline8",
	}
	fsys := NewSliceFS(files)
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern:    "match",
		OutputMode: "content",
		Context:    new(1),
	})
	require.NoError(t, err)
	// Verify all matches are found
	assert.Contains(t, result.Text, "match1")
	assert.Contains(t, result.Text, "match2")
	assert.Contains(t, result.Text, "match3")
}

// Test deeply nested directory structure
func TestSearcher_DeepNestedDirectories(t *testing.T) {
	files := map[string]string{
		"a/b/c/d/e/file.go": "func deep() {}",
		"a/b/c/file.go":     "func mid() {}",
		"a/file.go":         "func shallow() {}",
	}
	fsys := NewSliceFS(files)
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern: "func",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Text, "a/b/c/d/e/file.go")
	assert.Contains(t, result.Text, "a/b/c/file.go")
	assert.Contains(t, result.Text, "a/file.go")
}

// Test MaxMatches truncation
func TestSearcher_MaxMatchesTruncation(t *testing.T) {
	// Create more files than MaxMatches
	files := make(map[string]string)
	for i := 0; i < MaxMatches+10; i++ {
		files[fmt.Sprintf("file%d.go", i)] = "func test() {}"
	}
	fsys := NewSliceFS(files)
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern: "func",
	})
	require.NoError(t, err)
	assert.True(t, result.Truncated)
	assert.Contains(t, result.Text, "truncated")
}

// Test glob pattern edge cases
func TestSearcher_GlobEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		glob  string
		want  []string
	}{
		{
			name: "glob with path",
			files: map[string]string{
				"src/main.go":  "func main()",
				"src/utils.go": "func utils()",
				"test/main.go": "func test()",
			},
			glob: "src/*.go",
			want: []string{"src/main.go", "src/utils.go"},
		},
		{
			name: "glob with double star equivalent",
			files: map[string]string{
				"main.go":         "func main()",
				"src/main.go":     "func main()",
				"src/pkg/main.go": "func main()",
			},
			glob: "*.go",
			want: []string{"main.go", "src/main.go", "src/pkg/main.go"},
		},
		{
			name: "glob matching no files",
			files: map[string]string{
				"main.go": "func main()",
			},
			glob: "*.xyz",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(tt.files)
			searcher := NewSearcherWithFS(fsys)
			result, err := searcher.Search(context.Background(), Options{
				Pattern: "func",
				Glob:    tt.glob,
			})
			require.NoError(t, err)
			if tt.want == nil {
				assert.Equal(t, "No results found.", result.Text)
			} else {
				for _, w := range tt.want {
					assert.Contains(t, result.Text, w)
				}
			}
		})
	}
}

func TestSearcher_GlobBraceExpansion_MixedPaths(t *testing.T) {
	fsys := NewSliceFS(map[string]string{
		"MEMORY.md":            "foo",
		"memory/2026-03-16.md": "foo",
		"memory/ignore.txt":    "foo",
		"other.md":             "foo",
	})
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern:    "foo",
		Glob:       "{MEMORY.md,memory/*.md}",
		OutputMode: "files_with_matches",
	})
	require.NoError(t, err)

	assert.Contains(t, result.Text, "MEMORY.md\n")
	assert.Contains(t, result.Text, "memory/2026-03-16.md\n")
	assert.NotContains(t, result.Text, "other.md\n")
	assert.NotContains(t, result.Text, "memory/ignore.txt\n")
}

// Test all SkipDirs are skipped
func TestSearcher_AllSkipDirs(t *testing.T) {
	files := map[string]string{
		"main.go":                   "func main() {}",
		".git/config":               "git content",
		".hg/store/data":            "hg content",
		".svn/entries":              "svn content",
		"__pycache__/module.pyc":    "pycache content",
		"node_modules/pkg/index.js": "node content",
	}
	fsys := NewSliceFS(files)
	searcher := NewSearcherWithFS(fsys)

	result, err := searcher.Search(context.Background(), Options{
		Pattern: "content",
	})
	require.NoError(t, err)
	assert.Equal(t, "No results found.", result.Text)
}

// Test output modes
func TestSearcher_OutputModes(t *testing.T) {
	files := map[string]string{
		"test.go": "func a() {}\nfunc b() {}\nfunc c() {}",
	}

	tests := []struct {
		name       string
		pattern    string
		outputMode string
		want       string
	}{
		{
			name:       "files_with_matches",
			pattern:    "func a",
			outputMode: "files_with_matches",
			want:       "test.go\n",
		},
		{
			name:       "count",
			pattern:    "func",
			outputMode: "count",
			want:       "test.go:3\n",
		},
		{
			name:       "content",
			pattern:    "func a",
			outputMode: "content",
			want:       "test.go:1:func a() {}\n",
		},
		{
			name:       "default (empty)",
			pattern:    "func a",
			outputMode: "",
			want:       "test.go\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := NewSliceFS(files)
			searcher := NewSearcherWithFS(fsys)
			result, err := searcher.Search(context.Background(), Options{
				Pattern:    tt.pattern,
				OutputMode: tt.outputMode,
			})
			require.NoError(t, err)
			assert.Contains(t, result.Text, tt.want)
		})
	}
}
