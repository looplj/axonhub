package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	openairesponses "github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesSessionStoreExpandsPreviousResponseForAnyUpstreamTransport(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")

	store.record(ctx,
		[]byte(`{"model":"gpt-5","instructions":"old instructions","stream":true,"input":[{"type":"message","role":"user","content":"hello"}]}`),
		[]byte(`{"id":"resp_1","status":"completed","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"run","arguments":"{}"}]}`),
	)

	prepared, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","instructions":"new instructions","stream":false,"previous_response_id":"resp_1","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`))
	require.JSONEq(t,
		`{"model":"gpt-5","instructions":"new instructions","stream":false,"input":[{"type":"message","role":"user","content":"hello"},{"id":"fc_1","type":"function_call","call_id":"call_1","name":"run","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`,
		string(prepared),
	)
}

func TestResponsesSessionStoreRestoresPersistedCustomToolCallOnMemoryMiss(t *testing.T) {
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	loadCalls := 0
	store := newResponsesSessionStore(func(_ context.Context, responseID string) ([]byte, []byte, bool, error) {
		loadCalls++
		require.Equal(t, "resp_persisted", responseID)
		return []byte(`{"model":"gpt-5","input":[{"type":"message","role":"user","content":"run the tool"}]}`),
			[]byte(`{"id":"resp_persisted","status":"completed","output":[{"id":"ctc_1","type":"custom_tool_call","status":"completed","call_id":"call_1","name":"exec","input":"{}"}]}`),
			true, nil
	})

	prepared, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_persisted","stream":true,"input":[{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`))
	require.JSONEq(t,
		`{"model":"gpt-5","stream":true,"input":[{"type":"message","role":"user","content":"run the tool"},{"id":"ctc_1","type":"custom_tool_call","call_id":"call_1","name":"exec","input":"{}"},{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`,
		string(prepared),
	)
	require.Equal(t, 1, loadCalls)

	request := new(httpclient.Request)
	request.Headers = http.Header{"Openai-Beta": {openairesponses.WebSocketBetaHeaderValue}}
	request.Body = prepared
	httpRequest := openairesponses.PrepareHTTPTransportRequest(request, false)
	require.Empty(t, httpRequest.Headers.Get("Openai-Beta"))
	require.JSONEq(t,
		`{"model":"gpt-5","stream":true,"input":[{"type":"message","role":"user","content":"run the tool"},{"id":"ctc_1","type":"custom_tool_call","call_id":"call_1","name":"exec","input":"{}"},{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`,
		string(httpRequest.Body),
	)

	_, _ = store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_persisted","input":"again"}`))
	require.Equal(t, 1, loadCalls, "restored sessions should be cached in memory")
}

func TestResponsesSessionStoreDoesNotCrossScopes(t *testing.T) {
	store := newResponsesSessionStore()
	owner := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	other := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:2")

	store.record(owner,
		[]byte(`{"model":"gpt-5","input":"hello"}`),
		[]byte(`{"id":"resp_private","status":"completed","output":[]}`),
	)

	body := []byte(`{"model":"gpt-5","previous_response_id":"resp_private","input":"world"}`)
	prepared, _ := store.prepare(other, body)
	require.Equal(t, string(body), string(prepared))
}

func TestResponsesSessionStoreIgnoresMissingScope(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithResponsesAPI(context.Background())
	store.record(ctx,
		[]byte(`{"model":"gpt-5","input":"hello"}`),
		[]byte(`{"id":"resp_unscoped","status":"completed","output":[]}`),
	)

	body := []byte(`{"model":"gpt-5","previous_response_id":"resp_unscoped","input":"world"}`)
	prepared, _ := store.prepare(ctx, body)
	require.Equal(t, string(body), string(prepared))
}

func TestResponsesSessionStoreContinuesAcrossMultipleUpstreamSwitches(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")

	store.record(ctx,
		[]byte(`{"model":"gpt-5","input":"one"}`),
		[]byte(`{"id":"resp_sse","status":"completed","output":[{"type":"message","role":"assistant","content":"two"}]}`),
	)

	second, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_sse","input":"three"}`))
	store.record(ctx, second,
		[]byte(`{"id":"resp_websocket","status":"completed","output":[{"type":"message","role":"assistant","content":"four"}]}`),
	)

	third, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_websocket","input":"five"}`))
	require.JSONEq(t,
		`{"model":"gpt-5","input":[{"type":"message","role":"user","content":"one"},{"type":"message","role":"assistant","content":"two"},{"type":"message","role":"user","content":"three"},{"type":"message","role":"assistant","content":"four"},{"type":"message","role":"user","content":"five"}]}`,
		string(third),
	)
}

func TestResponsesSessionStoreRecordsCompletedStream(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	stream := store.wrapStream(ctx, []byte(`{"model":"gpt-5","input":"one"}`), streams.SliceStream([]*httpclient.StreamEvent{
		{
			LastEventID: "",
			Type:        "response.created",
			Data:        []byte(`{"type":"response.created","response":{"id":"resp_stream","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.output_item.added",
			Data:        []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","status":"in_progress","content":[]}}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.output_item.done",
			Data:        []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"two","annotations":[]}]}}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.completed",
			Data:        []byte(`{"type":"response.completed","response":{"id":"resp_stream","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`),
			Size:        0,
		},
	}))

	for stream.Next() {
	}
	require.NoError(t, stream.Err())
	require.NoError(t, stream.Close())

	prepared, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_stream","input":"three"}`))
	require.JSONEq(t,
		`{"model":"gpt-5","input":[{"type":"message","role":"user","content":"one"},{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"two"}]},{"type":"message","role":"user","content":"three"}]}`,
		string(prepared),
	)
}

func TestResponsesSessionStreamPreservesEveryEventForConsumer(t *testing.T) {
	// Given
	events := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created"}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added"}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed"}`)},
	}
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	stream := store.wrapStream(ctx, []byte(`{"model":"gpt-5","input":"one"}`), streams.SliceStream(events))

	// When
	var received []*httpclient.StreamEvent
	for stream.Next() {
		received = append(received, stream.Current())
	}

	// Then
	require.Equal(t, events, received)
}

func TestResponsesSessionStoreRecordsBeforeStreamClose(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	stream := store.wrapStream(ctx, []byte(`{"model":"gpt-5","input":[{"type":"message","role":"user","content":"run the tool"}]}`), streams.SliceStream([]*httpclient.StreamEvent{
		{
			LastEventID: "",
			Type:        "response.created",
			Data:        []byte(`{"type":"response.created","response":{"id":"resp_tool","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.output_item.added",
			Data:        []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ctc_1","type":"custom_tool_call","call_id":"call_1","name":"exec_command","input":""}}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.custom_tool_call_input.done",
			Data:        []byte(`{"type":"response.custom_tool_call_input.done","output_index":0,"item_id":"ctc_1","input":"{}"}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.output_item.done",
			Data:        []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"ctc_1","type":"custom_tool_call","status":"completed","call_id":"call_1","name":"exec_command","input":"{}"}}`),
			Size:        0,
		},
		{
			LastEventID: "",
			Type:        "response.completed",
			Data:        []byte(`{"type":"response.completed","response":{"id":"resp_tool","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`),
			Size:        0,
		},
	}))

	require.True(t, stream.Next())
	require.True(t, stream.Next())
	require.True(t, stream.Next())
	require.True(t, stream.Next())
	require.True(t, stream.Next())

	prepared, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_tool","input":[{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`))
	require.JSONEq(t,
		`{"model":"gpt-5","input":[{"type":"message","role":"user","content":"run the tool"},{"id":"ctc_1","type":"custom_tool_call","call_id":"call_1","name":"exec_command","input":"{}"},{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`,
		string(prepared),
	)
	require.NoError(t, stream.Close())
}

func TestResponsesSessionStoreNormalizesClientProvidedHistory(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")

	prepared, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","input":[{"id":"ctc_1","type":"custom_tool_call","status":"completed","call_id":"call_1","name":"exec","input":"{}","metadata":{"status":"preserved"}},{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`))
	require.JSONEq(t,
		`{"model":"gpt-5","input":[{"id":"ctc_1","type":"custom_tool_call","call_id":"call_1","name":"exec","input":"{}","metadata":{"status":"preserved"}},{"type":"custom_tool_call_output","call_id":"call_1","output":"ok"}]}`,
		string(prepared),
	)
}

func TestResponsesSessionStoreNormalizesClientHistoryOnCacheMiss(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")

	prepared, _ := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"missing","input":[{"type":"message","role":"assistant","status":"completed","content":"hello"}]}`))
	require.JSONEq(t,
		`{"model":"gpt-5","previous_response_id":"missing","input":[{"type":"message","role":"assistant","content":"hello"}]}`,
		string(prepared),
	)
}

func TestResponsesSessionStoreLeavesStatusFreeInputUnchanged(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	body := []byte(`{ "model": "gpt-5", "input": "hello" }`)

	prepared, _ := store.prepare(ctx, body)
	require.Equal(t, body, prepared)
}

func TestResponsesSessionStoreRestoresSessionIDForSSEToWebSocketCache(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")

	firstBody, firstSessionID := store.prepare(ctx, []byte(`{"model":"gpt-5","input":"one"}`))
	require.NotEmpty(t, firstSessionID)
	store.record(shared.WithSessionID(ctx, firstSessionID), firstBody,
		[]byte(`{"id":"resp_1","status":"completed","output":[]}`),
	)

	_, secondSessionID := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_1","input":"two"}`))
	require.Equal(t, firstSessionID, secondSessionID)
}

func TestResponsesSessionStorePreservesContextSessionIDWhenRecordHasNone(t *testing.T) {
	store := newResponsesSessionStore()
	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "api-key:1")
	store.record(ctx,
		[]byte(`{"model":"gpt-5","input":"one"}`),
		[]byte(`{"id":"resp_1","status":"completed","output":[]}`),
	)

	ctx = shared.WithSessionID(ctx, "context-session")
	_, sessionID := store.prepare(ctx, []byte(`{"model":"gpt-5","previous_response_id":"resp_1","input":"two"}`))
	require.Equal(t, "context-session", sessionID)
}

func TestResponsesSessionStreamStopsBufferingOversizedResponse(t *testing.T) {
	store := newResponsesSessionStore()
	wrapped := store.wrapStream(context.Background(), []byte(`{"model":"gpt-5","input":"one"}`), streams.SliceStream([]*httpclient.StreamEvent{
		{LastEventID: "", Type: "response.completed", Data: bytes.Repeat([]byte("x"), responsesSessionMaxResponse), Size: 0},
	}))

	require.True(t, wrapped.Next())
	require.False(t, wrapped.Next())
	require.NoError(t, wrapped.Close())
	require.True(t, wrapped.(*responsesSessionStream).overflow)
}

func TestResponsesSessionStoreEvictsExpiredRecords(t *testing.T) {
	store := newResponsesSessionStore()
	now := time.Now()
	store.byResponse[responsesSessionKey{scope: "scope", responseID: "expired"}] = &responsesSessionRecord{
		input:     nil,
		output:    nil,
		sessionID: "",
		updatedAt: now.Add(-responsesSessionTTL - time.Minute),
		size:      11,
	}
	store.byResponse[responsesSessionKey{scope: "scope", responseID: "fresh"}] = &responsesSessionRecord{
		input:     nil,
		output:    nil,
		sessionID: "",
		updatedAt: now,
		size:      13,
	}
	store.totalBytes = 24

	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "scope")
	require.NotNil(t, store.lookup(ctx, "fresh"))
	require.Nil(t, store.lookup(ctx, "expired"))
	require.Equal(t, 13, store.totalBytes)
}

func TestResponsesSessionStoreEnforcesMaximumRecordCount(t *testing.T) {
	store := newResponsesSessionStore()
	now := time.Now()
	for i := range responsesSessionMaxRecords {
		key := responsesSessionKey{scope: "scope", responseID: fmt.Sprintf("resp_%d", i)}
		store.byResponse[key] = &responsesSessionRecord{input: nil, output: nil, sessionID: "", updatedAt: now.Add(-time.Minute), size: 1}
	}
	store.totalBytes = responsesSessionMaxRecords

	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "scope")
	store.record(ctx,
		[]byte(`{"model":"gpt-5","input":"latest"}`),
		[]byte(`{"id":"resp_latest","status":"completed","output":[]}`),
	)

	require.LessOrEqual(t, len(store.byResponse), responsesSessionMaxRecords)
	require.NotNil(t, store.lookup(ctx, "resp_latest"))
}

func TestResponsesSessionStoreEnforcesMaximumBytes(t *testing.T) {
	store := newResponsesSessionStore()
	now := time.Now()
	perRecord := responsesSessionMaxBytes / 64
	for i := range 64 {
		key := responsesSessionKey{scope: "scope", responseID: fmt.Sprintf("resp_%d", i)}
		store.byResponse[key] = &responsesSessionRecord{input: nil, output: nil, sessionID: "", updatedAt: now.Add(-time.Minute), size: perRecord}
	}
	store.totalBytes = perRecord * 64

	ctx := shared.WithSessionScope(shared.WithResponsesAPI(context.Background()), "scope")
	store.record(ctx,
		[]byte(`{"model":"gpt-5","input":"latest"}`),
		[]byte(`{"id":"resp_latest","status":"completed","output":[]}`),
	)

	require.LessOrEqual(t, store.totalBytes, responsesSessionMaxBytes)
	require.NotNil(t, store.lookup(ctx, "resp_latest"))
}
