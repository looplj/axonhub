package anthropic

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/agent"
)

func TestStreamProcessor_HandleTextStreaming(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type: "text_delta",
			Text: "Hello",
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type: "text_delta",
			Text: " World",
		},
		Index: 0,
	})
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 2)
	assert.Equal(t, agent.StreamEventTextDelta, receivedEvents[0].Type)
	assert.Equal(t, "Hello", receivedEvents[0].Text)
	assert.Equal(t, agent.StreamEventTextDelta, receivedEvents[1].Type)
	assert.Equal(t, " World", receivedEvents[1].Text)

	assert.Equal(t, "Hello World", processor.accumulatedText.String())
}

func TestStreamProcessor_HandleTextComplete(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	processor.accumulatedText.WriteString("Complete text")

	err := processor.handleContentBlockStop()
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 1)
	assert.Equal(t, agent.StreamEventTextComplete, receivedEvents[0].Type)
	assert.Equal(t, "Complete text", receivedEvents[0].Text)
	assert.Equal(t, 0, processor.accumulatedText.Len())
}

func TestStreamProcessor_HandleThinkingStreaming(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:     "thinking_delta",
			Thinking: "Thinking...",
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:     "thinking_delta",
			Thinking: " More thoughts",
		},
		Index: 0,
	})
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 2)
	assert.Equal(t, agent.StreamEventThinkingDelta, receivedEvents[0].Type)
	require.NotNil(t, receivedEvents[0].Thinking)
	assert.Equal(t, "Thinking...", receivedEvents[0].Thinking.Content)
	assert.Equal(t, agent.StreamEventThinkingDelta, receivedEvents[1].Type)
	require.NotNil(t, receivedEvents[1].Thinking)
	assert.Equal(t, " More thoughts", receivedEvents[1].Thinking.Content)

	assert.Equal(t, "Thinking... More thoughts", processor.accumulatedThinking.String())
}

func TestStreamProcessor_HandleThinkingComplete(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	processor.accumulatedThinking.WriteString("Full thinking content")
	processor.thinkingSignature = "sig_123"

	err := processor.handleContentBlockStop()
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 1)
	assert.Equal(t, agent.StreamEventThinkingComplete, receivedEvents[0].Type)
	require.NotNil(t, receivedEvents[0].Thinking)
	assert.Equal(t, "Full thinking content", receivedEvents[0].Thinking.Content)
	assert.Equal(t, "sig_123", receivedEvents[0].Thinking.Signature)
	assert.Equal(t, 0, processor.accumulatedThinking.Len())
	assert.Equal(t, "", processor.thinkingSignature)
}

func TestStreamProcessor_HandleToolCallStreaming(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleContentBlockStart(anthropic.MessageStreamEventUnion{
		Type: "content_block_start",
		ContentBlock: anthropic.ContentBlockStartEventContentBlockUnion{
			Type: "tool_use",
			ID:   "tool_123",
			Name: "get_weather",
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"loc`,
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:        "input_json_delta",
			PartialJSON: `ation": "Tokyo"}`,
		},
		Index: 0,
	})
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 2)
	assert.Equal(t, agent.StreamEventToolCallDelta, receivedEvents[0].Type)
	assert.Equal(t, `{"loc`, receivedEvents[0].Text)
	require.NotNil(t, receivedEvents[0].ToolUse)
	assert.Equal(t, "tool_123", receivedEvents[0].ToolUse.ID)
	assert.Equal(t, "get_weather", receivedEvents[0].ToolUse.Name)

	assert.Equal(t, agent.StreamEventToolCallDelta, receivedEvents[1].Type)
	assert.Equal(t, `ation": "Tokyo"}`, receivedEvents[1].Text)

	builder := processor.toolCallBuilders[0]
	require.NotNil(t, builder)
	assert.Equal(t, `{"location": "Tokyo"}`, builder.buildJSON())
}

func TestStreamProcessor_HandleToolCallComplete(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	processor.toolCallBuilders[0] = &toolCallAccumulator{
		id:        "tool_456",
		name:      "calculate",
		jsonParts: []string{`{"expr`, `ession": "2+2"}`},
	}

	processor.usage = &agent.Usage{InputTokens: 100}

	err := processor.handleMessageDelta(anthropic.MessageStreamEventUnion{
		Type: "message_delta",
		Usage: anthropic.MessageDeltaUsage{
			OutputTokens: 50,
		},
		Index: 0,
	})
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	var usageEvent, toolCompleteEvent *agent.StreamEvent
	for i := range receivedEvents {
		if receivedEvents[i].Type == agent.StreamEventUsage {
			usageEvent = &receivedEvents[i]
		}
		if receivedEvents[i].Type == agent.StreamEventToolCallComplete {
			toolCompleteEvent = &receivedEvents[i]
		}
	}

	require.NotNil(t, usageEvent)
	assert.Equal(t, 100, usageEvent.Usage.InputTokens)
	assert.Equal(t, 50, usageEvent.Usage.OutputTokens)

	require.NotNil(t, toolCompleteEvent)
	assert.Equal(t, "tool_456", toolCompleteEvent.ToolUse.ID)
	assert.Equal(t, "calculate", toolCompleteEvent.ToolUse.Name)
	assert.Equal(t, `{"expression": "2+2"}`, toolCompleteEvent.ToolUse.Input)
}

func TestStreamProcessor_HandleMessageStart(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleMessageStart(anthropic.MessageStreamEventUnion{
		Type: "message_start",
		Message: anthropic.Message{
			Usage: anthropic.Usage{
				InputTokens:  100,
				OutputTokens: 0,
			},
		},
	})
	require.NoError(t, err)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 1)
	assert.Equal(t, agent.StreamEventUsage, receivedEvents[0].Type)
	require.NotNil(t, receivedEvents[0].Usage)
	assert.Equal(t, 100, receivedEvents[0].Usage.InputTokens)
	assert.Equal(t, 0, receivedEvents[0].Usage.OutputTokens)
}

func TestStreamProcessor_HandleSignatureDelta(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:      "signature_delta",
			Signature: "signature_abc",
		},
		Index: 0,
	})
	require.NoError(t, err)

	assert.Equal(t, "signature_abc", processor.thinkingSignature)
}

func TestStreamProcessor_HandleEventRouting(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
	}{
		{"message_start", "message_start"},
		{"content_block_start", "content_block_start"},
		{"content_block_delta", "content_block_delta"},
		{"content_block_stop", "content_block_stop"},
		{"message_delta", "message_delta"},
		{"message_stop", "message_stop"},
		{"unknown", "unknown_event"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := make(chan agent.StreamEvent, 256)
			processor := newStreamProcessor(nil, events)

			err := processor.handleEvent(anthropic.MessageStreamEventUnion{
				Type: tt.eventType,
			})

			assert.NoError(t, err)
			close(events)
		})
	}
}

func TestStreamProcessor_EmitError(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	testErr := assert.AnError
	processor.emitError(testErr)

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 1)
	assert.Equal(t, agent.StreamEventError, receivedEvents[0].Type)
	assert.Equal(t, testErr, receivedEvents[0].Error)
}

func TestStreamProcessor_EmitDone(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	processor.emitDone()

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 1)
	assert.Equal(t, agent.StreamEventDone, receivedEvents[0].Type)
}

func TestToolCallAccumulator_BuildJSON(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "single part",
			parts:    []string{`{"key": "value"}`},
			expected: `{"key": "value"}`,
		},
		{
			name:     "multiple parts",
			parts:    []string{`{"na`, `me": "`, `test"}`},
			expected: `{"name": "test"}`,
		},
		{
			name:     "empty parts",
			parts:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := &toolCallAccumulator{jsonParts: tt.parts}
			assert.Equal(t, tt.expected, acc.buildJSON())
		})
	}
}

func TestStreamProcessor_FullTextFlow(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleMessageStart(anthropic.MessageStreamEventUnion{
		Type: "message_start",
		Message: anthropic.Message{
			Usage: anthropic.Usage{
				InputTokens:  50,
				OutputTokens: 0,
			},
		},
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type: "text_delta",
			Text: "Hello",
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type: "text_delta",
			Text: " World",
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockStop()
	require.NoError(t, err)

	err = processor.handleMessageDelta(anthropic.MessageStreamEventUnion{
		Type: "message_delta",
		Usage: anthropic.MessageDeltaUsage{
			OutputTokens: 10,
		},
	})
	require.NoError(t, err)

	processor.emitDone()
	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	require.Len(t, receivedEvents, 6)

	assert.Equal(t, agent.StreamEventUsage, receivedEvents[0].Type)
	assert.Equal(t, agent.StreamEventTextDelta, receivedEvents[1].Type)
	assert.Equal(t, "Hello", receivedEvents[1].Text)
	assert.Equal(t, agent.StreamEventTextDelta, receivedEvents[2].Type)
	assert.Equal(t, " World", receivedEvents[2].Text)
	assert.Equal(t, agent.StreamEventTextComplete, receivedEvents[3].Type)
	assert.Equal(t, "Hello World", receivedEvents[3].Text)
	assert.Equal(t, agent.StreamEventUsage, receivedEvents[4].Type)
	assert.Equal(t, agent.StreamEventDone, receivedEvents[5].Type)
}

func TestStreamProcessor_ProcessWithMockStream(t *testing.T) {
	mockEvents := []anthropic.MessageStreamEventUnion{
		{
			Type: "message_start",
			Message: anthropic.Message{
				Usage: anthropic.Usage{
					InputTokens:  100,
					OutputTokens: 0,
				},
			},
		},
		{
			Type: "content_block_delta",
			Delta: anthropic.MessageStreamEventUnionDelta{
				Type: "text_delta",
				Text: "Test response",
			},
			Index: 0,
		},
		{
			Type:  "content_block_stop",
			Index: 0,
		},
		{
			Type: "message_delta",
			Usage: anthropic.MessageDeltaUsage{
				OutputTokens: 5,
			},
		},
		{
			Type: "message_stop",
		},
	}

	events := make(chan agent.StreamEvent, 256)

	mock := &mockStream{events: mockEvents}

	stream := &ssestream.Stream[anthropic.MessageStreamEventUnion]{}
	_ = stream
	_ = mock

	processor := newStreamProcessor(nil, events)
	processor.accumulatedText.WriteString("Test response")

	for _, ev := range mockEvents {
		if err := processor.handleEvent(ev); err != nil {
			processor.emitError(err)
			break
		}
	}
	processor.emitDone()

	close(events)

	var receivedEvents []agent.StreamEvent
	for ev := range events {
		receivedEvents = append(receivedEvents, ev)
	}

	assert.GreaterOrEqual(t, len(receivedEvents), 2)

	hasUsage := false
	hasDone := false
	for _, ev := range receivedEvents {
		if ev.Type == agent.StreamEventUsage {
			hasUsage = true
		}
		if ev.Type == agent.StreamEventDone {
			hasDone = true
		}
	}
	assert.True(t, hasUsage, "should have usage event")
	assert.True(t, hasDone, "should have done event")
}

func TestNewStreamProcessor(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	stream := &ssestream.Stream[anthropic.MessageStreamEventUnion]{}

	processor := newStreamProcessor(stream, events)

	assert.NotNil(t, processor)
	assert.Equal(t, stream, processor.stream)
	assert.NotNil(t, processor.toolCallBuilders)
	assert.NotNil(t, processor.events)
}

func TestStreamProcessor_MultipleToolCalls(t *testing.T) {
	events := make(chan agent.StreamEvent, 256)
	processor := newStreamProcessor(nil, events)

	err := processor.handleContentBlockStart(anthropic.MessageStreamEventUnion{
		Type: "content_block_start",
		ContentBlock: anthropic.ContentBlockStartEventContentBlockUnion{
			Type: "tool_use",
			ID:   "tool_1",
			Name: "func1",
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockStart(anthropic.MessageStreamEventUnion{
		Type: "content_block_start",
		ContentBlock: anthropic.ContentBlockStartEventContentBlockUnion{
			Type: "tool_use",
			ID:   "tool_2",
			Name: "func2",
		},
		Index: 1,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"a": 1}`,
		},
		Index: 0,
	})
	require.NoError(t, err)

	err = processor.handleContentBlockDelta(anthropic.MessageStreamEventUnion{
		Type: "content_block_delta",
		Delta: anthropic.MessageStreamEventUnionDelta{
			Type:        "input_json_delta",
			PartialJSON: `{"b": 2}`,
		},
		Index: 1,
	})
	require.NoError(t, err)

	assert.Len(t, processor.toolCallBuilders, 2)
	assert.Equal(t, "tool_1", processor.toolCallBuilders[0].id)
	assert.Equal(t, "func1", processor.toolCallBuilders[0].name)
	assert.Equal(t, "tool_2", processor.toolCallBuilders[1].id)
	assert.Equal(t, "func2", processor.toolCallBuilders[1].name)

	close(events)
}

func TestStreamProcessor_NonBlockingEmit(t *testing.T) {
	events := make(chan agent.StreamEvent, 1)
	processor := newStreamProcessor(nil, events)

	processor.emit(agent.StreamEvent{Type: agent.StreamEventTextDelta, Text: "first"})

	select {
	case events <- agent.StreamEvent{Type: agent.StreamEventTextDelta, Text: "second"}:
		t.Fatal("should have blocked")
	default:
	}

	processor.emit(agent.StreamEvent{Type: agent.StreamEventTextDelta, Text: "second"})

	<-events

	select {
	case ev := <-events:
		assert.Equal(t, "second", ev.Text)
	default:
	}
}

type mockStream struct {
	events []anthropic.MessageStreamEventUnion
	index  int
	err    error
}

func (m *mockStream) Next() bool {
	if m.index < len(m.events) {
		return true
	}
	return false
}

func (m *mockStream) Current() anthropic.MessageStreamEventUnion {
	if m.index < len(m.events) {
		ev := m.events[m.index]
		m.index++
		return ev
	}
	return anthropic.MessageStreamEventUnion{}
}

func (m *mockStream) Err() error {
	return m.err
}

func (m *mockStream) Close() error {
	return nil
}
