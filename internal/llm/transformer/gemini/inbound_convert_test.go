package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

// =============================================================================
// Basic Tests for convertGeminiToLLMRequest
// =============================================================================

func TestConvertGeminiToLLMRequest_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentRequest
		validate func(t *testing.T, result *llm.Request)
	}{
		{
			name: "simple text request",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Hello, Gemini!"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					MaxOutputTokens: 1024,
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, llm.APIFormatGeminiContents, result.RawAPIFormat)
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "Hello, Gemini!", *result.Messages[0].Content.Content)
				require.Equal(t, int64(1024), *result.MaxTokens)
			},
		},
		{
			name: "request with system instruction",
			input: &GenerateContentRequest{
				SystemInstruction: &Content{
					Parts: []*Part{
						{Text: "You are a helpful assistant."},
					},
				},
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Hello!"},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Messages, 2)
				require.Equal(t, "system", result.Messages[0].Role)
				require.Equal(t, "You are a helpful assistant.", *result.Messages[0].Content.Content)
				require.Equal(t, "user", result.Messages[1].Role)
			},
		},
		{
			name: "request with generation config",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					MaxOutputTokens:  2048,
					Temperature:      lo.ToPtr(0.7),
					TopP:             lo.ToPtr(0.9),
					PresencePenalty:  lo.ToPtr(0.5),
					FrequencyPenalty: lo.ToPtr(0.3),
					Seed:             lo.ToPtr(int64(42)),
					StopSequences:    []string{"END", "STOP"},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Equal(t, int64(2048), *result.MaxTokens)
				require.InDelta(t, 0.7, *result.Temperature, 0.01)
				require.InDelta(t, 0.9, *result.TopP, 0.01)
				require.InDelta(t, 0.5, *result.PresencePenalty, 0.01)
				require.InDelta(t, 0.3, *result.FrequencyPenalty, 0.01)
				require.Equal(t, int64(42), *result.Seed)
				require.NotNil(t, result.Stop)
				require.Equal(t, []string{"END", "STOP"}, result.Stop.MultipleStop)
			},
		},
		{
			name: "request with single stop sequence",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					StopSequences: []string{"END"},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.NotNil(t, result.Stop)
				require.Equal(t, "END", *result.Stop.Stop)
			},
		},
		{
			name: "request with thinking config",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Solve this problem"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					MaxOutputTokens: 4096,
					ThinkingConfig: &ThinkingConfig{
						IncludeThoughts: true,
						ThinkingBudget:  lo.ToPtr(int64(8192)),
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Equal(t, "medium", result.ReasoningEffort)
			},
		},
		{
			name: "request with thinking config low budget",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Quick question"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					ThinkingConfig: &ThinkingConfig{
						IncludeThoughts: true,
						ThinkingBudget:  lo.ToPtr(int64(512)),
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Equal(t, "low", result.ReasoningEffort)
			},
		},
		{
			name: "request with thinking config high budget",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Complex problem"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					ThinkingConfig: &ThinkingConfig{
						IncludeThoughts: true,
						ThinkingBudget:  lo.ToPtr(int64(32768)),
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Equal(t, "high", result.ReasoningEffort)
			},
		},
		{
			name: "request with thinking config no budget",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Question"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					ThinkingConfig: &ThinkingConfig{
						IncludeThoughts: true,
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Equal(t, "medium", result.ReasoningEffort)
			},
		},
		{
			name: "request with thinking config and budget preservation",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Question"},
						},
					},
				},
				GenerationConfig: &GenerationConfig{
					ThinkingConfig: &ThinkingConfig{
						IncludeThoughts: true,
						ThinkingBudget:  lo.ToPtr(int64(5000)),
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Equal(t, "medium", result.ReasoningEffort)
				require.NotNil(t, result.ReasoningBudget)
				require.Equal(t, int64(5000), *result.ReasoningBudget)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertGeminiToLLMRequest(tt.input)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

func TestConvertGeminiToLLMRequest_Tools(t *testing.T) {
	tests := []struct {
		name     string
		input    *GenerateContentRequest
		validate func(t *testing.T, result *llm.Request)
	}{
		{
			name: "request with tools",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "What's the weather?"},
						},
					},
				},
				Tools: []*Tool{
					{
						FunctionDeclarations: []*FunctionDeclaration{
							{
								Name:        "get_weather",
								Description: "Get weather information",
								Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Tools, 1)
				require.Equal(t, "function", result.Tools[0].Type)
				require.Equal(t, "get_weather", result.Tools[0].Function.Name)
				require.Equal(t, "Get weather information", result.Tools[0].Function.Description)
			},
		},
		{
			name: "request with multiple tools",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Help me"},
						},
					},
				},
				Tools: []*Tool{
					{
						FunctionDeclarations: []*FunctionDeclaration{
							{
								Name:        "tool1",
								Description: "First tool",
							},
							{
								Name:        "tool2",
								Description: "Second tool",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Tools, 2)
				require.Equal(t, "tool1", result.Tools[0].Function.Name)
				require.Equal(t, "tool2", result.Tools[1].Function.Name)
			},
		},
		{
			name: "request with tool config AUTO",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
				},
				ToolConfig: &ToolConfig{
					FunctionCallingConfig: &FunctionCallingConfig{
						Mode: "AUTO",
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.NotNil(t, result.ToolChoice)
				require.Equal(t, "auto", *result.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with tool config NONE",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
				},
				ToolConfig: &ToolConfig{
					FunctionCallingConfig: &FunctionCallingConfig{
						Mode: "NONE",
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.NotNil(t, result.ToolChoice)
				require.Equal(t, "none", *result.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with tool config ANY",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
				},
				ToolConfig: &ToolConfig{
					FunctionCallingConfig: &FunctionCallingConfig{
						Mode: "ANY",
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.NotNil(t, result.ToolChoice)
				require.Equal(t, "required", *result.ToolChoice.ToolChoice)
			},
		},
		{
			name: "request with tool config ANY with specific function",
			input: &GenerateContentRequest{
				Contents: []*Content{
					{
						Role: "user",
						Parts: []*Part{
							{Text: "Test"},
						},
					},
				},
				ToolConfig: &ToolConfig{
					FunctionCallingConfig: &FunctionCallingConfig{
						Mode:                 "ANY",
						AllowedFunctionNames: []string{"specific_function"},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.NotNil(t, result.ToolChoice)
				require.NotNil(t, result.ToolChoice.NamedToolChoice)
				require.Equal(t, "function", result.ToolChoice.NamedToolChoice.Type)
				require.Equal(t, "specific_function", result.ToolChoice.NamedToolChoice.Function.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertGeminiToLLMRequest(tt.input)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

func TestConvertGeminiContentToLLMMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    *Content
		validate func(t *testing.T, result *llm.Message)
	}{
		{
			name:  "nil content",
			input: nil,
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Nil(t, result)
			},
		},
		{
			name: "empty parts",
			input: &Content{
				Role:  "user",
				Parts: []*Part{},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Nil(t, result)
			},
		},
		{
			name: "text content",
			input: &Content{
				Role: "user",
				Parts: []*Part{
					{Text: "Hello"},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "user", result.Role)
				require.Equal(t, "Hello", *result.Content.Content)
			},
		},
		{
			name: "model role conversion",
			input: &Content{
				Role: "model",
				Parts: []*Part{
					{Text: "Response"},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Equal(t, "assistant", result.Role)
			},
		},
		{
			name: "thinking content",
			input: &Content{
				Role: "model",
				Parts: []*Part{
					{Text: "Let me think...", Thought: true},
					{Text: "The answer is 42"},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.NotNil(t, result.ReasoningContent)
				require.Equal(t, "Let me think...", *result.ReasoningContent)
				require.Equal(t, "The answer is 42", *result.Content.Content)
			},
		},
		{
			name: "inline data (image)",
			input: &Content{
				Role: "user",
				Parts: []*Part{
					{
						InlineData: &Blob{
							MIMEType: "image/jpeg",
							Data:     "base64data",
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Len(t, result.Content.MultipleContent, 1)
				require.Equal(t, "image_url", result.Content.MultipleContent[0].Type)
				require.Equal(t, "data:image/jpeg;base64,base64data", result.Content.MultipleContent[0].ImageURL.URL)
			},
		},
		{
			name: "file data",
			input: &Content{
				Role: "user",
				Parts: []*Part{
					{
						FileData: &FileData{
							MIMEType: "image/png",
							FileURI:  "gs://bucket/file.png",
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Len(t, result.Content.MultipleContent, 1)
				require.Equal(t, "image_url", result.Content.MultipleContent[0].Type)
				require.Equal(t, "gs://bucket/file.png", result.Content.MultipleContent[0].ImageURL.URL)
			},
		},
		{
			name: "function call",
			input: &Content{
				Role: "model",
				Parts: []*Part{
					{
						FunctionCall: &FunctionCall{
							ID:   "call_123",
							Name: "get_weather",
							Args: map[string]any{"location": "NYC"},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Len(t, result.ToolCalls, 1)
				require.Equal(t, "call_123", result.ToolCalls[0].ID)
				require.Equal(t, "function", result.ToolCalls[0].Type)
				require.Equal(t, "get_weather", result.ToolCalls[0].Function.Name)
				require.Contains(t, result.ToolCalls[0].Function.Arguments, "NYC")
			},
		},
		{
			name: "function response",
			input: &Content{
				Role: "user",
				Parts: []*Part{
					{
						FunctionResponse: &FunctionResponse{
							ID:       "call_123",
							Name:     "get_weather",
							Response: map[string]any{"temperature": 72},
						},
					},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Equal(t, "tool", result.Role)
				require.Equal(t, "call_123", *result.ToolCallID)
				require.Contains(t, *result.Content.Content, "72")
			},
		},
		{
			name: "multiple text parts",
			input: &Content{
				Role: "user",
				Parts: []*Part{
					{Text: "First part"},
					{Text: "Second part"},
				},
			},
			validate: func(t *testing.T, result *llm.Message) {
				t.Helper()
				require.Len(t, result.Content.MultipleContent, 2)
				require.Equal(t, "text", result.Content.MultipleContent[0].Type)
				require.Equal(t, "First part", *result.Content.MultipleContent[0].Text)
				require.Equal(t, "Second part", *result.Content.MultipleContent[1].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertGeminiContentToLLMMessage(tt.input, nil)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

// =============================================================================
// Basic Tests for convertLLMToGeminiResponse
// =============================================================================

func TestConvertLLMToGeminiResponse_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    *llm.Response
		validate func(t *testing.T, result *GenerateContentResponse)
	}{
		{
			name: "simple response",
			input: &llm.Response{
				ID:    "resp_123",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Hello!"),
							},
						},
						FinishReason: lo.ToPtr("stop"),
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Equal(t, "resp_123", result.ResponseID)
				require.Equal(t, "gemini-2.5-flash", result.ModelVersion)
				require.Len(t, result.Candidates, 1)
				require.Equal(t, "model", result.Candidates[0].Content.Role)
				require.Len(t, result.Candidates[0].Content.Parts, 1)
				require.Equal(t, "Hello!", result.Candidates[0].Content.Parts[0].Text)
				require.Equal(t, "STOP", result.Candidates[0].FinishReason)
			},
		},
		{
			name: "response with thinking",
			input: &llm.Response{
				ID:    "resp_think",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role:             "assistant",
							ReasoningContent: lo.ToPtr("Let me think..."),
							Content: llm.MessageContent{
								Content: lo.ToPtr("The answer is 42"),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Len(t, result.Candidates[0].Content.Parts, 2)
				require.True(t, result.Candidates[0].Content.Parts[0].Thought)
				require.Equal(t, "Let me think...", result.Candidates[0].Content.Parts[0].Text)
				require.False(t, result.Candidates[0].Content.Parts[1].Thought)
				require.Equal(t, "The answer is 42", result.Candidates[0].Content.Parts[1].Text)
			},
		},
		{
			name: "response with tool calls",
			input: &llm.Response{
				ID:    "resp_tool",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
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
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Len(t, result.Candidates[0].Content.Parts, 2)
				require.Equal(t, "I'll check the weather", result.Candidates[0].Content.Parts[0].Text)
				require.NotNil(t, result.Candidates[0].Content.Parts[1].FunctionCall)
				require.Equal(t, "call_001", result.Candidates[0].Content.Parts[1].FunctionCall.ID)
				require.Equal(t, "get_weather", result.Candidates[0].Content.Parts[1].FunctionCall.Name)
				require.Equal(t, "NYC", result.Candidates[0].Content.Parts[1].FunctionCall.Args["location"])
			},
		},
		{
			name: "response with usage",
			input: &llm.Response{
				ID:    "resp_usage",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Response"),
							},
						},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
					PromptTokensDetails: &llm.PromptTokensDetails{
						CachedTokens: 20,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 30,
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.NotNil(t, result.UsageMetadata)
				require.Equal(t, int64(100), result.UsageMetadata.PromptTokenCount)
				require.Equal(t, int64(20), result.UsageMetadata.CandidatesTokenCount)
				require.Equal(t, int64(150), result.UsageMetadata.TotalTokenCount)
				require.Equal(t, int64(20), result.UsageMetadata.CachedContentTokenCount)
				require.Equal(t, int64(30), result.UsageMetadata.ThoughtsTokenCount)
			},
		},
		{
			name: "response with multiple content parts",
			input: &llm.Response{
				ID:    "resp_multi",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{Type: "text", Text: lo.ToPtr("First part")},
									{Type: "text", Text: lo.ToPtr("Second part")},
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Len(t, result.Candidates[0].Content.Parts, 2)
				require.Equal(t, "First part", result.Candidates[0].Content.Parts[0].Text)
				require.Equal(t, "Second part", result.Candidates[0].Content.Parts[1].Text)
			},
		},
		{
			name: "response with delta instead of message",
			input: &llm.Response{
				ID:    "resp_delta",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Streaming content"),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result *GenerateContentResponse) {
				t.Helper()
				require.Len(t, result.Candidates[0].Content.Parts, 1)
				require.Equal(t, "Streaming content", result.Candidates[0].Content.Parts[0].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLLMToGeminiResponse(tt.input, false)
			tt.validate(t, result)
		})
	}
}

func TestConvertLLMToGeminiResponse_FinishReasons(t *testing.T) {
	finishReasons := map[string]string{
		"stop":           "STOP",
		"length":         "MAX_TOKENS",
		"content_filter": "SAFETY",
		"tool_calls":     "STOP",
		"unknown":        "STOP",
	}

	for llmReason, expectedGeminiReason := range finishReasons {
		t.Run("finish_reason_"+llmReason, func(t *testing.T) {
			input := &llm.Response{
				ID:    "resp_finish",
				Model: "gemini-2.5-flash",
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: lo.ToPtr("Test"),
							},
						},
						FinishReason: lo.ToPtr(llmReason),
					},
				},
			}

			result := convertLLMToGeminiResponse(input, false)
			require.Equal(t, expectedGeminiReason, result.Candidates[0].FinishReason)
		})
	}
}

// =============================================================================
// Testdata Tests
// =============================================================================

func TestConvertGeminiToLLMRequest_Testdata(t *testing.T) {
	testCases := []struct {
		name         string
		geminiFile   string
		validateFunc func(t *testing.T, result *llm.Request)
	}{
		{
			name:       "simple request",
			geminiFile: "gemini-simple.request.json",
			validateFunc: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Messages, 1)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "Output 1-20, 5 each line", *result.Messages[0].Content.Content)
				require.Equal(t, int64(4096), *result.MaxTokens)
				require.Equal(t, "low", result.ReasoningEffort)
			},
		},
		{
			name:       "tools request",
			geminiFile: "gemini-tools.request.json",
			validateFunc: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Messages, 1)
				require.Equal(t, "What is the weather in San Francisco, CA?", *result.Messages[0].Content.Content)
				require.Len(t, result.Tools, 2)
				require.Equal(t, "get_coordinates", result.Tools[0].Function.Name)
				require.Equal(t, "get_weather", result.Tools[1].Function.Name)
			},
		},
		{
			name:       "thinking request",
			geminiFile: "gemini-thinking.request.json",
			validateFunc: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Messages, 3)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(t, "assistant", result.Messages[1].Role)
				require.NotNil(t, result.Messages[1].ReasoningContent)
				require.Contains(t, *result.Messages[1].ReasoningContent, "25 * 47")
				require.Equal(t, "user", result.Messages[2].Role)
				require.Equal(t, "medium", result.ReasoningEffort)
			},
		},
		{
			name:       "tool result request",
			geminiFile: "gemini-tool-result.request.json",
			validateFunc: func(t *testing.T, result *llm.Request) {
				t.Helper()
				require.Len(t, result.Messages, 3)
				require.Equal(t, "user", result.Messages[0].Role)
				require.Equal(
					t,
					"I need help with some calculations and weather information for my trip planning. What's 100 / 4 and what's the weather in Tokyo?",
					*result.Messages[0].Content.Content,
				)

				// Check assistant message with tool calls
				require.Equal(t, "assistant", result.Messages[1].Role)
				require.Equal(t, "I'll help you with both calculations and weather information for your trip planning.", *result.Messages[1].Content.Content)
				require.Len(t, result.Messages[1].ToolCalls, 2)
				require.Equal(t, "call_00_IMEgeiAgajAZ47qX9hzSnjBP", result.Messages[1].ToolCalls[0].ID)
				require.Equal(t, "calculate", result.Messages[1].ToolCalls[0].Function.Name)
				require.Equal(t, "call_01_nyJz54P3fg9880GPr8O2QvER", result.Messages[1].ToolCalls[1].ID)
				require.Equal(t, "get_current_weather", result.Messages[1].ToolCalls[1].Function.Name)

				// Check tool response message with ID completion
				require.Equal(t, "tool", result.Messages[2].Role)
				require.Equal(t, "call_00_IMEgeiAgajAZ47qX9hzSnjBP", *result.Messages[2].ToolCallID)
				require.Equal(t, "calculate", *result.Messages[2].ToolCallName)
				require.Contains(t, *result.Messages[2].Content.Content, "25")

				// Check tools
				require.Len(t, result.Tools, 2)
				require.Equal(t, "calculate", result.Tools[0].Function.Name)
				require.Equal(t, "get_current_weather", result.Tools[1].Function.Name)

				// Check temperature
				require.InDelta(t, 0.7, *result.Temperature, 0.01)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var geminiReq GenerateContentRequest

			err := xtest.LoadTestData(t, tc.geminiFile, &geminiReq)
			require.NoError(t, err)

			result, err := convertGeminiToLLMRequest(&geminiReq)
			require.NoError(t, err)
			tc.validateFunc(t, result)
		})
	}
}

func TestConvertGeminiToLLMResponse_Testdata(t *testing.T) {
	testCases := []struct {
		name         string
		geminiFile   string
		validateFunc func(t *testing.T, result *llm.Response)
	}{
		{
			name:       "simple response",
			geminiFile: "gemini-simple.response.json",
			validateFunc: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "G34qaY30KYSk0-kPkIX5UA", result.ID)
				require.Equal(t, "gemini-2.5-flash", result.Model)
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].Message.ReasoningContent)
				require.Contains(t, *result.Choices[0].Message.ReasoningContent, "Organizing Numbers")
				require.Contains(t, *result.Choices[0].Message.Content.Content, "1 2 3 4 5")
			},
		},
		{
			name:       "tools response",
			geminiFile: "gemini-tools.response.json",
			validateFunc: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "tools-response-001", result.ID)
				require.Len(t, result.Choices, 1)
				require.Len(t, result.Choices[0].Message.ToolCalls, 1)
				require.Equal(t, "get_coordinates", result.Choices[0].Message.ToolCalls[0].Function.Name)
			},
		},
		{
			name:       "thinking response",
			geminiFile: "gemini-thinking.response.json",
			validateFunc: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "thinking-response-001", result.ID)
				require.Len(t, result.Choices, 1)
				require.NotNil(t, result.Choices[0].Message.ReasoningContent)
				require.Contains(t, *result.Choices[0].Message.ReasoningContent, "1175 by 3")
				require.Contains(t, *result.Choices[0].Message.Content.Content, "3525")
				require.NotNil(t, result.Usage)
				require.NotNil(t, result.Usage.CompletionTokensDetails)
				require.Equal(t, int64(100), result.Usage.CompletionTokensDetails.ReasoningTokens)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var geminiResp GenerateContentResponse

			err := xtest.LoadTestData(t, tc.geminiFile, &geminiResp)
			require.NoError(t, err)

			result := convertGeminiToLLMResponse(&geminiResp, false)
			tc.validateFunc(t, result)
		})
	}
}

// =============================================================================
// Round-trip Tests
// =============================================================================

func TestRoundTrip_GeminiRequest_ToLLM_BackToGemini(t *testing.T) {
	testCases := []struct {
		name       string
		geminiFile string
	}{
		{
			name:       "simple request round trip",
			geminiFile: "gemini-simple.request.json",
		},
		{
			name:       "tools request round trip",
			geminiFile: "gemini-tools.request.json",
		},
		{
			name:       "thinking request round trip",
			geminiFile: "gemini-thinking.request.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var originalGemini GenerateContentRequest

			err := xtest.LoadTestData(t, tc.geminiFile, &originalGemini)
			require.NoError(t, err)

			// Convert Gemini -> LLM
			llmReq, err := convertGeminiToLLMRequest(&originalGemini)
			require.NoError(t, err)

			// Convert LLM -> Gemini
			convertedGemini := convertLLMToGeminiRequest(llmReq)

			// Verify key fields are preserved
			require.Equal(t, len(originalGemini.Contents), len(convertedGemini.Contents))

			// Verify system instruction
			if originalGemini.SystemInstruction != nil {
				require.NotNil(t, convertedGemini.SystemInstruction)
			}

			// Verify tools
			if len(originalGemini.Tools) > 0 {
				require.NotEmpty(t, convertedGemini.Tools)

				originalToolCount := 0
				for _, tool := range originalGemini.Tools {
					originalToolCount += len(tool.FunctionDeclarations)
				}

				convertedToolCount := 0
				for _, tool := range convertedGemini.Tools {
					convertedToolCount += len(tool.FunctionDeclarations)
				}

				require.Equal(t, originalToolCount, convertedToolCount)
			}

			// Verify generation config
			if originalGemini.GenerationConfig != nil {
				require.NotNil(t, convertedGemini.GenerationConfig)

				if originalGemini.GenerationConfig.MaxOutputTokens > 0 {
					require.Equal(t, originalGemini.GenerationConfig.MaxOutputTokens, convertedGemini.GenerationConfig.MaxOutputTokens)
				}
			}
		})
	}
}

func TestRoundTrip_GeminiResponse_ToLLM_BackToGemini(t *testing.T) {
	testCases := []struct {
		name       string
		geminiFile string
	}{
		{
			name:       "simple response round trip",
			geminiFile: "gemini-simple.response.json",
		},
		{
			name:       "tools response round trip",
			geminiFile: "gemini-tools.response.json",
		},
		{
			name:       "thinking response round trip",
			geminiFile: "gemini-thinking.response.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load original Gemini response
			data, err := os.ReadFile(filepath.Join("testdata", tc.geminiFile))
			require.NoError(t, err)

			var originalGemini GenerateContentResponse

			err = json.Unmarshal(data, &originalGemini)
			require.NoError(t, err)

			// Convert Gemini -> LLM (non-streaming)
			llmResp := convertGeminiToLLMResponse(&originalGemini, false)

			// Convert LLM -> Gemini (non-streaming)
			convertedGemini := convertLLMToGeminiResponse(llmResp, false)

			// Verify key fields are preserved
			require.Equal(t, originalGemini.ResponseID, convertedGemini.ResponseID)
			require.Equal(t, originalGemini.ModelVersion, convertedGemini.ModelVersion)
			require.Equal(t, len(originalGemini.Candidates), len(convertedGemini.Candidates))

			// Verify candidate content
			for i, originalCandidate := range originalGemini.Candidates {
				convertedCandidate := convertedGemini.Candidates[i]
				require.Equal(t, originalCandidate.Index, convertedCandidate.Index)

				if originalCandidate.Content != nil {
					require.NotNil(t, convertedCandidate.Content)
					require.Equal(t, "model", convertedCandidate.Content.Role)
				}
			}

			// Verify usage metadata
			if originalGemini.UsageMetadata != nil {
				require.NotNil(t, convertedGemini.UsageMetadata)
				require.Equal(t, originalGemini.UsageMetadata.PromptTokenCount, convertedGemini.UsageMetadata.PromptTokenCount)
				require.Equal(t, originalGemini.UsageMetadata.TotalTokenCount, convertedGemini.UsageMetadata.TotalTokenCount)
			}
		})
	}
}

func TestRoundTrip_LLMRequest_ToGemini_BackToLLM(t *testing.T) {
	testCases := []struct {
		name    string
		llmFile string
	}{
		{
			name:    "simple request round trip",
			llmFile: "llm-simple.request.json",
		},
		{
			name:    "tools request round trip",
			llmFile: "llm-tools.request.json",
		},
		{
			name:    "thinking request round trip",
			llmFile: "llm-thinking.request.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load original LLM request
			data, err := os.ReadFile(filepath.Join("testdata", tc.llmFile))
			require.NoError(t, err)

			var originalLLM llm.Request

			err = json.Unmarshal(data, &originalLLM)
			require.NoError(t, err)

			// Convert LLM -> Gemini
			geminiReq := convertLLMToGeminiRequest(&originalLLM)

			// Convert Gemini -> LLM
			convertedLLM, err := convertGeminiToLLMRequest(geminiReq)
			require.NoError(t, err)

			// Verify key fields are preserved
			require.Equal(t, len(originalLLM.Messages), len(convertedLLM.Messages))

			// Verify max tokens
			if originalLLM.MaxTokens != nil {
				require.NotNil(t, convertedLLM.MaxTokens)
				require.Equal(t, *originalLLM.MaxTokens, *convertedLLM.MaxTokens)
			}

			// Verify tools
			require.Equal(t, len(originalLLM.Tools), len(convertedLLM.Tools))

			for i, originalTool := range originalLLM.Tools {
				require.Equal(t, originalTool.Function.Name, convertedLLM.Tools[i].Function.Name)
				require.Equal(t, originalTool.Function.Description, convertedLLM.Tools[i].Function.Description)
			}

			// Verify message roles
			for i, originalMsg := range originalLLM.Messages {
				require.Equal(t, originalMsg.Role, convertedLLM.Messages[i].Role)
			}
		})
	}
}

func TestRoundTrip_LLMResponse_ToGemini_BackToLLM(t *testing.T) {
	testCases := []struct {
		name    string
		llmFile string
	}{
		{
			name:    "simple response round trip",
			llmFile: "llm-simple.response.json",
		},
		{
			name:    "tools response round trip",
			llmFile: "llm-tools.response.json",
		},
		{
			name:    "thinking response round trip",
			llmFile: "llm-thinking.response.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load original LLM response
			data, err := os.ReadFile(filepath.Join("testdata", tc.llmFile))
			require.NoError(t, err)

			var originalLLM llm.Response

			err = json.Unmarshal(data, &originalLLM)
			require.NoError(t, err)

			// Convert LLM -> Gemini (non-streaming)
			geminiResp := convertLLMToGeminiResponse(&originalLLM, false)

			// Convert Gemini -> LLM (non-streaming)
			convertedLLM := convertGeminiToLLMResponse(geminiResp, false)

			// Verify key fields are preserved
			require.Equal(t, originalLLM.ID, convertedLLM.ID)
			require.Equal(t, originalLLM.Model, convertedLLM.Model)
			require.Equal(t, len(originalLLM.Choices), len(convertedLLM.Choices))

			// Verify choice content
			for i, originalChoice := range originalLLM.Choices {
				convertedChoice := convertedLLM.Choices[i]
				require.Equal(t, originalChoice.Index, convertedChoice.Index)

				if originalChoice.Message != nil {
					require.NotNil(t, convertedChoice.Message)
					require.Equal(t, "assistant", convertedChoice.Message.Role)

					// Verify tool calls
					require.Equal(t, len(originalChoice.Message.ToolCalls), len(convertedChoice.Message.ToolCalls))

					for j, originalToolCall := range originalChoice.Message.ToolCalls {
						require.Equal(t, originalToolCall.Function.Name, convertedChoice.Message.ToolCalls[j].Function.Name)
					}
				}
			}

			// Verify usage
			if originalLLM.Usage != nil {
				require.NotNil(t, convertedLLM.Usage)
				require.Equal(t, originalLLM.Usage.PromptTokens, convertedLLM.Usage.PromptTokens)
				require.Equal(t, originalLLM.Usage.TotalTokens, convertedLLM.Usage.TotalTokens)
			}
		})
	}
}
