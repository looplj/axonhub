package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestAggregateStreamChunks(t *testing.T) {
	tests := []struct {
		name         string
		streamFile   string
		responseFile string
	}{
		{
			name:         "openai stream chunks with stop finish reason",
			streamFile:   "openai-stop.stream.jsonl",
			responseFile: "openai-stop.response.json",
		},
		{
			name:         "openai stream chunks with tool calls",
			streamFile:   "openai-tool.stream.jsonl",
			responseFile: "openai-tool.response.json",
		},
		{
			name:         "openai stream chunks with parallel multiple tool calls",
			streamFile:   "openai-parallel_multiple_tool.stream.jsonl",
			responseFile: "openai-parallel_multiple_tool.response.json",
		},
		{
			name:         "openai stream chunks with tool calls (tool_2)",
			streamFile:   "openai-tool_2.stream.jsonl",
			responseFile: "openai-tool_2.response.json",
		},
		{
			name:         "openai stream chunks with multiple choice tool calls",
			streamFile:   "openai-multiple_choice_tool.stream.jsonl",
			responseFile: "openai-multiple_choice_tool.response.json",
		},
		{
			name:         "openai stream chunks with multiple choice tool calls (tool_2)",
			streamFile:   "openai-multiple_choice_tool_2.stream.jsonl",
			responseFile: "openai-multiple_choice_tool_2.response.json",
		},
		{
			name:         "openai stream chunks with multiple choice tool calls (tool_3)",
			streamFile:   "openai-multiple_choice_tool_3.stream.jsonl",
			responseFile: "openai-multiple_choice_tool_3.response.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load test data
			chunks, err := xtest.LoadStreamChunks(tt.streamFile)
			require.NoError(t, err)

			// Load expected response
			var want llm.Response

			err = xtest.LoadTestData(tt.responseFile, &want)
			require.NoError(t, err)

			// Test the function
			gotBytes, err := AggregateStreamChunks(context.Background(), chunks)
			require.NoError(t, err)

			// Parse the result
			var got llm.Response

			err = json.Unmarshal(gotBytes, &got)
			require.NoError(t, err)

			// Assert the result
			assert.Equal(t, want.ID, got.ID)
			assert.Equal(t, want.Model, got.Model)
			assert.Equal(t, want.Object, got.Object)
			assert.Equal(t, want.Created, got.Created)
			assert.Equal(t, want.SystemFingerprint, got.SystemFingerprint)
			assert.Len(t, got.Choices, len(want.Choices))

			// Check all choices
			for i, wantChoice := range want.Choices {
				require.Less(t, i, len(got.Choices), "Missing choice at index %d", i)
				gotChoice := got.Choices[i]

				assert.Equal(t, wantChoice.Index, gotChoice.Index)
				assert.Equal(t, wantChoice.Message.Role, gotChoice.Message.Role)

				// Check content
				if wantChoice.Message.Content.Content != nil {
					require.NotNil(t, gotChoice.Message.Content.Content)
					assert.Equal(t, *wantChoice.Message.Content.Content, *gotChoice.Message.Content.Content)
				}

				// Check tool calls
				if len(wantChoice.Message.ToolCalls) > 0 {
					require.Len(t, gotChoice.Message.ToolCalls, len(wantChoice.Message.ToolCalls))

					for j, wantToolCall := range wantChoice.Message.ToolCalls {
						gotToolCall := gotChoice.Message.ToolCalls[j]
						assert.Equal(t, wantToolCall.ID, gotToolCall.ID)
						assert.Equal(t, wantToolCall.Type, gotToolCall.Type)
						assert.Equal(t, wantToolCall.Function.Name, gotToolCall.Function.Name)
						assert.Equal(t, wantToolCall.Function.Arguments, gotToolCall.Function.Arguments)
					}
				}

				// Check finish reason
				if wantChoice.FinishReason != nil {
					require.NotNil(t, gotChoice.FinishReason)
					assert.Equal(t, *wantChoice.FinishReason, *gotChoice.FinishReason)
				}
			}

			// Check usage
			if want.Usage != nil {
				require.NotNil(t, got.Usage)
				assert.Equal(t, want.Usage.PromptTokens, got.Usage.PromptTokens)
				assert.Equal(t, want.Usage.CompletionTokens, got.Usage.CompletionTokens)
				assert.Equal(t, want.Usage.TotalTokens, got.Usage.TotalTokens)
			}
		})
	}
}

func TestAggregateStreamChunks_EmptyChunks(t *testing.T) {
	gotBytes, err := AggregateStreamChunks(context.Background(), nil)
	require.NoError(t, err)

	var got llm.Response

	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	assert.Equal(t, llm.Response{}, got)
}
