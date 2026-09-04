package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	responsestransformer "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type responsesErrorAfterEventsStream struct {
	events []*httpclient.StreamEvent
	index  int
	err    error
}

func (s *responsesErrorAfterEventsStream) Next() bool {
	return s.index < len(s.events)
}

func (s *responsesErrorAfterEventsStream) Current() *httpclient.StreamEvent {
	event := s.events[s.index]
	s.index++

	return event
}

func (s *responsesErrorAfterEventsStream) Err() error {
	return s.err
}

func (s *responsesErrorAfterEventsStream) Close() error {
	return nil
}

func TestPipeline_ResponsesDisconnectAfterCompletedDoesNotEmitSecondTerminal(t *testing.T) {
	inbound := responsestransformer.NewInboundTransformer()
	outbound, err := responsestransformer.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	streamEvents := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{"type":"response.created","response":{` +
				`"id":"resp_disconnect","object":"response","created_at":1700000000,` +
				`"model":"gpt-5","status":"in_progress","output":[]}}`),
		},
		{
			Type: "response.output_item.added",
			Data: []byte(`{"type":"response.output_item.added","output_index":0,` +
				`"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"enc_1"}}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{"type":"response.output_item.done","output_index":0,` +
				`"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"enc_1"}}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{"type":"response.completed","response":{` +
				`"id":"resp_disconnect","object":"response","created_at":1700000000,` +
				`"model":"gpt-5","status":"completed","output":[],` +
				`"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}}}`),
		},
	}

	executor := &mockExecutor{doStreamFunc: func(
		ctx context.Context,
		request *httpclient.Request,
	) (streams.Stream[*httpclient.StreamEvent], error) {
		return &responsesErrorAfterEventsStream{
			events: streamEvents,
			err:    fmt.Errorf("read body: %w", io.ErrUnexpectedEOF),
		}, nil
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

	var eventTypes []responsestransformer.StreamEventType
	for result.EventStream.Next() {
		var event responsestransformer.StreamEvent
		require.NoError(t, json.Unmarshal(result.EventStream.Current().Data, &event))
		eventTypes = append(eventTypes, event.Type)
	}

	require.NoError(t, result.EventStream.Err())
	require.Contains(t, eventTypes, responsestransformer.StreamEventTypeResponseCompleted)
	require.NotContains(t, eventTypes, responsestransformer.StreamEventTypeResponseFailed)
	require.NotContains(t, eventTypes, responsestransformer.StreamEventTypeError)
}
