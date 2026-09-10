package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/samber/lo"
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

func TestPipeline_ResponsesDisconnectAfterTerminalPreservesOutcome(t *testing.T) {
	tests := []struct {
		name             string
		eventType        string
		status           string
		expectedType     responsestransformer.StreamEventType
		incompleteReason string
		errorDetail      *responsestransformer.Error
	}{
		{name: "completed", eventType: "response.completed", status: "completed", expectedType: responsestransformer.StreamEventTypeResponseCompleted},
		{name: "token limit", eventType: "response.incomplete", status: "incomplete", expectedType: responsestransformer.StreamEventTypeResponseIncomplete, incompleteReason: "max_output_tokens"},
		{name: "content filter", eventType: "response.incomplete", status: "incomplete", expectedType: responsestransformer.StreamEventTypeResponseIncomplete, incompleteReason: "content_filter"},
		{name: "provider reason", eventType: "response.incomplete", status: "incomplete", expectedType: responsestransformer.StreamEventTypeResponseIncomplete, incompleteReason: "provider_limit"},
		{
			name: "failed", eventType: "response.failed", status: "failed", expectedType: responsestransformer.StreamEventTypeResponseFailed,
			errorDetail: &responsestransformer.Error{Type: "server_error", Code: "provider_error", Message: "provider failed", Param: "model"},
		},
		// Cancellation remains compatible with the existing downstream format.
		{name: "cancelled", eventType: "response.cancelled", status: "cancelled", expectedType: responsestransformer.StreamEventTypeResponseCompleted},
		{
			name: "completed with failed status", eventType: "response.completed", status: "failed", expectedType: responsestransformer.StreamEventTypeResponseFailed,
			errorDetail: &responsestransformer.Error{Code: "provider_error", Message: "provider failed"},
		},
		{name: "completed with incomplete status", eventType: "response.completed", status: "incomplete", expectedType: responsestransformer.StreamEventTypeResponseIncomplete, incompleteReason: "content_filter"},
	}
	for _, tt := range tests {
		for _, withUsage := range []bool{false, true} {
			for _, sourceError := range []struct {
				name string
				err  error
			}{
				{"clean EOF", nil},
				{"unexpected EOF", fmt.Errorf("read body: %w", io.ErrUnexpectedEOF)},
				{"canceled", context.Canceled},
				{"deadline", context.DeadlineExceeded},
			} {
				t.Run(fmt.Sprintf("%s/usage=%t/%s", tt.name, withUsage, sourceError.name), func(t *testing.T) {
					response := map[string]any{
						"id": "resp_disconnect", "object": "response", "created_at": 1700000000,
						"model": "gpt-5", "status": tt.status, "output": []any{},
					}
					if tt.incompleteReason != "" {
						response["incomplete_details"] = map[string]string{"reason": tt.incompleteReason}
					}
					if tt.errorDetail != nil {
						response["error"] = tt.errorDetail
					}
					if withUsage {
						response["usage"] = map[string]int{"input_tokens": 100, "output_tokens": 50, "total_tokens": 150}
					}
					terminalData, err := json.Marshal(map[string]any{"type": tt.eventType, "response": response})
					require.NoError(t, err)
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
							Type: tt.eventType,
							Data: terminalData,
						},
					}

					executor := &mockExecutor{doStreamFunc: func(
						ctx context.Context,
						request *httpclient.Request,
					) (streams.Stream[*httpclient.StreamEvent], error) {
						return &responsesErrorAfterEventsStream{
							events: streamEvents,
							err:    sourceError.err,
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
					var terminal *responsestransformer.Response
					terminalCount := 0
					for result.EventStream.Next() {
						var event responsestransformer.StreamEvent
						require.NoError(t, json.Unmarshal(result.EventStream.Current().Data, &event))
						eventTypes = append(eventTypes, event.Type)
						switch event.Type {
						case responsestransformer.StreamEventTypeResponseCompleted,
							responsestransformer.StreamEventTypeResponseFailed,
							responsestransformer.StreamEventTypeResponseIncomplete,
							responsestransformer.StreamEventTypeResponseCancelled:
							require.Equal(t, tt.expectedType, event.Type)
							terminal = event.Response
							terminalCount++
						}
					}

					require.NoError(t, result.EventStream.Err())
					require.Contains(t, eventTypes, tt.expectedType)
					require.Equal(t, 1, terminalCount)
					require.NotNil(t, terminal)
					require.Equal(t, tt.status, lo.FromPtr(terminal.Status))
					require.Equal(t, tt.errorDetail, terminal.Error)
					if tt.incompleteReason != "" {
						require.NotNil(t, terminal.IncompleteDetails)
						require.Equal(t, tt.incompleteReason, terminal.IncompleteDetails.Reason)
					} else {
						require.Nil(t, terminal.IncompleteDetails)
					}
					if withUsage {
						require.NotNil(t, terminal.Usage)
						require.EqualValues(t, 100, terminal.Usage.InputTokens)
						require.EqualValues(t, 50, terminal.Usage.OutputTokens)
					} else {
						require.Nil(t, terminal.Usage)
					}
					require.NotContains(t, eventTypes, responsestransformer.StreamEventTypeError)
					require.NoError(t, result.EventStream.Close())
				})
			}
		}
	}
}
