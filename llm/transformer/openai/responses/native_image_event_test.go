package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesNativeImageEventPreserved(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.created","response":{"id":"resp_native","model":"gpt-5.5","output":[]}}`)},
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"img_native","type":"image_generation_call","status":"in_progress"}}`)},
		{Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"img_native","type":"image_generation_call","status":"completed","action":"generate","result":"aW1hZ2U="}}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_native","model":"gpt-5.5","status":"completed","output":[]}}`)},
	}
	body, _, err := AggregateStreamChunks(t.Context(), chunks)
	require.NoError(t, err)
	var response Response
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Output, 1)
	require.Equal(t, "image_generation_call", response.Output[0].Type)
	require.Equal(t, "aW1hZ2U=", *response.Output[0].Result)
	var event StreamEvent
	require.NoError(t, json.Unmarshal(chunks[2].Data, &event))
	require.Equal(t, "generate", event.Item.Action.ImageGenerationAction)
}

func TestItemActionUnmarshalJSON(t *testing.T) {
	var strAction ItemAction
	require.NoError(t, json.Unmarshal([]byte(`"generate"`), &strAction))
	require.Equal(t, "generate", strAction.ImageGenerationAction)
	require.Nil(t, strAction.WebSearch)

	var objAction ItemAction
	require.NoError(t, json.Unmarshal([]byte(`{"type":"search","query":"cats"}`), &objAction))
	require.NotNil(t, objAction.WebSearch)
	require.Equal(t, "search", objAction.WebSearch.Type)
	require.Equal(t, "cats", objAction.WebSearch.Query)
	require.Empty(t, objAction.ImageGenerationAction)

	var nullAction ItemAction
	require.NoError(t, json.Unmarshal([]byte(`null`), &nullAction))
	require.Empty(t, nullAction.ImageGenerationAction)
	require.Nil(t, nullAction.WebSearch)
}
