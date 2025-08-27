package openai

import (
	"context"
	"encoding/json"
	"testing"

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
		{
			name:         "deepseek reasoning stream chunks with stop finish reason",
			streamFile:   "deepseek-reasoninig.stream.jsonl",
			responseFile: "deepseek-reasoning.response.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load test data
			chunks, err := xtest.LoadStreamChunks(t, tt.streamFile)
			require.NoError(t, err)

			// Load expected response
			var want llm.Response

			err = xtest.LoadTestData(t, tt.responseFile, &want)
			require.NoError(t, err)

			// Test the function
			gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks)
			require.NoError(t, err)

			// Parse the result
			var got llm.Response

			err = json.Unmarshal(gotBytes, &got)
			require.NoError(t, err)

			// Assert the result
			require.Equal(t, want.ID, got.ID)
			require.Equal(t, want.Model, got.Model)
			require.Equal(t, want.Object, got.Object)
			require.Equal(t, want.Created, got.Created)
			require.Equal(t, want.SystemFingerprint, got.SystemFingerprint)
			require.Len(t, got.Choices, len(want.Choices))

			// Check all choices
			for i, wantChoice := range want.Choices {
				require.Less(t, i, len(got.Choices), "Missing choice at index %d", i)
				gotChoice := got.Choices[i]

				require.Equal(t, wantChoice.Index, gotChoice.Index)
				require.Equal(t, wantChoice.Message.Role, gotChoice.Message.Role)

				// Check content
				if wantChoice.Message.Content.Content != nil {
					require.NotNil(t, gotChoice.Message.Content.Content)
					require.Equal(t, *wantChoice.Message.Content.Content, *gotChoice.Message.Content.Content)
				}

				if wantChoice.Message.ReasoningContent != nil {
					require.NotNil(t, gotChoice.Message.ReasoningContent)
					require.Equal(t, *wantChoice.Message.ReasoningContent, *gotChoice.Message.ReasoningContent)
				}

				// Check tool calls
				if len(wantChoice.Message.ToolCalls) > 0 {
					require.Len(t, gotChoice.Message.ToolCalls, len(wantChoice.Message.ToolCalls))

					for j, wantToolCall := range wantChoice.Message.ToolCalls {
						gotToolCall := gotChoice.Message.ToolCalls[j]
						require.Equal(t, wantToolCall.ID, gotToolCall.ID)
						require.Equal(t, wantToolCall.Type, gotToolCall.Type)
						require.Equal(t, wantToolCall.Function.Name, gotToolCall.Function.Name)
						require.Equal(t, wantToolCall.Function.Arguments, gotToolCall.Function.Arguments)
					}
				}

				// Check finish reason
				if wantChoice.FinishReason != nil {
					require.NotNil(t, gotChoice.FinishReason)
					require.Equal(t, *wantChoice.FinishReason, *gotChoice.FinishReason)
				}
			}

			// Check usage
			if want.Usage != nil {
				require.NotNil(t, got.Usage)
				require.Equal(t, want.Usage.PromptTokens, got.Usage.PromptTokens)
				require.Equal(t, want.Usage.CompletionTokens, got.Usage.CompletionTokens)
				require.Equal(t, want.Usage.TotalTokens, got.Usage.TotalTokens)
			}
		})
	}
}

func TestAggregateStreamChunks_EmptyChunks(t *testing.T) {
	gotBytes, _, err := AggregateStreamChunks(context.Background(), nil)
	require.NoError(t, err)

	var got llm.Response

	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	require.Equal(t, llm.Response{}, got)
}
