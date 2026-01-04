package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
)

func TestApplyTransformOptions_NilSettings(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: strPtr("Hello"),
				},
			},
		},
	}

	result := applyTransformOptions(req, nil)

	assert.Same(t, req, result)
}

func TestApplyTransformOptions_NilTransformOptions(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{},
	}

	result := applyTransformOptions(req, settings)

	assert.Same(t, req, result)
}

func TestApplyTransformOptions_NoForceOptions(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: false,
			ForceArrayInputs:       false,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.Same(t, req, result)
}

func TestApplyTransformOptions_ForceArrayInstructions(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: strPtr("Hello"),
				},
			},
		},
		Temperature: float64Ptr(0.7),
		MaxTokens:   int64Ptr(1000),
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
			ForceArrayInputs:       false,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, req.Model, result.Model)
	assert.Equal(t, req.Temperature, result.Temperature)
	assert.Equal(t, req.MaxTokens, result.MaxTokens)
	assert.Equal(t, len(req.Messages), len(result.Messages))

	assert.Equal(t, "system", result.Messages[0].Role)
	assert.NotNil(t, result.Messages[0].Content.Content)
	assert.Nil(t, result.Messages[0].Content.MultipleContent)
	assert.Equal(t, "You are a helpful assistant", *result.Messages[0].Content.Content)

	assert.Equal(t, "user", result.Messages[1].Role)
	assert.NotNil(t, result.Messages[1].Content.Content)
	assert.Nil(t, result.Messages[1].Content.MultipleContent)
	assert.Equal(t, "Hello", *result.Messages[1].Content.Content)

	assert.NotNil(t, result.TransformOptions.ArrayInstructions)
	assert.True(t, *result.TransformOptions.ArrayInstructions)
	assert.Nil(t, result.TransformOptions.ArrayInputs)
}

func TestApplyTransformOptions_ForceArrayInputs(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: strPtr("Hello"),
				},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: false,
			ForceArrayInputs:       true,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, len(req.Messages), len(result.Messages))

	assert.Equal(t, "system", result.Messages[0].Role)
	assert.NotNil(t, result.Messages[0].Content.Content)
	assert.Nil(t, result.Messages[0].Content.MultipleContent)
	assert.Equal(t, "You are a helpful assistant", *result.Messages[0].Content.Content)

	assert.Equal(t, "user", result.Messages[1].Role)
	assert.NotNil(t, result.Messages[1].Content.Content)
	assert.Nil(t, result.Messages[1].Content.MultipleContent)
	assert.Equal(t, "Hello", *result.Messages[1].Content.Content)

	assert.Nil(t, result.TransformOptions.ArrayInstructions)
	assert.NotNil(t, result.TransformOptions.ArrayInputs)
	assert.True(t, *result.TransformOptions.ArrayInputs)
}

func TestApplyTransformOptions_ForceBoth(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: strPtr("Hello"),
				},
			},
			{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: strPtr("Hi there!"),
				},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
			ForceArrayInputs:       true,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, len(req.Messages), len(result.Messages))

	assert.Equal(t, "system", result.Messages[0].Role)
	assert.NotNil(t, result.Messages[0].Content.Content)
	assert.Nil(t, result.Messages[0].Content.MultipleContent)
	assert.Equal(t, "You are a helpful assistant", *result.Messages[0].Content.Content)

	assert.Equal(t, "user", result.Messages[1].Role)
	assert.NotNil(t, result.Messages[1].Content.Content)
	assert.Nil(t, result.Messages[1].Content.MultipleContent)
	assert.Equal(t, "Hello", *result.Messages[1].Content.Content)

	assert.Equal(t, "assistant", result.Messages[2].Role)
	assert.NotNil(t, result.Messages[2].Content.Content)
	assert.Nil(t, result.Messages[2].Content.MultipleContent)

	assert.NotNil(t, result.TransformOptions.ArrayInstructions)
	assert.True(t, *result.TransformOptions.ArrayInstructions)
	assert.NotNil(t, result.TransformOptions.ArrayInputs)
	assert.True(t, *result.TransformOptions.ArrayInputs)
}

func TestApplyTransformOptions_ExistingMultipleContent(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: strPtr("Instruction 1"),
						},
						{
							Type: "text",
							Text: strPtr("Instruction 2"),
						},
					},
				},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, len(req.Messages), len(result.Messages))

	assert.Equal(t, "system", result.Messages[0].Role)
	assert.Nil(t, result.Messages[0].Content.Content)
	assert.NotNil(t, result.Messages[0].Content.MultipleContent)
	assert.Len(t, result.Messages[0].Content.MultipleContent, 2)
	assert.Equal(t, "Instruction 1", *result.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "Instruction 2", *result.Messages[0].Content.MultipleContent[1].Text)

	assert.NotNil(t, result.TransformOptions.ArrayInstructions)
	assert.True(t, *result.TransformOptions.ArrayInstructions)
}

func TestApplyTransformOptions_PreservesToolCalls(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
			{
				Role: "assistant",
				Content: llm.MessageContent{
					Content: strPtr("Let me help"),
				},
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location": "NYC"}`,
						},
					},
				},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, len(req.Messages), len(result.Messages))

	assert.Equal(t, "system", result.Messages[0].Role)
	assert.NotNil(t, result.Messages[0].Content.Content)
	assert.Nil(t, result.Messages[0].Content.MultipleContent)
	assert.Equal(t, "You are a helpful assistant", *result.Messages[0].Content.Content)

	assert.Equal(t, "assistant", result.Messages[1].Role)
	assert.NotNil(t, result.Messages[1].Content.Content)
	assert.Nil(t, result.Messages[1].Content.MultipleContent)
	assert.Equal(t, "Let me help", *result.Messages[1].Content.Content)
	assert.Len(t, result.Messages[1].ToolCalls, 1)
	assert.Equal(t, "call_123", result.Messages[1].ToolCalls[0].ID)
	assert.Equal(t, "get_weather", result.Messages[1].ToolCalls[0].Function.Name)
	assert.Equal(t, `{"location": "NYC"}`, result.Messages[1].ToolCalls[0].Function.Arguments)

	assert.NotNil(t, result.TransformOptions.ArrayInstructions)
	assert.True(t, *result.TransformOptions.ArrayInstructions)
}

func TestApplyTransformOptions_PreservesAllFields(t *testing.T) {
	req := &llm.Request{
		Model:               "gpt-4",
		Temperature:         float64Ptr(0.7),
		TopP:                float64Ptr(0.9),
		MaxTokens:           int64Ptr(1000),
		MaxCompletionTokens: int64Ptr(2000),
		Stream:              boolPtr(true),
		FrequencyPenalty:    float64Ptr(0.1),
		PresencePenalty:     float64Ptr(0.2),
		Seed:                int64Ptr(42),
		Stop:                &llm.Stop{Stop: strPtr("\n")},
		TopLogprobs:         int64Ptr(5),
		Logprobs:            boolPtr(true),
		ParallelToolCalls:   boolPtr(true),
		ResponseFormat:      &llm.ResponseFormat{Type: "json_object"},
		Verbosity:           strPtr("medium"),
		ReasoningEffort:     "high",
		ReasoningBudget:     int64Ptr(5000),
		ServiceTier:         strPtr("standard"),
		Store:               boolPtr(false),
		SafetyIdentifier:    strPtr("user-123"),
		User:                strPtr("user-456"),
		PromptCacheKey:      boolPtr(true),
		Metadata:            map[string]string{"key": "value"},
		LogitBias:           map[string]int64{"123": -10},
		Modalities:          []string{"text", "audio"},
		Messages: []llm.Message{
			{
				Role: "system",
				Content: llm.MessageContent{
					Content: strPtr("You are a helpful assistant"),
				},
			},
		},
		TransformOptions: llm.TransformOptions{
			ArrayInstructions: boolPtr(false),
			ArrayInputs:       boolPtr(false),
		},
		TransformerMetadata: map[string]any{"key": "value"},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, req.Model, result.Model)
	assert.Equal(t, req.Temperature, result.Temperature)
	assert.Equal(t, req.TopP, result.TopP)
	assert.Equal(t, req.MaxTokens, result.MaxTokens)
	assert.Equal(t, req.MaxCompletionTokens, result.MaxCompletionTokens)
	assert.Equal(t, req.Stream, result.Stream)
	assert.Equal(t, req.FrequencyPenalty, result.FrequencyPenalty)
	assert.Equal(t, req.PresencePenalty, result.PresencePenalty)
	assert.Equal(t, req.Seed, result.Seed)
	assert.Equal(t, req.Stop, result.Stop)
	assert.Equal(t, req.TopLogprobs, result.TopLogprobs)
	assert.Equal(t, req.Logprobs, result.Logprobs)
	assert.Equal(t, req.ParallelToolCalls, result.ParallelToolCalls)
	assert.Equal(t, req.ResponseFormat, result.ResponseFormat)
	assert.Equal(t, req.Verbosity, result.Verbosity)
	assert.Equal(t, req.ReasoningEffort, result.ReasoningEffort)
	assert.Equal(t, req.ReasoningBudget, result.ReasoningBudget)
	assert.Equal(t, req.ServiceTier, result.ServiceTier)
	assert.Equal(t, req.Store, result.Store)
	assert.Equal(t, req.SafetyIdentifier, result.SafetyIdentifier)
	assert.Equal(t, req.User, result.User)
	assert.Equal(t, req.PromptCacheKey, result.PromptCacheKey)
	assert.Equal(t, req.Metadata, result.Metadata)
	assert.Equal(t, req.LogitBias, result.LogitBias)
	assert.Equal(t, req.Modalities, result.Modalities)
	assert.Equal(t, req.TransformerMetadata, result.TransformerMetadata)

	assert.Equal(t, len(req.Messages), len(result.Messages))
	assert.Equal(t, req.Messages[0].Role, result.Messages[0].Role)
	assert.Equal(t, req.Messages[0].Content, result.Messages[0].Content)

	assert.NotNil(t, result.TransformOptions.ArrayInstructions)
	assert.True(t, *result.TransformOptions.ArrayInstructions)
}

func TestApplyTransformOptions_EmptyContent(t *testing.T) {
	req := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: llm.MessageContent{},
			},
		},
	}

	settings := &objects.ChannelSettings{
		TransformOptions: objects.TransformOptions{
			ForceArrayInstructions: true,
		},
	}

	result := applyTransformOptions(req, settings)

	assert.NotSame(t, req, result)
	assert.Equal(t, len(req.Messages), len(result.Messages))
	assert.Equal(t, req.Messages[0].Content, result.Messages[0].Content)

	assert.NotNil(t, result.TransformOptions.ArrayInstructions)
	assert.True(t, *result.TransformOptions.ArrayInstructions)
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func float64Ptr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}
