package responses

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// R2: inbound Responses stream must emit response.completed when finish_reason is
// observed even if no usage chunk ever arrives. Completed must not depend on usage.
func TestR2_InboundStream_CompletedIndependentOfUsage(t *testing.T) {
	source := streams.SliceStream([]*llm.Response{
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_no_usage",
			Model:   "gpt-5",
			Created: 1700000100,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr("hello without usage"),
					},
				},
			}},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_no_usage",
			Model:   "gpt-5",
			Created: 1700000100,
			Choices: []llm.Choice{{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: lo.ToPtr("stop"),
			}},
		},
		// No usage chunk. Stream ends after finish_reason.
	})

	stream, err := NewInboundTransformer().TransformStream(context.Background(), source)
	require.NoError(t, err)

	var events []StreamEvent
	for stream.Next() {
		cur := stream.Current()
		require.NotNil(t, cur)
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(cur.Data, &ev))
		events = append(events, ev)
	}
	require.NoError(t, stream.Err())

	var completed *StreamEvent
	for i := range events {
		if events[i].Type == StreamEventTypeResponseCompleted {
			completed = &events[i]
			break
		}
	}
	require.NotNil(t, completed, "must emit response.completed without any usage chunk")
	require.NotNil(t, completed.Response)
	require.NotNil(t, completed.Response.Status)
	require.Equal(t, "completed", *completed.Response.Status)
	require.Nil(t, completed.Response.Usage, "must not invent usage when source omitted it")
	require.NotEmpty(t, completed.Response.Output)
	require.Equal(t, "hello without usage", lo.FromPtr(completed.Response.Output[0].Content.Items[0].Text))
}

// R2: when usage was already observed before finish_reason, completed should fire on
// finish (not wait for a second usage-bearing chunk).
func TestR2_InboundStream_CompletedWhenUsagePrecededFinish(t *testing.T) {
	source := streams.SliceStream([]*llm.Response{
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_usage_first",
			Model:   "gpt-5",
			Created: 1700000200,
			Choices: []llm.Choice{{
				Index: 0,
				Delta: &llm.Message{
					Role: "assistant",
					Content: llm.MessageContent{
						Content: lo.ToPtr("done"),
					},
				},
			}},
			Usage: &llm.Usage{PromptTokens: 3, CompletionTokens: 1, TotalTokens: 4},
		},
		{
			Object:  "chat.completion.chunk",
			ID:      "resp_usage_first",
			Model:   "gpt-5",
			Created: 1700000200,
			Choices: []llm.Choice{{
				Index:        0,
				Delta:        &llm.Message{},
				FinishReason: lo.ToPtr("stop"),
			}},
			// Finish chunk intentionally omits usage; usage was already tracked.
		},
	})

	stream, err := NewInboundTransformer().TransformStream(context.Background(), source)
	require.NoError(t, err)

	var completed *StreamEvent
	for stream.Next() {
		cur := stream.Current()
		require.NotNil(t, cur)
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(cur.Data, &ev))
		if ev.Type == StreamEventTypeResponseCompleted {
			completed = &ev
			break
		}
	}
	require.NoError(t, stream.Err())
	require.NotNil(t, completed, "finish after prior usage must complete without waiting for another usage chunk")
	require.NotNil(t, completed.Response)
	require.NotNil(t, completed.Response.Usage)
	require.Equal(t, int64(3), completed.Response.Usage.InputTokens)
	require.Equal(t, int64(1), completed.Response.Usage.OutputTokens)
	require.Equal(t, int64(4), completed.Response.Usage.TotalTokens)
}

// R2: once Chat has emitted finish_reason, that protocol terminal signal wins
// over a trailing transport error. Some compatible providers close their SSE
// stream without a clean terminator after the finish chunk; Responses clients
// must still receive response.completed rather than reconnecting the request.
func TestR2_InboundStream_FinishReasonWinsOverTrailingStreamError(t *testing.T) {
	source := &errorResponseStream{
		items: []*llm.Response{
			{
				Object:  "chat.completion.chunk",
				ID:      "resp_finish_then_eof",
				Model:   "grok-4.5",
				Created: 1700000201,
				Choices: []llm.Choice{{
					Index: 0,
					Delta: &llm.Message{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("completed before disconnect"),
						},
					},
				}},
			},
			{
				Object:  "chat.completion.chunk",
				ID:      "resp_finish_then_eof",
				Model:   "grok-4.5",
				Created: 1700000201,
				Choices: []llm.Choice{{
					Index:        0,
					Delta:        &llm.Message{},
					FinishReason: lo.ToPtr("stop"),
				}},
			},
		},
		err: io.ErrUnexpectedEOF,
	}

	stream, err := NewInboundTransformer().TransformStream(context.Background(), source)
	require.NoError(t, err)

	var events []StreamEvent
	for stream.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
		events = append(events, event)
	}
	require.NoError(t, stream.Err(), "trailing transport error after finish must be consumed")

	terminalCount := 0
	for i := range events {
		require.NotEqual(t, StreamEventTypeResponseFailed, events[i].Type)
		require.NotEqual(t, StreamEventTypeError, events[i].Type)
		if events[i].Type == StreamEventTypeResponseCompleted {
			terminalCount++
			require.NotNil(t, events[i].Response)
			require.Equal(t, "completed", lo.FromPtr(events[i].Response.Status))
			require.Equal(t,
				"completed before disconnect",
				lo.FromPtr(events[i].Response.Output[0].Content.Items[0].Text),
			)
		}
	}
	require.Equal(t, 1, terminalCount, "finish_reason must produce exactly one terminal response")
}

// R2: terminal Chat finish reasons must retain the corresponding Responses
// lifecycle event and final response status rather than always becoming
// response.completed.
func TestR2_InboundStream_PreservesTerminalStatus(t *testing.T) {
	tests := []struct {
		name      string
		finish    string
		eventType StreamEventType
		status    string
	}{
		{name: "incomplete", finish: "length", eventType: StreamEventTypeResponseIncomplete, status: "incomplete"},
		{name: "failed", finish: "error", eventType: StreamEventTypeResponseFailed, status: "failed"},
		{name: "cancelled", finish: "cancelled", eventType: StreamEventTypeResponseCancelled, status: "canceled"},
		{name: "canceled", finish: "canceled", eventType: StreamEventTypeResponseCancelled, status: "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := streams.SliceStream([]*llm.Response{
				{
					Object:  "chat.completion.chunk",
					ID:      "resp_terminal_" + tt.name,
					Model:   "gpt-5",
					Created: 1700000201,
					Choices: []llm.Choice{{
						Index:        0,
						Delta:        &llm.Message{},
						FinishReason: lo.ToPtr(tt.finish),
					}},
				},
			})

			stream, err := NewInboundTransformer().TransformStream(context.Background(), source)
			require.NoError(t, err)

			var terminal *StreamEvent
			terminalCount := 0
			for stream.Next() {
				var event StreamEvent
				require.NoError(t, json.Unmarshal(stream.Current().Data, &event))
				if event.Type == tt.eventType {
					terminal = &event
					terminalCount++
				}
				require.NotEqual(t, StreamEventTypeResponseCompleted, event.Type,
					"non-completed finish reason must not also emit response.completed")
			}
			require.NoError(t, stream.Err())
			require.NotNil(t, terminal)
			require.Equal(t, 1, terminalCount)
			require.NotNil(t, terminal.Response)
			require.NotNil(t, terminal.Response.Status)
			require.Equal(t, tt.status, *terminal.Response.Status)
		})
	}
}

// R2: AggregateStreamChunks must not swallow a standalone top-level error event.
// Contract: produce a failed response carrying the Response error object's
// code/message. The stream event's param has no field on Response.Error.
func TestR2_AggregateStreamChunks_TopLevelErrorContract(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"sequence_number":0,
				"response":{
					"id":"resp_top_error",
					"object":"response",
					"created_at":1700000300,
					"model":"gpt-5",
					"status":"in_progress",
					"output":[]
				}
			}`),
		},
		{
			Type: "error",
			Data: []byte(`{
				"type":"error",
				"sequence_number":1,
				"code":"ERR_SOMETHING",
				"message":"Something went wrong",
				"param":"model"
			}`),
		},
	}

	resultBytes, meta, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)
	require.NotNil(t, resultBytes)

	var resp Response
	require.NoError(t, json.Unmarshal(resultBytes, &resp))
	require.Equal(t, "resp_top_error", resp.ID)
	require.Equal(t, "gpt-5", resp.Model)
	require.NotNil(t, resp.Status)
	require.Equal(t, "failed", *resp.Status,
		"top-level error must not leave status looking like a successful/in-progress response")
	require.NotNil(t, resp.Error)
	require.Equal(t, "ERR_SOMETHING", resp.Error.Code)
	require.Equal(t, "Something went wrong", resp.Error.Message)
	// Type is Responses Error.type; top-level SSE error has no separate type field.
	// Keep a stable non-empty classification rather than inventing provider-specific types.
	require.NotEmpty(t, resp.Error.Type)
	require.Equal(t, "resp_top_error", meta.ID)
}

// R2: error-only stream (no response.created) still surfaces a failed envelope.
func TestR2_AggregateStreamChunks_ErrorOnlyStream(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Type: "error",
			Data: []byte(`{"type":"error","sequence_number":0,"code":"server_error","message":"upstream boom","param":null}`),
		},
	}

	resultBytes, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(resultBytes, &resp))
	require.NotNil(t, resp.Status)
	require.Equal(t, "failed", *resp.Status)
	require.NotNil(t, resp.Error)
	require.Equal(t, "server_error", resp.Error.Code)
	require.Equal(t, "upstream boom", resp.Error.Message)
}

// A response.output_item lifecycle may carry a compaction item. It is already
// a native Responses Item, so aggregation must retain it rather than treating
// it like an unsupported stream event.
func TestR2_AggregateStreamChunks_CompactionOutputItem(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{
				"type":"response.created",
				"sequence_number":0,
				"response":{
					"id":"resp_compaction",
					"object":"response",
					"created_at":1700000400,
					"model":"gpt-5",
					"status":"in_progress",
					"output":[]
				}
			}`),
		},
		{
			Type: "response.output_item.added",
			Data: []byte(`{
				"type":"response.output_item.added",
				"sequence_number":1,
				"output_index":0,
				"item":{
					"id":"ctc_123",
					"type":"compaction",
					"status":"in_progress",
					"encrypted_content":"enc_payload",
					"created_by":"user_42"
				}
			}`),
		},
		{
			Type: "response.output_item.done",
			Data: []byte(`{
				"type":"response.output_item.done",
				"sequence_number":2,
				"output_index":0,
				"item":{
					"id":"ctc_123",
					"type":"compaction",
					"status":"completed",
					"encrypted_content":"enc_payload",
					"created_by":"user_42"
				}
			}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{
				"type":"response.completed",
				"sequence_number":3,
				"response":{"id":"resp_compaction","status":"completed","output":[]}
			}`),
		},
	}

	resultBytes, _, err := AggregateStreamChunks(context.Background(), chunks)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(resultBytes, &resp))
	require.Len(t, resp.Output, 1)
	require.Equal(t, "ctc_123", resp.Output[0].ID)
	require.Equal(t, "compaction", resp.Output[0].Type)
	require.Equal(t, "completed", lo.FromPtr(resp.Output[0].Status))
	require.Equal(t, "enc_payload", lo.FromPtr(resp.Output[0].EncryptedContent))
	require.Equal(t, "user_42", lo.FromPtr(resp.Output[0].CreatedBy))
}
