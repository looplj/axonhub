package xjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanSchema_SimpleFields(t *testing.T) {
	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id": "https://example.com/schema",
		"title": "Test Schema",
		"description": "A test schema",
		"type": "object",
		"additionalProperties": {"type": "string"},
		"properties": {
			"name": {"type": "string"}
		}
	}`

	result, err := CleanSchema([]byte(schema), "$schema", "$id", "additionalProperties")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	require.NotContains(t, cleaned, "$schema")
	require.NotContains(t, cleaned, "$id")
	require.NotContains(t, cleaned, "additionalProperties")
	require.Equal(t, "Test Schema", cleaned["title"])
	require.Equal(t, "A test schema", cleaned["description"])
	require.Contains(t, cleaned, "properties")
	require.Contains(t, cleaned, "type")
}

func TestCleanSchema_RecursiveSubSchemas(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"user": {
				"type": "object",
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"additionalProperties": {"type": "string"},
				"properties": {
					"name": {
						"type": "string",
						"$schema": "https://json-schema.org/draft/2020-12/schema"
					}
				}
			}
		},
		"items": {
			"type": "array",
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"additionalProperties": {"type": "number"}
		}
	}`

	result, err := CleanSchema([]byte(schema), "$schema", "additionalProperties")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	// Check main schema doesn't have these fields
	require.NotContains(t, cleaned, "$schema")
	require.NotContains(t, cleaned, "additionalProperties")

	// Check nested user schema
	if userProp, ok := cleaned["properties"].(map[string]any)["user"].(map[string]any); ok {
		require.NotContains(t, userProp, "$schema")
		require.NotContains(t, userProp, "additionalProperties")

		// Check deeply nested name schema
		if nameProp, ok := userProp["properties"].(map[string]any)["name"].(map[string]any); ok {
			require.NotContains(t, nameProp, "$schema")
		}
	}

	// Check items schema
	if items, ok := cleaned["items"].(map[string]any); ok {
		require.NotContains(t, items, "$schema")
		require.NotContains(t, items, "additionalProperties")
	}
}

func TestCleanSchema_ArraySchemas(t *testing.T) {
	schema := `{
		"type": "object",
		"allOf": [
			{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type": "string",
				"additionalProperties": {"type": "string"}
			},
			{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type": "number",
				"additionalProperties": {"type": "number"}
			}
		],
		"anyOf": [
			{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"additionalProperties": {"type": "boolean"}
			}
		],
		"oneOf": [
			{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"additionalProperties": {"type": "null"}
			}
		]
	}`

	result, err := CleanSchema([]byte(schema), "$schema", "additionalProperties")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	// Check allOf schemas
	if allOf, ok := cleaned["allOf"].([]any); ok {
		for _, schema := range allOf {
			if schemaMap, ok := schema.(map[string]any); ok {
				require.NotContains(t, schemaMap, "$schema")
				require.NotContains(t, schemaMap, "additionalProperties")
			}
		}
	}

	// Check anyOf schemas
	if anyOf, ok := cleaned["anyOf"].([]any); ok {
		for _, schema := range anyOf {
			if schemaMap, ok := schema.(map[string]any); ok {
				require.NotContains(t, schemaMap, "$schema")
				require.NotContains(t, schemaMap, "additionalProperties")
			}
		}
	}

	// Check oneOf schemas
	if oneOf, ok := cleaned["oneOf"].([]any); ok {
		for _, schema := range oneOf {
			if schemaMap, ok := schema.(map[string]any); ok {
				require.NotContains(t, schemaMap, "$schema")
				require.NotContains(t, schemaMap, "additionalProperties")
			}
		}
	}
}

func TestCleanSchema_MapSchemas(t *testing.T) {
	schema := `{
		"type": "object",
		"definitions": {
			"user": {
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type": "object",
				"additionalProperties": {"type": "string"}
			},
			"address": {
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type": "object",
				"properties": {
					"street": {
						"$schema": "https://json-schema.org/draft/2020-12/schema",
						"type": "string"
					}
				}
			}
		},
		"properties": {
			"data": {
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"additionalProperties": {"type": "object"}
			}
		}
	}`

	result, err := CleanSchema([]byte(schema), "$schema", "additionalProperties")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	// Check definitions
	if definitions, ok := cleaned["definitions"].(map[string]any); ok {
		for _, def := range definitions {
			if defMap, ok := def.(map[string]any); ok {
				require.NotContains(t, defMap, "$schema")
				require.NotContains(t, defMap, "additionalProperties")

				// Check nested properties in address definition
				if props, ok := defMap["properties"].(map[string]any); ok {
					for _, prop := range props {
						if propMap, ok := prop.(map[string]any); ok {
							require.NotContains(t, propMap, "$schema")
						}
					}
				}
			}
		}
	}

	// Check properties
	if properties, ok := cleaned["properties"].(map[string]any); ok {
		for _, prop := range properties {
			if propMap, ok := prop.(map[string]any); ok {
				require.NotContains(t, propMap, "$schema")
				require.NotContains(t, propMap, "additionalProperties")
			}
		}
	}
}

func TestCleanSchema_ConditionalSchemas(t *testing.T) {
	schema := `{
		"type": "object",
		"if": {
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"additionalProperties": {"type": "string"},
			"properties": {
				"type": {"type": "string"}
			}
		},
		"then": {
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"additionalProperties": {"type": "number"},
			"properties": {
				"value": {"type": "number"}
			}
		},
		"else": {
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"additionalProperties": {"type": "boolean"},
			"properties": {
				"flag": {"type": "boolean"}
			}
		},
		"not": {
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"additionalProperties": {"type": "null"},
			"properties": {
				"null_field": {"type": "null"}
			}
		}
	}`

	result, err := CleanSchema([]byte(schema), "$schema", "additionalProperties")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	// Check conditional schemas
	for _, key := range []string{"if", "then", "else", "not"} {
		if schema, ok := cleaned[key].(map[string]any); ok {
			require.NotContains(t, schema, "$schema")
			require.NotContains(t, schema, "additionalProperties")

			// Check nested properties
			if props, ok := schema["properties"].(map[string]any); ok {
				for _, prop := range props {
					if propMap, ok := prop.(map[string]any); ok {
						// These nested properties shouldn't have $schema or additionalProperties
						// since they weren't in the original schema
						require.NotContains(t, propMap, "$schema")
						require.NotContains(t, propMap, "additionalProperties")
					}
				}
			}
		}
	}
}

func TestCleanSchema_ArrayItemSchemas(t *testing.T) {
	schema := `{
		"type": "object",
		"prefixItems": [
			{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type": "string",
				"additionalProperties": {"type": "string"}
			},
			{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type": "number",
				"additionalProperties": {"type": "number"}
			}
		],
		"items": {
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type": "boolean",
			"additionalProperties": {"type": "boolean"}
		},
		"contains": {
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type": "null",
			"additionalProperties": {"type": "null"}
		}
	}`

	result, err := CleanSchema([]byte(schema), "$schema", "additionalProperties")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	// Check prefixItems
	if prefixItems, ok := cleaned["prefixItems"].([]any); ok {
		for _, item := range prefixItems {
			if itemMap, ok := item.(map[string]any); ok {
				require.NotContains(t, itemMap, "$schema")
				require.NotContains(t, itemMap, "additionalProperties")
			}
		}
	}

	// Check items
	if items, ok := cleaned["items"].(map[string]any); ok {
		require.NotContains(t, items, "$schema")
		require.NotContains(t, items, "additionalProperties")
	}

	// Check contains
	if contains, ok := cleaned["contains"].(map[string]any); ok {
		require.NotContains(t, contains, "$schema")
		require.NotContains(t, contains, "additionalProperties")
	}
}

func TestCleanSchema_SpecialFields(t *testing.T) {
	schema := `{
		"$id": "https://example.com/schema",
		"$ref": "#/definitions/user",
		"$comment": "A comment",
		"$anchor": "user",
		"$dynamicAnchor": "dynamic-user",
		"$dynamicRef": "#/dynamic-user",
		"$vocabulary": {"https://json-schema.org/draft/2020-12/vocab/core": true},
		"definitions": {
			"user": {
				"$id": "https://example.com/user",
				"$ref": "#/definitions/name",
				"$comment": "User definition"
			}
		}
	}`

	result, err := CleanSchema([]byte(schema), "$id", "$ref", "$comment", "$anchor", "$dynamicAnchor", "$dynamicRef", "$vocabulary")
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	require.NotContains(t, cleaned, "$id")
	require.NotContains(t, cleaned, "$ref")
	require.NotContains(t, cleaned, "$comment")
	require.NotContains(t, cleaned, "$anchor")
	require.NotContains(t, cleaned, "$dynamicAnchor")
	require.NotContains(t, cleaned, "$dynamicRef")
	require.NotContains(t, cleaned, "$vocabulary")

	// Check nested definitions
	if definitions, ok := cleaned["definitions"].(map[string]any); ok {
		if userDef, ok := definitions["user"].(map[string]any); ok {
			require.NotContains(t, userDef, "$id")
			require.NotContains(t, userDef, "$ref")
			require.NotContains(t, userDef, "$comment")
		}
	}
}

func TestCleanSchema_InvalidJSON(t *testing.T) {
	invalidSchema := `{"invalid": json}`

	_, err := CleanSchema([]byte(invalidSchema), "$schema")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid character")
}

func TestCleanSchema_NoFieldsToClear(t *testing.T) {
	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"additionalProperties": {"type": "string"}
	}`

	result, err := CleanSchema([]byte(schema))
	require.NoError(t, err)

	var cleaned map[string]any
	require.NoError(t, json.Unmarshal(result, &cleaned))

	// Should contain all original fields
	require.Contains(t, cleaned, "$schema")
	require.Contains(t, cleaned, "additionalProperties")
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", cleaned["$schema"])
}

func TestCleanSchema_NilSchemaInput(t *testing.T) {
	_, err := CleanSchema(nil, "$schema")
	require.Error(t, err)
}
