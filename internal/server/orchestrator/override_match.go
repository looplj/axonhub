package orchestrator

import (
	"fmt"

	"github.com/gobwas/glob"

	"github.com/looplj/axonhub/internal/pkg/xmap"
)

type compiledGlob struct {
	pattern *glob.Pattern
	err     error
}

var globPatternCache = xmap.New[string, compiledGlob]()

// globMatch matches value against a glob pattern in which only "/" separates
// segments: "*" and "?" never cross a slash, while "**/" spans zero or more
// whole segments.
func globMatch(pattern, value string) (bool, error) {
	cached, ok := globPatternCache.Load(pattern)
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
		globPatternCache.Store(pattern, cached)
	}

	if cached.err != nil {
		return false, cached.err
	}

	return cached.pattern.Match(value), nil
}
