package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestMergeOverrideHeaders(t *testing.T) {
	setOp := func(path string, value string) objects.OverrideOperation {
		return objects.OverrideOperation{Op: objects.OverrideOpSet, Path: path, Value: value}
	}
	deleteOp := func(path string) objects.OverrideOperation {
		return objects.OverrideOperation{Op: objects.OverrideOpDelete, Path: path}
	}
	renameOp := func(from, to string) objects.OverrideOperation {
		return objects.OverrideOperation{Op: objects.OverrideOpRename, From: from, To: to}
	}

	tests := []struct {
		name     string
		existing []objects.OverrideOperation
		template []objects.OverrideOperation
		expected []objects.OverrideOperation
	}{
		{
			name:     "empty existing and template",
			existing: []objects.OverrideOperation{},
			template: []objects.OverrideOperation{},
			expected: []objects.OverrideOperation{},
		},
		{
			name:     "add new set op",
			existing: []objects.OverrideOperation{setOp("Authorization", "Bearer token1")},
			template: []objects.OverrideOperation{setOp("X-API-Key", "key123")},
			expected: []objects.OverrideOperation{
				setOp("Authorization", "Bearer token1"),
				setOp("X-API-Key", "key123"),
			},
		},
		{
			name: "override existing set op case-insensitive",
			existing: []objects.OverrideOperation{
				setOp("Authorization", "Bearer token1"),
				setOp("Content-Type", "application/json"),
			},
			template: []objects.OverrideOperation{setOp("authorization", "Bearer token2")},
			expected: []objects.OverrideOperation{
				setOp("authorization", "Bearer token2"),
				setOp("Content-Type", "application/json"),
			},
		},
		{
			name: "delete op replaces existing set with same path",
			existing: []objects.OverrideOperation{
				setOp("Authorization", "Bearer token1"),
				setOp("X-API-Key", "key123"),
			},
			template: []objects.OverrideOperation{deleteOp("Authorization")},
			expected: []objects.OverrideOperation{
				deleteOp("Authorization"),
				setOp("X-API-Key", "key123"),
			},
		},
		{
			name:     "delete non-existent header adds delete op",
			existing: []objects.OverrideOperation{setOp("X-API-Key", "key123")},
			template: []objects.OverrideOperation{deleteOp("Authorization")},
			expected: []objects.OverrideOperation{
				setOp("X-API-Key", "key123"),
				deleteOp("Authorization"),
			},
		},
		{
			name:     "rename/copy ops always appended",
			existing: []objects.OverrideOperation{setOp("Authorization", "Bearer token1")},
			template: []objects.OverrideOperation{renameOp("Authorization", "X-Auth")},
			expected: []objects.OverrideOperation{
				setOp("Authorization", "Bearer token1"),
				renameOp("Authorization", "X-Auth"),
			},
		},
		{
			name: "preserve order of non-overridden ops",
			existing: []objects.OverrideOperation{
				setOp("Header1", "value1"),
				setOp("Header2", "value2"),
				setOp("Header3", "value3"),
			},
			template: []objects.OverrideOperation{setOp("Header2", "newvalue2")},
			expected: []objects.OverrideOperation{
				setOp("Header1", "value1"),
				setOp("Header2", "newvalue2"),
				setOp("Header3", "value3"),
			},
		},
		{
			name: "mixed case header paths",
			existing: []objects.OverrideOperation{
				setOp("Content-Type", "application/json"),
				setOp("x-api-key", "key123"),
			},
			template: []objects.OverrideOperation{
				setOp("CONTENT-TYPE", "text/plain"),
				setOp("X-API-KEY", "newkey"),
			},
			expected: []objects.OverrideOperation{
				setOp("CONTENT-TYPE", "text/plain"),
				setOp("X-API-KEY", "newkey"),
			},
		},
		{
			name: "large number of ops merge",
			existing: []objects.OverrideOperation{
				setOp("H1", "v1"), setOp("H2", "v2"), setOp("H3", "v3"),
				setOp("H4", "v4"), setOp("H5", "v5"),
			},
			template: []objects.OverrideOperation{
				setOp("H3", "newv3"), setOp("H6", "v6"), setOp("H7", "v7"),
			},
			expected: []objects.OverrideOperation{
				setOp("H1", "v1"), setOp("H2", "v2"), setOp("H3", "newv3"),
				setOp("H4", "v4"), setOp("H5", "v5"),
				setOp("H6", "v6"), setOp("H7", "v7"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeOverrideHeaders(tt.existing, tt.template)
			require.Equal(t, tt.expected, result)
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, tt.expected, result)
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateOverrideHeaders(t *testing.T) {
	tests := []struct {
		name        string
		ops         []objects.OverrideOperation
		expectError bool
	}{
		{
			name:        "empty ops is valid",
			ops:         []objects.OverrideOperation{},
			expectError: false,
		},
		{
			name: "valid set ops",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpSet, Path: "Authorization", Value: "Bearer token"},
				{Op: objects.OverrideOpSet, Path: "X-API-Key", Value: "key123"},
			},
			expectError: false,
		},
		{
			name: "set with empty path",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpSet, Path: "", Value: "value"},
			},
			expectError: true,
		},
		{
			name: "set with whitespace path",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpSet, Path: "   ", Value: "value"},
			},
			expectError: true,
		},
		{
			name: "delete with empty path",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpDelete, Path: ""},
			},
			expectError: true,
		},
		{
			name: "valid delete op",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpDelete, Path: "Authorization"},
			},
			expectError: false,
		},
		{
			name: "valid rename op",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpRename, From: "Old-Header", To: "New-Header"},
			},
			expectError: false,
		},
		{
			name: "rename with empty from",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpRename, From: "", To: "New-Header"},
			},
			expectError: true,
		},
		{
			name: "rename with empty to",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpRename, From: "Old-Header", To: ""},
			},
			expectError: true,
		},
		{
			name: "valid copy op",
			ops: []objects.OverrideOperation{
				{Op: objects.OverrideOpCopy, From: "Source", To: "Dest"},
			},
			expectError: false,
		},
		{
			name: "unknown op",
			ops: []objects.OverrideOperation{
				{Op: "unknown", Path: "X-Header"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOverrideHeaders(tt.ops)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
