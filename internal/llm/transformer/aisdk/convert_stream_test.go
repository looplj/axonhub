package aisdk

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// Helper functions for tests.
func stringPtr(s string) *string {
	return &s
}

// NewConvertStreamTransformer creates a new DataStreamTransformer for testing convert stream functionality.
func NewConvertStreamTransformer() *DataStreamTransformer {
	return NewDataStreamTransformer()
}

// mockLLMStream implements streams.Stream[*llm.Response] for testing.
type mockLLMStream struct {
	responses []*llm.Response
	index     int
	err       error
}

func (m *mockLLMStream) Next() bool {
	return m.index < len(m.responses)
}

func (m *mockLLMStream) Current() *llm.Response {
	if m.index < len(m.responses) {
		response := m.responses[m.index]
		m.index++

		return response
	}

	return nil
}

func (m *mockLLMStream) Err() error {
	return m.err
}

func (m *mockLLMStream) Close() error {
	return nil
}

func TestConvertStreamTransformer_TransformStream_TextContent(t *testing.T) {
	transformer := NewConvertStreamTransformer()
	ctx := context.Background()

	// Create mock LLM responses for text streaming
	responses := []*llm.Response{
		{
			ID:     "msg_123",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Delta: &llm.Message{
						Content: llm.MessageContent{
							Content: stringPtr("Hello"),
						},
					},
				},
			},
		},
		{
			ID:     "msg_123",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Delta: &llm.Message{
						Content: llm.MessageContent{
							Content: stringPtr(" world!"),
						},
					},
				},
			},
		},
		{
			ID:     "msg_123",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					FinishReason: stringPtr("stop"),
				},
			},
		},
	}

	mockStream := &mockLLMStream{responses: responses}
	resultStream, err := transformer.TransformStream(ctx, mockStream)
	require.NoError(t, err)

	// Collect all events
	var events []*httpclient.StreamEvent
	for resultStream.Next() {
		events = append(events, resultStream.Current())
	}

	require.NoError(t, resultStream.Err())

	// Verify we have the expected number of events
	assert.GreaterOrEqual(t, len(events), 6)

	// Verify we have text events
	hasStart := false
	hasStartStep := false
	hasTextStart := false
	hasTextDelta := false
	hasTextEnd := false
	hasFinishStep := false
	hasFinish := false
	hasDone := false

	for _, event := range events {
		// Skip [DONE] events as they are not JSON
		if string(event.Data) == "[DONE]" {
			hasDone = true
			continue
		}

		var streamEvent StreamEvent

		err = json.Unmarshal(event.Data, &streamEvent)
		if err != nil {
			// Skip non-JSON events
			continue
		}

		switch streamEvent.Type {
		case "start":
			hasStart = true

			assert.Equal(t, "msg_123", streamEvent.MessageID)
		case "start-step":
			hasStartStep = true
		case "text-start":
			hasTextStart = true

			assert.NotEmpty(t, streamEvent.ID)
		case "text-delta":
			hasTextDelta = true

			assert.NotEmpty(t, streamEvent.ID)
			assert.NotEmpty(t, streamEvent.Delta)
		case "text-end":
			hasTextEnd = true

			assert.NotEmpty(t, streamEvent.ID)
		case "finish-step":
			hasFinishStep = true
		case "finish":
			hasFinish = true
		}
	}

	assert.True(t, hasStart, "Should have start event")
	assert.True(t, hasStartStep, "Should have start-step event")
	assert.True(t, hasTextStart, "Should have text-start event")
	assert.True(t, hasTextDelta, "Should have text-delta event")
	assert.True(t, hasTextEnd, "Should have text-end event")
	assert.True(t, hasFinishStep, "Should have finish-step event")
	assert.True(t, hasFinish, "Should have finish event")
	assert.True(t, hasDone, "Should have [DONE] event")
}

func TestConvertStreamTransformer_TransformStream_ReasoningContent(t *testing.T) {
	transformer := NewConvertStreamTransformer()
	ctx := context.Background()

	// Create mock LLM responses for reasoning content
	responses := []*llm.Response{
		{
			ID:     "msg_456",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Delta: &llm.Message{
						ReasoningContent: stringPtr("Let me think about this..."),
					},
				},
			},
		},
		{
			ID:     "msg_456",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					FinishReason: stringPtr("stop"),
				},
			},
		},
	}

	mockStream := &mockLLMStream{responses: responses}
	resultStream, err := transformer.TransformStream(ctx, mockStream)
	require.NoError(t, err)

	// Collect all events
	var events []*httpclient.StreamEvent
	for resultStream.Next() {
		events = append(events, resultStream.Current())
	}

	require.NoError(t, resultStream.Err())

	// Verify we have reasoning events
	hasStart := false
	hasStartStep := false
	hasReasoningStart := false
	hasReasoningDelta := false
	hasReasoningEnd := false
	hasFinishStep := false
	hasFinish := false
	hasDone := false

	for _, event := range events {
		// Skip [DONE] events as they are not JSON
		if string(event.Data) == "[DONE]" {
			hasDone = true
			continue
		}

		var streamEvent StreamEvent

		err = json.Unmarshal(event.Data, &streamEvent)
		if err != nil {
			// Skip non-JSON events
			continue
		}

		switch streamEvent.Type {
		case "start":
			hasStart = true

			assert.Equal(t, "msg_456", streamEvent.MessageID)
		case "start-step":
			hasStartStep = true
		case "reasoning-start":
			hasReasoningStart = true

			assert.NotEmpty(t, streamEvent.ID)
		case "reasoning-delta":
			hasReasoningDelta = true

			assert.NotEmpty(t, streamEvent.ID)
			assert.Equal(t, "Let me think about this...", streamEvent.Delta)
		case "reasoning-end":
			hasReasoningEnd = true

			assert.NotEmpty(t, streamEvent.ID)
		case "finish-step":
			hasFinishStep = true
		case "finish":
			hasFinish = true
		}
	}

	assert.True(t, hasStart, "Should have start event")
	assert.True(t, hasStartStep, "Should have start-step event")
	assert.True(t, hasReasoningStart, "Should have reasoning-start event")
	assert.True(t, hasReasoningDelta, "Should have reasoning-delta event")
	assert.True(t, hasReasoningEnd, "Should have reasoning-end event")
	assert.True(t, hasFinishStep, "Should have finish-step event")
	assert.True(t, hasFinish, "Should have finish event")
	assert.True(t, hasDone, "Should have [DONE] event")
}

func TestConvertStreamTransformer_TransformStream_ToolCalls(t *testing.T) {
	transformer := NewConvertStreamTransformer()
	ctx := context.Background()

	// Create mock LLM responses for tool calls
	responses := []*llm.Response{
		{
			ID:     "msg_789",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Delta: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								ID:   "tool_call_123",
								Type: "function",
								Function: llm.FunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":`,
								},
							},
						},
					},
				},
			},
		},
		{
			ID:     "msg_789",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Delta: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								ID:   "tool_call_123",
								Type: "function",
								Function: llm.FunctionCall{
									Arguments: `"San Francisco"}`,
								},
							},
						},
					},
				},
			},
		},
		{
			ID:     "msg_789",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					Message: &llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								ID:   "tool_call_123",
								Type: "function",
								Function: llm.FunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco"}`,
								},
							},
						},
					},
				},
			},
		},
		{
			ID:     "msg_789",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{
				{
					FinishReason: stringPtr("tool_calls"),
				},
			},
		},
	}

	mockStream := &mockLLMStream{responses: responses}
	resultStream, err := transformer.TransformStream(ctx, mockStream)
	require.NoError(t, err)

	// Collect all events
	var events []*httpclient.StreamEvent
	for resultStream.Next() {
		events = append(events, resultStream.Current())
	}

	require.NoError(t, resultStream.Err())

	// Verify we have tool call events
	hasStart := false
	hasToolInputStart := false
	hasToolInputDelta := false
	hasToolInputAvailable := false
	hasFinishStep := false
	hasFinish := false
	hasDone := false

	for _, event := range events {
		// Skip [DONE] events as they are not JSON
		if string(event.Data) == "[DONE]" {
			hasDone = true
			continue
		}

		var streamEvent StreamEvent

		err = json.Unmarshal(event.Data, &streamEvent)
		if err != nil {
			// Skip non-JSON events
			continue
		}

		switch streamEvent.Type {
		case "start":
			hasStart = true

			assert.Equal(t, "msg_789", streamEvent.MessageID)
		case "tool-input-start":
			hasToolInputStart = true

			assert.Equal(t, "tool_call_123", streamEvent.ToolCallID)
			assert.Equal(t, "get_weather", streamEvent.ToolName)
		case "tool-input-delta":
			hasToolInputDelta = true

			assert.Equal(t, "tool_call_123", streamEvent.ToolCallID)
			assert.NotEmpty(t, streamEvent.InputTextDelta)
		case "tool-input-available":
			hasToolInputAvailable = true

			assert.Equal(t, "tool_call_123", streamEvent.ToolCallID)
			assert.Equal(t, "get_weather", streamEvent.ToolName)
			assert.NotNil(t, streamEvent.Input)
		case "finish-step":
			hasFinishStep = true
		case "finish":
			hasFinish = true
		}
	}

	assert.True(t, hasStart, "Should have start event")
	// Note: Tool calls don't generate start-step events automatically in the current implementation
	assert.True(t, hasToolInputStart, "Should have tool-input-start event")
	assert.True(t, hasToolInputDelta, "Should have tool-input-delta event")
	assert.True(t, hasToolInputAvailable, "Should have tool-input-available event")
	assert.True(t, hasFinishStep, "Should have finish-step event")
	assert.True(t, hasFinish, "Should have finish event")
	assert.True(t, hasDone, "Should have [DONE] event")
}
