package gemini

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestConvertLLMToGeminiRequest_UsesParametersJsonSchema verifies that
// function declarations use parametersJsonSchema (new format) instead of
// parameters (old format) to support full JSON Schema including const, enum, etc.
func TestConvertLLMToGeminiRequest_UsesParametersJsonSchema(t *testing.T) {
	req := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("test"),
				},
			},
		},
		Tools: []llm.Tool{
			{
				Type: "function",
				Function: llm.Function{
					Name:        "test_tool",
					Description: "A test tool",
					ParametersJsonSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"mode": {
								"type": "string",
								"const": "strict",
								"description": "Must be strict"
							},
							"options": {
								"type": "string",
								"enum": ["opt1", "opt2"],
								"description": "Available options"
							}
						},
						"required": ["mode"]
					}`),
				},
			},
		},
	}

	result := convertLLMToGeminiRequest(req)

	// Verify tools were converted
	require.NotNil(t, result.Tools)
	require.Len(t, result.Tools, 1)
	require.NotNil(t, result.Tools[0].FunctionDeclarations)
	require.Len(t, result.Tools[0].FunctionDeclarations, 1)

	fd := result.Tools[0].FunctionDeclarations[0]
	require.Equal(t, "test_tool", fd.Name)
	require.Equal(t, "A test tool", fd.Description)

	// CRITICAL: Must use ParametersJsonSchema (new format), not Parameters (old format)
	require.Nil(t, fd.Parameters, "Parameters field (old format) should not be used")
	require.NotNil(t, fd.ParametersJsonSchema, "ParametersJsonSchema field (new format) must be used")

	// Verify the schema content is correct
	var schema map[string]any

	err := json.Unmarshal(fd.ParametersJsonSchema, &schema)
	require.NoError(t, err)

	// Verify const and enum are preserved
	props := schema["properties"].(map[string]any)
	mode := props["mode"].(map[string]any)
	options := props["options"].(map[string]any)

	require.Equal(t, "strict", mode["const"], "const field must be preserved")
	require.ElementsMatch(t, []any{"opt1", "opt2"}, options["enum"], "enum field must be preserved")
}
