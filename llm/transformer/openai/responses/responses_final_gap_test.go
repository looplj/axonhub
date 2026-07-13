package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/stretchr/testify/require"
)

// The official Responses output message content union includes refusal parts.
// AggregateStreamChunks must retain the native refusal payload in the final response.
func TestResponsesFinalGap_AggregateStreamChunks_PreservesRefusalContent(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{
			"type":"response.created","sequence_number":0,
			"response":{"id":"resp_refusal","object":"response","created_at":1700000500,"model":"gpt-5","status":"in_progress","output":[]}
		}`)},
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":1,"output_index":0,
			"item":{"id":"msg_refusal","type":"message","status":"in_progress","role":"assistant","content":[]}
		}`)},
		{Type: "response.content_part.added", Data: []byte(`{
			"type":"response.content_part.added","sequence_number":2,"item_id":"msg_refusal","output_index":0,"content_index":0,
			"part":{"type":"refusal","refusal":"I cannot help with that."}
		}`)},
		{Type: "response.content_part.done", Data: []byte(`{
			"type":"response.content_part.done","sequence_number":3,"item_id":"msg_refusal","output_index":0,"content_index":0,
			"part":{"type":"refusal","refusal":"I cannot help with that."}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":4,"output_index":0,
			"item":{"id":"msg_refusal","type":"message","status":"completed","role":"assistant","content":[{"type":"refusal","refusal":"I cannot help with that."}]}
		}`)},
		{Type: "response.completed", Data: []byte(`{
			"type":"response.completed","sequence_number":5,
			"response":{"id":"resp_refusal","object":"response","created_at":1700000500,"model":"gpt-5","status":"completed","output":[]}
		}`)},
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 1)
	require.NotNil(t, response.Output[0].Content)
	require.Len(t, response.Output[0].Content.Items, 1)
	part := response.Output[0].Content.Items[0]
	require.Equal(t, "refusal", part.Type)
	require.NotNil(t, part.Refusal)
	require.Equal(t, "I cannot help with that.", *part.Refusal)
	require.Nil(t, part.Text, "refusal must not be rewritten as output_text")
}

// Web search calls are a fully modeled Responses output item family. Their
// official action union must survive output_item aggregation unchanged.
func TestResponsesFinalGap_AggregateStreamChunks_PreservesWebSearchAction(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{
			"type":"response.created","sequence_number":0,
			"response":{"id":"resp_web","object":"response","created_at":1700000600,"model":"gpt-5","status":"in_progress","output":[]}
		}`)},
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":1,"output_index":0,
			"item":{"id":"ws_123","type":"web_search_call","status":"searching","action":{"type":"search","queries":["axon hub","responses api"],"sources":[{"type":"url","url":"https://example.com"}]}}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":2,"output_index":0,
			"item":{"id":"ws_123","type":"web_search_call","status":"completed","action":{"type":"search","queries":["axon hub","responses api"],"sources":[{"type":"url","url":"https://example.com"}]}}
		}`)},
		{Type: "response.completed", Data: []byte(`{
			"type":"response.completed","sequence_number":3,
			"response":{"id":"resp_web","object":"response","created_at":1700000600,"model":"gpt-5","status":"completed","output":[]}
		}`)},
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 1)
	item := response.Output[0]
	require.Equal(t, "ws_123", item.ID)
	require.Equal(t, "web_search_call", item.Type)
	require.Equal(t, "completed", *item.Status)
	require.NotNil(t, item.Action)
	require.True(t, item.Action.IsWebSearch())
	require.Equal(t, "search", item.Action.WebSearch.Type)
	require.Equal(t, []string{"axon hub", "responses api"}, item.Action.WebSearch.Queries)
	require.Equal(t, []WebSearchSource{{Type: "url", URL: "https://example.com"}}, item.Action.WebSearch.Sources)
}

// The checked-in Responses stream fixture contains modeled image generation
// fields in the authoritative output_item.done snapshot. Aggregation must not
// narrow that native item to result/status only.
func TestResponsesFinalGap_AggregateStreamChunks_PreservesModeledImageFields(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{
			"type":"response.created","sequence_number":0,
			"response":{"id":"resp_image","object":"response","created_at":1700000700,"model":"gpt-image-1","status":"in_progress","output":[]}
		}`)},
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":1,"output_index":0,
			"item":{"id":"ig_123","type":"image_generation_call","status":"in_progress"}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":2,"output_index":0,
			"item":{"id":"ig_123","type":"image_generation_call","status":"completed","action":"edit","background":"transparent","output_format":"webp","quality":"high","result":"aW1hZ2U=","revised_prompt":"A revised prompt","size":"1024x1536"}
		}`)},
		{Type: "response.completed", Data: []byte(`{
			"type":"response.completed","sequence_number":3,
			"response":{"id":"resp_image","object":"response","created_at":1700000700,"model":"gpt-image-1","status":"completed","output":[]}
		}`)},
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 1)
	item := response.Output[0]
	require.NotNil(t, item.Action)
	require.Equal(t, "edit", item.Action.ImageGenerationAction)
	require.Equal(t, "transparent", *item.Background)
	require.Equal(t, "webp", *item.OutputFormat)
	require.Equal(t, "high", *item.Quality)
	require.Equal(t, "A revised prompt", *item.RevisedPrompt)
	require.Equal(t, "1024x1536", *item.Size)
	require.Equal(t, "aW1hZ2U=", *item.Result)
}

// output_item.done is the authoritative final snapshot. Fully modeled function
// and custom tool fields may arrive there even when the added item was skeletal.
func TestResponsesFinalGap_AggregateStreamChunks_UsesFinalToolSnapshots(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{
			"type":"response.created","sequence_number":0,
			"response":{"id":"resp_tools","object":"response","created_at":1700000800,"model":"gpt-5","status":"in_progress","output":[]}
		}`)},
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":1,"output_index":0,
			"item":{"id":"fc_123","type":"function_call","status":"in_progress","arguments":""}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":2,"output_index":0,
			"item":{"id":"fc_123","type":"function_call","status":"completed","call_id":"call_123","name":"lookup","namespace":"catalog","arguments":"{\"id\":7}"}
		}`)},
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":3,"output_index":1,
			"item":{"id":"ct_123","type":"custom_tool_call","status":"in_progress"}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":4,"output_index":1,
			"item":{"id":"ct_123","type":"custom_tool_call","status":"completed","call_id":"custom_call_123","name":"render","namespace":"ui","input":"final input"}
		}`)},
		{Type: "response.completed", Data: []byte(`{
			"type":"response.completed","sequence_number":5,
			"response":{"id":"resp_tools","object":"response","created_at":1700000800,"model":"gpt-5","status":"completed","output":[]}
		}`)},
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 2)
	functionCall := response.Output[0]
	require.Equal(t, "call_123", functionCall.CallID)
	require.Equal(t, "lookup", functionCall.Name)
	require.Equal(t, "catalog", functionCall.Namespace)
	require.Equal(t, `{"id":7}`, functionCall.Arguments)
	customCall := response.Output[1]
	require.Equal(t, "custom_call_123", customCall.CallID)
	require.Equal(t, "render", customCall.Name)
	require.Equal(t, "ui", customCall.Namespace)
	require.NotNil(t, customCall.Input)
	require.Equal(t, "final input", *customCall.Input)
}

// The official web search action union also includes open_page and find_in_page
// fields. These are native Responses fields, not generic tool metadata.
func TestResponsesFinalGap_AggregateStreamChunks_PreservesFindInPageAction(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":0,"output_index":0,
			"item":{"id":"ws_find","type":"web_search_call","status":"in_progress","action":{"type":"find_in_page","url":"https://example.com/guide","pattern":"aggregation"}}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":1,"output_index":0,
			"item":{"id":"ws_find","type":"web_search_call","status":"completed","action":{"type":"find_in_page","url":"https://example.com/guide","pattern":"aggregation"}}
		}`)},
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 1)
	require.NotNil(t, response.Output[0].Action)
	require.NotNil(t, response.Output[0].Action.WebSearch)
	require.Equal(t, "https://example.com/guide", response.Output[0].Action.WebSearch.URL)
	require.Equal(t, "aggregation", response.Output[0].Action.WebSearch.Pattern)
}

// FunctionCall.arguments is required on the official item. An empty final
// string is still authoritative and must replace earlier streamed deltas.
func TestResponsesFinalGap_AggregateStreamChunks_FinalEmptyArgumentsWin(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Type: "response.output_item.added", Data: []byte(`{
			"type":"response.output_item.added","sequence_number":0,"output_index":0,
			"item":{"id":"fc_empty","type":"function_call","status":"in_progress","call_id":"call_empty","name":"noop","arguments":"stale"}
		}`)},
		{Type: "response.output_item.done", Data: []byte(`{
			"type":"response.output_item.done","sequence_number":1,"output_index":0,
			"item":{"id":"fc_empty","type":"function_call","status":"completed","call_id":"call_empty","name":"noop","arguments":""}
		}`)},
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 1)
	require.Empty(t, response.Output[0].Arguments)
}
