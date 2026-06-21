package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

// TestAggregateStreamChunks_CodexImage guards the codex image streaming path.
//
// In this stream the image_generation_call result only ever arrives in
// response.output_item.done (with status "generating" and a string
// "action":"generate"); there is no response.image_generation_call.completed
// event and response.completed carries an empty output array.
//
// Regression: the image item's bare string "action" used to fail
// Item/StreamEvent unmarshalling, the aggregator silently skipped the event,
// and BuildImageResponse reported "codex image response did not contain
// image_generation_call result". Item.Action is now a polymorphic ItemAction.
func TestAggregateStreamChunks_CodexImage(t *testing.T) {
	events := []string{
		`{"type":"response.created","response":{"id":"resp_x","object":"response","created_at":1781880909,"status":"in_progress","model":"gpt-5.4-mini","output":[]},"sequence_number":0}`,
		`{"type":"response.in_progress","response":{"id":"resp_x","object":"response","created_at":1781880909,"status":"in_progress","model":"gpt-5.4-mini","output":[]},"sequence_number":1}`,
		`{"type":"response.output_item.added","item":{"id":"ig_1","type":"image_generation_call","status":"in_progress"},"output_index":0,"sequence_number":2}`,
		`{"type":"response.image_generation_call.in_progress","item_id":"ig_1","output_index":0,"sequence_number":3}`,
		`{"type":"response.image_generation_call.generating","item_id":"ig_1","output_index":0,"sequence_number":4}`,
		`{"type":"response.image_generation_call.partial_image","item_id":"ig_1","output_index":0,"partial_image_b64":"UFART","partial_image_index":0,"sequence_number":5}`,
		`{"type":"response.output_item.done","item":{"id":"ig_1","type":"image_generation_call","status":"generating","action":"generate","background":"opaque","output_format":"png","quality":"low","result":"iVBORw0KGgo=","revised_prompt":"a puppy","size":"1254x1254"},"output_index":0,"sequence_number":6}`,
		`{"type":"response.output_item.added","item":{"id":"msg_1","type":"message","status":"in_progress","content":[],"role":"assistant"},"output_index":1,"sequence_number":7}`,
		`{"type":"response.content_part.added","content_index":0,"item_id":"msg_1","output_index":1,"part":{"type":"output_text","annotations":[],"text":""},"sequence_number":8}`,
		`{"type":"response.output_text.done","content_index":0,"item_id":"msg_1","output_index":1,"text":"","sequence_number":9}`,
		`{"type":"response.content_part.done","content_index":0,"item_id":"msg_1","output_index":1,"part":{"type":"output_text","annotations":[],"text":""},"sequence_number":10}`,
		`{"type":"response.output_item.done","item":{"id":"msg_1","type":"message","status":"completed","content":[{"type":"output_text","annotations":[],"text":""}],"role":"assistant"},"output_index":1,"sequence_number":11}`,
		`{"type":"response.completed","response":{"id":"resp_x","object":"response","created_at":1781880909,"status":"completed","model":"gpt-5.4-mini","output":[]},"sequence_number":12}`,
	}

	chunks := make([]*httpclient.StreamEvent, 0, len(events))
	for _, e := range events {
		chunks = append(chunks, &httpclient.StreamEvent{Data: []byte(e)})
	}

	body, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var upstream Response
	require.NoError(t, json.Unmarshal(body, &upstream))

	resp, err := BuildImageResponse(&upstream, map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, resp.Image)
	require.Len(t, resp.Image.Data, 1)
	require.Equal(t, "iVBORw0KGgo=", resp.Image.Data[0].B64JSON)
}

// TestItemActionUnmarshalJSON verifies the polymorphic action field accepts both
// the image_generation_call string form and the web_search_call object form.
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
	require.Equal(t, "", objAction.ImageGenerationAction)

	var nullAction ItemAction
	require.NoError(t, json.Unmarshal([]byte(`null`), &nullAction))
	require.Equal(t, "", nullAction.ImageGenerationAction)
	require.Nil(t, nullAction.WebSearch)
}
