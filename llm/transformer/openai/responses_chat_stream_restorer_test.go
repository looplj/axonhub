package openai

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func TestResponsesChatToolStreamRestorer_FragmentedCustomCallPerChoice(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(
		map[string]responsesChatToolMapping{
			"axonhub_client_tool": {
				Kind:     responsesChatToolCustom,
				ChatName: "axonhub_client_tool",
				Name:     "exec",
			},
		},
		[]string{"thread_create", "axonhub_client_tool"},
	)

	first := &llm.Response{Choices: []llm.Choice{
		{
			Index: 0,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{Index: 0, Function: llm.FunctionCall{Name: "thr"}},
				{Index: 1, Function: llm.FunctionCall{Name: "axonhub_client_", Arguments: `{"code`}},
			}},
		},
		{
			Index: 1,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{Index: 0, Function: llm.FunctionCall{Name: "thr"}},
				{Index: 1, Function: llm.FunctionCall{Name: "axonhub_client_", Arguments: `{"code`}},
			}},
		},
	}}
	restorer.restore(first)
	require.Empty(t, first.Choices[0].Delta.ToolCalls)
	require.Empty(t, first.Choices[1].Delta.ToolCalls)

	second := &llm.Response{Choices: []llm.Choice{
		{
			Index: 0,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{ID: "call_thread_0", Index: 0, Function: llm.FunctionCall{Name: "ead_create"}},
				{ID: "call_exec_0", Index: 1, Function: llm.FunctionCall{Name: "tool", Arguments: `":"go"}`}},
			}},
		},
		{
			Index: 1,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{ID: "call_thread_1", Index: 0, Function: llm.FunctionCall{Name: "ead_create"}},
				{ID: "call_exec_1", Index: 1, Function: llm.FunctionCall{Name: "tool", Arguments: `":"python"}`}},
			}},
		},
	}}
	restorer.restore(second)

	for _, choice := range second.Choices {
		require.Len(t, choice.Delta.ToolCalls, 2)
		require.Equal(t, "thread_create", choice.Delta.ToolCalls[0].Function.Name)

		custom := choice.Delta.ToolCalls[1]
		require.Equal(t, llm.ToolTypeResponsesCustomTool, custom.Type)
		require.Equal(t, "call_exec_"+map[bool]string{true: "0", false: "1"}[choice.Index == 0], custom.ID)
		require.Equal(t, "exec", custom.ResponseCustomToolCall.Name)
		require.Equal(t, `{"code":"`+map[bool]string{true: "go", false: "python"}[choice.Index == 0]+`"}`, custom.ResponseCustomToolCall.Input)
	}
	require.Empty(t, restorer.flushBuffered())
}

func TestResponsesChatToolStreamRestorer_AbnormalFinishDoesNotReleaseOtherChoice(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(nil, []string{"thread_create"})

	first := &llm.Response{Choices: []llm.Choice{
		{
			Index: 0,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{Index: 0, Function: llm.FunctionCall{Name: "thr"}},
				{ID: "call_ready_0", Index: 1, Function: llm.FunctionCall{Name: "thread_create", Arguments: "{}"}},
			}},
		},
		{
			Index: 1,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{
				{Index: 0, Function: llm.FunctionCall{Name: "thr"}},
				{ID: "call_ready_1", Index: 1, Function: llm.FunctionCall{Name: "thread_create", Arguments: "{}"}},
			}},
		},
	}}
	restorer.restore(first)
	require.Empty(t, first.Choices[0].Delta.ToolCalls)
	require.Empty(t, first.Choices[1].Delta.ToolCalls)

	abnormal := &llm.Response{Choices: []llm.Choice{{
		Index:        1,
		FinishReason: lo.ToPtr("length"),
		Delta: &llm.Message{ToolCalls: []llm.ToolCall{
			{ID: "call_pending_1", Index: 0, Function: llm.FunctionCall{Name: "ead_create"}},
			{Index: 1, Function: llm.FunctionCall{Arguments: `{"done":true}`}},
		}},
	}}}
	restorer.restore(abnormal)

	require.Len(t, abnormal.Choices, 1)
	calls := abnormal.Choices[0].Delta.ToolCalls
	require.Len(t, calls, 2)
	require.Equal(t, "call_pending_1", calls[0].ID)
	require.Equal(t, "call_ready_1", calls[1].ID)
	require.Equal(t, `{}{"done":true}`, calls[1].Function.Arguments)

	flushed := restorer.flushBuffered()
	require.Len(t, flushed, 1)
	require.Equal(t, 0, flushed[0].Choices[0].Index)
	require.Len(t, flushed[0].Choices[0].Delta.ToolCalls, 2)
	require.Equal(t, "thr", flushed[0].Choices[0].Delta.ToolCalls[0].Function.Name)
	require.Equal(t, "call_ready_0", flushed[0].Choices[0].Delta.ToolCalls[1].ID)
}

func TestResponsesChatToolStreamRestorer_TracksFinishWithoutTools(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(nil)
	restorer.restore(&llm.Response{Choices: []llm.Choice{{
		Index:        2,
		FinishReason: lo.ToPtr("stop"),
	}}})

	require.Empty(t, restorer.unfinishedChoiceIndexes())
	require.Empty(t, restorer.flushBuffered())
}

func TestResponsesChatToolFlushStream_SyntheticFinishCoversOnlyUnfinishedChoices(t *testing.T) {
	restorer := newResponsesChatToolStreamRestorer(nil)
	restorer.restore(&llm.Response{Choices: []llm.Choice{
		{Index: 0, Delta: &llm.Message{}},
		{Index: 1, FinishReason: lo.ToPtr("stop"), Delta: &llm.Message{}},
		{Index: 2, Delta: &llm.Message{}},
	}})

	stream := &responsesChatToolFlushStream{
		inner:        streams.SliceStream([]*llm.Response{llm.DoneResponse}),
		restorer:     restorer,
		strictFinish: true,
	}

	var unfinishedIndexes []int
	sawDone := false
	for stream.Next() {
		response := stream.Current()
		if response == llm.DoneResponse {
			sawDone = true
			continue
		}
		require.Len(t, response.Choices, 1)
		if response.Choices[0].FinishReason != nil {
			require.Equal(t, "stop", *response.Choices[0].FinishReason)
			unfinishedIndexes = append(unfinishedIndexes, response.Choices[0].Index)
		}
	}
	require.NoError(t, stream.Err())
	require.True(t, sawDone)
	require.Equal(t, []int{0, 2}, unfinishedIndexes)
}
