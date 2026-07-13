package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneProviderExtensions_RawStreamOptionsDeepCopy(t *testing.T) {
	srcBytes := []byte(`{"include_obfuscation":false,"reasoning_summary_delivery":"sequential_cutoff"}`)
	src := &ProviderExtensions{
		OpenAIResponses: &OpenAIResponsesProviderExtensions{
			Request: &OpenAIResponsesRequestExtensions{
				RawStreamOptions: append(json.RawMessage(nil), srcBytes...),
			},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.OpenAIResponses)
	require.NotNil(t, cloned.OpenAIResponses.Request)
	require.JSONEq(t, string(srcBytes), string(cloned.OpenAIResponses.Request.RawStreamOptions))

	// Mutating the clone must not alias into the source buffer.
	require.True(t, len(cloned.OpenAIResponses.Request.RawStreamOptions) > 0)
	cloned.OpenAIResponses.Request.RawStreamOptions[2] = 'X'
	require.NotEqual(t,
		string(src.OpenAIResponses.Request.RawStreamOptions),
		string(cloned.OpenAIResponses.Request.RawStreamOptions),
		"RawStreamOptions clone must be a deep copy, not an alias",
	)
	require.JSONEq(t, string(srcBytes), string(src.OpenAIResponses.Request.RawStreamOptions))
}

func TestCloneProviderExtensions_RawOutputItemsDeepCopy(t *testing.T) {
	srcBytes := []byte(`{"id":"fs_1","type":"file_search_call"}`)
	src := &ProviderExtensions{
		OpenAIResponses: &OpenAIResponsesProviderExtensions{
			Response: &OpenAIResponsesResponseExtensions{
				RawOutputItems: []OpenAIResponsesRawFragment{{
					Type:          "file_search_call",
					OriginalIndex: 1,
					Raw:           append(json.RawMessage(nil), srcBytes...),
				}},
			},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.OpenAIResponses)
	require.NotNil(t, cloned.OpenAIResponses.Response)
	require.Len(t, cloned.OpenAIResponses.Response.RawOutputItems, 1)
	require.JSONEq(t, string(srcBytes), string(cloned.OpenAIResponses.Response.RawOutputItems[0].Raw))

	cloned.OpenAIResponses.Response.RawOutputItems[0].Raw[2] = 'X'
	require.NotEqual(t,
		string(src.OpenAIResponses.Response.RawOutputItems[0].Raw),
		string(cloned.OpenAIResponses.Response.RawOutputItems[0].Raw),
		"RawOutputItems clone must not alias the original raw item",
	)
	require.JSONEq(t, string(srcBytes), string(src.OpenAIResponses.Response.RawOutputItems[0].Raw))
}

func TestCloneProviderExtensions_RawStreamEventsDeepCopy(t *testing.T) {
	srcBytes := []byte(`{"type":"response.audio.delta","delta":"AQID"}`)
	src := &ProviderExtensions{
		OpenAIResponses: &OpenAIResponsesProviderExtensions{
			Response: &OpenAIResponsesResponseExtensions{
				RawStreamEvents: []OpenAIResponsesRawStreamEvent{{
					Type: "response.audio.delta",
					Raw:  append(json.RawMessage(nil), srcBytes...),
				}},
			},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.Len(t, cloned.OpenAIResponses.Response.RawStreamEvents, 1)
	require.JSONEq(t, string(srcBytes), string(cloned.OpenAIResponses.Response.RawStreamEvents[0].Raw))

	cloned.OpenAIResponses.Response.RawStreamEvents[0].Raw[2] = 'X'
	require.NotEqual(t,
		string(src.OpenAIResponses.Response.RawStreamEvents[0].Raw),
		string(cloned.OpenAIResponses.Response.RawStreamEvents[0].Raw),
		"RawStreamEvents clone must not alias the original raw event",
	)
	require.JSONEq(t, string(srcBytes), string(src.OpenAIResponses.Response.RawStreamEvents[0].Raw))
}

func TestCloneProviderExtensions_ResponsesStatusDeepCopy(t *testing.T) {
	status := "queued"
	src := &ProviderExtensions{
		OpenAIResponses: &OpenAIResponsesProviderExtensions{
			Response: &OpenAIResponsesResponseExtensions{Status: &status},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.OpenAIResponses)
	require.NotNil(t, cloned.OpenAIResponses.Response)
	require.NotNil(t, cloned.OpenAIResponses.Response.Status)
	require.NotSame(t, src.OpenAIResponses.Response.Status, cloned.OpenAIResponses.Response.Status)
	require.Equal(t, "queued", *cloned.OpenAIResponses.Response.Status)

	*cloned.OpenAIResponses.Response.Status = "in_progress"
	require.Equal(t, "queued", *src.OpenAIResponses.Response.Status)
}

func TestCloneProviderExtensions_AnthropicResponseNativeFieldsDeepCopy(t *testing.T) {
	stopSequence := "###END###"
	stopDetails := json.RawMessage(`{"type":"stop_sequence","nested":{"future":true}}`)
	rawUsage := json.RawMessage(`{"input_tokens":3,"output_tokens":1,"server_tool_use":{"web_search_requests":2}}`)
	src := &ProviderExtensions{
		Anthropic: &AnthropicProviderExtensions{
			Response: &AnthropicResponseExtensions{
				StopSequence: &stopSequence,
				StopDetails:  append(json.RawMessage(nil), stopDetails...),
				RawUsage:     append(json.RawMessage(nil), rawUsage...),
			},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.Anthropic)
	require.NotNil(t, cloned.Anthropic.Response)
	require.NotSame(t, src.Anthropic.Response.StopSequence, cloned.Anthropic.Response.StopSequence)
	require.Equal(t, stopSequence, *cloned.Anthropic.Response.StopSequence)
	require.JSONEq(t, string(stopDetails), string(cloned.Anthropic.Response.StopDetails))
	require.JSONEq(t, string(rawUsage), string(cloned.Anthropic.Response.RawUsage))

	*cloned.Anthropic.Response.StopSequence = "changed"
	cloned.Anthropic.Response.StopDetails[2] = 'X'
	cloned.Anthropic.Response.RawUsage[2] = 'X'
	require.Equal(t, stopSequence, *src.Anthropic.Response.StopSequence)
	require.JSONEq(t, string(stopDetails), string(src.Anthropic.Response.StopDetails))
	require.JSONEq(t, string(rawUsage), string(src.Anthropic.Response.RawUsage))
}

func TestCloneProviderExtensions_AnthropicRequestRawContentFragmentsDeepCopy(t *testing.T) {
	srcRaw := json.RawMessage(`{"type":"future_block","payload":{"keep":true}}`)
	src := &ProviderExtensions{
		Anthropic: &AnthropicProviderExtensions{
			Request: &AnthropicRequestExtensions{
				RawContentFragments: []AnthropicRawContentFragment{{
					MessageIndex:       3,
					PartIndex:          2,
					NestedInToolResult: true,
					Raw:                append(json.RawMessage(nil), srcRaw...),
				}},
			},
		},
	}

	cloned := CloneProviderExtensions(src)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.Anthropic)
	require.NotNil(t, cloned.Anthropic.Request)
	require.Len(t, cloned.Anthropic.Request.RawContentFragments, 1)
	fragment := cloned.Anthropic.Request.RawContentFragments[0]
	require.Equal(t, 3, fragment.MessageIndex)
	require.Equal(t, 2, fragment.PartIndex)
	require.True(t, fragment.NestedInToolResult)
	require.JSONEq(t, string(srcRaw), string(fragment.Raw))

	cloned.Anthropic.Request.RawContentFragments[0].Raw[2] = 'X'
	require.NotEqual(t, string(src.Anthropic.Request.RawContentFragments[0].Raw), string(cloned.Anthropic.Request.RawContentFragments[0].Raw))
	require.JSONEq(t, string(srcRaw), string(src.Anthropic.Request.RawContentFragments[0].Raw))
}
