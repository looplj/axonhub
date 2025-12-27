package xregexp

import (
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/looplj/axonhub/internal/pkg/xmap"
)

type patternCache struct {
	regex      *regexp2.Regexp
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

	match, _ := cached.regex.MatchString(str)

	return match
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
			if match, _ := cached.regex.MatchString(item); match {
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

	compiled, err := regexp2.Compile(ensureAnchored(pattern), regexp2.None)
	if err != nil {
		cached.compileErr = true
	} else {
		cached.regex = compiled
	}

	globalCache.Store(pattern, cached)

	return cached
}

func ensureAnchored(pattern string) string {
	// Check if pattern starts with ^ (accounting for inline modifiers)
	hasStartAnchor := false
	if strings.HasPrefix(pattern, "^") {
		hasStartAnchor = true
	} else if strings.HasPrefix(pattern, "(?i)^") {
		hasStartAnchor = true
	} else if strings.HasPrefix(pattern, "(?m)^") {
		hasStartAnchor = true
	} else if strings.HasPrefix(pattern, "(?s)^") {
		hasStartAnchor = true
	} else if strings.HasPrefix(pattern, "(?is)^") || strings.HasPrefix(pattern, "(?si)^") {
		hasStartAnchor = true
	} else if strings.HasPrefix(pattern, "(?im)^") || strings.HasPrefix(pattern, "(?mi)^") {
		hasStartAnchor = true
	} else if strings.HasPrefix(pattern, "(?ism)^") || strings.HasPrefix(pattern, "(?sim)^") || strings.HasPrefix(pattern, "(?mis)^") || strings.HasPrefix(pattern, "(?msi)^") || strings.HasPrefix(pattern, "(?smi)^") || strings.HasPrefix(pattern, "(?ims)^") {
		hasStartAnchor = true
	}

	// Check if pattern ends with $ (accounting for inline modifiers)
	hasEndAnchor := strings.HasSuffix(pattern, "$")

	if !hasStartAnchor {
		pattern = "^" + pattern
	}

	if !hasEndAnchor {
		pattern = pattern + "$"
	}

	return pattern
}

func containsRegexChars(pattern string) bool {
	return strings.ContainsAny(pattern, "*?+[]{}()^$.|\\")
}
