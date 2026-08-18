package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesWebSocketProcessesSequentialResponseCreateEvents(t *testing.T) {
	requests := make(chan *httpclient.Request, 2)
	process := func(ctx context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		if _, ok := shared.GetSessionID(ctx); !ok {
			return orchestrator.ChatCompletionResult{}, errors.New("missing websocket session ID")
		}
		if !shared.IsResponsesWebSocket(ctx) {
			return orchestrator.ChatCompletionResult{}, errors.New("missing responses websocket marker")
		}
		if _, ok := ctx.Deadline(); !ok {
			return orchestrator.ChatCompletionResult{}, errors.New("missing per-event timeout")
		}
		requests <- request

		return orchestrator.ChatCompletionResult{
			ChatCompletion: nil,
			ChatCompletionStream: streams.SliceStream([]*httpclient.StreamEvent{
				{LastEventID: "", Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_1"}}`), Size: 0},
				{LastEventID: "", Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_1","status":"completed"}}`), Size: 0},
			}),
		}, nil
	}

	server := newResponsesWebSocketTestServer(t, process, nil, time.Second)
	conn := dialResponsesWebSocket(t, server.URL, http.Header{"Authorization": []string{"Bearer test-key"}})
	defer conn.Close()

	for _, input := range []string{
		`{"type":"response.create","model":"gpt-test","stream":false,"background":true,"input":"hello"}`,
		`{"type":"response.create","model":"gpt-test","previous_response_id":"resp_1","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`,
	} {
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(input)))

		_, created, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "response.created", gjson.GetBytes(created, "type").String())

		_, completed, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())
	}

	first := <-requests
	second := <-requests

	require.Equal(t, http.MethodPost, first.Method)
	require.Equal(t, "/v1/responses", first.Path)
	require.Equal(t, "Bearer test-key", first.Headers.Get("Authorization"))
	require.Equal(t, "application/json", first.Headers.Get("Content-Type"))
	require.Empty(t, first.Headers.Get("Connection"))
	require.Empty(t, first.Headers.Get("Upgrade"))
	require.True(t, gjson.GetBytes(first.Body, "stream").Bool())
	require.False(t, gjson.GetBytes(first.Body, "type").Exists())
	require.False(t, gjson.GetBytes(first.Body, "background").Exists())
	require.Equal(t, "response.create", gjson.GetBytes(first.JSONBody, "type").String())
	require.False(t, gjson.GetBytes(first.JSONBody, "stream").Exists())

	require.True(t, gjson.GetBytes(second.Body, "stream").Bool())
	require.Equal(t, "resp_1", gjson.GetBytes(second.Body, "previous_response_id").String())
	require.Equal(t, "response.create", gjson.GetBytes(second.JSONBody, "type").String())
	require.False(t, gjson.GetBytes(second.JSONBody, "stream").Exists())
}

func TestResponsesWebSocketStreamsHTTPSSEResponseIncrementally(t *testing.T) {
	type capturedRequest struct {
		method string
		header http.Header
		body   []byte
	}

	requests := make(chan capturedRequest, 1)
	releaseUpstream := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(releaseUpstream)
		}
	}()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			requests <- capturedRequest{method: r.Method, header: r.Header.Clone(), body: nil}
			return
		}
		requests <- capturedRequest{method: r.Method, header: r.Header.Clone(), body: body}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_sse\"}}\n\n")
		flusher.Flush()

		<-releaseUpstream
		_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_sse\",\"status\":\"completed\"}}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	client := httpclient.NewHttpClientWithClient(upstream.Client())
	defer client.CloseIdleConnections()
	process := func(ctx context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		upstreamRequest := *request
		upstreamRequest.URL = upstream.URL + "/v1/responses"
		upstreamRequest.Auth = nil
		stream, err := client.DoStream(ctx, &upstreamRequest)
		if err != nil {
			return orchestrator.ChatCompletionResult{}, err
		}

		return orchestrator.ChatCompletionResult{ChatCompletion: nil, ChatCompletionStream: stream}, nil
	}

	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))

	_, created, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.created", gjson.GetBytes(created, "type").String(), string(created))

	captured := <-requests
	require.Equal(t, http.MethodPost, captured.method)
	require.Equal(t, "text/event-stream", captured.header.Get("Accept"))
	require.True(t, gjson.GetBytes(captured.body, "stream").Bool())

	close(releaseUpstream)
	released = true

	_, delta, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.output_text.delta", gjson.GetBytes(delta, "type").String())
	_, completed, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())
}

func TestResponsesWebSocketWritesProtocolAndProcessingErrorsWithoutClosingConnection(t *testing.T) {
	process := func(_ context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		if gjson.GetBytes(request.Body, "model").String() == "fail" {
			return orchestrator.ChatCompletionResult{}, errors.New("upstream unavailable")
		}

		return orchestrator.ChatCompletionResult{
			ChatCompletion: &httpclient.Response{
				StatusCode:  http.StatusOK,
				Headers:     nil,
				Body:        []byte(`{"id":"resp_ok","object":"response","status":"completed","output":[]}`),
				Stream:      nil,
				Request:     nil,
				RawResponse: nil,
				RawRequest:  nil,
			},
			ChatCompletionStream: nil,
		}, nil
	}
	transformError := func(_ context.Context, err error) *httpclient.Error {
		return &httpclient.Error{
			Method:     "",
			URL:        "",
			StatusCode: http.StatusBadGateway,
			Status:     "",
			Body:       []byte(`{"error":{"message":"` + err.Error() + `","type":"upstream_error","code":"provider_unavailable"}}`),
			Headers:    nil,
		}
	}

	server := newResponsesWebSocketTestServer(t, process, transformError)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"not.response.create"}`)))
	_, invalidEvent, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "error", gjson.GetBytes(invalidEvent, "type").String())
	require.Equal(t, int64(http.StatusBadRequest), gjson.GetBytes(invalidEvent, "status").Int())
	require.Equal(t, "type", gjson.GetBytes(invalidEvent, "error.param").String())

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"fail","input":"hello"}`)))
	_, processError, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "error", gjson.GetBytes(processError, "type").String())
	require.Equal(t, int64(http.StatusBadGateway), gjson.GetBytes(processError, "status").Int())
	require.Equal(t, "provider_unavailable", gjson.GetBytes(processError, "error.code").String())

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"ok","input":"hello"}`)))
	_, completed, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())
	require.Equal(t, "resp_ok", gjson.GetBytes(completed, "response.id").String())
}

func TestResponsesWebSocketWarmupReturnsIDAndMergesContinuation(t *testing.T) {
	requests := make(chan *httpclient.Request, 2)
	process := func(_ context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		requests <- request
		return orchestrator.ChatCompletionResult{
			ChatCompletion: nil,
			ChatCompletionStream: streams.SliceStream([]*httpclient.StreamEvent{
				{LastEventID: "", Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_generated","status":"completed"}}`), Size: 0},
			}),
		}, nil
	}

	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","generate":false,"previous_response_id":"resp_upstream","instructions":"be concise","input":"hello"}`)))
	_, created, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.created", gjson.GetBytes(created, "type").String())
	warmupID := gjson.GetBytes(created, "response.id").String()
	require.NotEmpty(t, warmupID)
	require.Equal(t, "resp_upstream", gjson.GetBytes(created, "response.previous_response_id").String())

	_, inProgress, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.in_progress", gjson.GetBytes(inProgress, "type").String())

	_, completed, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())
	require.Equal(t, warmupID, gjson.GetBytes(completed, "response.id").String())

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"unrelated"}`)))
	_, unrelated, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "resp_generated", gjson.GetBytes(unrelated, "response.id").String())

	continuation := `{"type":"response.create","model":"gpt-test","previous_response_id":"` + warmupID + `","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(continuation)))
	_, generated, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "resp_generated", gjson.GetBytes(generated, "response.id").String())

	unrelatedRequest := <-requests
	require.Equal(t, "unrelated", gjson.GetBytes(unrelatedRequest.Body, "input").String())
	require.False(t, gjson.GetBytes(unrelatedRequest.Body, "instructions").Exists())

	continuationRequest := <-requests
	require.Equal(t, "be concise", gjson.GetBytes(continuationRequest.Body, "instructions").String())
	require.Equal(t, "resp_upstream", gjson.GetBytes(continuationRequest.Body, "previous_response_id").String())
	require.Len(t, gjson.GetBytes(continuationRequest.Body, "input").Array(), 2)
	require.Equal(t, "hello", gjson.GetBytes(continuationRequest.Body, "input.0.content").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(continuationRequest.Body, "input.1.type").String())
}

func TestResponsesWebSocketRejectsOversizedMessages(t *testing.T) {
	server := newResponsesWebSocketTestServer(t, nil, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()

	message := bytes.Repeat([]byte("x"), responsesWebSocketMaxMessageSize+1)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, message))

	_, _, err := conn.ReadMessage()
	require.Error(t, err)
	var closeErr *websocket.CloseError
	require.ErrorAs(t, err, &closeErr)
	require.Equal(t, websocket.CloseMessageTooBig, closeErr.Code)
}

func TestResponsesWebSocketRequiresUpgrade(t *testing.T) {
	router := gin.New()
	router.GET("/v1/responses", func(c *gin.Context) {
		serveResponsesWebSocket(c, 0, nil, nil)
	})

	request := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "websocket upgrade required")
}

func newResponsesWebSocketTestServer(
	t *testing.T,
	process responsesWebSocketProcessFunc,
	transformError responsesWebSocketErrorFunc,
	requestTimeout ...time.Duration,
) *httptest.Server {
	t.Helper()

	timeout := time.Duration(0)
	if len(requestTimeout) > 0 {
		timeout = requestTimeout[0]
	}

	router := gin.New()
	router.GET("/v1/responses", func(c *gin.Context) {
		serveResponsesWebSocket(c, timeout, process, transformError)
	})

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	return server
}

func dialResponsesWebSocket(t *testing.T, serverURL string, headers http.Header) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/v1/responses"
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if response != nil {
		defer response.Body.Close()
	}
	require.NoError(t, err)

	return conn
}
