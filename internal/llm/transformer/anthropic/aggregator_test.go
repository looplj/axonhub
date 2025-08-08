package anthropic

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestAggregateStreamChunks(t *testing.T) {
	tests := []struct {
		name      string
		chunks    []*httpclient.StreamEvent
		expected  string
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "empty chunks",
			chunks:   []*httpclient.StreamEvent{},
			expected: "",
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.ErrorContains(t, err, "empty stream chunks")
			},
		},
		{
			name: "single chunk",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte(`{
						"type": "message_start",
						"message": {
							"id": "msg_123",
							"type": "message",
							"role": "assistant",
							"content": [],
							"model": "claude-3-sonnet-20240229"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "content_block_delta",
						"index": 0,
						"delta": {
							"type": "text_delta",
							"text": "Hello!"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "message_delta",
						"delta": {
							"stop_reason": "end_turn"
						},
						"usage": {
							"input_tokens": 10,
							"output_tokens": 5
						}
					}`),
				},
			},
			expected: "Hello!",
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.NoError(t, err)
			},
		},
		{
			name: "multiple content chunks",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte(`{
						"type": "message_start",
						"message": {
							"id": "msg_456",
							"type": "message",
							"role": "assistant",
							"content": [],
							"model": "claude-3-sonnet-20240229"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "content_block_delta",
						"index": 0,
						"delta": {
							"type": "text_delta",
							"text": "Hello"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "content_block_delta",
						"index": 0,
						"delta": {
							"type": "text_delta",
							"text": ", world!"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "message_delta",
						"delta": {
							"stop_reason": "end_turn"
						}
					}`),
				},
			},
			expected: "Hello, world!",
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultBytes, err := AggregateStreamChunks(t.Context(), tt.chunks)
			tt.assertErr(t, err)

			if tt.expected == "" {
				if err == nil {
					var result Message

					err := json.Unmarshal(resultBytes, &result)
					require.NoError(t, err)
					require.Empty(t, result.Content)
				}
			} else {
				require.NotNil(t, resultBytes)

				var result Message

				err := json.Unmarshal(resultBytes, &result)
				require.NoError(t, err)
				require.NotEmpty(t, result.Content)
				require.Equal(t, tt.expected, result.Content[0].Text)
				require.Equal(t, "assistant", result.Role)
			}
		})
	}
}

func TestAggregateStreamChunks_EdgeCases(t *testing.T) {
	t.Run("Streaming edge cases", func(t *testing.T) {
		tests := []struct {
			name        string
			chunks      []*httpclient.StreamEvent
			expectError bool
			validate    func(t *testing.T, result *Message)
			errorMsg    string
		}{
			{
				name:        "nil chunks",
				chunks:      nil,
				expectError: true,
				errorMsg:    "empty stream chunks",
			},
			{
				name: "chunks with invalid JSON",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_123",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{invalid json}`), // This should be skipped
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Hello"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(t, "Hello", result.Content[0].Text)
				},
			},
			{
				name: "chunks with unknown event types",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_123",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "unknown_event",
							"some_field": "value"
						}`), // Should be skipped
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Hello"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(t, "Hello", result.Content[0].Text)
				},
			},
			{
				name: "chunks missing message_start",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Hello"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					// Should handle gracefully, might have empty fields
				},
			},
			{
				name: "chunks with all event types",
				chunks: []*httpclient.StreamEvent{
					{
						Type: "message_start",
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_complete",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229",
								"usage": {"input_tokens": 5, "output_tokens": 0}
							}
						}`),
					},
					{
						Type: "content_block_start",
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "text",
								"text": ""
							}
						}`),
					},
					{
						Type: "content_block_delta",
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Complete"
							}
						}`),
					},
					{
						Type: "content_block_stop",
						Data: []byte(`{
							"type": "content_block_stop",
							"index": 0
						}`),
					},
					{
						Type: "message_delta",
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							},
							"usage": {"output_tokens": 8}
						}`),
					},
					{
						Type: "message_stop",
						Data: []byte(`{
							"type": "message_stop"
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(t, "Complete", result.Content[0].Text)
					require.Equal(t, "msg_complete", result.ID)
					require.NotNil(t, result.StopReason)
					require.Equal(t, "end_turn", *result.StopReason)
					require.NotNil(t, result.Usage)
					require.Equal(t, int64(5), result.Usage.InputTokens)
					require.Equal(t, int64(8), result.Usage.OutputTokens)
				},
			},
			{
				name: "chunks with detailed usage information",
				chunks: []*httpclient.StreamEvent{
					{
						Type: "message_start",
						Data: []byte(
							`{"type": "message_start", "message": {"id": "msg_detailed_usage", "type": "message", "role": "assistant", "content": [], "model": "claude-3-sonnet-20240229", "usage": {"input_tokens": 100, "output_tokens": 0, "cache_creation_input_tokens": 20, "cache_read_input_tokens": 50}}}`,
						),
					},
					{
						Type: "content_block_start",
						Data: []byte(
							`{"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}`,
						),
					},
					{
						Type: "content_block_delta",
						Data: []byte(
							`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Response with detailed usage"}}`,
						),
					},
					{
						Type: "message_delta",
						Data: []byte(
							`{"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"output_tokens": 25, "cache_creation_input_tokens": 20, "cache_read_input_tokens": 50}}`,
						),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(
						t,
						"Response with detailed usage",
						result.Content[0].Text,
					)
					require.Equal(t, "msg_detailed_usage", result.ID)
					require.NotNil(t, result.StopReason)
					require.Equal(t, "end_turn", *result.StopReason)
					require.NotNil(t, result.Usage)
					require.Equal(t, int64(100), result.Usage.InputTokens)
					require.Equal(t, int64(25), result.Usage.OutputTokens)
					// Verify detailed token information
					require.Equal(t, int64(50), result.Usage.CacheReadInputTokens)
					require.Equal(t, int64(20), result.Usage.CacheCreationInputTokens)
				},
			},
			{
				name: "chunks with usage but no cache tokens",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(
							`{"type": "message_start", "message": {"id": "msg_no_cache_stream", "type": "message", "role": "assistant", "content": [], "model": "claude-3-sonnet-20240229", "usage": {"input_tokens": 80, "output_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "No cache response"}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"output_tokens": 40, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}`,
						),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(
						t,
						"No cache response",
						result.Content[0].Text,
					)
					require.Equal(t, "msg_no_cache_stream", result.ID)
					require.NotNil(t, result.StopReason)
					require.Equal(t, "end_turn", *result.StopReason)
					require.NotNil(t, result.Usage)
					require.Equal(t, int64(80), result.Usage.InputTokens)
					require.Equal(t, int64(40), result.Usage.OutputTokens)
					// Verify no cache token information when cache tokens are 0
					require.Equal(t, int64(0), result.Usage.CacheCreationInputTokens)
					require.Equal(t, int64(0), result.Usage.CacheReadInputTokens)
				},
			},
			{
				name: "chunks with thinking blocks",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_thinking",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "thinking",
								"thinking": "Let me think about this..."
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "thinking_delta",
								"thinking": " some more"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 1,
							"content_block": {
								"type": "text",
								"text": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 1,
							"delta": {
								"type": "text_delta",
								"text": "Final answer"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					// Should contain both thinking and text content
					require.Len(t, result.Content, 2)
					require.Equal(t, "thinking", result.Content[0].Type)
					require.Equal(t, "text", result.Content[1].Type)
				},
			},
			{
				name: "chunks with tool use",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_tool",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "text",
								"text": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "I'll use a tool"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 1,
							"content_block": {
								"type": "tool_use",
								"id": "tool_123",
								"name": "calculator",
								"input": "{\"expression\": \"2+2\"}"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "tool_use"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.NotNil(t, result.StopReason)
					require.Equal(t, "tool_use", *result.StopReason)
					require.Len(t, result.Content, 2)
					require.Equal(t, "text", result.Content[0].Type)
					require.Equal(t, "tool_use", result.Content[1].Type)
					require.Equal(t, "tool_123", result.Content[1].ID)
				},
			},
			{
				name: "chunks with partial JSON",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_partial",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "tool_use",
								"id": "tool_456",
								"name": "json_tool"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "input_json_delta",
								"partial_json": "{\"key\":"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "input_json_delta",
								"partial_json": "\"value\"}"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "tool_use"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.NotNil(t, result.StopReason)
					require.Equal(t, "tool_use", *result.StopReason)
					require.Len(t, result.Content, 1)
					require.Equal(t, "tool_use", result.Content[0].Type)
					require.Equal(t, "tool_456", result.Content[0].ID)
					require.Equal(
						t,
						`{"key":"value"}`,
						string(result.Content[0].Input),
					)
				},
			},
			{
				name: "chunks with ping events",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_ping",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "ping"
						}`), // Should be ignored
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "After ping"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(t, "After ping", result.Content[0].Text)
				},
			},
			{
				name: "chunks with signature delta",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_sig",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "thinking",
								"thinking": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "thinking_delta",
								"thinking": "Thinking..."
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "signature_delta",
								"signature": "abc123"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.Equal(t, "thinking", result.Content[0].Type)
					require.Equal(t, "Thinking...", result.Content[0].Thinking)
				},
			},
			{
				name: "chunks with multiple stop reasons",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_multi_stop",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Test"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "max_tokens"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *Message) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Content)
					require.NotNil(t, result.StopReason)
					require.Equal(t, "max_tokens", *result.StopReason)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resultBytes, err := AggregateStreamChunks(t.Context(), tt.chunks)
				if tt.expectError {
					require.Error(t, err)

					if tt.errorMsg != "" {
						require.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					require.NoError(t, err)
					// Parse the response
					var result Message

					err = json.Unmarshal(resultBytes, &result)
					require.NoError(t, err)
					tt.validate(t, &result)
				}
			})
		}
	})
}

func TestAggregateStreamChunks_WithTestData(t *testing.T) {
	t.Run("anthropic-stop stream data", func(t *testing.T) {
		// Load the stream data
		streamData, err := loadStreamTestData("testdata/anthropic-stop.stream.jsonl")
		require.NoError(t, err)
		require.NotEmpty(t, streamData)

		// Load the expected aggregated result
		expectedData, err := loadExpectedResult("testdata/llm-stop.aggregator.json")
		require.NoError(t, err)

		// Run the aggregation
		resultBytes, err := AggregateStreamChunks(t.Context(), streamData)
		require.NoError(t, err)
		require.NotNil(t, resultBytes)

		// Parse the result
		var result Message

		err = json.Unmarshal(resultBytes, &result)
		require.NoError(t, err)

		// Compare with expected result
		require.Equal(t, expectedData.ID, result.ID)
		require.Equal(t, expectedData.Type, result.Type)
		require.Equal(t, expectedData.Role, result.Role)
		require.Equal(t, expectedData.Model, result.Model)
		require.Equal(t, expectedData.StopReason, result.StopReason)

		// Compare content
		require.Len(t, result.Content, len(expectedData.Content))

		for i, expectedContent := range expectedData.Content {
			require.Equal(t, expectedContent.Type, result.Content[i].Type)
			require.Equal(t, expectedContent.Text, result.Content[i].Text)
		}

		// Compare usage
		if expectedData.Usage != nil {
			require.NotNil(t, result.Usage)
			require.Equal(t, expectedData.Usage.InputTokens, result.Usage.InputTokens)
			require.Equal(t, expectedData.Usage.OutputTokens, result.Usage.OutputTokens)
			require.Equal(t, expectedData.Usage.CacheCreationInputTokens, result.Usage.CacheCreationInputTokens)
			require.Equal(t, expectedData.Usage.CacheReadInputTokens, result.Usage.CacheReadInputTokens)
		}
	})
}

// Helper function to load stream test data from JSONL file.
func loadStreamTestData(filename string) ([]*httpclient.StreamEvent, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	var events []*httpclient.StreamEvent

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the line as a temporary struct to handle the Data field correctly
		var temp struct {
			LastEventID string `json:"LastEventID"`
			Type        string `json:"Type"`
			Data        string `json:"Data"` // Data is a JSON string in the test file
		}

		if err := json.Unmarshal([]byte(line), &temp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal line %q: %w", line, err)
		}

		// Create the StreamEvent with Data as []byte
		streamEvent := &httpclient.StreamEvent{
			LastEventID: temp.LastEventID,
			Type:        temp.Type,
			Data:        []byte(temp.Data), // Convert string to []byte
		}

		events = append(events, streamEvent)
	}

	return events, nil
}

// Helper function to load expected result from JSON file.
func loadExpectedResult(filename string) (*Message, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal expected result: %w", err)
	}

	return &message, nil
}
