package xregexp

import (
	"regexp"
	"strings"

	"github.com/looplj/axonhub/internal/pkg/xmap"
)

type patternCache struct {
	regex      *regexp.Regexp
	exactMatch bool
	compileErr bool
}

var globalCache = xmap.New[string, *patternCache]()

func MatchString(pattern string, str string) bool {
	cached := getOrCreatePattern(pattern)

	if cached.compileErr {
		return false
	}

	if cached.exactMatch {
		return pattern == str
	}

	return cached.regex.MatchString(str)
}

func Filter(items []string, pattern string) []string {
	if pattern == "" {
		return []string{}
	}

	cached := getOrCreatePattern(pattern)

	if cached.compileErr {
		return []string{}
	}

	matched := make([]string, 0)

	if cached.exactMatch {
		for _, item := range items {
			if pattern == item {
				matched = append(matched, item)
			}
		}
	} else {
		for _, item := range items {
			if cached.regex.MatchString(item) {
				matched = append(matched, item)
			}
		}
	}

	return matched
}

func getOrCreatePattern(pattern string) *patternCache {
	if cached, ok := globalCache.Load(pattern); ok {
		return cached
	}

	cached := &patternCache{}

	if !containsRegexChars(pattern) {
		cached.exactMatch = true
		globalCache.Store(pattern, cached)

		return cached
	}

	compiled, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		cached.compileErr = true
	} else {
		cached.regex = compiled
	}

	globalCache.Store(pattern, cached)

	return cached
}

func containsRegexChars(pattern string) bool {
	return strings.ContainsAny(pattern, "*?+[]{}()^$.|\\")
}
