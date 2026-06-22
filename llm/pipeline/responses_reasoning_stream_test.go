package pipeline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	responsestransformer "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestPipeline_ResponsesReasoningThenMessageCompletes(t *testing.T) {
	inbound := responsestransformer.NewInboundTransformer()
	outbound, err := responsestransformer.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	streamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_reasoning","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAA_added_1"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAA_done_1"}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_1","type":"message","status":"in_progress","role":"assistant","content":[]}}`)},
		{Type: "response.content_part.added", Data: []byte(`{"type":"response.content_part.added","item_id":"msg_1","output_index":1,"content_index":0,"part":{"type":"output_text","text":""}}`)},
		{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":1,"content_index":0,"delta":"hello"}`)},
		{Type: "response.output_text.done", Data: []byte(`{"type":"response.output_text.done","item_id":"msg_1","output_index":1,"content_index":0,"text":"hello"}`)},
		{Type: "response.content_part.done", Data: []byte(`{"type":"response.content_part.done","item_id":"msg_1","output_index":1,"content_index":0,"part":{"type":"output_text","text":"hello"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":1,"item":{"id":"msg_1","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_reasoning","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
	}

	executor := &mockExecutor{doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
		return streams.SliceStream(streamEvents), nil
	}}

	pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound)
	body, err := json.Marshal(map[string]any{
		"model":  "gpt-5",
		"stream": true,
		"input":  "hello",
	})
	require.NoError(t, err)

	result, err := pipe.Process(context.Background(), &httpclient.Request{
		Method:      http.MethodPost,
		URL:         "/v1/responses",
		ContentType: "application/json",
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		Body:        body,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	var eventTypes []string
	for result.EventStream.Next() {
		cur := result.EventStream.Current()
		var ev map[string]any
		require.NoError(t, json.Unmarshal(cur.Data, &ev))
		eventTypes = append(eventTypes, ev["type"].(string))
	}
	require.NoError(t, result.EventStream.Err())
	require.Contains(t, eventTypes, string(responsestransformer.StreamEventTypeResponseCompleted))
}
