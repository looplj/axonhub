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

func TestInboundTransformer_SynthesizesMissingFinishReasonForText(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		openAITextChunk("Hello"),
		openAIUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 4)

	var finish Response
	require.NoError(t, json.Unmarshal(events[2].Data, &finish))
	require.Len(t, finish.Choices, 1)
	require.NotNil(t, finish.Choices[0].Delta)
	require.NotNil(t, finish.Choices[0].FinishReason)
	require.Equal(t, "stop", *finish.Choices[0].FinishReason)
	require.JSONEq(t, `{}`, string(mustMarshalJSON(t, finish.Choices[0].Delta)))
	require.Equal(t, "[DONE]", string(events[3].Data))
}

func TestInboundTransformer_SynthesizesToolCallFinishReason(t *testing.T) {
	toolCall := llm.ToolCall{
		ID:    "call_probe",
		Type:  "function",
		Index: 0,
		Function: llm.FunctionCall{
			Name:      "probe_tool",
			Arguments: `{"value":"ping"}`,
		},
	}

	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "chatcmpl-tool",
			Object:  "chat.completion.chunk",
			Created: 123,
			Model:   "muse-spark-1.2",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ToolCalls: []llm.ToolCall{toolCall}},
			}},
		},
		openAIUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 4)

	var toolChunk Response
	require.NoError(t, json.Unmarshal(events[0].Data, &toolChunk))
	require.Len(t, toolChunk.Choices[0].Delta.ToolCalls, 1)
	require.Equal(t, `{"value":"ping"}`, toolChunk.Choices[0].Delta.ToolCalls[0].Function.Arguments)

	var finish Response
	require.NoError(t, json.Unmarshal(events[2].Data, &finish))
	require.Len(t, finish.Choices, 1)
	require.Equal(t, "tool_calls", lo.FromPtr(finish.Choices[0].FinishReason))
	require.Equal(t, "[DONE]", string(events[3].Data))
}

func TestInboundTransformer_SynthesizesFinishForReasoningOnlyOutput(t *testing.T) {
	reasoning := "thinking"
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "chatcmpl-reasoning",
			Object:  "chat.completion.chunk",
			Created: 123,
			Model:   "muse-spark-1.2",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{ReasoningContent: &reasoning},
			}},
		},
		openAIUsageChunk(),
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

func TestInboundTransformer_PreservesExistingFinishReason(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		openAITextChunk("Hello"),
		{
			ID:      "chatcmpl-finished",
			Object:  "chat.completion.chunk",
			Created: 123,
			Model:   "muse-spark-1.2",
			Choices: []llm.Choice{{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: lo.ToPtr("length"),
			}},
		},
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 2)

	var finish Response
	require.NoError(t, json.Unmarshal(events[1].Data, &finish))
	require.Equal(t, "length", lo.FromPtr(finish.Choices[0].FinishReason))
	require.NotEqual(t, "[DONE]", string(events[1].Data))
}

func TestInboundTransformer_DoesNotDuplicateExistingDone(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		openAITextChunk("Hello"),
		{
			Object: "[DONE]",
		},
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 2)
	require.Equal(t, "[DONE]", string(events[1].Data))
}

func TestInboundTransformer_PreservesUpstreamErrorWithoutSyntheticFinish(t *testing.T) {
	upstreamErr := errors.New("upstream stream failed")
	source := &openAIInboundErrorStream{
		events: []*llm.Response{openAITextChunk("partial"), openAIUsageChunk()},
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

func TestInboundTransformer_DoesNotSynthesizeWithoutUsage(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		openAITextChunk("partial"),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 1)
	require.NotContains(t, string(events[0].Data), `"finish_reason":"stop"`)
}

func TestInboundTransformer_SynthesizesOnlyMissingChoices(t *testing.T) {
	stream, err := NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		{
			ID:      "chatcmpl-multi",
			Object:  "chat.completion.chunk",
			Created: 123,
			Model:   "muse-spark-1.2",
			Choices: []llm.Choice{
				{Index: 0, Delta: &llm.Message{Content: llm.MessageContent{Content: lo.ToPtr("done")}}, FinishReason: lo.ToPtr("stop")},
				{Index: 1, Delta: &llm.Message{Content: llm.MessageContent{Content: lo.ToPtr("pending")}}},
			},
		},
		openAIUsageChunk(),
	}))
	require.NoError(t, err)

	events := collectOpenAIInboundEvents(t, stream)
	require.NoError(t, stream.Err())
	require.Len(t, events, 4)

	var finish Response
	require.NoError(t, json.Unmarshal(events[2].Data, &finish))
	require.Len(t, finish.Choices, 1)
	require.Equal(t, 1, finish.Choices[0].Index)
	require.Equal(t, "stop", lo.FromPtr(finish.Choices[0].FinishReason))
	require.Equal(t, "[DONE]", string(events[3].Data))
}

func openAITextChunk(content string) *llm.Response {
	return &llm.Response{
		ID:      "chatcmpl-text",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "muse-spark-1.2",
		Choices: []llm.Choice{{
			Index: 0,
			Delta: &llm.Message{Content: llm.MessageContent{Content: &content}},
		}},
	}
}

func openAIUsageChunk() *llm.Response {
	return &llm.Response{
		ID:    "chatcmpl-usage",
		Model: "muse-spark-1.2",
		Usage: &llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
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

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return data
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

func (s *openAIInboundErrorStream) Close() error {
	return nil
}
