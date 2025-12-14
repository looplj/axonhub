package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestMergeOverrideHeaders(t *testing.T) {
	tests := []struct {
		name     string
		existing []objects.HeaderEntry
		template []objects.HeaderEntry
		expected []objects.HeaderEntry
	}{
		{
			name:     "empty existing and template",
			existing: []objects.HeaderEntry{},
			template: []objects.HeaderEntry{},
			expected: []objects.HeaderEntry{},
		},
		{
			name: "add new header",
			existing: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token1"},
			},
			template: []objects.HeaderEntry{
				{Key: "X-API-Key", Value: "key123"},
			},
			expected: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token1"},
				{Key: "X-API-Key", Value: "key123"},
			},
		},
		{
			name: "override existing header case-insensitive",
			existing: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token1"},
				{Key: "Content-Type", Value: "application/json"},
			},
			template: []objects.HeaderEntry{
				{Key: "authorization", Value: "Bearer token2"},
			},
			expected: []objects.HeaderEntry{
				{Key: "authorization", Value: "Bearer token2"},
				{Key: "Content-Type", Value: "application/json"},
			},
		},
		{
			name: "clear header with directive",
			existing: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token1"},
				{Key: "X-API-Key", Value: "key123"},
			},
			template: []objects.HeaderEntry{
				{Key: "Authorization", Value: clearHeaderDirective},
			},
			expected: []objects.HeaderEntry{
				{Key: "X-API-Key", Value: "key123"},
			},
		},
		{
			name: "clear non-existent header has no effect",
			existing: []objects.HeaderEntry{
				{Key: "X-API-Key", Value: "key123"},
			},
			template: []objects.HeaderEntry{
				{Key: "Authorization", Value: clearHeaderDirective},
			},
			expected: []objects.HeaderEntry{
				{Key: "X-API-Key", Value: "key123"},
			},
		},
		{
			name: "complex merge with add, override, and clear",
			existing: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token1"},
				{Key: "X-API-Key", Value: "key123"},
				{Key: "Content-Type", Value: "application/json"},
			},
			template: []objects.HeaderEntry{
				{Key: "Authorization", Value: clearHeaderDirective},
				{Key: "X-API-Key", Value: "newkey456"},
				{Key: "X-Custom-Header", Value: "custom"},
			},
			expected: []objects.HeaderEntry{
				{Key: "X-API-Key", Value: "newkey456"},
				{Key: "Content-Type", Value: "application/json"},
				{Key: "X-Custom-Header", Value: "custom"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeOverrideHeaders(tt.existing, tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeOverrideParameters(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		template    string
		expected    string
		expectError bool
	}{
		{
			name:     "empty existing and template",
			existing: "{}",
			template: "{}",
			expected: "{}",
		},
		{
			name:     "empty strings treated as empty objects",
			existing: "",
			template: "",
			expected: "{}",
		},
		{
			name:     "add new field",
			existing: `{"temperature": 0.7}`,
			template: `{"max_tokens": 1000}`,
			expected: `{"max_tokens":1000,"temperature":0.7}`,
		},
		{
			name:     "override existing field",
			existing: `{"temperature": 0.7, "max_tokens": 500}`,
			template: `{"temperature": 0.9}`,
			expected: `{"max_tokens":500,"temperature":0.9}`,
		},
		{
			name:     "deep merge nested objects",
			existing: `{"model_config": {"temperature": 0.7, "top_p": 0.9}}`,
			template: `{"model_config": {"temperature": 0.8}}`,
			expected: `{"model_config":{"temperature":0.8,"top_p":0.9}}`,
		},
		{
			name:     "template overwrites array",
			existing: `{"tags": ["a", "b"]}`,
			template: `{"tags": ["c"]}`,
			expected: `{"tags":["c"]}`,
		},
		{
			name:     "complex nested merge",
			existing: `{"model": "gpt-4", "config": {"temperature": 0.7, "nested": {"key1": "value1"}}}`,
			template: `{"config": {"max_tokens": 1000, "nested": {"key2": "value2"}}}`,
			expected: `{"config":{"max_tokens":1000,"nested":{"key1":"value1","key2":"value2"},"temperature":0.7},"model":"gpt-4"}`,
		},
		{
			name:        "invalid existing JSON",
			existing:    `{invalid`,
			template:    `{}`,
			expectError: true,
		},
		{
			name:        "invalid template JSON",
			existing:    `{}`,
			template:    `{invalid`,
			expectError: true,
		},
		{
			name:        "existing is array not object",
			existing:    `[]`,
			template:    `{}`,
			expectError: true,
		},
		{
			name:        "template is array not object",
			existing:    `{}`,
			template:    `[]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MergeOverrideParameters(tt.existing, tt.template)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.JSONEq(t, tt.expected, result)
			}
		})
	}
}

func TestValidateOverrideParameters(t *testing.T) {
	tests := []struct {
		name        string
		params      string
		expectError bool
	}{
		{
			name:        "empty string is valid",
			params:      "",
			expectError: false,
		},
		{
			name:        "whitespace string is valid",
			params:      "   ",
			expectError: false,
		},
		{
			name:        "valid JSON object",
			params:      `{"temperature": 0.7}`,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			params:      `{invalid}`,
			expectError: true,
		},
		{
			name:        "array not object",
			params:      `["a", "b"]`,
			expectError: true,
		},
		{
			name:        "stream field is forbidden",
			params:      `{"temperature": 0.7, "stream": true}`,
			expectError: true,
		},
		{
			name:        "stream field false is also forbidden",
			params:      `{"stream": false}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOverrideParameters(tt.params)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateOverrideHeaders(t *testing.T) {
	tests := []struct {
		name        string
		headers     []objects.HeaderEntry
		expectError bool
	}{
		{
			name:        "empty headers is valid",
			headers:     []objects.HeaderEntry{},
			expectError: false,
		},
		{
			name: "valid headers",
			headers: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token"},
				{Key: "X-API-Key", Value: "key123"},
			},
			expectError: false,
		},
		{
			name: "empty key",
			headers: []objects.HeaderEntry{
				{Key: "", Value: "value"},
			},
			expectError: true,
		},
		{
			name: "whitespace key",
			headers: []objects.HeaderEntry{
				{Key: "   ", Value: "value"},
			},
			expectError: true,
		},
		{
			name: "duplicate keys case-insensitive",
			headers: []objects.HeaderEntry{
				{Key: "Authorization", Value: "Bearer token1"},
				{Key: "authorization", Value: "Bearer token2"},
			},
			expectError: true,
		},
		{
			name: "duplicate keys different case",
			headers: []objects.HeaderEntry{
				{Key: "X-API-Key", Value: "key1"},
				{Key: "x-api-key", Value: "key2"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOverrideHeaders(tt.headers)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
