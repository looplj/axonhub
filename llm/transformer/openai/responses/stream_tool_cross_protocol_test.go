package responses_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesStreamCorrectedFunctionArgumentsRemainValidAcrossInboundFormats(t *testing.T) {
	const finalArguments = `{"description":"different"}`

	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_cross_protocol","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_cross_protocol","type":"function_call","status":"in_progress","call_id":"call_cross_protocol","name":"spawn_agent","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_cross_protocol","output_index":0,"delta":"{\"description\":\"first\"}"}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_cross_protocol","output_index":0,"arguments":"{\"description\":\"function-done\"}"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_cross_protocol","type":"function_call","status":"completed","call_id":"call_cross_protocol","name":"spawn_agent","arguments":"{\"description\":\"different\"}"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_cross_protocol","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_cross_protocol","type":"function_call","status":"completed","call_id":"call_cross_protocol","name":"spawn_agent","arguments":"{\"description\":\"different\"}"}]}}`)},
	}

	t.Run("OpenAI Chat", func(t *testing.T) {
		canonical := responsesCanonicalStream(t, upstreamEvents)
		client, err := openai.NewInboundTransformer().TransformStream(t.Context(), canonical)
		require.NoError(t, err)

		var arguments strings.Builder
		for client.Next() {
			if string(client.Current().Data) == "[DONE]" {
				continue
			}

			var event openai.Response
			require.NoError(t, json.Unmarshal(client.Current().Data, &event))
			for _, choice := range event.Choices {
				if choice.Delta == nil {
					continue
				}
				for _, toolCall := range choice.Delta.ToolCalls {
					arguments.WriteString(toolCall.Function.Arguments)
				}
			}
		}
		require.NoError(t, client.Err())
		require.Equal(t, finalArguments, arguments.String())
	})

	t.Run("Anthropic Messages", func(t *testing.T) {
		canonical := responsesCanonicalStream(t, upstreamEvents)
		client, err := anthropic.NewInboundTransformer().TransformStream(t.Context(), canonical)
		require.NoError(t, err)

		var arguments strings.Builder
		for client.Next() {
			var event struct {
				Delta *struct {
					PartialJSON *string `json:"partial_json"`
				} `json:"delta"`
			}
			require.NoError(t, json.Unmarshal(client.Current().Data, &event))
			if event.Delta != nil && event.Delta.PartialJSON != nil {
				arguments.WriteString(*event.Delta.PartialJSON)
			}
		}
		require.NoError(t, client.Err())
		require.Equal(t, finalArguments, arguments.String())
	})
}

func TestResponsesStreamMultipleToolsRemainContiguousForAnthropic(t *testing.T) {
	upstreamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_multiple_tools","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_multiple_tools_0","type":"function_call","status":"in_progress","call_id":"call_multiple_tools_0","name":"first","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_multiple_tools_0","output_index":0,"delta":"{\"value\":"}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":1,"item":{"id":"fc_multiple_tools_1","type":"function_call","status":"in_progress","call_id":"call_multiple_tools_1","name":"second","arguments":""}}`)},
		{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_multiple_tools_1","output_index":1,"delta":"{\"value\":2}"}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_multiple_tools_0","output_index":0,"name":"first","arguments":"{\"value\":1}"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_multiple_tools_0","type":"function_call","status":"completed","call_id":"call_multiple_tools_0","name":"first","arguments":"{\"value\":1}"}}`)},
		{Type: "response.function_call_arguments.done", Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_multiple_tools_1","output_index":1,"name":"second","arguments":"{\"value\":2}"}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"fc_multiple_tools_1","type":"function_call","status":"completed","call_id":"call_multiple_tools_1","name":"second","arguments":"{\"value\":2}"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_multiple_tools","object":"response","created_at":1700000000,"model":"gpt-5.5","status":"completed","output":[{"id":"fc_multiple_tools_0","type":"function_call","status":"completed","call_id":"call_multiple_tools_0","name":"first","arguments":"{\"value\":1}"},{"id":"fc_multiple_tools_1","type":"function_call","status":"completed","call_id":"call_multiple_tools_1","name":"second","arguments":"{\"value\":2}"}]}}`)},
	}

	canonical := responsesCanonicalStream(t, upstreamEvents)
	client, err := anthropic.NewInboundTransformer().TransformStream(t.Context(), canonical)
	require.NoError(t, err)

	toolNameByBlock := map[int]string{}
	argumentsByTool := map[string]*strings.Builder{}
	for client.Next() {
		var event struct {
			Type         string `json:"type"`
			Index        *int   `json:"index"`
			ContentBlock *struct {
				Type string  `json:"type"`
				Name *string `json:"name"`
			} `json:"content_block"`
			Delta *struct {
				PartialJSON *string `json:"partial_json"`
			} `json:"delta"`
		}
		require.NoError(t, json.Unmarshal(client.Current().Data, &event))
		if event.Index == nil {
			continue
		}
		if event.Type == "content_block_start" && event.ContentBlock != nil && event.ContentBlock.Name != nil {
			toolNameByBlock[*event.Index] = *event.ContentBlock.Name
			argumentsByTool[*event.ContentBlock.Name] = &strings.Builder{}
		}
		if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.PartialJSON != nil {
			name := toolNameByBlock[*event.Index]
			if argumentsByTool[name] != nil {
				argumentsByTool[name].WriteString(*event.Delta.PartialJSON)
			}
		}
	}
	require.NoError(t, client.Err())
	require.Equal(t, `{"value":1}`, argumentsByTool["first"].String())
	require.Equal(t, `{"value":2}`, argumentsByTool["second"].String())
}

func responsesCanonicalStream(t *testing.T, upstreamEvents []*httpclient.StreamEvent) streams.Stream[*llm.Response] {
	t.Helper()

	upstream, err := responses.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	canonical, err := upstream.TransformStream(t.Context(), nil, streams.SliceStream(upstreamEvents))
	require.NoError(t, err)

	return canonical
}
