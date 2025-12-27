package xregexp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchString(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		str      string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "gpt-4",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "exact no match",
			pattern:  "gpt-4",
			str:      "gpt-3.5",
			expected: false,
		},
		{
			name:     "wildcard match",
			pattern:  "gpt-.*",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "wildcard match multiple",
			pattern:  "gpt-.*",
			str:      "gpt-3.5-turbo",
			expected: true,
		},
		{
			name:     "wildcard no match",
			pattern:  "gpt-.*",
			str:      "claude-3",
			expected: false,
		},
		{
			name:     "question mark zero or one",
			pattern:  "gpt-?",
			str:      "gpt",
			expected: true,
		},
		{
			name:     "question mark with char",
			pattern:  "gpt-?",
			str:      "gpt-",
			expected: true,
		},
		{
			name:     "plus one or more",
			pattern:  "gpt-4+",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "plus multiple",
			pattern:  "gpt-4+",
			str:      "gpt-444",
			expected: true,
		},
		{
			name:     "character class",
			pattern:  "gpt-[34]",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "character class no match",
			pattern:  "gpt-[34]",
			str:      "gpt-5",
			expected: false,
		},
		{
			name:     "alternation",
			pattern:  "gpt-(4|3.5)",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "alternation second option",
			pattern:  "gpt-(4|3.5)",
			str:      "gpt-3.5",
			expected: true,
		},
		{
			name:     "complex pattern",
			pattern:  "gpt-[34](\\.[0-9]+)?(-turbo)?",
			str:      "gpt-4-turbo",
			expected: true,
		},
		{
			name:     "complex pattern with version",
			pattern:  "gpt-[34](\\.[0-9]+)?(-turbo)?",
			str:      "gpt-3.5-turbo",
			expected: true,
		},
		{
			name:     "empty pattern matches empty string",
			pattern:  "",
			str:      "",
			expected: true,
		},
		{
			name:     "empty pattern no match non-empty",
			pattern:  "",
			str:      "gpt-4",
			expected: false,
		},
		{
			name:     "invalid regex returns false",
			pattern:  "gpt-[",
			str:      "gpt-[",
			expected: false,
		},
		{
			name:     "invalid regex returns false for any string",
			pattern:  "gpt-[",
			str:      "gpt-4",
			expected: false,
		},
		{
			name:     "dot matches any char",
			pattern:  "gpt.4",
			str:      "gpt-4",
			expected: true,
		},
		{
			name:     "escaped dot literal",
			pattern:  "gpt\\.4",
			str:      "gpt.4",
			expected: true,
		},
		{
			name:     "escaped dot no match",
			pattern:  "gpt\\.4",
			str:      "gpt-4",
			expected: false,
		},
		{
			name:     "caret and dollar anchors implicit",
			pattern:  "gpt-4",
			str:      "prefix-gpt-4-suffix",
			expected: false,
		},
		{
			name:     "partial match with wildcard",
			pattern:  ".*gpt-4.*",
			str:      "prefix-gpt-4-suffix",
			expected: true,
		},
		{
			name:     "partial match with model modifer no match",
			pattern:  "(?i)^(?=.*gpt-5)(?!.*mini)(?!.*nano).*$",
			str:      "gpt-5.1-codex-mini",
			expected: false,
		},
		{
			name:     "partial match with model modifer match",
			pattern:  "(?i)^(?=.*gpt-5)(?!.*mini)(?!.*nano).*$",
			str:      "gpt-5.1-codex",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchString(tt.pattern, tt.str)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		pattern  string
		expected []string
	}{
		{
			name:     "exact match single",
			items:    []string{"gpt-4", "gpt-3.5", "claude-3"},
			pattern:  "gpt-4",
			expected: []string{"gpt-4"},
		},
		{
			name:     "wildcard match multiple",
			items:    []string{"gpt-4", "gpt-3.5-turbo", "claude-3"},
			pattern:  "gpt-.*",
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:     "no matches",
			items:    []string{"gpt-4", "gpt-3.5", "claude-3"},
			pattern:  "gemini-.*",
			expected: []string{},
		},
		{
			name:     "empty pattern",
			items:    []string{"gpt-4", "gpt-3.5"},
			pattern:  "",
			expected: []string{},
		},
		{
			name:     "empty items",
			items:    []string{},
			pattern:  "gpt-.*",
			expected: []string{},
		},
		{
			name:     "character class",
			items:    []string{"gpt-3", "gpt-4", "gpt-5"},
			pattern:  "gpt-[34]",
			expected: []string{"gpt-3", "gpt-4"},
		},
		{
			name:     "complex pattern",
			items:    []string{"gpt-4", "gpt-4-turbo", "gpt-3.5", "gpt-3.5-turbo", "claude-3"},
			pattern:  "gpt-[34](\\.[0-9]+)?(-turbo)?",
			expected: []string{"gpt-4", "gpt-4-turbo", "gpt-3.5", "gpt-3.5-turbo"},
		},
		{
			name:     "invalid regex returns empty",
			items:    []string{"gpt-4", "gpt-["},
			pattern:  "gpt-[",
			expected: []string{},
		},
		{
			name:     "alternation",
			items:    []string{"gpt-4", "gpt-3.5", "claude-3", "claude-2"},
			pattern:  "(gpt-4|claude-3)",
			expected: []string{"gpt-4", "claude-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filter(tt.items, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsRegexChars(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "no regex chars",
			pattern:  "gpt-4",
			expected: false,
		},
		{
			name:     "asterisk",
			pattern:  "gpt-*",
			expected: true,
		},
		{
			name:     "question mark",
			pattern:  "gpt-?",
			expected: true,
		},
		{
			name:     "plus",
			pattern:  "gpt-+",
			expected: true,
		},
		{
			name:     "brackets",
			pattern:  "gpt-[4]",
			expected: true,
		},
		{
			name:     "braces",
			pattern:  "gpt-{1,2}",
			expected: true,
		},
		{
			name:     "parentheses",
			pattern:  "gpt-(4)",
			expected: true,
		},
		{
			name:     "caret",
			pattern:  "^gpt",
			expected: true,
		},
		{
			name:     "dollar",
			pattern:  "gpt$",
			expected: true,
		},
		{
			name:     "dot",
			pattern:  "gpt.4",
			expected: true,
		},
		{
			name:     "pipe",
			pattern:  "gpt|claude",
			expected: true,
		},
		{
			name:     "backslash",
			pattern:  "gpt\\-4",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsRegexChars(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatternCacheTypes(t *testing.T) {
	exactPattern := "gpt-4"
	cached := getOrCreatePattern(exactPattern)
	require.NotNil(t, cached)
	assert.True(t, cached.exactMatch)
	assert.Nil(t, cached.regex)
	assert.False(t, cached.compileErr)

	regexPattern := "gpt-.*"
	cached = getOrCreatePattern(regexPattern)
	require.NotNil(t, cached)
	assert.False(t, cached.exactMatch)
	assert.NotNil(t, cached.regex)
	assert.False(t, cached.compileErr)

	invalidPattern := "gpt-["
	cached = getOrCreatePattern(invalidPattern)
	require.NotNil(t, cached)
	assert.False(t, cached.exactMatch)
	assert.Nil(t, cached.regex)
	assert.True(t, cached.compileErr)
}

func TestEnsureAnchored(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "no anchors",
			pattern:  "gpt-.*",
			expected: "^gpt-.*$",
		},
		{
			name:     "start anchor only",
			pattern:  "^gpt-.*",
			expected: "^gpt-.*$",
		},
		{
			name:     "end anchor only",
			pattern:  "gpt-.*$",
			expected: "^gpt-.*$",
		},
		{
			name:     "both anchors",
			pattern:  "^gpt-.*$",
			expected: "^gpt-.*$",
		},
		{
			name:     "case insensitive with start anchor",
			pattern:  "(?i)^gpt-.*",
			expected: "(?i)^gpt-.*$",
		},
		{
			name:     "case insensitive with both anchors",
			pattern:  "(?i)^gpt-.*$",
			expected: "(?i)^gpt-.*$",
		},
		{
			name:     "complex pattern with anchors",
			pattern:  "(?i)^(?=.*gpt-5)(?!.*mini)(?!.*nano).*$",
			expected: "(?i)^(?=.*gpt-5)(?!.*mini)(?!.*nano).*$",
		},
		{
			name:     "multiline modifier with start anchor",
			pattern:  "(?m)^gpt-.*",
			expected: "(?m)^gpt-.*$",
		},
		{
			name:     "singleline modifier with start anchor",
			pattern:  "(?s)^gpt-.*",
			expected: "(?s)^gpt-.*$",
		},
		{
			name:     "multiple modifiers with start anchor",
			pattern:  "(?is)^gpt-.*",
			expected: "(?is)^gpt-.*$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureAnchored(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkMatchStringExact(b *testing.B) {
	pattern := "gpt-4-turbo"
	str := "gpt-4-turbo"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		MatchString(pattern, str)
	}
}

func BenchmarkMatchStringRegex(b *testing.B) {
	pattern := "gpt-[34](\\.[0-9]+)?(-turbo)?"
	str := "gpt-4-turbo"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		MatchString(pattern, str)
	}
}

func BenchmarkFilterByPattern(b *testing.B) {
	items := []string{
		"gpt-4", "gpt-4-turbo", "gpt-3.5", "gpt-3.5-turbo",
		"claude-3", "claude-2", "gemini-pro", "gemini-ultra",
	}
	pattern := "gpt-.*"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Filter(items, pattern)
	}
}
