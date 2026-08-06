package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestEchoFields_ForwardedThroughPipeline(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{"type":"response.created","response":{"id":"resp_123","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[],"conversation":{"id":"conv_abc"},"metadata":{"agent_id":"agent_1"},"background":false,"temperature":1.0,"service_tier":"auto","reasoning":{"effort":"high","summary":"auto"},"parallel_tool_calls":true,"tools":[{"type":"function","name":"test_tool","parameters":{"type":"object","properties":{}}}]}}`),
		},
		{
			Type: "response.output_text.delta",
			Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello"}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{"type":"response.completed","response":{"id":"resp_123","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[],"conversation":{"id":"conv_abc"},"metadata":{"agent_id":"agent_1"},"background":false,"temperature":1.0,"service_tier":"auto","reasoning":{"effort":"high","summary":"auto"},"parallel_tool_calls":true,"tools":[{"type":"function","name":"test_tool","parameters":{"type":"object","properties":{}}}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`),
		},
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)

	llmStream, err := outbound.TransformStream(context.Background(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	sseStream, err := inbound.TransformStream(context.Background(), llmStream)
	require.NoError(t, err)

	var sseEvents []*httpclient.StreamEvent
	for sseStream.Next() {
		sseEvents = append(sseEvents, sseStream.Current())
	}
	require.NoError(t, sseStream.Err())

	var createdEvent *StreamEvent
	var completedEvent *StreamEvent
	for _, ev := range sseEvents {
		var se StreamEvent
		require.NoError(t, json.Unmarshal(ev.Data, &se))
		switch se.Type {
		case StreamEventTypeResponseCreated:
			createdEvent = &se
		case StreamEventTypeResponseCompleted:
			completedEvent = &se
		}
	}

	require.NotNil(t, createdEvent, "response.created event should exist")
	require.NotNil(t, createdEvent.Response, "response.created should have a Response object")
	assertEchoFields(t, "response.created", createdEvent.Response)

	require.NotNil(t, completedEvent, "response.completed event should exist")
	require.NotNil(t, completedEvent.Response, "response.completed should have a Response object")
	assertEchoFields(t, "response.completed", completedEvent.Response)
}

func assertEchoFields(t *testing.T, label string, resp *Response) {
	t.Helper()

	require.NotNil(t, resp.Conversation, "%s: conversation should be present", label)
	require.Equal(t, "conv_abc", resp.Conversation.ID, "%s: conversation ID", label)

	require.NotNil(t, resp.Metadata, "%s: metadata should be present", label)
	require.Equal(t, "agent_1", resp.Metadata["agent_id"], "%s: metadata agent_id", label)

	require.NotNil(t, resp.ServiceTier, "%s: service_tier should be present", label)
	require.Equal(t, "auto", *resp.ServiceTier, "%s: service_tier value", label)

	require.NotNil(t, resp.Reasoning, "%s: reasoning should be present", label)
	require.Equal(t, "high", resp.Reasoning.Effort, "%s: reasoning effort", label)
	require.Equal(t, "auto", resp.Reasoning.Summary, "%s: reasoning summary", label)

	require.NotNil(t, resp.ParallelToolCalls, "%s: parallel_tool_calls should be present", label)
	require.True(t, *resp.ParallelToolCalls, "%s: parallel_tool_calls value", label)

	require.NotNil(t, resp.Temperature, "%s: temperature should be present", label)
	require.InDelta(t, 1.0, *resp.Temperature, 0.001, "%s: temperature value", label)

	require.NotNil(t, resp.Background, "%s: background should be present", label)
	require.False(t, *resp.Background, "%s: background value", label)

	require.NotEmpty(t, resp.Tools, "%s: tools should be present", label)
	require.Equal(t, "test_tool", resp.Tools[0].Name, "%s: tool name", label)
}
