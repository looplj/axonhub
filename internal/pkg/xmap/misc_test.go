package xmap

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestGetStringPtr(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected *string
	}{
		{
			name:     "nil map",
			m:        nil,
			key:      "key",
			expected: nil,
		},
		{
			name:     "key not found",
			m:        map[string]any{"other": lo.ToPtr("value")},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with *string value",
			m:        map[string]any{"key": lo.ToPtr("value")},
			key:      "key",
			expected: lo.ToPtr("value"),
		},
		{
			name:     "key found with wrong type",
			m:        map[string]any{"key": "not a pointer"},
			key:      "key",
			expected: lo.ToPtr("not a pointer"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringPtr(tt.m, tt.key)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetInt64Ptr(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected *int64
	}{
		{
			name:     "nil map",
			m:        nil,
			key:      "key",
			expected: nil,
		},
		{
			name:     "key not found",
			m:        map[string]any{"other": lo.ToPtr(int64(42))},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with *int64 value",
			m:        map[string]any{"key": lo.ToPtr(int64(42))},
			key:      "key",
			expected: lo.ToPtr(int64(42)),
		},
		{
			name:     "key found with wrong type",
			m:        map[string]any{"key": int64(42)},
			key:      "key",
			expected: lo.ToPtr(int64(42)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInt64Ptr(tt.m, tt.key)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBoolPtr(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected *bool
	}{
		{
			name:     "nil map",
			m:        nil,
			key:      "key",
			expected: nil,
		},
		{
			name:     "key not found",
			m:        map[string]any{"other": lo.ToPtr(true)},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with *bool true",
			m:        map[string]any{"key": lo.ToPtr(true)},
			key:      "key",
			expected: lo.ToPtr(true),
		},
		{
			name:     "key found with *bool false",
			m:        map[string]any{"key": lo.ToPtr(false)},
			key:      "key",
			expected: lo.ToPtr(false),
		},
		{
			name:     "key found with wrong type",
			m:        map[string]any{"key": true},
			key:      "key",
			expected: lo.ToPtr(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBoolPtr(tt.m, tt.key)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected []string
	}{
		{
			name:     "nil map",
			m:        nil,
			key:      "key",
			expected: nil,
		},
		{
			name:     "key not found",
			m:        map[string]any{"other": []string{"a", "b"}},
			key:      "key",
			expected: nil,
		},
		{
			name:     "key found with []string value",
			m:        map[string]any{"key": []string{"a", "b", "c"}},
			key:      "key",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "key found with empty slice",
			m:        map[string]any{"key": []string{}},
			key:      "key",
			expected: []string{},
		},
		{
			name:     "key found with wrong type",
			m:        map[string]any{"key": []int{1, 2, 3}},
			key:      "key",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringSlice(tt.m, tt.key)
			require.Equal(t, tt.expected, result)
		})
	}
}
