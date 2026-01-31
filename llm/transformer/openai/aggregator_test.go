package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
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
			gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
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
				} else {
					// When expected content is nil, the field should be omitted due to omitzero
					// So we check that the actual content is also nil/empty
					if gotChoice.Message.Content.Content != nil {
						require.Equal(t, "", *gotChoice.Message.Content.Content)
					}
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
	gotBytes, _, err := AggregateStreamChunks(context.Background(), nil, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response

	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	require.Equal(t, llm.Response{}, got)
}

func TestAggregateStreamChunks_WithCitations(t *testing.T) {
	// Create chunks with citations spread across them
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama-3.1-sonar-small-128k-online","choices":[{"index":0,"delta":{"role":"assistant","content":"The meaning"}}],"citations":["https://example.com/source1" ]}`),
		},
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama-3.1-sonar-small-128k-online","choices":[{"index":0,"delta":{"content":" of life"}}],"citations":["https://example.com/source2" ]}`),
		},
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama-3.1-sonar-small-128k-online","choices":[{"index":0,"delta":{"content":" is..."},"finish_reason":"stop"}],"citations":["https://example.com/source1" ]}`),
		},
	}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	// Verify the aggregated content
	require.Equal(t, "chatcmpl-123", got.ID)
	require.Equal(t, "chat.completion", got.Object)
	require.Len(t, got.Choices, 1)
	require.NotNil(t, got.Choices[0].Message)
	require.Equal(t, "assistant", got.Choices[0].Message.Role)
	require.NotNil(t, got.Choices[0].Message.Content.Content)
	require.Equal(t, "The meaning of life is...", *got.Choices[0].Message.Content.Content)

	// Verify citations are aggregated and deduplicated
	require.NotNil(t, got.TransformerMetadata)
	citationsRaw, ok := got.TransformerMetadata[TransformerMetadataKeyCitations]
	require.True(t, ok)

	// After JSON marshaling/unmarshaling, the citations will be []interface{}
	citationsSlice, ok := citationsRaw.([]interface{})
	require.True(t, ok)
	require.Len(t, citationsSlice, 2)

	// Convert to []string for easier assertion
	citations := make([]string, len(citationsSlice))
	for i, v := range citationsSlice {
		citations[i] = v.(string)
	}
	require.Contains(t, citations, "https://example.com/source1")
	require.Contains(t, citations, "https://example.com/source2")
}

func TestAggregateStreamChunks_WithoutCitations(t *testing.T) {
	// Create chunks without citations
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`),
		},
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}]}`),
		},
	}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	// Verify no citations in metadata
	require.Nil(t, got.TransformerMetadata)
}

func TestAggregateStreamChunks_WithAnnotations(t *testing.T) {
	// Create chunks with annotations in the Message field (as seen in Perplexity responses)
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"sonar-deep-research","choices":[{"index":0,"delta":{"role":"assistant","content":"The meaning"},"message":{"role":"assistant","content":"The meaning","annotations":[{"type":"url_citation","url_citation":{"url":"https://en.wikipedia.org/wiki/Meaning_of_life","title":"Meaning of life - Wikipedia"}}]}}]}`),
		},
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"sonar-deep-research","choices":[{"index":0,"delta":{"content":" of life"},"message":{"role":"assistant","content":"The meaning of life","annotations":[{"type":"url_citation","url_citation":{"url":"https://plato.stanford.edu/entries/life-meaning/","title":"Stanford Encyclopedia"}}]}}]}`),
		},
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"sonar-deep-research","choices":[{"index":0,"delta":{"content":" is..."},"finish_reason":"stop"}]}`),
		},
	}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	// Verify the aggregated content
	require.Equal(t, "chatcmpl-123", got.ID)
	require.Equal(t, "chat.completion", got.Object)
	require.Len(t, got.Choices, 1)
	require.NotNil(t, got.Choices[0].Message)
	require.Equal(t, "assistant", got.Choices[0].Message.Role)
	require.NotNil(t, got.Choices[0].Message.Content.Content)
	require.Equal(t, "The meaning of life is...", *got.Choices[0].Message.Content.Content)

	// Verify annotations are aggregated and deduplicated
	require.Len(t, got.Choices[0].Message.Annotations, 2)

	// Check first annotation
	require.Equal(t, "url_citation", got.Choices[0].Message.Annotations[0].Type)
	require.NotNil(t, got.Choices[0].Message.Annotations[0].URLCitation)

	// Check second annotation
	require.Equal(t, "url_citation", got.Choices[0].Message.Annotations[1].Type)
	require.NotNil(t, got.Choices[0].Message.Annotations[1].URLCitation)
}

func TestAggregateStreamChunks_WithAnnotationsInMessage(t *testing.T) {
	// Test annotations that come in the Message field (non-streaming style chunks)
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"sonar-deep-research","choices":[{"index":0,"message":{"role":"assistant","content":"The meaning of life...","annotations":[{"type":"url_citation","url_citation":{"url":"https://example.com/source1","title":"Source 1"}}]}}]}`),
		},
	}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	// Verify annotations from Message field are captured
	require.Len(t, got.Choices, 1)
	require.NotNil(t, got.Choices[0].Message)
	require.Len(t, got.Choices[0].Message.Annotations, 1)
	require.Equal(t, "url_citation", got.Choices[0].Message.Annotations[0].Type)
	require.NotNil(t, got.Choices[0].Message.Annotations[0].URLCitation)
	require.Equal(t, "https://example.com/source1", got.Choices[0].Message.Annotations[0].URLCitation.URL)
}

func TestAggregateStreamChunks_WithInvalidAnnotations(t *testing.T) {
	// Test that annotations with nil URLCitation or empty URL are skipped
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"sonar-deep-research","choices":[{"index":0,"message":{"role":"assistant","content":"Test content","annotations":[{"type":"url_citation","url_citation":null},{"type":"url_citation","url_citation":{"url":"","title":"Empty URL"}},{"type":"url_citation","url_citation":{"url":"https://example.com/valid","title":"Valid Source"}}]}}]}`),
		},
	}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	err = json.Unmarshal(gotBytes, &got)
	require.NoError(t, err)

	// Verify only the valid annotation is captured
	require.Len(t, got.Choices, 1)
	require.NotNil(t, got.Choices[0].Message)
	require.Len(t, got.Choices[0].Message.Annotations, 1)
	require.Equal(t, "url_citation", got.Choices[0].Message.Annotations[0].Type)
	require.NotNil(t, got.Choices[0].Message.Annotations[0].URLCitation)
	require.Equal(t, "https://example.com/valid", got.Choices[0].Message.Annotations[0].URLCitation.URL)
	require.Equal(t, "Valid Source", got.Choices[0].Message.Annotations[0].URLCitation.Title)
}
