package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestPersistentStreams_ResponsesTerminalSurvivesProtocolConversion(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		status         string
		details        string
		expectedStatus request.Status
	}{
		{"completed", "response.completed", "completed", "", request.StatusCompleted},
		{"failed", "response.failed", "failed", `,"error":{"message":"provider failed"}`, request.StatusFailed},
		{"incomplete", "response.incomplete", "incomplete", `,"incomplete_details":{"reason":"max_output_tokens"}`, request.StatusFailed},
		{"content filter", "response.incomplete", "incomplete", `,"incomplete_details":{"reason":"content_filter"}`, request.StatusFailed},
		{"canceled", "response.cancelled", "canceled", "", request.StatusCanceled},
		{"completed with failed status", "response.completed", "failed", `,"error":{"message":"provider failed"}`, request.StatusFailed},
		{"completed with incomplete status", "response.completed", "incomplete", `,"incomplete_details":{"reason":"content_filter"}`, request.StatusFailed},
		{"completed with canceled status", "response.completed", "canceled", "", request.StatusCanceled},
	}
	inbounds := []struct {
		transformer.Inbound

		name string
	}{
		{Inbound: openai.NewInboundTransformer(), name: "chat completions"},
		{Inbound: anthropic.NewInboundTransformer(), name: "anthropic"},
		{Inbound: responses.NewInboundTransformer(), name: "responses"},
	}

	for _, inbound := range inbounds {
		for _, tt := range tests {
			t.Run(inbound.name+"/"+tt.name, func(t *testing.T) {
				client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
				defer client.Close()
				ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
				streamCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				project := createTestProject(t, ctx, client)
				channel := createTestChannel(t, ctx, client)
				_, requestService, systemService, usageLogService := setupTestServices(t, client)
				require.NoError(t, systemService.SetStoragePolicy(ctx, &biz.StoragePolicy{
					StoreChunks: true, StoreRequestBody: true, StoreResponseBody: true,
				}))
				req, err := client.Request.Create().
					SetProjectID(project.ID).SetChannelID(channel.ID).SetModelID("gpt-5").
					SetStatus(request.StatusProcessing).SetRequestBody([]byte(`{"stream":true}`)).
					SetStream(true).Save(ctx)
				require.NoError(t, err)
				execution, err := client.RequestExecution.Create().
					SetRequestID(req.ID).SetProjectID(project.ID).SetChannelID(channel.ID).SetModelID("gpt-5").
					SetFormat(llm.APIFormatOpenAIResponse.String()).SetStatus(requestexecution.StatusProcessing).
					SetRequestBody([]byte(`{"stream":true}`)).SetStream(true).Save(ctx)
				require.NoError(t, err)

				events := []*httpclient.StreamEvent{
					{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_terminal","model":"gpt-5","status":"in_progress","output":[]}}`)},
					{Type: tt.eventType, Data: []byte(`{"type":"` + tt.eventType + `","response":{"id":"resp_terminal","status":"` + tt.status + `","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}` + tt.details + `}}`)},
				}
				outbound, err := responses.NewOutboundTransformer("https://example.test", "test-key")
				require.NoError(t, err)
				state := &PersistenceState{}
				raw := NewOutboundPersistentStream(streamCtx, streams.SliceStream(events), req, execution,
					requestService, usageLogService, outbound, nil, state)
				normalized, err := outbound.TransformStream(streamCtx, nil, raw)
				require.NoError(t, err)
				converted, err := inbound.TransformStream(streamCtx, normalized)
				require.NoError(t, err)
				stream := NewInboundPersistentStream(streamCtx, converted, req, execution, requestService, inbound, nil, state)
				var lastEvent *httpclient.StreamEvent
				for stream.Next() {
					lastEvent = stream.Current()
				}
				require.NoError(t, stream.Err())
				if inbound.name == "responses" && (tt.status == "failed" || tt.status == "incomplete") {
					require.NotNil(t, lastEvent)
					require.Equal(t, "response."+tt.status, lastEvent.Type)
				}
				// Persistence must retain the terminal outcome after client disconnect.
				cancel()
				require.NoError(t, stream.Close())

				savedRequest, err := client.Request.Get(ctx, req.ID)
				require.NoError(t, err)
				savedExecution, err := client.RequestExecution.Get(ctx, execution.ID)
				require.NoError(t, err)
				require.Equal(t, tt.expectedStatus, savedRequest.Status)
				require.Equal(t, string(tt.expectedStatus), string(savedExecution.Status))
				require.Equal(t, tt.expectedStatus == request.StatusCompleted, state.StreamCompleted)
				require.Equal(t, tt.status, gjson.GetBytes(savedExecution.ResponseBody, "status").String())
				require.Equal(t, "resp_terminal", savedExecution.ExternalID)
				require.NotEmpty(t, savedRequest.ResponseChunks)
				require.NotEmpty(t, savedExecution.ResponseChunks)
				if inbound.name == "responses" {
					require.Equal(t, gjson.GetBytes(events[1].Data, "response.error.message").String(),
						gjson.GetBytes(savedRequest.ResponseBody, "error.message").String())
					require.Equal(t, gjson.GetBytes(events[1].Data, "response.incomplete_details.reason").String(),
						gjson.GetBytes(savedRequest.ResponseBody, "incomplete_details.reason").String())
				}
			})
		}
	}
}

func TestInboundPersistentStream_ProviderSuccessDoesNotHideConversionFailure(t *testing.T) {
	state := &PersistenceState{OutboundStreamTerminal: streamTerminalCompleted}
	stream := NewInboundPersistentStream(t.Context(), streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "response.failed", Data: []byte(`{"type":"response.failed","response":{"status":"failed"}}`)},
	}), nil, nil, nil, nil, nil, state)
	require.True(t, stream.Next())
	stream.Current()
	require.Equal(t, request.StatusFailed, stream.finalTerminalState().requestStatus())
	require.False(t, state.StreamCompleted)
}

func TestPersistentStreams_DisconnectBeforeConvertedTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_canceled","model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"partial output"}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_canceled","status":"canceled","output":[],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`)},
	}
	outbound, err := responses.NewOutboundTransformer("https://example.test", "test-key")
	require.NoError(t, err)
	state := &PersistenceState{}
	raw := NewOutboundPersistentStream(ctx, streams.SliceStream(events), nil, nil, nil, nil, outbound, nil, state)
	normalized, err := outbound.TransformStream(ctx, nil, raw)
	require.NoError(t, err)
	inbound := responses.NewInboundTransformer()
	converted, err := inbound.TransformStream(ctx, normalized)
	require.NoError(t, err)
	stream := NewInboundPersistentStream(ctx, converted, nil, nil, nil, inbound, nil, state)
	for stream.Next() {
		event := stream.Current()
		if state.OutboundStreamTerminal != streamTerminalNone {
			// The converter first emits output_text.done; the client has not
			// consumed the converted response terminal event yet.
			require.False(t, IsTerminalStreamEvent(event))
			break
		}
	}
	require.Equal(t, streamTerminalCanceled, state.OutboundStreamTerminal)
	require.Equal(t, streamTerminalNone, stream.terminalState)
	cancel()
	require.NoError(t, stream.Close())
	require.Equal(t, streamTerminalCanceled, stream.terminalState)
	require.False(t, state.StreamCompleted)
}
