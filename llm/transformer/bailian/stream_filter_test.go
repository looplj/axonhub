package bailian

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func collectStream(stream streams.Stream[*llm.Response]) []*llm.Response {
	var out []*llm.Response
	for stream.Next() {
		out = append(out, stream.Current())
	}

	return out
}

func TestBailianStreamFilter_DropsTextAfterToolCalls(t *testing.T) {
	responses := []*llm.Response{
		{
			ID:     "resp_1",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("hello ")},
				},
			}},
		},
		{
			ID:     "resp_1",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("world ")},
				},
			}},
		},
		{
			ID:     "resp_1",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("this is ")},
				},
			}},
		},
		{
			ID:     "resp_1",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{{
						Index: 0,
						Type:  "function",
						Function: llm.FunctionCall{
							Name:      "list_dir",
							Arguments: "{\"path\":\"/\"}",
						},
					}},
				},
			}},
		},
		{
			ID:     "resp_1",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("after tool calls")},
				},
			}},
		},
		{
			ID:     "resp_1",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: lo.ToPtr("tool_calls"),
			}},
		},
	}

	stream := newBailianStreamFilter(streams.SliceStream(responses))
	output := collectStream(stream)

	require.Len(t, output, 7, "expected 7 chunks: 3 text chunks before tool call, tool_call, empty text, flushed text, finish")

	// Verify: text before tool calls is preserved as separate chunks (streaming), text after is buffered
	var textChunks []string

	for _, resp := range output {
		if resp == nil || resp == llm.DoneResponse {
			continue
		}

		for _, choice := range resp.Choices {
			if choice.Delta == nil || choice.Delta.Content.Content == nil {
				continue
			}

			if *choice.Delta.Content.Content != "" {
				textChunks = append(textChunks, *choice.Delta.Content.Content)
			}
		}
	}

	// Multiple text chunks before tool calls should pass through as separate chunks (true streaming)
	// "after tool calls" should be buffered and output at the end
	require.Len(t, textChunks, 4, "expected 3 separate text chunks before tool call + 1 buffered chunk after")
	require.Equal(t, "hello ", textChunks[0], "first text chunk before tool calls should stream through")
	require.Equal(t, "world ", textChunks[1], "second text chunk before tool calls should stream through")
	require.Equal(t, "this is ", textChunks[2], "third text chunk before tool calls should stream through")
	require.Equal(t, "after tool calls", textChunks[3], "text after tool calls should be buffered and output at finish")
}

func TestBailianStreamFilter_IgnoresRedundantEmptyToolArgs(t *testing.T) {
	responses := []*llm.Response{
		{
			ID:     "resp_2",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{{
						Index: 0,
						Type:  "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: "{\"loc\":\"SF\"}",
						},
					}},
				},
			}},
		},
		{
			ID:     "resp_2",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					ToolCalls: []llm.ToolCall{{
						Index: 0,
						Type:  "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: "{}",
						},
					}},
				},
			}},
		},
	}

	stream := newBailianStreamFilter(streams.SliceStream(responses))
	output := collectStream(stream)

	require.Len(t, output, 2)
	second := output[1]
	require.NotNil(t, second)
	require.Len(t, second.Choices, 1)
	require.NotNil(t, second.Choices[0].Delta)
	require.Len(t, second.Choices[0].Delta.ToolCalls, 1)
	require.Empty(t, second.Choices[0].Delta.ToolCalls[0].Function.Arguments, "redundant '{}' should be stripped")
}

func TestBailianStreamFilter_PassesThroughTextWhenNoToolCalls(t *testing.T) {
	responses := []*llm.Response{
		{
			ID:     "resp_3",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("Hello ")},
				},
			}},
		},
		{
			ID:     "resp_3",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("world")},
				},
			}},
		},
		{
			ID:     "resp_3",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: lo.ToPtr("stop"),
			}},
		},
	}

	stream := newBailianStreamFilter(streams.SliceStream(responses))
	output := collectStream(stream)

	// Without tool calls, chunks should pass through unchanged for true streaming
	require.Len(t, output, 3, "all chunks should pass through")

	var textChunks []string

	for _, resp := range output {
		if resp == nil || resp == llm.DoneResponse {
			continue
		}

		for _, choice := range resp.Choices {
			if choice.Delta == nil || choice.Delta.Content.Content == nil {
				continue
			}

			if *choice.Delta.Content.Content != "" {
				textChunks = append(textChunks, *choice.Delta.Content.Content)
			}
		}
	}

	// Text should be streamed as separate chunks, not buffered
	require.Len(t, textChunks, 2)
	require.Equal(t, "Hello ", textChunks[0])
	require.Equal(t, "world", textChunks[1])
}
