package openai

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestInboundTransformer_CleanEOFTextWithUsageSynthesizesFinishAndDone(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		openAIInboundTextChunk("hello"),
		openAIInboundUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 4)

	var finish Response
	require.NoError(t, json.Unmarshal(events[2].Data, &finish))
	require.Equal(t, "stop", lo.FromPtr(finish.Choices[0].FinishReason))
	require.Equal(t, "[DONE]", string(events[3].Data))
}

func TestInboundTransformer_CleanEOFReasoningWithUsageSynthesizesFinishAndDone(t *testing.T) {
	reasoning := "thinking"
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "chatcmpl-reasoning",
			Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ReasoningContent: &reasoning}}},
		},
		openAIInboundUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Equal(t, "stop", inboundFinishReason(t, events[2]))
	require.Equal(t, "[DONE]", string(events[3].Data))
}

func TestInboundTransformer_CleanEOFValidToolCallSynthesizesToolFinishAndDone(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID: "chatcmpl-tool",
			Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
				Index:    0,
				Type:     "function",
				Function: llm.FunctionCall{Name: "lookup", Arguments: `{"q":"ping"}`},
			}}}}},
		},
		openAIInboundUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Equal(t, "tool_calls", inboundFinishReason(t, events[2]))
	require.Equal(t, "[DONE]", string(events[3].Data))
}

func TestInboundTransformer_CleanEOFMalformedToolArgumentsDoesNotSucceed(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID: "chatcmpl-tool-invalid",
			Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{ToolCalls: []llm.ToolCall{{
				Index:    0,
				Type:     "function",
				Function: llm.FunctionCall{Name: "lookup", Arguments: `{"q":"unterminated`},
			}}}}},
		},
		openAIInboundUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 2)
	for _, event := range events {
		require.NotEqual(t, "[DONE]", string(event.Data))
	}
}

func TestInboundTransformer_UpstreamErrorDoesNotSynthesizeFinish(t *testing.T) {
	upstreamErr := errors.New("provider disconnected")
	source := &openAIInboundErrorStream{
		events: []*llm.Response{openAIInboundTextChunk("partial"), openAIInboundUsageChunk()},
		err:    upstreamErr,
	}
	stream, err := NewInboundTransformer().TransformStream(t.Context(), source)
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.ErrorIs(t, stream.Err(), upstreamErr)
	require.Len(t, events, 2)
	for _, event := range events {
		require.NotEqual(t, "[DONE]", string(event.Data))
	}
}

func openAIInboundTextChunk(content string) *llm.Response {
	return &llm.Response{
		ID: "chatcmpl-text",
		Choices: []llm.Choice{{
			Index: 0,
			Delta: &llm.Message{Content: llm.MessageContent{Content: &content}},
		}},
	}
}

func openAIInboundUsageChunk() *llm.Response {
	return &llm.Response{
		ID:    "chatcmpl-usage",
		Usage: &llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
}

func collectOpenAIInboundEvents(t *testing.T, stream streams.Stream[*httpclient.StreamEvent]) []*httpclient.StreamEvent {
	t.Helper()
	var events []*httpclient.StreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}
	return events
}

func inboundFinishReason(t *testing.T, event *httpclient.StreamEvent) string {
	t.Helper()
	var response Response
	require.NoError(t, json.Unmarshal(event.Data, &response))
	return lo.FromPtr(response.Choices[0].FinishReason)
}

type openAIInboundErrorStream struct {
	events []*llm.Response
	index  int
	err    error
}

func (s *openAIInboundErrorStream) Next() bool {
	return s.index < len(s.events)
}

func (s *openAIInboundErrorStream) Current() *llm.Response {
	response := s.events[s.index]
	s.index++
	return response
}

func (s *openAIInboundErrorStream) Err() error {
	if s.index >= len(s.events) {
		return s.err
	}
	return nil
}

func (s *openAIInboundErrorStream) Close() error { return nil }
