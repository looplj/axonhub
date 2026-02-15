package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// =============================================================================
// Basic Tests for convertLLMToGeminiRequest
// =============================================================================

func TestConvertLLMToGeminiRequest_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "simple text request",
			input: &llm.Request{
				Model:     "gemini-2.5-flash",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.Equal(t, int64(1024), result.GenerationConfig.MaxOutputTokens)
				require.Len(t, result.Contents, 1)
				require.Equal(t, "user", result.Contents[0].Role)
				require.Len(t, result.Contents[0].Parts, 1)
				require.Equal(t, "Hello, Gemini!", result.Contents[0].Parts[0].Text)
			},
		},
		{
			name: "request with system message",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: lo.ToPtr("You are a helpful assistant."),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.SystemInstruction)
				require.Len(t, result.SystemInstruction.Parts, 1)
				require.Equal(t, "You are a helpful assistant.", result.SystemInstruction.Parts[0].Text)
				require.Len(t, result.Contents, 1)
				require.Equal(t, "user", result.Contents[0].Role)
			},
		},
		{
			name: "request with multiple system messages",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: lo.ToPtr("First instruction."),
						},
					},
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Second instruction."),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.SystemInstruction)
				require.Len(t, result.SystemInstruction.Parts, 2)
				require.Equal(t, "First instruction.", result.SystemInstruction.Parts[0].Text)
				require.Equal(t, "Second instruction.", result.SystemInstruction.Parts[1].Text)
			},
		},
		{
			name: "request with system message containing multiple content parts",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{Type: "text", Text: lo.ToPtr("Part A")},
								{Type: "text", Text: lo.ToPtr("Part B")},
								{Type: "text", Text: lo.ToPtr("Part C")},
							},
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.SystemInstruction)
				require.Len(t, result.SystemInstruction.Parts, 3)
				require.Equal(t, "Part A", result.SystemInstruction.Parts[0].Text)
				require.Equal(t, "Part B", result.SystemInstruction.Parts[1].Text)
				require.Equal(t, "Part C", result.SystemInstruction.Parts[2].Text)
			},
		},
		{
			name: "request with generation config",
			input: &llm.Request{
				MaxTokens:        lo.ToPtr(int64(2048)),
				Temperature:      lo.ToPtr(0.7),
				TopP:             lo.ToPtr(0.9),
				PresencePenalty:  lo.ToPtr(0.5),
				FrequencyPenalty: lo.ToPtr(0.3),
				Seed:             lo.ToPtr(int64(42)),
				Stop: &llm.Stop{
					MultipleStop: []string{"END", "STOP"},
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.Equal(t, int64(2048), result.GenerationConfig.MaxOutputTokens)
				require.InDelta(t, float32(0.7), *result.GenerationConfig.Temperature, 0.01)
				require.InDelta(t, float32(0.9), *result.GenerationConfig.TopP, 0.01)
				require.InDelta(t, float32(0.5), *result.GenerationConfig.PresencePenalty, 0.01)
				require.InDelta(t, float32(0.3), *result.GenerationConfig.FrequencyPenalty, 0.01)
				require.Equal(t, int64(42), *result.GenerationConfig.Seed)
				require.Equal(t, []string{"END", "STOP"}, result.GenerationConfig.StopSequences)
			},
		},
		{
			name: "request with single stop sequence",
			input: &llm.Request{
				Stop: &llm.Stop{
					Stop: lo.ToPtr("END"),
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.Equal(t, []string{"END"}, result.GenerationConfig.StopSequences)
			},
		},
		{
			name: "request with max_completion_tokens",
			input: &llm.Request{
				MaxCompletionTokens: lo.ToPtr(int64(512)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.Equal(t, int64(512), result.GenerationConfig.MaxOutputTokens)
			},
		},
		{
			name: "request with reasoning effort low",
			input: &llm.Request{
				ReasoningEffort: "low",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Quick question"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
				require.Equal(t, "low", result.GenerationConfig.ThinkingConfig.ThinkingLevel)
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			},
		},
		{
			name: "request with reasoning effort medium",
			input: &llm.Request{
				ReasoningEffort: "medium",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Normal question"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
				require.Equal(t, "medium", result.GenerationConfig.ThinkingConfig.ThinkingLevel)
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			},
		},
		{
			name: "request with reasoning effort high",
			input: &llm.Request{
				ReasoningEffort: "high",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Complex problem"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
				require.Equal(t, "high", result.GenerationConfig.ThinkingConfig.ThinkingLevel)
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			},
		},
		{
			name: "request with reasoning effort and budget preservation",
			input: &llm.Request{
				ReasoningEffort: "medium",
				ReasoningBudget: lo.ToPtr(int64(12000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Question with custom budget"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
				require.Equal(t, int64(12000), *result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			},
		},
		{
			name: "request with reasoning effort and budget exceeding max",
			input: &llm.Request{
				ReasoningEffort: "high",
				ReasoningBudget: lo.ToPtr(int64(50000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Question with large budget"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
				// Should be capped at 24576 (Gemini max)
				require.Equal(t, int64(24576), *result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			},
		},
		{
			name: "request with reasoning budget priority - budget exceeds max",
			input: &llm.Request{
				ReasoningBudget: lo.ToPtr(int64(50000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, int64(24576), *result.GenerationConfig.ThinkingConfig.ThinkingBudget)
			},
		},
		{
			name: "request with reasoning budget priority - budget overrides effort",
			input: &llm.Request{
				ReasoningEffort: "low",                  // Would map to 1024
				ReasoningBudget: lo.ToPtr(int64(20000)), // Budget should take priority
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Complex task"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, int64(20000), *result.GenerationConfig.ThinkingConfig.ThinkingBudget)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
		{
			name: "request with reasoning budget only - no effort",
			input: &llm.Request{
				ReasoningBudget: lo.ToPtr(int64(15000)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Task"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, int64(15000), *result.GenerationConfig.ThinkingConfig.ThinkingBudget)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
		{
			name: "request with reasoning effort only - no budget",
			input: &llm.Request{
				ReasoningEffort: "high",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Task"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, "high", result.GenerationConfig.ThinkingConfig.ThinkingLevel) // Should use ThinkingLevel for standard values
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
		{
			name: "request with ExtraBody ThinkingLevel priority",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ExtraBody: json.RawMessage(`{"google":{"thinking_config":{"thinking_level":"high","thinking_budget":1024,"include_thoughts":true}}}`),
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, "high", result.GenerationConfig.ThinkingConfig.ThinkingLevel) // Level takes priority
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)          // Budget should not be set when level is present
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
		{
			name: "request with ExtraBody minimal mapping to low",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ExtraBody: json.RawMessage(`{"google":{"thinking_config":{"thinking_level":"minimal","include_thoughts":true}}}`),
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, "low", result.GenerationConfig.ThinkingConfig.ThinkingLevel) // minimal maps to low
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
		{
			name: "request with ExtraBody string budget converts to level",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ExtraBody: json.RawMessage(`{"google":{"thinking_config":{"thinking_budget":"high","include_thoughts":true}}}`),
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.Equal(t, "high", result.GenerationConfig.ThinkingConfig.ThinkingLevel) // String budget converts to level
				require.Nil(t, result.GenerationConfig.ThinkingConfig.ThinkingBudget)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertLLMToGeminiRequest_ResponseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "json_schema converts to ResponseSchema and ResponseMIMEType",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate JSON"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type:       "json_schema",
					JSONSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}}}`),
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ResponseSchema)
				require.Equal(t, "application/json", result.GenerationConfig.ResponseMIMEType)
				require.Contains(t, string(result.GenerationConfig.ResponseSchema), "name")
				require.Contains(t, string(result.GenerationConfig.ResponseSchema), "age")
			},
		},
		{
			name: "json_object converts to ResponseMIMEType only",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate JSON"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_object",
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.Equal(t, "application/json", result.GenerationConfig.ResponseMIMEType)
				require.Nil(t, result.GenerationConfig.ResponseSchema)
			},
		},
		{
			name: "text format does not set ResponseMIMEType or ResponseSchema",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "text",
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()

				if result.GenerationConfig != nil {
					require.Empty(t, result.GenerationConfig.ResponseMIMEType)
					require.Nil(t, result.GenerationConfig.ResponseSchema)
				}
			},
		},
		{
			name: "no ResponseFormat does not set ResponseMIMEType or ResponseSchema",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()

				if result.GenerationConfig != nil {
					require.Empty(t, result.GenerationConfig.ResponseMIMEType)
					require.Nil(t, result.GenerationConfig.ResponseSchema)
				}
			},
		},
		{
			name: "json_schema without JSONSchema only sets ResponseMIMEType",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate JSON"),
						},
					},
				},
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_schema",
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()

				if result.GenerationConfig != nil {
					require.Empty(t, result.GenerationConfig.ResponseMIMEType)
					require.Nil(t, result.GenerationConfig.ResponseSchema)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertLLMToGeminiRequest_Tools(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "request with tools",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("What's the weather?"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "get_weather",
							Description: "Get weather information",
							Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.Len(t, result.Tools[0].FunctionDeclarations, 1)
				require.Equal(t, "get_weather", result.Tools[0].FunctionDeclarations[0].Name)
				require.Equal(t, "Get weather information", result.Tools[0].FunctionDeclarations[0].Description)
			},
		},
		{
			name: "request with multiple tools",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Help me"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "tool1",
							Description: "First tool",
						},
					},
					{
						Type: "function",
						Function: llm.Function{
							Name:        "tool2",
							Description: "Second tool",
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.Len(t, result.Tools[0].FunctionDeclarations, 2)
				require.Equal(t, "tool1", result.Tools[0].FunctionDeclarations[0].Name)
				require.Equal(t, "tool2", result.Tools[0].FunctionDeclarations[1].Name)
			},
		},
		{
			name: "request with google search tool",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Search the web"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeGoogleSearch,
						Google: &llm.GoogleTools{
							Search: &llm.GoogleSearch{},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.NotNil(t, result.Tools[0].GoogleSearch)
				require.Nil(t, result.Tools[0].FunctionDeclarations)
				require.Nil(t, result.Tools[0].CodeExecution)
			},
		},
		{
			name: "request with code execution tool",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Run some code"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeGoogleCodeExecution,
						Google: &llm.GoogleTools{
							CodeExecution: &llm.GoogleCodeExecution{},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.NotNil(t, result.Tools[0].CodeExecution)
				require.Nil(t, result.Tools[0].FunctionDeclarations)
				require.Nil(t, result.Tools[0].GoogleSearch)
			},
		},
		{
			name: "request with url context tool",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Fetch URL content"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: llm.ToolTypeGoogleUrlContext,
						Google: &llm.GoogleTools{
							UrlContext: &llm.GoogleUrlContext{},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.NotNil(t, result.Tools[0].UrlContext)
				require.Nil(t, result.Tools[0].FunctionDeclarations)
				require.Nil(t, result.Tools[0].GoogleSearch)
				require.Nil(t, result.Tools[0].CodeExecution)
			},
		},
		{
			name: "request with mixed tools (function, google search, code execution)",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Use all tools"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "function",
						Function: llm.Function{
							Name:        "get_weather",
							Description: "Get weather info",
							Parameters:  json.RawMessage(`{"type":"object"}`),
						},
					},
					{
						Type: llm.ToolTypeGoogleSearch,
						Google: &llm.GoogleTools{
							Search: &llm.GoogleSearch{},
						},
					},
					{
						Type: llm.ToolTypeGoogleCodeExecution,
						Google: &llm.GoogleTools{
							CodeExecution: &llm.GoogleCodeExecution{},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 3)
				// First tool should have function declarations
				require.NotNil(t, result.Tools[0].FunctionDeclarations)
				require.Len(t, result.Tools[0].FunctionDeclarations, 1)
				require.Equal(t, "get_weather", result.Tools[0].FunctionDeclarations[0].Name)
				// Second tool should be google search
				require.NotNil(t, result.Tools[1].GoogleSearch)
				// Third tool should be code execution
				require.NotNil(t, result.Tools[2].CodeExecution)
			},
		},
		{
			name: "request with tool choice auto",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ToolChoice: &llm.ToolChoice{
					ToolChoice: lo.ToPtr("auto"),
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.ToolConfig)
				require.NotNil(t, result.ToolConfig.FunctionCallingConfig)
				require.Equal(t, "AUTO", result.ToolConfig.FunctionCallingConfig.Mode)
			},
		},
		{
			name: "request with tool choice none",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ToolChoice: &llm.ToolChoice{
					ToolChoice: lo.ToPtr("none"),
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.ToolConfig)
				require.NotNil(t, result.ToolConfig.FunctionCallingConfig)
				require.Equal(t, "NONE", result.ToolConfig.FunctionCallingConfig.Mode)
			},
		},
		{
			name: "request with tool choice required",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ToolChoice: &llm.ToolChoice{
					ToolChoice: lo.ToPtr("required"),
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.ToolConfig)
				require.NotNil(t, result.ToolConfig.FunctionCallingConfig)
				require.Equal(t, "ANY", result.ToolConfig.FunctionCallingConfig.Mode)
			},
		},
		{
			name: "request with named tool choice",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				ToolChoice: &llm.ToolChoice{
					NamedToolChoice: &llm.NamedToolChoice{
						Type: "function",
						Function: llm.ToolFunction{
							Name: "specific_function",
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.ToolConfig)
				require.NotNil(t, result.ToolConfig.FunctionCallingConfig)
				require.Equal(t, "ANY", result.ToolConfig.FunctionCallingConfig.Mode)
				require.Equal(t, []string{"specific_function"}, result.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertLLMMessageToGeminiContent(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Message
		validate func(t *testing.T, result *Content)
	}{
		{
			name: "simple text message",
			input: &llm.Message{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello"),
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.Equal(t, "Hello", result.Parts[0].Text)
			},
		},
		{
			name: "assistant role conversion",
			input: &llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Response"),
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "model", result.Role)
			},
		},
		{
			name: "message with reasoning content",
			input: &llm.Message{
				Role:             "assistant",
				ReasoningContent: lo.ToPtr("Let me think..."),
				Content: llm.MessageContent{
					Content: lo.ToPtr("The answer is 42"),
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Len(t, result.Parts, 2)
				require.True(t, result.Parts[0].Thought)
				require.Equal(t, "Let me think...", result.Parts[0].Text)
				require.False(t, result.Parts[1].Thought)
				require.Equal(t, "The answer is 42", result.Parts[1].Text)
			},
		},
		{
			name: "message with multiple content parts",
			input: &llm.Message{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("First part")},
						{Type: "text", Text: lo.ToPtr("Second part")},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Len(t, result.Parts, 2)
				require.Equal(t, "First part", result.Parts[0].Text)
				require.Equal(t, "Second part", result.Parts[1].Text)
			},
		},
		{
			name: "message with image URL (data URL)",
			input: &llm.Message{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].InlineData)
				require.Equal(t, "image/jpeg", result.Parts[0].InlineData.MIMEType)
				require.Equal(t, "/9j/4AAQSkZJRg==", result.Parts[0].InlineData.Data)
			},
		},
		{
			name: "message with image URL (regular URL)",
			input: &llm.Message{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: "https://example.com/image.jpg",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FileData)
				require.Equal(t, "https://example.com/image.jpg", result.Parts[0].FileData.FileURI)
			},
		},
		{
			name: "message with tool calls",
			input: &llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: lo.ToPtr("I'll check the weather"),
				},
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_001",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Len(t, result.Parts, 2)
				require.Equal(t, "I'll check the weather", result.Parts[0].Text)
				require.NotNil(t, result.Parts[1].FunctionCall)
				require.Equal(t, "call_001", result.Parts[1].FunctionCall.ID)
				require.Equal(t, "get_weather", result.Parts[1].FunctionCall.Name)
				require.Equal(t, "NYC", result.Parts[1].FunctionCall.Args["location"])
			},
		},
		{
			name: "empty message",
			input: &llm.Message{
				Role:    "user",
				Content: llm.MessageContent{},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMMessageToGeminiContent(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertLLMToolMessageToGeminiContent(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Message
		req      *GenerateContentRequest
		validate func(t *testing.T, result *Content)
	}{
		{
			name: "tool message with JSON content",
			input: &llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_123"),
				Content: llm.MessageContent{
					Content: lo.ToPtr(`{"temperature": 72, "unit": "F"}`),
				},
			},
			req: &GenerateContentRequest{},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionResponse)
				require.Equal(t, "call_123", result.Parts[0].FunctionResponse.ID)
				require.Equal(t, 72.0, result.Parts[0].FunctionResponse.Response["temperature"])
			},
		},
		{
			name: "tool message with non-JSON content",
			input: &llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_456"),
				Content: llm.MessageContent{
					Content: lo.ToPtr("Plain text result"),
				},
			},
			req: &GenerateContentRequest{},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionResponse)
				require.Equal(t, "call_456", result.Parts[0].FunctionResponse.ID)
				require.Equal(t, "Plain text result", result.Parts[0].FunctionResponse.Response["result"])
			},
		},
		{
			name: "tool message without tool call ID",
			input: &llm.Message{
				Role: "tool",
				Content: llm.MessageContent{
					Content: lo.ToPtr(`{"result": "success"}`),
				},
			},
			req: &GenerateContentRequest{},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionResponse)
				require.Equal(t, "", result.Parts[0].FunctionResponse.ID)
			},
		},
		{
			name: "tool message with name from ToolCallName",
			input: &llm.Message{
				Role:         "tool",
				ToolCallID:   lo.ToPtr("call_789"),
				ToolCallName: lo.ToPtr("get_weather"),
				Content: llm.MessageContent{
					Content: lo.ToPtr(`{"temp": 25}`),
				},
			},
			req: &GenerateContentRequest{},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionResponse)
				require.Equal(t, "call_789", result.Parts[0].FunctionResponse.ID)
				require.Equal(t, "get_weather", result.Parts[0].FunctionResponse.Name)
			},
		},
		{
			name: "tool message finds name from previous function call by ID",
			input: &llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_abc"),
				Content: llm.MessageContent{
					Content: lo.ToPtr(`{"result": "found"}`),
				},
			},
			req: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "model",
						Parts: []*Part{
							{
								FunctionCall: &FunctionCall{
									ID:   "call_abc",
									Name: "search_function",
									Args: map[string]any{"query": "test"},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionResponse)
				require.Equal(t, "call_abc", result.Parts[0].FunctionResponse.ID)
				require.Equal(t, "search_function", result.Parts[0].FunctionResponse.Name)
			},
		},
		{
			name: "tool message with name priority - ToolCallName over lookup",
			input: &llm.Message{
				Role:         "tool",
				ToolCallID:   lo.ToPtr("call_xyz"),
				ToolCallName: lo.ToPtr("explicit_name"),
				Content: llm.MessageContent{
					Content: lo.ToPtr(`{"data": "value"}`),
				},
			},
			req: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "model",
						Parts: []*Part{
							{
								FunctionCall: &FunctionCall{
									ID:   "call_xyz",
									Name: "lookup_name",
									Args: map[string]any{},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.Equal(t, "user", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionResponse)
				require.Equal(t, "call_xyz", result.Parts[0].FunctionResponse.ID)
				// ToolCallName takes priority over lookup
				require.Equal(t, "explicit_name", result.Parts[0].FunctionResponse.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToolResultToGeminiContent(tt.input, tt.req.Contents)
			tt.validate(t, result)
		})
	}
}

// =============================================================================
// Basic Tests for convertGeminiToLLMResponse
// =============================================================================

func TestConvertGeminiToLLMResponse_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentResponse
		validate func(t *testing.T, result *llm.Response)
	}{
		{
			name: "simple response",
			input: &GenerateContentResponse{
				ResponseID:   "resp_123",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Hello!"},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "resp_123", result.ID)
				require.Equal(t, "chat.completion", result.Object)
				require.Equal(t, "gemini-2.5-flash", result.Model)
				require.Len(t, result.Choices, 1)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
				require.Equal(t, "Hello!", *result.Choices[0].Message.Content.Content)
				require.Equal(t, "stop", *result.Choices[0].FinishReason)
			},
		},
		{
			name: "response without ID generates one",
			input: &GenerateContentResponse{
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response"},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.NotEmpty(t, result.ID)
				require.True(t, len(result.ID) > 0)
			},
		},
		{
			name: "response with thinking content",
			input: &GenerateContentResponse{
				ResponseID:   "resp_think",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Let me think...", Thought: true},
								{Text: "The answer is 42"},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.NotNil(t, result.Choices[0].Message.ReasoningContent)
				require.Equal(t, "Let me think...", *result.Choices[0].Message.ReasoningContent)
				require.Equal(t, "The answer is 42", *result.Choices[0].Message.Content.Content)
			},
		},
		{
			name: "response with function call",
			input: &GenerateContentResponse{
				ResponseID:   "resp_func",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "I'll check the weather"},
								{
									FunctionCall: &FunctionCall{
										ID:   "call_001",
										Name: "get_weather",
										Args: map[string]any{"location": "NYC"},
									},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "I'll check the weather", *result.Choices[0].Message.Content.Content)
				require.Len(t, result.Choices[0].Message.ToolCalls, 1)
				require.Equal(t, "call_001", result.Choices[0].Message.ToolCalls[0].ID)
				require.Equal(t, "function", result.Choices[0].Message.ToolCalls[0].Type)
				require.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
				require.Contains(t, result.Choices[0].Message.ToolCalls[0].Function.Arguments, "NYC")
			},
		},
		{
			name: "response with usage metadata",
			input: &GenerateContentResponse{
				ResponseID:   "resp_usage",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response"},
							},
						},
					},
				},
				UsageMetadata: &UsageMetadata{
					PromptTokenCount:        100,
					CandidatesTokenCount:    50,
					TotalTokenCount:         150,
					CachedContentTokenCount: 20,
					ThoughtsTokenCount:      30,
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.NotNil(t, result.Usage)
				require.Equal(t, int64(100), result.Usage.PromptTokens)
				require.Equal(t, int64(80), result.Usage.CompletionTokens) // 50 + 30 thoughts
				require.Equal(t, int64(150), result.Usage.TotalTokens)
				require.NotNil(t, result.Usage.PromptTokensDetails)
				require.Equal(t, int64(20), result.Usage.PromptTokensDetails.CachedTokens)
				require.NotNil(t, result.Usage.CompletionTokensDetails)
				require.Equal(t, int64(30), result.Usage.CompletionTokensDetails.ReasoningTokens)
			},
		},
		{
			name: "response with multiple text parts",
			input: &GenerateContentResponse{
				ResponseID:   "resp_multi",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "First "},
								{Text: "Second "},
								{Text: "Third"},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "First Second Third", *result.Choices[0].Message.Content.Content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGeminiToLLMResponse(tt.input, false)
			tt.validate(t, result)
		})
	}
}

func TestConvertGeminiToLLMResponse_ThoughtSignature(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentResponse
		validate func(t *testing.T, result *llm.Response)
	}{
		{
			name: "function call with thought signature",
			input: &GenerateContentResponse{
				ResponseID:   "resp_sig",
				ModelVersion: "gemini-3-pro",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call_001",
										Name: "check_flight",
										Args: map[string]any{"flight": "AA100"},
									},
									ThoughtSignature: "signature_A",
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].Message)
				require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
				require.True(t, shared.IsGeminiThoughtSignature(result.Choices[0].Message.ReasoningSignature))
				decoded := shared.DecodeGeminiThoughtSignature(result.Choices[0].Message.ReasoningSignature)
				require.NotNil(t, decoded)
				require.Equal(t, "signature_A", *decoded)
				require.Len(t, result.Choices[0].Message.ToolCalls, 1)
				tc := result.Choices[0].Message.ToolCalls[0]
				require.Equal(t, "call_001", tc.ID)
				require.Equal(t, "check_flight", tc.Function.Name)
				require.Nil(t, tc.TransformerMetadata)
			},
		},
		{
			name: "parallel function calls - only first has signature",
			input: &GenerateContentResponse{
				ResponseID:   "resp_parallel",
				ModelVersion: "gemini-3-pro",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call_paris",
										Name: "get_weather",
										Args: map[string]any{"location": "Paris"},
									},
									ThoughtSignature: "signature_parallel",
								},
								{
									FunctionCall: &FunctionCall{
										ID:   "call_london",
										Name: "get_weather",
										Args: map[string]any{"location": "London"},
									},
								},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.NotNil(t, result.Choices[0].Message)
				require.NotNil(t, result.Choices[0].Message.ReasoningSignature)
				require.True(t, shared.IsGeminiThoughtSignature(result.Choices[0].Message.ReasoningSignature))
				decoded := shared.DecodeGeminiThoughtSignature(result.Choices[0].Message.ReasoningSignature)
				require.NotNil(t, decoded)
				require.Equal(t, "signature_parallel", *decoded)
				require.Len(t, result.Choices[0].Message.ToolCalls, 2)

				// First call should have signature
				tc1 := result.Choices[0].Message.ToolCalls[0]
				require.Equal(t, "call_paris", tc1.ID)
				require.Nil(t, tc1.TransformerMetadata)

				// Second call should not have signature
				tc2 := result.Choices[0].Message.ToolCalls[1]
				require.Equal(t, "call_london", tc2.ID)
				require.Nil(t, tc2.TransformerMetadata)
			},
		},
		{
			name: "function call without signature",
			input: &GenerateContentResponse{
				ResponseID:   "resp_no_sig",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{
									FunctionCall: &FunctionCall{
										ID:   "call_002",
										Name: "get_weather",
										Args: map[string]any{"location": "NYC"},
									},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.NotNil(t, result.Choices[0].Message)
				require.Nil(t, result.Choices[0].Message.ReasoningSignature)
				require.Len(t, result.Choices[0].Message.ToolCalls, 1)
				tc := result.Choices[0].Message.ToolCalls[0]
				require.Equal(t, "call_002", tc.ID)
				require.Nil(t, tc.TransformerMetadata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGeminiToLLMResponse(tt.input, false)
			tt.validate(t, result)
		})
	}
}

func TestConvertLLMMessageToGeminiContent_ThoughtSignature(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Message
		validate func(t *testing.T, result *Content)
	}{
		{
			name: "tool call with thought signature",
			input: &llm.Message{
				Role:               "assistant",
				ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("signature_A")),
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_001",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "check_flight",
							Arguments: `{"flight":"AA100"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "model", result.Role)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionCall)
				require.Equal(t, "check_flight", result.Parts[0].FunctionCall.Name)
				require.Equal(t, "signature_A", result.Parts[0].ThoughtSignature)
			},
		},
		{
			name: "multiple tool calls - only first has signature",
			input: &llm.Message{
				Role:               "assistant",
				ReasoningSignature: shared.EncodeGeminiThoughtSignature(lo.ToPtr("signature_A")),
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_001",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "check_flight",
							Arguments: `{"flight":"AA100"}`,
						},
					},
					{
						ID:   "call_002",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "book_taxi",
							Arguments: `{"time":"10 AM"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.Parts, 2)

				require.Equal(t, "check_flight", result.Parts[0].FunctionCall.Name)
				require.Equal(t, "signature_A", result.Parts[0].ThoughtSignature)

				require.Equal(t, "book_taxi", result.Parts[1].FunctionCall.Name)
				require.Empty(t, result.Parts[1].ThoughtSignature)
			},
		},
		{
			name: "tool call without signature",
			input: &llm.Message{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_001",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"NYC"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.Parts, 1)
				require.NotNil(t, result.Parts[0].FunctionCall)
				require.Equal(t, "context_engineering_is_the_way_to_go", result.Parts[0].ThoughtSignature)
			},
		},
		{
			name: "parallel tool calls without signature - only first gets default",
			input: &llm.Message{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_paris",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"Paris"}`,
						},
					},
					{
						ID:   "call_london",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"London"}`,
						},
					},
				},
			},
			validate: func(t *testing.T, result *Content) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result.Parts, 2)
				require.Equal(t, "context_engineering_is_the_way_to_go", result.Parts[0].ThoughtSignature)
				require.Empty(t, result.Parts[1].ThoughtSignature)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMMessageToGeminiContent(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertGeminiToLLMResponse_FinishReasons(t *testing.T) {
	finishReasons := map[string]string{
		"STOP":                    "stop",
		"MAX_TOKENS":              "length",
		"SAFETY":                  "content_filter",
		"RECITATION":              "content_filter",
		"MALFORMED_FUNCTION_CALL": "stop", // Error condition, maps to stop
		"UNKNOWN":                 "stop",
	}

	for geminiReason, expectedLLMReason := range finishReasons {
		t.Run("finish_reason_"+geminiReason, func(t *testing.T) {
			input := &GenerateContentResponse{
				ResponseID:   "resp_finish",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Test"},
							},
						},
						FinishReason: geminiReason,
					},
				},
			}

			result := convertGeminiToLLMResponse(input, false)
			require.NotNil(t, result.Choices[0].FinishReason)
			require.Equal(t, expectedLLMReason, *result.Choices[0].FinishReason)
		})
	}

	// Test empty finish reason returns nil
	t.Run("empty_finish_reason_returns_nil", func(t *testing.T) {
		input := &GenerateContentResponse{
			ResponseID:   "resp_finish",
			ModelVersion: "gemini-2.5-flash",
			Candidates: []*Candidate{
				{
					Index: 0,
					Content: &Content{
						Role: "model",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
					FinishReason: "",
				},
			},
		}

		result := convertGeminiToLLMResponse(input, false)
		require.Nil(t, result.Choices[0].FinishReason)
	})
}

// =============================================================================
// Testdata Tests
// =============================================================================

func TestConvertLLMToGeminiRequest_Testdata(t *testing.T) {
	testCases := []struct {
		name         string
		llmFile      string
		validateFunc func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name:    "simple request",
			llmFile: "llm-simple.request.json",
			validateFunc: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Contents, 1)
				require.Equal(t, "user", result.Contents[0].Role)
				require.Equal(t, "Output 1-20, 5 each line", result.Contents[0].Parts[0].Text)
				require.NotNil(t, result.GenerationConfig)
				require.Equal(t, int64(4096), result.GenerationConfig.MaxOutputTokens)
				require.NotNil(t, result.GenerationConfig.ThinkingConfig)
				require.True(t, result.GenerationConfig.ThinkingConfig.IncludeThoughts)
			},
		},
		{
			name:    "tools request",
			llmFile: "llm-tools.request.json",
			validateFunc: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Contents, 1)
				require.Equal(t, "What is the weather in San Francisco, CA?", result.Contents[0].Parts[0].Text)
				require.Len(t, result.Tools, 1)
				require.Len(t, result.Tools[0].FunctionDeclarations, 2)
				require.Equal(t, "get_coordinates", result.Tools[0].FunctionDeclarations[0].Name)
				require.Equal(t, "get_weather", result.Tools[0].FunctionDeclarations[1].Name)
			},
		},
		{
			name:    "thinking request",
			llmFile: "llm-thinking.request.json",
			validateFunc: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Contents, 3)
				require.Equal(t, "user", result.Contents[0].Role)
				require.Equal(t, "model", result.Contents[1].Role)
				// Check that thinking content is converted to thought part
				hasThought := false

				for _, part := range result.Contents[1].Parts {
					if part.Thought {
						hasThought = true

						require.Contains(t, part.Text, "25 * 47")
					}
				}

				require.True(t, hasThought)
				require.Equal(t, "user", result.Contents[2].Role)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.llmFile))
			require.NoError(t, err)

			var llmReq llm.Request

			err = json.Unmarshal(data, &llmReq)
			require.NoError(t, err)

			result := convertLLMToGeminiRequest(&llmReq)
			tc.validateFunc(t, result)
		})
	}
}

func TestConvertLLMToGeminiResponse_Testdata(t *testing.T) {
	testCases := []struct {
		name         string
		llmFile      string
		validateFunc func(t *testing.T, result *GenerateContentResponse)
	}{
		{
			name:    "simple response",
			llmFile: "llm-simple.response.json",
			validateFunc: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "G34qaY30KYSk0-kPkIX5UA", result.ResponseID)
				require.Equal(t, "gemini-2.5-flash", result.ModelVersion)
				require.Len(t, result.Candidates, 1)
				// Check that reasoning content is converted to thought part
				hasThought := false
				hasText := false

				for _, part := range result.Candidates[0].Content.Parts {
					if part.Thought {
						hasThought = true

						require.Contains(t, part.Text, "Organizing Numbers")
					} else if part.Text != "" {
						hasText = true

						require.Contains(t, part.Text, "1 2 3 4 5")
					}
				}

				require.True(t, hasThought)
				require.True(t, hasText)
			},
		},
		{
			name:    "tools response",
			llmFile: "llm-tools.response.json",
			validateFunc: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "tools-response-001", result.ResponseID)
				require.Len(t, result.Candidates, 1)
				// Check for function call part
				hasFunctionCall := false

				for _, part := range result.Candidates[0].Content.Parts {
					if part.FunctionCall != nil {
						hasFunctionCall = true

						require.Equal(t, "get_coordinates", part.FunctionCall.Name)
					}
				}

				require.True(t, hasFunctionCall)
			},
		},
		{
			name:    "thinking response",
			llmFile: "llm-thinking.response.json",
			validateFunc: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "thinking-response-001", result.ResponseID)
				require.Len(t, result.Candidates, 1)
				// Check that reasoning content is converted to thought part
				hasThought := false

				for _, part := range result.Candidates[0].Content.Parts {
					if part.Thought {
						hasThought = true

						require.Contains(t, part.Text, "1175 by 3")
					}
				}

				require.True(t, hasThought)
				require.NotNil(t, result.UsageMetadata)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.llmFile))
			require.NoError(t, err)

			var llmResp llm.Response

			err = json.Unmarshal(data, &llmResp)
			require.NoError(t, err)

			result := convertLLMToGeminiResponse(&llmResp, false)
			tc.validateFunc(t, result)
		})
	}
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestConvertLLMToGeminiRequest_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "empty messages",
			input: &llm.Request{
				Messages: []llm.Message{},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Empty(t, result.Contents)
			},
		},
		{
			name: "nil generation config fields",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Nil(t, result.GenerationConfig)
			},
		},
		{
			name: "empty tool list",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				Tools: []llm.Tool{},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Empty(t, result.Tools)
			},
		},
		{
			name: "tool with non-function type is skipped",
			input: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Test"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "not_function",
						Function: llm.Function{
							Name: "should_be_skipped",
						},
					},
					{
						Type: "function",
						Function: llm.Function{
							Name: "should_be_included",
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.Len(t, result.Tools[0].FunctionDeclarations, 1)
				require.Equal(t, "should_be_included", result.Tools[0].FunctionDeclarations[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertGeminiToLLMResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentResponse
		validate func(t *testing.T, result *llm.Response)
	}{
		{
			name: "empty candidates",
			input: &GenerateContentResponse{
				ResponseID:   "resp_empty",
				ModelVersion: "gemini-2.5-flash",
				Candidates:   []*Candidate{},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Empty(t, result.Choices)
			},
		},
		{
			name: "candidate with nil content",
			input: &GenerateContentResponse{
				ResponseID:   "resp_nil_content",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index:   0,
						Content: nil,
					},
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.Nil(t, result.Choices[0].Message)
			},
		},
		{
			name: "nil usage metadata",
			input: &GenerateContentResponse{
				ResponseID:   "resp_no_usage",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response"},
							},
						},
					},
				},
				UsageMetadata: nil,
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Nil(t, result.Usage)
			},
		},
		{
			name: "usage metadata without cache or thoughts",
			input: &GenerateContentResponse{
				ResponseID:   "resp_basic_usage",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response"},
							},
						},
					},
				},
				UsageMetadata: &UsageMetadata{
					PromptTokenCount:     100,
					CandidatesTokenCount: 50,
					TotalTokenCount:      150,
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.NotNil(t, result.Usage)
				require.Equal(t, int64(100), result.Usage.PromptTokens)
				require.Equal(t, int64(50), result.Usage.CompletionTokens)
				require.Equal(t, int64(150), result.Usage.TotalTokens)
				require.Nil(t, result.Usage.PromptTokensDetails)
				require.Nil(t, result.Usage.CompletionTokensDetails)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGeminiToLLMResponse(tt.input, false)
			tt.validate(t, result)
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestExtractPartsFromLLMMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Message
		validate func(t *testing.T, result []*Part)
	}{
		{
			name: "single text content",
			input: &llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Hello world"),
				},
			},
			validate: func(t *testing.T, result []*Part) {
				t.Helper()
				require.Len(t, result, 1)
				require.Equal(t, "Hello world", result[0].Text)
			},
		},
		{
			name: "empty content returns empty parts",
			input: &llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					Content: lo.ToPtr(""),
				},
			},
			validate: func(t *testing.T, result []*Part) {
				t.Helper()
				require.Empty(t, result)
			},
		},
		{
			name: "nil content returns empty parts",
			input: &llm.Message{
				Role:    "system",
				Content: llm.MessageContent{},
			},
			validate: func(t *testing.T, result []*Part) {
				t.Helper()
				require.Empty(t, result)
			},
		},
		{
			name: "multiple content parts",
			input: &llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("First part")},
						{Type: "text", Text: lo.ToPtr("Second part")},
						{Type: "text", Text: lo.ToPtr("Third part")},
					},
				},
			},
			validate: func(t *testing.T, result []*Part) {
				t.Helper()
				require.Len(t, result, 3)
				require.Equal(t, "First part", result[0].Text)
				require.Equal(t, "Second part", result[1].Text)
				require.Equal(t, "Third part", result[2].Text)
			},
		},
		{
			name: "multiple content parts with empty text filtered out",
			input: &llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("Valid part")},
						{Type: "text", Text: lo.ToPtr("")},
						{Type: "text", Text: nil},
						{Type: "text", Text: lo.ToPtr("Another valid")},
					},
				},
			},
			validate: func(t *testing.T, result []*Part) {
				t.Helper()
				require.Len(t, result, 2)
				require.Equal(t, "Valid part", result[0].Text)
				require.Equal(t, "Another valid", result[1].Text)
			},
		},
		{
			name: "non-text types are ignored",
			input: &llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: lo.ToPtr("Text part")},
						{Type: "image_url", ImageURL: &llm.ImageURL{URL: "http://example.com/img.jpg"}},
						{Type: "text", Text: lo.ToPtr("Another text")},
					},
				},
			},
			validate: func(t *testing.T, result []*Part) {
				t.Helper()
				require.Len(t, result, 2)
				require.Equal(t, "Text part", result[0].Text)
				require.Equal(t, "Another text", result[1].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPartsFromLLMMessage(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestConvertImageURLToGeminiPart(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		validate func(t *testing.T, result *Part)
	}{
		{
			name: "data URL with base64",
			url:  "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
			validate: func(t *testing.T, result *Part) {
				t.Helper()
				require.NotNil(t, result)
				require.NotNil(t, result.InlineData)
				require.Equal(t, "image/jpeg", result.InlineData.MIMEType)
				require.Equal(t, "/9j/4AAQSkZJRg==", result.InlineData.Data)
			},
		},
		{
			name: "data URL with PNG",
			url:  "data:image/png;base64,iVBORw0KGgo=",
			validate: func(t *testing.T, result *Part) {
				t.Helper()
				require.NotNil(t, result)
				require.NotNil(t, result.InlineData)
				require.Equal(t, "image/png", result.InlineData.MIMEType)
				require.Equal(t, "iVBORw0KGgo=", result.InlineData.Data)
			},
		},
		{
			name: "regular HTTPS URL",
			url:  "https://example.com/image.jpg",
			validate: func(t *testing.T, result *Part) {
				t.Helper()
				require.NotNil(t, result)
				require.NotNil(t, result.FileData)
				require.Equal(t, "https://example.com/image.jpg", result.FileData.FileURI)
			},
		},
		{
			name: "GCS URL",
			url:  "gs://bucket/path/to/image.png",
			validate: func(t *testing.T, result *Part) {
				t.Helper()
				require.NotNil(t, result)
				require.NotNil(t, result.FileData)
				require.Equal(t, "gs://bucket/path/to/image.png", result.FileData.FileURI)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertImageURLToGeminiPart(tt.url)
			tt.validate(t, result)
		})
	}
}

func TestThinkingBudgetToReasoningEffort(t *testing.T) {
	tests := []struct {
		budget   int64
		expected string
	}{
		{512, "low"},
		{1024, "low"},
		{2048, "medium"},
		{8192, "medium"},
		{16384, "high"},
		{32768, "high"},
	}

	for _, tt := range tests {
		t.Run("budget_"+string(rune(tt.budget)), func(t *testing.T) {
			result := thinkingBudgetToReasoningEffort(tt.budget)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestReasoningEffortToThinkingBudget(t *testing.T) {
	tests := []struct {
		effort   string
		expected int64
	}{
		{"low", 1024},
		{"medium", 8192},
		{"high", 32768},
		{"unknown", 8192},
		{"", 8192},
	}

	for _, tt := range tests {
		t.Run("effort_"+tt.effort, func(t *testing.T) {
			result := reasoningEffortToThinkingBudget(tt.effort)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestReasoningEffortToThinkingBudgetWithConfig(t *testing.T) {
	tests := []struct {
		name     string
		effort   string
		config   *Config
		expected int64
	}{
		{
			name:     "low effort with no config",
			effort:   "low",
			config:   nil,
			expected: 1024,
		},
		{
			name:     "medium effort with no config",
			effort:   "medium",
			config:   nil,
			expected: 8192,
		},
		{
			name:     "high effort with no config",
			effort:   "high",
			config:   nil,
			expected: 32768,
		},
		{
			name:   "low effort with custom config",
			effort: "low",
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low":    2000,
					"medium": 9000,
					"high":   35000,
				},
			},
			expected: 2000,
		},
		{
			name:   "medium effort with custom config",
			effort: "medium",
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low":    2000,
					"medium": 9000,
					"high":   35000,
				},
			},
			expected: 9000,
		},
		{
			name:   "high effort with custom config",
			effort: "high",
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low":    2000,
					"medium": 9000,
					"high":   35000,
				},
			},
			expected: 35000,
		},
		{
			name:   "unknown effort falls back to default",
			effort: "unknown",
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low":    2000,
					"medium": 9000,
					"high":   35000,
				},
			},
			expected: 8192, // default medium
		},
		{
			name:   "effort not in config falls back to default",
			effort: "medium",
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{
					"low":  2000,
					"high": 35000,
				},
			},
			expected: 8192, // default medium
		},
		{
			name:   "empty config mapping",
			effort: "high",
			config: &Config{
				ReasoningEffortToBudget: map[string]int64{},
			},
			expected: 32768, // default high
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reasoningEffortToThinkingBudgetWithConfig(tt.effort, tt.config)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertGeminiRoleToLLMRole(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"model", "assistant"},
		{"user", "user"},
		{"", "user"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run("role_"+tt.input, func(t *testing.T) {
			result := convertGeminiRoleToLLMRole(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertLLMRoleToGeminiRole(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"assistant", "model"},
		{"user", "user"},
		{"system", "system"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run("role_"+tt.input, func(t *testing.T) {
			result := convertLLMRoleToGeminiRole(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertGeminiToLLMResponse_GroundingMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentResponse
		isStream bool
		validate func(t *testing.T, result *llm.Response)
	}{
		{
			name: "response with grounding metadata - web search",
			input: &GenerateContentResponse{
				ResponseID:   "resp_grounding",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Based on my search, here is the answer."},
							},
						},
						FinishReason: "STOP",
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries: []string{"latest news about AI"},
							GroundingChunks: []*GroundingChunk{
								{
									Web: &GroundingChunkWeb{
										URI:    "https://example.com/article1",
										Title:  "AI News Article",
										Domain: "example.com",
									},
								},
								{
									Web: &GroundingChunkWeb{
										URI:    "https://example.org/article2",
										Title:  "Another AI Article",
										Domain: "example.org",
									},
								},
							},
							GroundingSupports: []*GroundingSupport{
								{
									Segment: &Segment{
										StartIndex: 0,
										EndIndex:   38,
										Text:       "Based on my search, here is the answer",
									},
									GroundingChunkIndices: []int32{0, 1},
									ConfidenceScores:      []float32{0.95, 0.87},
								},
							},
							SearchEntryPoint: &SearchEntryPoint{
								RenderedContent: "<div>Search results</div>",
							},
							RetrievalMetadata: &RetrievalMetadata{
								GoogleSearchDynamicRetrievalScore: 0.92,
							},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Equal(t, []string{"latest news about AI"}, gm.WebSearchQueries)
				require.Len(t, gm.GroundingChunks, 2)
				require.Equal(t, "https://example.com/article1", gm.GroundingChunks[0].Web.URI)
				require.Equal(t, "AI News Article", gm.GroundingChunks[0].Web.Title)
				require.Equal(t, "example.com", gm.GroundingChunks[0].Web.Domain)
				require.Len(t, gm.GroundingSupports, 1)
				require.Equal(t, []int32{0, 1}, gm.GroundingSupports[0].GroundingChunkIndices)
				require.InDelta(t, 0.95, gm.GroundingSupports[0].ConfidenceScores[0], 0.01)
				require.NotNil(t, gm.SearchEntryPoint)
				require.Equal(t, "<div>Search results</div>", gm.SearchEntryPoint.RenderedContent)
				require.NotNil(t, gm.RetrievalMetadata)
				require.InDelta(t, 0.92, gm.RetrievalMetadata.GoogleSearchDynamicRetrievalScore, 0.01)
			},
		},
		{
			name: "response with grounding metadata - retrieved context",
			input: &GenerateContentResponse{
				ResponseID:   "resp_retrieval",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "According to the document..."},
							},
						},
						FinishReason: "STOP",
						GroundingMetadata: &GroundingMetadata{
							GroundingChunks: []*GroundingChunk{
								{
									RetrievedContext: &GroundingChunkRetrievedContext{
										URI:   "gs://bucket/document.pdf",
										Title: "Important Document",
										Text:  "Relevant excerpt from the document.",
									},
								},
							},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Len(t, gm.GroundingChunks, 1)
				require.NotNil(t, gm.GroundingChunks[0].RetrievedContext)
				require.Equal(t, "gs://bucket/document.pdf", gm.GroundingChunks[0].RetrievedContext.URI)
				require.Equal(t, "Important Document", gm.GroundingChunks[0].RetrievedContext.Title)
				require.Equal(t, "Relevant excerpt from the document.", gm.GroundingChunks[0].RetrievedContext.Text)
			},
		},
		{
			name: "response without grounding metadata",
			input: &GenerateContentResponse{
				ResponseID:   "resp_no_grounding",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Simple response without grounding."},
							},
						},
						FinishReason: "STOP",
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.Nil(t, result.Choices[0].TransformerMetadata)
			},
		},
		{
			name: "streaming response with grounding metadata",
			input: &GenerateContentResponse{
				ResponseID:   "resp_stream_grounding",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Streaming chunk with grounding."},
							},
						},
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries: []string{"streaming search query"},
							GroundingChunks: []*GroundingChunk{
								{
									Web: &GroundingChunkWeb{
										URI:   "https://stream.example.com",
										Title: "Stream Source",
									},
								},
							},
						},
					},
				},
			},
			isStream: true,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "chat.completion.chunk", result.Object)
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Equal(t, []string{"streaming search query"}, gm.WebSearchQueries)
				require.Len(t, gm.GroundingChunks, 1)
				require.Equal(t, "https://stream.example.com", gm.GroundingChunks[0].Web.URI)
			},
		},
		{
			name: "multiple candidates with grounding metadata",
			input: &GenerateContentResponse{
				ResponseID:   "resp_multi_candidates",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "First candidate response."},
							},
						},
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries: []string{"query for candidate 1"},
						},
					},
					{
						Index: 1,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Second candidate response."},
							},
						},
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries: []string{"query for candidate 2"},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 2)

				gm0 := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm0)
				require.Equal(t, []string{"query for candidate 1"}, gm0.WebSearchQueries)

				gm1 := xmap.GetPtr[GroundingMetadata](result.Choices[1].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm1)
				require.Equal(t, []string{"query for candidate 2"}, gm1.WebSearchQueries)
			},
		},
		{
			name: "grounding metadata with segment details",
			input: &GenerateContentResponse{
				ResponseID:   "resp_segment",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "The capital of France is Paris."},
							},
						},
						FinishReason: "STOP",
						GroundingMetadata: &GroundingMetadata{
							GroundingChunks: []*GroundingChunk{
								{
									Web: &GroundingChunkWeb{
										URI:   "https://example.com/france",
										Title: "France Facts",
									},
								},
							},
							GroundingSupports: []*GroundingSupport{
								{
									Segment: &Segment{
										StartIndex: 0,
										EndIndex:   31,
										PartIndex:  0,
										Text:       "The capital of France is Paris.",
									},
									GroundingChunkIndices: []int32{0},
									ConfidenceScores:      []float32{0.99},
								},
							},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Len(t, gm.GroundingSupports, 1)
				seg := gm.GroundingSupports[0].Segment
				require.NotNil(t, seg)
				require.Equal(t, int32(0), seg.StartIndex)
				require.Equal(t, int32(31), seg.EndIndex)
				require.Equal(t, int32(0), seg.PartIndex)
				require.Equal(t, "The capital of France is Paris.", seg.Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGeminiToLLMResponse(tt.input, tt.isStream)
			tt.validate(t, result)
		})
	}
}

// =============================================================================
// SafetySettings Tests
// =============================================================================

func TestConvertLLMToGeminiRequest_SafetySettings(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "request with safety settings",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeySafetySettings: []*SafetySetting{
						{
							Category:  "HARM_CATEGORY_HARASSMENT",
							Threshold: "BLOCK_LOW_AND_ABOVE",
						},
						{
							Category:  "HARM_CATEGORY_HATE_SPEECH",
							Threshold: "BLOCK_MEDIUM_AND_ABOVE",
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.SafetySettings)
				require.Len(t, result.SafetySettings, 2)
				require.Equal(t, "HARM_CATEGORY_HARASSMENT", result.SafetySettings[0].Category)
				require.Equal(t, "BLOCK_LOW_AND_ABOVE", result.SafetySettings[0].Threshold)
				require.Equal(t, "HARM_CATEGORY_HATE_SPEECH", result.SafetySettings[1].Category)
				require.Equal(t, "BLOCK_MEDIUM_AND_ABOVE", result.SafetySettings[1].Threshold)
			},
		},
		{
			name: "request without safety settings",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Nil(t, result.SafetySettings)
			},
		},
		{
			name: "request with empty safety settings",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeySafetySettings: []*SafetySetting{},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Nil(t, result.SafetySettings)
			},
		},
		{
			name: "request with all safety categories",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeySafetySettings: []*SafetySetting{
						{
							Category:  "HARM_CATEGORY_HARASSMENT",
							Threshold: "BLOCK_NONE",
						},
						{
							Category:  "HARM_CATEGORY_HATE_SPEECH",
							Threshold: "BLOCK_LOW_AND_ABOVE",
						},
						{
							Category:  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
							Threshold: "BLOCK_MEDIUM_AND_ABOVE",
						},
						{
							Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
							Threshold: "BLOCK_HIGH_AND_ABOVE",
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.SafetySettings)
				require.Len(t, result.SafetySettings, 4)
				require.Equal(t, "HARM_CATEGORY_HARASSMENT", result.SafetySettings[0].Category)
				require.Equal(t, "BLOCK_NONE", result.SafetySettings[0].Threshold)
				require.Equal(t, "HARM_CATEGORY_HATE_SPEECH", result.SafetySettings[1].Category)
				require.Equal(t, "BLOCK_LOW_AND_ABOVE", result.SafetySettings[1].Threshold)
				require.Equal(t, "HARM_CATEGORY_SEXUALLY_EXPLICIT", result.SafetySettings[2].Category)
				require.Equal(t, "BLOCK_MEDIUM_AND_ABOVE", result.SafetySettings[2].Threshold)
				require.Equal(t, "HARM_CATEGORY_DANGEROUS_CONTENT", result.SafetySettings[3].Category)
				require.Equal(t, "BLOCK_HIGH_AND_ABOVE", result.SafetySettings[3].Threshold)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestExtractSafetySettingsFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		validate func(t *testing.T, result []*SafetySetting)
	}{
		{
			name: "extract multiple safety settings",
			input: map[string]any{
				TransformerMetadataKeySafetySettings: []*SafetySetting{
					{
						Category:  "HARM_CATEGORY_HARASSMENT",
						Threshold: "BLOCK_LOW_AND_ABOVE",
					},
					{
						Category:  "HARM_CATEGORY_HATE_SPEECH",
						Threshold: "BLOCK_MEDIUM_AND_ABOVE",
					},
				},
			},
			validate: func(t *testing.T, result []*SafetySetting) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result, 2)
				require.Equal(t, "HARM_CATEGORY_HARASSMENT", result[0].Category)
				require.Equal(t, "BLOCK_LOW_AND_ABOVE", result[0].Threshold)
				require.Equal(t, "HARM_CATEGORY_HATE_SPEECH", result[1].Category)
				require.Equal(t, "BLOCK_MEDIUM_AND_ABOVE", result[1].Threshold)
			},
		},
		{
			name:  "extract nil safety settings",
			input: nil,
			validate: func(t *testing.T, result []*SafetySetting) {
				t.Helper()
				require.Nil(t, result)
			},
		},
		{
			name: "extract empty safety settings",
			input: map[string]any{
				TransformerMetadataKeySafetySettings: []*SafetySetting{},
			},
			validate: func(t *testing.T, result []*SafetySetting) {
				t.Helper()
				require.Nil(t, result)
			},
		},
		{
			name: "extract single safety setting",
			input: map[string]any{
				TransformerMetadataKeySafetySettings: []*SafetySetting{
					{
						Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
						Threshold: "BLOCK_NONE",
					},
				},
			},
			validate: func(t *testing.T, result []*SafetySetting) {
				t.Helper()
				require.NotNil(t, result)
				require.Len(t, result, 1)
				require.Equal(t, "HARM_CATEGORY_DANGEROUS_CONTENT", result[0].Category)
				require.Equal(t, "BLOCK_NONE", result[0].Threshold)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSafetySettingsFromMetadata(tt.input)
			tt.validate(t, result)
		})
	}
}

// =============================================================================
// ImageConfig Tests
// =============================================================================

func TestConvertLLMToGeminiRequest_ImageConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Request
		validate func(t *testing.T, result *GenerateContentRequest)
	}{
		{
			name: "request with image config",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate an image"),
						},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyImageConfig: &ImageConfig{
						AspectRatio: "16:9",
						ImageSize:   "2K",
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ImageConfig)
				require.Equal(t, "16:9", result.GenerationConfig.ImageConfig.AspectRatio)
				require.Equal(t, "2K", result.GenerationConfig.ImageConfig.ImageSize)
			},
		},
		{
			name: "request without image config",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Nil(t, result.GenerationConfig)
			},
		},
		{
			name: "request with nil image config",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, Gemini!"),
						},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyImageConfig: nil,
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.Nil(t, result.GenerationConfig)
			},
		},
		{
			name: "request with only aspect ratio",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate an image"),
						},
					},
				},
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyImageConfig: &ImageConfig{
						AspectRatio: "1:1",
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ImageConfig)
				require.Equal(t, "1:1", result.GenerationConfig.ImageConfig.AspectRatio)
				require.Empty(t, result.GenerationConfig.ImageConfig.ImageSize)
			},
		},
		{
			name: "request with image config and other generation config",
			input: &llm.Request{
				Model: "gemini-2.5-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate an image"),
						},
					},
				},
				Temperature: lo.ToPtr(0.7),
				TransformerMetadata: map[string]any{
					TransformerMetadataKeyImageConfig: &ImageConfig{
						AspectRatio: "9:16",
						ImageSize:   "4K",
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentRequest) {
				t.Helper()
				require.NotNil(t, result.GenerationConfig)
				require.NotNil(t, result.GenerationConfig.ImageConfig)
				require.Equal(t, "9:16", result.GenerationConfig.ImageConfig.AspectRatio)
				require.Equal(t, "4K", result.GenerationConfig.ImageConfig.ImageSize)
				require.NotNil(t, result.GenerationConfig.Temperature)
				require.Equal(t, 0.7, *result.GenerationConfig.Temperature)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiRequestWithConfig(tt.input, nil)
			tt.validate(t, result)
		})
	}
}

func TestConvertGeminiToLLMResponse_GroundingMetadata_Additional(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentResponse
		isStream bool
		validate func(t *testing.T, result *llm.Response)
	}{
		{
			name: "grounding metadata with all fields populated",
			input: &GenerateContentResponse{
				ResponseID:   "resp_all_fields",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Comprehensive response with all grounding fields."},
							},
						},
						FinishReason: "STOP",
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries: []string{"comprehensive search"},
							GroundingChunks: []*GroundingChunk{
								{
									Web: &GroundingChunkWeb{
										URI:    "https://example.com/comprehensive",
										Title:  "Comprehensive Article",
										Domain: "example.com",
									},
								},
								{
									RetrievedContext: &GroundingChunkRetrievedContext{
										URI:   "gs://bucket/comprehensive.pdf",
										Title: "Comprehensive Document",
										Text:  "Comprehensive document content.",
									},
								},
							},
							GroundingSupports: []*GroundingSupport{
								{
									Segment: &Segment{
										StartIndex: 0,
										EndIndex:   45,
										PartIndex:  0,
										Text:       "Comprehensive response with all grounding fields",
									},
									GroundingChunkIndices: []int32{0, 1},
									ConfidenceScores:      []float32{0.98, 0.92},
								},
							},
							SearchEntryPoint: &SearchEntryPoint{
								RenderedContent: "<div>Comprehensive search entry point</div>",
							},
							RetrievalMetadata: &RetrievalMetadata{
								GoogleSearchDynamicRetrievalScore: 0.95,
							},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Equal(t, []string{"comprehensive search"}, gm.WebSearchQueries)
				require.Len(t, gm.GroundingChunks, 2)
				require.NotNil(t, gm.GroundingChunks[0].Web)
				require.NotNil(t, gm.GroundingChunks[1].RetrievedContext)
				require.Len(t, gm.GroundingSupports, 1)
				require.NotNil(t, gm.SearchEntryPoint)
				require.NotNil(t, gm.RetrievalMetadata)
			},
		},
		{
			name: "grounding metadata with empty arrays",
			input: &GenerateContentResponse{
				ResponseID:   "resp_empty_arrays",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response with empty grounding arrays."},
							},
						},
						FinishReason: "STOP",
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries:  []string{},
							GroundingChunks:   []*GroundingChunk{},
							GroundingSupports: []*GroundingSupport{},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Empty(t, gm.WebSearchQueries)
				require.Empty(t, gm.GroundingChunks)
				require.Empty(t, gm.GroundingSupports)
			},
		},
		{
			name: "mixed candidates with and without grounding metadata",
			input: &GenerateContentResponse{
				ResponseID:   "resp_mixed_grounding",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response with grounding."},
							},
						},
						GroundingMetadata: &GroundingMetadata{
							WebSearchQueries: []string{"candidate 1 search"},
						},
					},
					{
						Index: 1,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Response without grounding."},
							},
						},
					},
					{
						Index: 2,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Another response with grounding."},
							},
						},
						GroundingMetadata: &GroundingMetadata{
							RetrievalMetadata: &RetrievalMetadata{
								GoogleSearchDynamicRetrievalScore: 0.88,
							},
						},
					},
				},
			},
			isStream: false,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Len(t, result.Choices, 3)

				// First choice has grounding metadata
				require.NotNil(t, result.Choices[0].TransformerMetadata)
				gm0 := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm0)
				require.Equal(t, []string{"candidate 1 search"}, gm0.WebSearchQueries)

				// Second choice has no grounding metadata
				require.Nil(t, result.Choices[1].TransformerMetadata)

				// Third choice has grounding metadata
				require.NotNil(t, result.Choices[2].TransformerMetadata)
				gm2 := xmap.GetPtr[GroundingMetadata](result.Choices[2].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm2)
				require.NotNil(t, gm2.RetrievalMetadata)
				require.InDelta(t, 0.88, gm2.RetrievalMetadata.GoogleSearchDynamicRetrievalScore, 0.01)
			},
		},
		{
			name: "streaming response with multiple grounding chunks",
			input: &GenerateContentResponse{
				ResponseID:   "resp_stream_multi_chunks",
				ModelVersion: "gemini-2.5-flash",
				Candidates: []*Candidate{
					{
						Index: 0,
						Content: &Content{
							Role: "model",
							Parts: []*Part{
								{Text: "Streaming with multiple chunks."},
							},
						},
						GroundingMetadata: &GroundingMetadata{
							GroundingChunks: []*GroundingChunk{
								{
									Web: &GroundingChunkWeb{
										URI:   "https://source1.example.com",
										Title: "Source 1",
									},
								},
								{
									Web: &GroundingChunkWeb{
										URI:   "https://source2.example.com",
										Title: "Source 2",
									},
								},
								{
									RetrievedContext: &GroundingChunkRetrievedContext{
										URI:   "gs://bucket/internal.pdf",
										Title: "Internal Document",
									},
								},
							},
							GroundingSupports: []*GroundingSupport{
								{
									Segment: &Segment{
										StartIndex: 0,
										EndIndex:   31,
										Text:       "Streaming with multiple chunks",
									},
									GroundingChunkIndices: []int32{0, 1, 2},
									ConfidenceScores:      []float32{0.95, 0.87, 0.91},
								},
							},
						},
					},
				},
			},
			isStream: true,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "chat.completion.chunk", result.Object)
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].TransformerMetadata)

				gm := xmap.GetPtr[GroundingMetadata](result.Choices[0].TransformerMetadata, TransformerMetadataKeyGroundingMetadata)
				require.NotNil(t, gm)
				require.Len(t, gm.GroundingChunks, 3)
				require.NotNil(t, gm.GroundingChunks[0].Web)
				require.NotNil(t, gm.GroundingChunks[1].Web)
				require.NotNil(t, gm.GroundingChunks[2].RetrievedContext)
				require.Len(t, gm.GroundingSupports, 1)
				require.Equal(t, []int32{0, 1, 2}, gm.GroundingSupports[0].GroundingChunkIndices)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertGeminiToLLMResponse(tt.input, tt.isStream)
			tt.validate(t, result)
		})
	}
}
