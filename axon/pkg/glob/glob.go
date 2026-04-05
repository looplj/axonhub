package glob

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxResults = 200

var SkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".hg":          true,
	".svn":         true,
	"__pycache__":  true,
	".DS_Store":    true,
}

type Options struct {
	Pattern string
	Path    string
}

type Result struct {
	Matches   []string
	Truncated bool
}

type Globber struct {
	fsys fs.FS
}

func NewGlobber(workspace string) *Globber {
	return &Globber{fsys: os.DirFS(workspace)}
}

func NewGlobberWithFS(fsys fs.FS) *Globber {
	return &Globber{fsys: fsys}
}

func (g *Globber) Glob(ctx context.Context, opts Options) (*Result, error) {
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	searchPath := "."
	if opts.Path != "" {
		searchPath = opts.Path
	}

	info, err := fs.Stat(g.fsys, searchPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access %q: %w", searchPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", searchPath)
	}

	type fileEntry struct {
		path    string
		modTime int64
	}

	var matches []fileEntry

	err = fs.WalkDir(g.fsys, searchPath, func(path string, d fs.DirEntry, err error) error {
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

		relPath, _ := filepathRel(searchPath, path)
		if Match(opts.Pattern, relPath) {
			info, infoErr := d.Info()
			var modTime int64
			if infoErr == nil {
				modTime = info.ModTime().UnixNano()
			}
			matches = append(matches, fileEntry{path: path, modTime: modTime})
		}

		return nil
	})

	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return nil, err
	}

	if len(matches) == 0 {
		return &Result{Matches: nil, Truncated: false}, nil
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime > matches[j].modTime
	})

	truncated := len(matches) > MaxResults
	if truncated {
		matches = matches[:MaxResults]
	}

	var paths []string
	for _, m := range matches {
		paths = append(paths, m.path)
	}

	return &Result{Matches: paths, Truncated: truncated}, nil
}

func filepathRel(base, target string) (string, error) {
	if base == "." {
		return target, nil
	}
	if strings.HasPrefix(target, base+"/") {
		return target[len(base)+1:], nil
	}
	return target, nil
}

func Match(pattern, path string) bool {
	pattern = toSlash(pattern)
	path = toSlash(path)

	return doMatch(pattern, path)
}

func toSlash(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}

func doMatch(pattern, path string) bool {
	for {
		if pattern == "" {
			return path == ""
		}

		if strings.HasPrefix(pattern, "**/") {
			rest := pattern[3:]
			if doMatch(rest, path) {
				return true
			}
			for i := 0; i < len(path); i++ {
				if path[i] == '/' {
					if doMatch(rest, path[i+1:]) {
						return true
					}
				}
			}
			return false
		}

		if pattern == "**" {
			return true
		}

		patSeg, patRest := splitFirst(pattern, '/')
		pathSeg, pathRest := splitFirst(path, '/')

		if patSeg == "**" {
			if doMatch(pattern, pathRest) {
				return true
			}
			if pathRest != "" || pathSeg != "" {
				for i := 0; i <= len(path); i++ {
					if i == len(path) || path[i] == '/' {
						sub := ""
						if i < len(path) {
							sub = path[i+1:]
						}
						if doMatch(patRest, sub) {
							return true
						}
					}
				}
			}
			return false
		}

		matched, _ := filepath.Match(patSeg, pathSeg)
		if !matched {
			return false
		}

		if patRest == "" && pathRest == "" {
			return true
		}
		if patRest == "" || pathRest == "" {
			return false
		}

		pattern = patRest
		path = pathRest
	}
}

func splitFirst(s string, sep byte) (string, string) {
	i := strings.IndexByte(s, sep)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+1:]
}
