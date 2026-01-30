package antigravity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUppercaseSchemaTypes(t *testing.T) {
	t.Run("uppercases simple type", func(t *testing.T) {
		input := map[string]any{
			"type": "string",
		}
		result := UppercaseSchemaTypes(input)
		assert.Equal(t, "STRING", result["type"])
	})

	t.Run("uppercases object with properties", func(t *testing.T) {
		input := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
				"age": map[string]any{
					"type": "integer",
				},
			},
		}
		result := UppercaseSchemaTypes(input)
		assert.Equal(t, "OBJECT", result["type"])

		props := result["properties"].(map[string]any)
		nameSchema := props["name"].(map[string]any)
		assert.Equal(t, "STRING", nameSchema["type"])

		ageSchema := props["age"].(map[string]any)
		assert.Equal(t, "INTEGER", ageSchema["type"])
	})

	t.Run("uppercases array types", func(t *testing.T) {
		input := map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "string",
			},
		}
		result := UppercaseSchemaTypes(input)
		assert.Equal(t, "ARRAY", result["type"])

		items := result["items"].(map[string]any)
		assert.Equal(t, "STRING", items["type"])
	})

	t.Run("uppercases union types (anyOf)", func(t *testing.T) {
		input := map[string]any{
			"anyOf": []any{
				map[string]any{"type": "string"},
				map[string]any{"type": "number"},
			},
		}
		result := UppercaseSchemaTypes(input)

		anyOf := result["anyOf"].([]any)
		assert.Equal(t, "STRING", anyOf[0].(map[string]any)["type"])
		assert.Equal(t, "NUMBER", anyOf[1].(map[string]any)["type"])
	})

	t.Run("preserves non-type fields", func(t *testing.T) {
		input := map[string]any{
			"type":        "string",
			"description": "A string field",
			"minLength":   1,
			"maxLength":   100,
		}
		result := UppercaseSchemaTypes(input)
		assert.Equal(t, "STRING", result["type"])
		assert.Equal(t, "A string field", result["description"])
		assert.Equal(t, 1, result["minLength"])
		assert.Equal(t, 100, result["maxLength"])
	})

	t.Run("handles nested objects", func(t *testing.T) {
		input := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"address": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"street": map[string]any{
							"type": "string",
						},
						"zipCode": map[string]any{
							"type": "integer",
						},
					},
				},
			},
		}
		result := UppercaseSchemaTypes(input)
		assert.Equal(t, "OBJECT", result["type"])

		props := result["properties"].(map[string]any)
		addressSchema := props["address"].(map[string]any)
		assert.Equal(t, "OBJECT", addressSchema["type"])

		addressProps := addressSchema["properties"].(map[string]any)
		streetSchema := addressProps["street"].(map[string]any)
		assert.Equal(t, "STRING", streetSchema["type"])

		zipSchema := addressProps["zipCode"].(map[string]any)
		assert.Equal(t, "INTEGER", zipSchema["type"])
	})

	t.Run("handles nil schema", func(t *testing.T) {
		result := UppercaseSchemaTypes(nil)
		assert.Nil(t, result)
	})

	t.Run("handles empty schema", func(t *testing.T) {
		input := map[string]any{}
		result := UppercaseSchemaTypes(input)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("handles allOf", func(t *testing.T) {
		input := map[string]any{
			"allOf": []any{
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"age": map[string]any{"type": "integer"},
					},
				},
			},
		}
		result := UppercaseSchemaTypes(input)

		allOf := result["allOf"].([]any)
		firstSchema := allOf[0].(map[string]any)
		assert.Equal(t, "OBJECT", firstSchema["type"])

		secondSchema := allOf[1].(map[string]any)
		assert.Equal(t, "OBJECT", secondSchema["type"])
	})
}
