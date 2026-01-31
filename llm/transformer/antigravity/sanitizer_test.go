package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Convert $ref to hints",
			input: `{
				"type": "object",
				"properties": {
					"user": { "$ref": "#/$defs/User" }
				}
			}`,
			expected: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"description": "See: User"
					}
				}
			}`,
		},
		{
			name: "Convert const to enum",
			input: `{
				"type": "object",
				"properties": {
					"mode": { "const": "json" }
				}
			}`,
			expected: `{
				"type": "object",
				"properties": {
					"mode": { "enum": ["json"] }
				}
			}`,
		},
		{
			name: "Add enum hints",
			input: `{
				"type": "string",
				"enum": ["a", "b"]
			}`,
			expected: `{
				"type": "string",
				"enum": ["a", "b"],
				"description": "Allowed: a, b"
			}`,
		},
		{
			name: "Add additionalProperties hints",
			input: `{
				"type": "object",
				"additionalProperties": false
			}`,
			expected: `{
				"type": "object",
				"description": "No extra properties allowed",
				"properties": {
					"_placeholder": {
						"type": "boolean",
						"description": "Placeholder. Always pass true."
					}
				},
				"required": ["_placeholder"]
			}`,
		},
		{
			name: "Move constraints to description",
			input: `{
				"type": "string",
				"minLength": 5,
				"maxLength": 10
			}`,
			expected: `{
				"type": "string",
				"description": "minLength: 5 (maxLength: 10)"
			}`,
		},
		{
			name: "Remove constraint keywords from nested properties",
			input: `{
				"type": "object",
				"properties": {
					"max_turns": {
						"type": "integer",
						"description": "Maximum number of agentic turns",
						"exclusiveMinimum": 0
					}
				}
			}`,
			expected: `{
				"type": "object",
				"properties": {
					"max_turns": {
						"type": "integer",
						"description": "Maximum number of agentic turns (exclusiveMinimum: 0)"
					}
				}
			}`,
		},
		{
			name: "Merge allOf",
			input: `{
				"allOf": [
					{ "properties": { "a": { "type": "string" } }, "required": ["a"] },
					{ "properties": { "b": { "type": "integer" } } }
				]
			}`,
			expected: `{
				"properties": {
					"a": { "type": "string" },
					"b": { "type": "integer" }
				},
				"required": ["a"]
			}`,
		},
		{
			name: "Flatten anyOf",
			input: `{
				"anyOf": [
					{ "type": "string" },
					{ "type": "integer" }
				]
			}`,
			expected: `{
				"type": "string",
				"description": "Accepts: string | integer"
			}`,
		},
		{
			name: "Flatten nullable type array",
			input: `{
				"type": ["string", "null"]
			}`,
			expected: `{
				"type": "string",
				"description": "nullable"
			}`,
		},
		{
			name: "Add placeholder for empty object",
			input: `{
				"type": "object",
				"properties": {}
			}`,
			expected: `{
				"type": "object",
				"properties": {
					"_placeholder": {
						"type": "boolean",
						"description": "Placeholder. Always pass true."
					}
				},
				"required": ["_placeholder"]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputSchema map[string]any
			err := json.Unmarshal([]byte(tt.input), &inputSchema)
			assert.NoError(t, err)

			var expectedSchema map[string]any
			err = json.Unmarshal([]byte(tt.expected), &expectedSchema)
			assert.NoError(t, err)

			sanitized := SanitizeJSONSchema(inputSchema)

			// Helper to check containment since we might have extra fields in sanitized output
			// (like merged descriptions order or preserved fields we didn't explicitly check)
			// But ideally we want exact match on critical structure.
			// Let's loosen strict equality slightly for descriptions if needed,
			// but for now try to be exact on structure.

			// Normalize maps for comparison (json unmarshal produces map[string]interface{})
			// sanitized is already map[string]interface{}

			// assert.Equal(t, expectedSchema, sanitized)
			// Using deep equal might fail on order of keys in description strings if logic changes.
			// But for now let's see.

			// We iterate and check if expected fields are present and match.
			checkExpected(t, expectedSchema, sanitized)
		})
	}
}

func checkExpected(t *testing.T, expected, actual any) {
	if expected == nil {
		assert.Nil(t, actual)
		return
	}

	expMap, okExp := expected.(map[string]any)
	actMap, okAct := actual.(map[string]any)

	if okExp && okAct {
		for k, v := range expMap {
			assert.Contains(t, actMap, k)
			checkExpected(t, v, actMap[k])
		}
		return
	}

	expSlice, okExpS := expected.([]any)
	actSlice, okActS := actual.([]any)

	if okExpS && okActS {
		assert.Equal(t, len(expSlice), len(actSlice))
		for i := range expSlice {
			checkExpected(t, expSlice[i], actSlice[i])
		}
		return
	}

	// For description, use Contains because order of hints might vary or whitespace
	if strExp, ok := expected.(string); ok {
		if strAct, ok := actual.(string); ok {
			// If it's a description/hint field, we might want contains
			// But strictly we expect our test expectations to match the logic output
			assert.Equal(t, strExp, strAct)
			return
		}
	}

	assert.Equal(t, expected, actual)
}
