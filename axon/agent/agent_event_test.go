package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/bus"
)

type streamTestProvider struct {
	streamFactory func() <-chan StreamEvent
}

func (p *streamTestProvider) Chat(_ context.Context, _ string, _ []ToolDefinition, _ []Message) (Response, error) {
	return Response{}, nil
}

func (p *streamTestProvider) ChatStream(_ context.Context, _ string, _ []ToolDefinition, _ []Message) (<-chan StreamEvent, error) {
	return p.streamFactory(), nil
}

func TestProcessStreamFiltersEventsByRunID(t *testing.T) {
	eventBus := bus.NewInProcess()

	provider := &streamTestProvider{
		streamFactory: func() <-chan StreamEvent {
			ch := make(chan StreamEvent, 3)
			text := "hello"

			time.Sleep(20 * time.Millisecond)

			ch <- StreamEvent{Type: StreamEventTextDelta, Text: text}

			ch <- StreamEvent{Type: StreamEventDone}

			close(ch)

			return ch
		},
	}

	a := New(Config{Model: "test-model"}, provider, WithBus(eventBus))

	ctx := context.Background()
	eventsCh := a.ProcessStream(ctx, Content{Text: new("user")})

	err := eventBus.Publish(ctx, bus.Event{
		Topic: TopicAgentEvent,
		Type:  string(EventTextDelta),
		Payload: AgentEvent{
			RunID: "foreign-run",
			Type:  EventTextDelta,
			Delta: "foreign",
		},
	})
	require.NoError(t, err)

	var events []AgentEvent
	for ev := range eventsCh {
		events = append(events, ev)
	}

	require.NotEmpty(t, events)
	runID := events[0].RunID
	require.NotEmpty(t, runID)

	for _, ev := range events {
		require.Equal(t, runID, ev.RunID)
		require.NotEqual(t, "foreign", ev.Delta)
	}
}

type loopTestProvider struct {
	chatResponses   []Response
	chatStreamItems [][]StreamEvent

	chatCalls       int
	chatStreamCalls int
}

func (p *loopTestProvider) Chat(_ context.Context, _ string, _ []ToolDefinition, _ []Message) (Response, error) {
	if p.chatCalls >= len(p.chatResponses) {
		return Response{}, errors.New("unexpected Chat call")
	}

	resp := p.chatResponses[p.chatCalls]
	p.chatCalls++

	return resp, nil
}

func (p *loopTestProvider) ChatStream(_ context.Context, _ string, _ []ToolDefinition, _ []Message) (<-chan StreamEvent, error) {
	if p.chatStreamCalls >= len(p.chatStreamItems) {
		return nil, errors.New("unexpected ChatStream call")
	}

	stream := make(chan StreamEvent, len(p.chatStreamItems[p.chatStreamCalls]))
	for _, ev := range p.chatStreamItems[p.chatStreamCalls] {
		stream <- ev
	}

	close(stream)

	p.chatStreamCalls++

	return stream, nil
}

type recordingTool struct {
	name  string
	calls []string
}

func (t *recordingTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        t.name,
		Description: "test tool",
		Parameters: jsonschema.Schema{
			Type: "object",
		},
	}
}

func (t *recordingTool) Execute(_ context.Context, arguments json.RawMessage) ToolResult {
	t.calls = append(t.calls, string(arguments))
	text := "ok"

	return ToolResult{Content: Content{Text: &text}}
}

func TestProcess_LoopRecoverySkipsRemainingToolCalls(t *testing.T) {
	provider := &loopTestProvider{
		chatResponses: []Response{
			{
				Messages: []Message{
					{Role: RoleAssistant, ToolCall: &ToolCall{ID: "call-1", Name: "loop_tool", Input: `{"n":1}`}},
					{Role: RoleAssistant, ToolCall: &ToolCall{ID: "call-2", Name: "loop_tool", Input: `{"n":1}`}},
				},
			},
			{
				Messages: []Message{
					{Role: RoleAssistant, Content: &Content{Text: new("done")}},
				},
			},
		},
	}
	tool := &recordingTool{name: "loop_tool"}
	a := New(Config{
		Model: "test-model",
		LoopDetector: &LoopDetectorConfig{
			Enabled:           true,
			ToolCallThreshold: 1,
			MaxRecoveries:     1,
		},
	}, provider)
	a.RegisterTool(tool)

	result, err := a.Process(context.Background(), Content{Text: new("start")})
	require.NoError(t, err)
	require.Equal(t, "done", result.Output)
	require.Equal(t, []string{`{"n":1}`}, tool.calls)

	msgs := a.Messages()
	require.Len(t, msgs, 7)
	require.Equal(t, RoleUser, msgs[3].Role)
	require.Contains(t, msgs[3].Content.String(), "Potential loop detected")
	require.Equal(t, RoleAssistant, msgs[4].Role)
	require.NotNil(t, msgs[4].ToolCall)
	require.Equal(t, "call-2", msgs[4].ToolCall.ID)
	require.Equal(t, RoleTool, msgs[5].Role)
	require.NotNil(t, msgs[5].IsError)
	require.True(t, *msgs[5].IsError)
	require.Equal(t, "Skipped due to steering message.", msgs[5].Content.String())
}

func TestProcess_LoopRecoveryExhaustedReturnsError(t *testing.T) {
	provider := &loopTestProvider{
		chatResponses: []Response{
			{
				Messages: []Message{
					{Role: RoleAssistant, ToolCall: &ToolCall{ID: "call-1", Name: "loop_tool", Input: `{"n":1}`}},
				},
			},
			{
				Messages: []Message{
					{Role: RoleAssistant, ToolCall: &ToolCall{ID: "call-2", Name: "loop_tool", Input: `{"n":1}`}},
				},
			},
		},
	}
	tool := &recordingTool{name: "loop_tool"}
	a := New(Config{
		Model: "test-model",
		LoopDetector: &LoopDetectorConfig{
			Enabled:           true,
			ToolCallThreshold: 1,
			MaxRecoveries:     1,
		},
	}, provider)
	a.RegisterTool(tool)

	_, err := a.Process(context.Background(), Content{Text: new("start")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "loop detected and recovery exhausted")
	require.Equal(t, []string{`{"n":1}`, `{"n":1}`}, tool.calls)
}

func TestProcessStream_LoopRecoverySkipsRemainingToolCalls(t *testing.T) {
	provider := &loopTestProvider{
		chatStreamItems: [][]StreamEvent{
			{
				{Type: StreamEventToolCallDelta, Text: `{"n":1}`, ToolCall: &ToolCall{ID: "call-1", Name: "loop_tool"}},
				{Type: StreamEventToolCallDelta, Text: `{"n":1}`, ToolCall: &ToolCall{ID: "call-2", Name: "loop_tool"}},
				{Type: StreamEventDone},
			},
			{
				{Type: StreamEventTextDelta, Text: "done"},
				{Type: StreamEventTextComplete, Text: "done"},
				{Type: StreamEventDone},
			},
		},
	}
	tool := &recordingTool{name: "loop_tool"}
	a := New(Config{
		Model: "test-model",
		LoopDetector: &LoopDetectorConfig{
			Enabled:           true,
			ToolCallThreshold: 1,
			MaxRecoveries:     1,
		},
	}, provider)
	a.RegisterTool(tool)

	var events []AgentEvent
	for ev := range a.ProcessStream(context.Background(), Content{Text: new("start")}) {
		events = append(events, ev)
	}

	require.Equal(t, []string{`{"n":1}`}, tool.calls)

	hasLoopDetected := false
	hasLoopRecovery := false
	hasToolSkipped := false

	for _, ev := range events {
		switch ev.Type {
		case EventLoopDetected:
			hasLoopDetected = true
		case EventLoopRecovery:
			hasLoopRecovery = true
		case EventToolSkipped:
			hasToolSkipped = true
		}
	}

	require.True(t, hasLoopDetected)
	require.True(t, hasLoopRecovery)
	require.True(t, hasToolSkipped)

	msgs := a.Messages()
	require.Len(t, msgs, 7)
	require.Equal(t, RoleUser, msgs[3].Role)
	require.Contains(t, msgs[3].Content.String(), "Potential loop detected")
	require.Equal(t, "Skipped due to steering message.", msgs[5].Content.String())
}

func TestProcessStream_FallbackToolCallOrderIsStable(t *testing.T) {
	provider := &loopTestProvider{
		chatStreamItems: [][]StreamEvent{
			{
				{Type: StreamEventToolCallDelta, Text: `{"order":1}`, ToolCall: &ToolCall{ID: "call-1", Name: "tool_one"}},
				{Type: StreamEventToolCallDelta, Text: `{"order":2}`, ToolCall: &ToolCall{ID: "call-2", Name: "tool_two"}},
				{Type: StreamEventDone},
			},
			{
				{Type: StreamEventTextDelta, Text: "done"},
				{Type: StreamEventTextComplete, Text: "done"},
				{Type: StreamEventDone},
			},
		},
	}
	toolOne := &recordingTool{name: "tool_one"}
	toolTwo := &recordingTool{name: "tool_two"}
	a := New(Config{
		Model: "test-model",
		LoopDetector: &LoopDetectorConfig{
			Enabled:           true,
			ToolCallThreshold: 10,
			MaxRecoveries:     1,
		},
	}, provider)
	a.RegisterTool(toolOne)
	a.RegisterTool(toolTwo)

	for range a.ProcessStream(context.Background(), Content{Text: new("start")}) {
	}

	require.Equal(t, []string{`{"order":1}`}, toolOne.calls)
	require.Equal(t, []string{`{"order":2}`}, toolTwo.calls)
}
