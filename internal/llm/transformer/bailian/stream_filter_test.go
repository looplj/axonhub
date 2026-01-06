package bailian

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

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

func strPtr(s string) *string {
	return &s
}

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
					Content: llm.MessageContent{Content: strPtr("hello ")},
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
				Index:        0,
				FinishReason: strPtr("tool_calls"),
			}},
		},
	}

	stream := newBailianStreamFilter(&mockLLMStream{responses: responses})
	output := collectStream(stream)

	for _, resp := range output {
		if resp == nil || resp == llm.DoneResponse {
			continue
		}
		for _, choice := range resp.Choices {
			if choice.Delta == nil {
				continue
			}
			if choice.Delta.Content.Content != nil {
				require.Empty(t, *choice.Delta.Content.Content, "text delta should be suppressed after tool calls")
			}
			for _, part := range choice.Delta.Content.MultipleContent {
				if part.Type == "text" && part.Text != nil {
					require.Empty(t, *part.Text, "text delta should be suppressed after tool calls")
				}
			}
		}
	}
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

	stream := newBailianStreamFilter(&mockLLMStream{responses: responses})
	output := collectStream(stream)

	require.Len(t, output, 2)
	second := output[1]
	require.NotNil(t, second)
	require.Len(t, second.Choices, 1)
	require.NotNil(t, second.Choices[0].Delta)
	require.Len(t, second.Choices[0].Delta.ToolCalls, 1)
	require.Empty(t, second.Choices[0].Delta.ToolCalls[0].Function.Arguments, "redundant '{}' should be stripped")
}

func TestBailianStreamFilter_FlushesBufferedTextWhenNoToolCalls(t *testing.T) {
	responses := []*llm.Response{
		{
			ID:     "resp_3",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: strPtr("Hello ")},
				},
			}},
		},
		{
			ID:     "resp_3",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Content: llm.MessageContent{Content: strPtr("world")},
				},
			}},
		},
		{
			ID:     "resp_3",
			Object: "chat.completion.chunk",
			Choices: []llm.Choice{{
				Index:        0,
				FinishReason: strPtr("stop"),
			}},
		},
	}

	stream := newBailianStreamFilter(&mockLLMStream{responses: responses})
	output := collectStream(stream)

	var textChunks []string
	for _, resp := range output {
		if resp == nil || resp == llm.DoneResponse {
			continue
		}
		for _, choice := range resp.Choices {
			if choice.Delta == nil || choice.Delta.Content.Content == nil {
				continue
			}
			textChunks = append(textChunks, *choice.Delta.Content.Content)
		}
	}

	require.Len(t, textChunks, 1)
	require.Equal(t, "Hello world", textChunks[0])
}
