package openai

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func collectFallbackChunks(t *testing.T, input []*llm.Response) []*llm.Response {
	t.Helper()

	source := streams.SliceStream(input)
	fallback := newFinishReasonFallbackStream(source)

	var out []*llm.Response
	for fallback.Next() {
		out = append(out, fallback.Current())
	}
	require.NoError(t, fallback.Err())

	return out
}

func chunkWithFinishReason(finish string) *llm.Response {
	return &llm.Response{
		ID:      "chunk-1",
		Object:  "chat.completion.chunk",
		Choices: []llm.Choice{{Index: 0, FinishReason: lo.ToPtr(finish), Delta: &llm.Message{}}},
	}
}

func plainChunk() *llm.Response {
	return &llm.Response{
		ID:      "chunk-0",
		Object:  "chat.completion.chunk",
		Choices: []llm.Choice{{Index: 0, Delta: &llm.Message{Role: "assistant"}}},
	}
}

func TestFinishReasonFallbackStream_PassthroughWhenFinishReasonPresent(t *testing.T) {
	out := collectFallbackChunks(t, []*llm.Response{plainChunk(), chunkWithFinishReason("stop"), llm.DoneResponse})

	require.Len(t, out, 3)
	require.Equal(t, plainChunk(), out[0])
	require.Equal(t, "stop", lo.FromPtr(out[1].Choices[0].FinishReason))
	require.Equal(t, "[DONE]", out[2].Object)
}

func TestFinishReasonFallbackStream_SynthesizesFinishBeforeDone(t *testing.T) {
	out := collectFallbackChunks(t, []*llm.Response{plainChunk(), llm.DoneResponse})

	require.Len(t, out, 3)
	require.Equal(t, plainChunk(), out[0])
	require.Equal(t, "stop", lo.FromPtr(out[1].Choices[0].FinishReason))
	require.NotNil(t, out[1].Choices[0].Delta)
	require.Equal(t, "[DONE]", out[2].Object)
}

func TestFinishReasonFallbackStream_SynthesizesTerminalSequenceWithoutDone(t *testing.T) {
	// Upstream ended without finish_reason and without [DONE].
	out := collectFallbackChunks(t, []*llm.Response{plainChunk()})

	require.Len(t, out, 3)
	require.Equal(t, plainChunk(), out[0])
	require.Equal(t, "stop", lo.FromPtr(out[1].Choices[0].FinishReason))
	require.Equal(t, "[DONE]", out[2].Object)
}

func TestFinishReasonFallbackStream_ToolCallsFinishReasonPassthrough(t *testing.T) {
	out := collectFallbackChunks(t, []*llm.Response{plainChunk(), chunkWithFinishReason("tool_calls"), llm.DoneResponse})

	require.Len(t, out, 3)
	require.Equal(t, "tool_calls", lo.FromPtr(out[1].Choices[0].FinishReason))
	require.Equal(t, "[DONE]", out[2].Object)
}

func TestFinishReasonFallbackStream_EmptyStreamStaysEmpty(t *testing.T) {
	// A completely empty stream is an upstream failure: no terminal chunk may
	// be synthesized, the pipeline keeps reporting the error.
	out := collectFallbackChunks(t, nil)

	require.Empty(t, out)
}
