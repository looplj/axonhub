package grep

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samber/lo"
)

const (
	MaxMatches = 32
	MaxPerFile = 16
	MaxLineLen = 1024
)

var FileTypeExts = map[string][]string{
	"go":      {".go"},
	"js":      {".js", ".mjs", ".cjs"},
	"ts":      {".ts", ".tsx", ".mts", ".cts"},
	"py":      {".py", ".pyi"},
	"rust":    {".rs"},
	"java":    {".java"},
	"c":       {".c", ".h"},
	"cpp":     {".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".h"},
	"css":     {".css"},
	"html":    {".html", ".htm"},
	"json":    {".json"},
	"yaml":    {".yml", ".yaml"},
	"xml":     {".xml"},
	"md":      {".md", ".markdown"},
	"sh":      {".sh", ".bash"},
	"sql":     {".sql"},
	"ruby":    {".rb"},
	"php":     {".php"},
	"swift":   {".swift"},
	"kotlin":  {".kt", ".kts"},
	"scala":   {".scala"},
	"lua":     {".lua"},
	"r":       {".r", ".R"},
	"dart":    {".dart"},
	"vue":     {".vue"},
	"svelte":  {".svelte"},
	"toml":    {".toml"},
	"proto":   {".proto"},
	"graphql": {".graphql", ".gql"},
}

var SkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".hg":          true,
	".svn":         true,
	"__pycache__":  true,
	".DS_Store":    true,
}

type Options struct {
	Pattern    string
	Path       string
	PathPrefix string
	Glob       string
	OutputMode string
	Before     *int
	After      *int
	Context    *int
	LineNumber *bool
	IgnoreCase *bool
	FileType   string
	HeadLimit  int
	Offset     int
	Multiline  bool
	Literal    bool
	MaxLineLen int
}

type Result struct {
	Text      string
	Truncated bool
}

type Searcher struct {
	fsys fs.FS
}

func NewSearcher(workspace string) *Searcher {
	return &Searcher{fsys: os.DirFS(workspace)}
}

func NewSearcherWithFS(fsys fs.FS) *Searcher {
	return &Searcher{fsys: fsys}
}

func (s *Searcher) Search(ctx context.Context, opts Options) (*Result, error) {
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	searchPath := "."
	if opts.Path != "" {
		searchPath = opts.Path
	}

	re, err := s.compilePattern(opts)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	outputMode := opts.OutputMode
	if outputMode == "" {
		outputMode = "files_with_matches"
	}

	globs, err := expandGlob(opts.Glob)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	var typeExts []string
	if opts.FileType != "" {
		exts, ok := FileTypeExts[strings.ToLower(opts.FileType)]
		if !ok {
			return nil, fmt.Errorf("unknown file type %q", opts.FileType)
		}
		typeExts = exts
	}

	files, err := s.collectFiles(ctx, searchPath, globs, typeExts)
	if err != nil {
		return nil, err
	}

	var results []string
	totalEntries := 0

	for _, file := range files {
		if ctx.Err() != nil {
			break
		}
		if totalEntries >= MaxMatches {
			break
		}

		entries := s.searchFile(file, re, opts, outputMode)
		if len(entries) == 0 {
			continue
		}

		remaining := MaxMatches - totalEntries
		if len(entries) > remaining {
			entries = entries[:remaining]
		}
		results = append(results, entries...)
		totalEntries += len(entries)
	}

	if len(results) == 0 {
		return &Result{Text: "No results found."}, nil
	}

	output := s.applyPagination(results, opts.Offset, opts.HeadLimit)
	if len(output) == 0 {
		return &Result{Text: "No results found."}, nil
	}

	truncated := totalEntries >= MaxMatches
	text := strings.Join(output, "\n") + "\n"
	if truncated && opts.HeadLimit == 0 {
		text += fmt.Sprintf("... (truncated, showing first %d results)\n", MaxMatches)
	}

	return &Result{Text: text, Truncated: truncated}, nil
}

func (s *Searcher) compilePattern(opts Options) (*regexp.Regexp, error) {
	pattern := opts.Pattern
	if opts.Literal {
		pattern = regexp.QuoteMeta(pattern)
	}

	flags := ""
	if lo.FromPtrOr(opts.IgnoreCase, false) {
		flags += "i"
	}
	if opts.Multiline {
		flags += "s"
	}
	if flags != "" {
		pattern = "(?" + flags + ")" + pattern
	}

	return regexp.Compile(pattern)
}

func (s *Searcher) searchFile(path string, re *regexp.Regexp, opts Options, outputMode string) []string {
	if opts.Multiline {
		return s.searchFileMultiline(path, re, opts, outputMode)
	}

	f, err := s.fsys.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil
	}

	var matchIdxs []int
	for i, line := range lines {
		if re.MatchString(line) {
			matchIdxs = append(matchIdxs, i)
			if len(matchIdxs) >= MaxPerFile {
				break
			}
		}
	}

	if len(matchIdxs) == 0 {
		return nil
	}

	switch outputMode {
	case "files_with_matches":
		return []string{displayPath(path, opts.PathPrefix)}
	case "count":
		return []string{fmt.Sprintf("%s:%d", displayPath(path, opts.PathPrefix), len(matchIdxs))}
	default:
		return s.formatContentMatches(path, lines, matchIdxs, opts)
	}
}

func (s *Searcher) searchFileMultiline(path string, re *regexp.Regexp, opts Options, outputMode string) []string {
	f, err := s.fsys.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	content := string(data)

	matches := re.FindAllStringIndex(content, MaxPerFile)
	if len(matches) == 0 {
		return nil
	}

	switch outputMode {
	case "files_with_matches":
		return []string{displayPath(path, opts.PathPrefix)}
	case "count":
		return []string{fmt.Sprintf("%s:%d", displayPath(path, opts.PathPrefix), len(matches))}
	default:
		maxLineLen := opts.MaxLineLen
		if maxLineLen == 0 {
			maxLineLen = MaxLineLen
		}
		var results []string
		for _, loc := range matches {
			snippet := content[loc[0]:loc[1]]
			line := truncateLine(snippet, maxLineLen)
			results = append(results, fmt.Sprintf("%s: %s", displayPath(path, opts.PathPrefix), line))
		}
		return results
	}
}

func (s *Searcher) formatContentMatches(path string, lines []string, matchIdxs []int, opts Options) []string {
	path = displayPath(path, opts.PathPrefix)
	showLineNum := lo.FromPtrOr(opts.LineNumber, true)

	beforeN, afterN := 0, 0
	if opts.Context != nil {
		beforeN = *opts.Context
		afterN = *opts.Context
	}
	if opts.Before != nil {
		beforeN = *opts.Before
	}
	if opts.After != nil {
		afterN = *opts.After
	}

	hasContext := beforeN > 0 || afterN > 0

	maxLineLen := opts.MaxLineLen
	if maxLineLen == 0 {
		maxLineLen = MaxLineLen
	}

	type lineEntry struct {
		idx     int
		isMatch bool
	}
	seen := make(map[int]bool)
	var entries []lineEntry

	for _, mi := range matchIdxs {
		start := max(mi-beforeN, 0)
		end := mi + afterN
		if end >= len(lines) {
			end = len(lines) - 1
		}
		for i := start; i <= end; i++ {
			if !seen[i] {
				seen[i] = true
				entries = append(entries, lineEntry{idx: i, isMatch: i == mi})
			}
		}
	}

	var results []string
	prevIdx := -2
	for _, e := range entries {
		if hasContext && prevIdx >= 0 && e.idx > prevIdx+1 {
			results = append(results, "--")
		}
		prevIdx = e.idx

		line := truncateLine(lines[e.idx], maxLineLen)
		if showLineNum {
			sep := ":"
			if hasContext && !e.isMatch {
				sep = "-"
			}
			results = append(results, fmt.Sprintf("%s%s%d%s%s", path, ":", e.idx+1, sep, line))
		} else {
			results = append(results, fmt.Sprintf("%s:%s", path, line))
		}
	}

	return results
}

func (s *Searcher) collectFiles(ctx context.Context, root string, globs []string, typeExts []string) ([]string, error) {
	info, err := fs.Stat(s.fsys, root)
	if err != nil {
		return nil, fmt.Errorf("cannot access %q: %w", root, err)
	}

	if !info.IsDir() {
		return []string{root}, nil
	}

	var files []string
	err = fs.WalkDir(s.fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if d.IsDir() {
			if SkipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		if isBinaryExt(filepath.Ext(path)) {
			return nil
		}

		if len(typeExts) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if !lo.Contains(typeExts, ext) {
				return nil
			}
		}

		if len(globs) > 0 {
			name := d.Name()
			relPath, _ := filepath.Rel(root, path)
			if relPath == "." {
				relPath = name
			}
			matched := false
			for _, g := range globs {
				if ok, _ := filepath.Match(g, name); ok {
					matched = true
					break
				}
				if ok, _ := filepath.Match(g, relPath); ok {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return nil, err
	}

	return files, nil
}

func (s *Searcher) applyPagination(entries []string, offset, limit int) []string {
	if offset > 0 {
		if offset >= len(entries) {
			return nil
		}
		entries = entries[offset:]
	}
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	return entries
}

func truncateLine(s string, maxLen int) string {
	if maxLen == 0 {
		return s
	}
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func displayPath(path, prefix string) string {
	if prefix == "" {
		return path
	}

	return filepath.Clean(filepath.Join(prefix, path))
}

func expandGlob(pattern string) ([]string, error) {
	if pattern == "" {
		return nil, nil
	}

	braceStart := strings.IndexByte(pattern, '{')
	if braceStart < 0 {
		return []string{pattern}, nil
	}

	braceEnd := strings.IndexByte(pattern[braceStart:], '}')
	if braceEnd < 0 {
		return []string{pattern}, nil
	}
	braceEnd += braceStart

	prefix := pattern[:braceStart]
	suffix := pattern[braceEnd+1:]
	alts := strings.Split(pattern[braceStart+1:braceEnd], ",")

	var results []string
	for _, alt := range alts {
		expanded, err := expandGlob(prefix + strings.TrimSpace(alt) + suffix)
		if err != nil {
			return nil, err
		}
		results = append(results, expanded...)
	}
	return results, nil
}

func isBinaryExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".obj", ".o", ".a",
		".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar",
		".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico", ".webp", ".svg",
		".mp3", ".mp4", ".avi", ".mov", ".wav", ".flac",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
		".pyc", ".class", ".jar", ".war",
		".db", ".sqlite", ".sqlite3",
		".wasm":
		return true
	}
	return false
}
