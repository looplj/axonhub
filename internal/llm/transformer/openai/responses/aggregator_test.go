package responses

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xtest"
)

func TestAggregateStreamChunks(t *testing.T) {
	tests := []struct {
		name      string
		chunks    []*httpclient.StreamEvent
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:   "empty chunks",
			chunks: []*httpclient.StreamEvent{},
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.ErrorContains(t, err, "empty stream chunks")
			},
		},
		{
			name:   "nil chunks",
			chunks: nil,
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.ErrorContains(t, err, "empty stream chunks")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := AggregateStreamChunks(t.Context(), tt.chunks)
			tt.assertErr(t, err)
		})
	}
}

func TestAggregateStreamChunks_WithTestData(t *testing.T) {
	tests := []struct {
		name             string
		streamFile       string
		expectedFile     string
		expectedMetaID   string
		expectedHasUsage bool
	}{
		{
			name:             "tool stream with text and multiple function calls",
			streamFile:       "tool-2.stream.jsonl",
			expectedFile:     "tool-2.response.json",
			expectedMetaID:   "resp_020592949fb9ce090069355e9a54788196911d78a6360a88f2",
			expectedHasUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the stream test data
			chunks, err := xtest.LoadStreamChunks(t, tt.streamFile)
			require.NoError(t, err)
			require.NotEmpty(t, chunks)

			// Aggregate the chunks
			resultBytes, meta, err := AggregateStreamChunks(t.Context(), chunks)
			require.NoError(t, err)
			require.NotNil(t, resultBytes)

			// Parse the actual result
			var actual Response

			err = json.Unmarshal(resultBytes, &actual)
			require.NoError(t, err)

			// Load expected response
			var expected Response

			err = xtest.LoadTestData(t, tt.expectedFile, &expected)
			require.NoError(t, err)

			// Compare using xtest.Equal with cmp.Diff output on mismatch
			if !xtest.Equal(expected, actual) {
				t.Fatalf("response mismatch:\n%s", cmp.Diff(expected, actual))
			}

			// Verify meta
			require.Equal(t, tt.expectedMetaID, meta.ID)

			if tt.expectedHasUsage {
				require.NotNil(t, meta.Usage)
			}
		})
	}
}

func TestAggregateStreamChunks_BasicEvents(t *testing.T) {
	// Create basic stream events for a simple text response
	chunks := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type": "response.created",
				"sequence_number": 0,
				"response": {
					"id": "resp_test_123",
					"object": "response",
					"created_at": 1700000000,
					"model": "gpt-4o",
					"status": "in_progress",
					"output": []
				}
			}`),
		},
		{
			Type: "response.output_item.added",
			Data: []byte(`{
				"type": "response.output_item.added",
				"sequence_number": 1,
				"output_index": 0,
				"item": {
					"id": "msg_test_456",
					"type": "message",
					"status": "in_progress",
					"role": "assistant"
				}
			}`),
		},
		{
			Type: "response.content_part.added",
			Data: []byte(`{
				"type": "response.content_part.added",
				"sequence_number": 2,
				"item_id": "msg_test_456",
				"output_index": 0,
				"content_index": 0,
				"part": {
					"type": "output_text",
					"text": ""
				}
			}`),
		},
		{
			Type: "response.output_text.delta",
			Data: []byte(`{
				"type": "response.output_text.delta",
				"sequence_number": 3,
				"item_id": "msg_test_456",
				"output_index": 0,
				"content_index": 0,
				"delta": "Hello"
			}`),
		},
		{
			Type: "response.output_text.delta",
			Data: []byte(`{
				"type": "response.output_text.delta",
				"sequence_number": 4,
				"item_id": "msg_test_456",
				"output_index": 0,
				"content_index": 0,
				"delta": " World!"
			}`),
		},
		{
			Type: "response.output_text.done",
			Data: []byte(`{
				"type": "response.output_text.done",
				"sequence_number": 5,
				"item_id": "msg_test_456",
				"output_index": 0,
				"content_index": 0,
				"text": "Hello World!"
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type": "response.output_item.done",
				"sequence_number": 6,
				"output_index": 0,
				"item": {
					"id": "msg_test_456",
					"type": "message",
					"status": "completed",
					"role": "assistant"
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type": "response.completed",
				"sequence_number": 7,
				"response": {
					"id": "resp_test_123",
					"object": "response",
					"created_at": 1700000000,
					"model": "gpt-4o",
					"status": "completed",
					"output": [],
					"usage": {
						"input_tokens": 10,
						"output_tokens": 5,
						"total_tokens": 15
					}
				}
			}`),
		},
	}

	// Aggregate the chunks
	resultBytes, meta, err := AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	require.NotNil(t, resultBytes)

	// Parse the result
	var resp Response

	err = json.Unmarshal(resultBytes, &resp)
	require.NoError(t, err)

	// Verify response
	require.Equal(t, "response", resp.Object)
	require.Equal(t, "resp_test_123", resp.ID)
	require.Equal(t, "gpt-4o", resp.Model)
	require.NotNil(t, resp.Status)
	require.Equal(t, "completed", *resp.Status)

	// Verify output items
	require.Len(t, resp.Output, 1)
	output := resp.Output[0]
	require.Equal(t, "message", output.Type)
	require.Equal(t, "assistant", output.Role)
	require.NotEmpty(t, output.GetContentItems())
	require.Equal(t, "Hello World!", output.GetContentItems()[0].Text)

	// Verify usage
	require.NotNil(t, resp.Usage)
	require.Equal(t, int64(10), resp.Usage.InputTokens)
	require.Equal(t, int64(5), resp.Usage.OutputTokens)
	require.Equal(t, int64(15), resp.Usage.TotalTokens)

	// Verify meta
	require.Equal(t, "resp_test_123", meta.ID)
	require.NotNil(t, meta.Usage)
	require.Equal(t, int64(10), meta.Usage.PromptTokens)
	require.Equal(t, int64(5), meta.Usage.CompletionTokens)
}

func TestAggregateStreamChunks_SkipsInvalidJSON(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type": "response.created",
				"sequence_number": 0,
				"response": {
					"id": "resp_test",
					"object": "response",
					"created_at": 1700000000,
					"model": "gpt-4o",
					"status": "in_progress"
				}
			}`),
		},
		{
			Type: "invalid",
			Data: []byte(`{invalid json}`), // Should be skipped
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type": "response.completed",
				"sequence_number": 1,
				"response": {
					"id": "resp_test",
					"status": "completed"
				}
			}`),
		},
	}

	resultBytes, _, err := AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	require.NotNil(t, resultBytes)

	var resp Response

	err = json.Unmarshal(resultBytes, &resp)
	require.NoError(t, err)
	require.Equal(t, "completed", *resp.Status)
}

func TestAggregateStreamChunks_SkipsDONEMarker(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type": "response.created",
				"sequence_number": 0,
				"response": {
					"id": "resp_test",
					"object": "response",
					"model": "gpt-4o",
					"status": "in_progress"
				}
			}`),
		},
		{
			Type: "",
			Data: []byte(`[DONE]`), // Should be skipped
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type": "response.completed",
				"sequence_number": 1,
				"response": {
					"id": "resp_test",
					"status": "completed"
				}
			}`),
		},
	}

	resultBytes, _, err := AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	require.NotNil(t, resultBytes)

	var resp Response

	err = json.Unmarshal(resultBytes, &resp)
	require.NoError(t, err)
	require.Equal(t, "resp_test", resp.ID)
}
