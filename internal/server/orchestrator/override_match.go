package orchestrator

import (
	"fmt"

	"github.com/gobwas/glob"

	lru "github.com/hashicorp/golang-lru/v2"
)

// globPatternCacheSize bounds the compiled pattern cache so patterns derived
// from request values cannot grow memory without limit.
const globPatternCacheSize = 256

// compiledGlob caches the outcome of compiling a glob pattern, including the
// error, so an invalid pattern is not recompiled on every request.
type compiledGlob struct {
	pattern *glob.Pattern
	err     error
}

var globPatternCache, _ = lru.New[string, compiledGlob](globPatternCacheSize)

// globMatch matches value against a glob pattern in which only "/" separates
// segments: "*" and "?" never cross a slash, while "**/" spans zero or more
// whole segments.
func globMatch(pattern, value string) (bool, error) {
	cached, ok := globPatternCache.Get(pattern)
	if !ok {
		compiler := glob.Compiler{
			Separators: []rune{'/'},
			Globstar:   true,
		}

		compiled, err := compiler.Compile(pattern)
		if err != nil {
			err = fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}

		cached = compiledGlob{pattern: compiled, err: err}
		globPatternCache.Add(pattern, cached)
	}

	if cached.err != nil {
		return false, cached.err
	}

	return cached.pattern.Match(value), nil
}
