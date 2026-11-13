package responses

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type httpTransportCaptureExecutor struct {
	request *httpclient.Request
}

func (e *httpTransportCaptureExecutor) Do(_ context.Context, request *httpclient.Request) (*httpclient.Response, error) {
	e.request = request
	return &httpclient.Response{StatusCode: http.StatusOK}, nil
}

func (e *httpTransportCaptureExecutor) DoStream(_ context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	e.request = request
	return streams.SliceStream([]*httpclient.StreamEvent{}), nil
}

func TestHTTPTransportExecutorStripsWebSocketMetadataAndKeepsContinuation(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.example.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		Transport:      TransportHTTP,
	})
	require.NoError(t, err)

	inner := &httpTransportCaptureExecutor{}
	executor := outbound.CustomizeExecutor(inner)
	request := &httpclient.Request{
		Headers: http.Header{
			"Openai-Beta": {WebSocketBetaHeaderValue + ", other_beta=v1"},
		},
		Body: []byte(`{"model":"gpt-5","previous_response_id":"resp_ws","stream":true,"input":"hello"}`),
	}

	stream, err := executor.DoStream(context.Background(), request)
	require.NoError(t, err)
	require.NoError(t, stream.Close())
	require.NotNil(t, inner.request)
	require.Equal(t, "other_beta=v1", inner.request.Headers.Get("OpenAI-Beta"))
	require.Equal(t, "resp_ws", gjson.GetBytes(inner.request.Body, "previous_response_id").String())
	require.True(t, gjson.GetBytes(inner.request.Body, "stream").Bool())
	require.Equal(t, "hello", gjson.GetBytes(inner.request.Body, "input").String())

	// Transport cleanup must not mutate the request retained by the pipeline.
	require.Equal(t, WebSocketBetaHeaderValue+", other_beta=v1", request.Headers.Get("OpenAI-Beta"))
	require.Equal(t, "resp_ws", gjson.GetBytes(request.Body, "previous_response_id").String())
}

func TestPrepareHTTPTransportRequestKeepsContinuationWithoutInput(t *testing.T) {
	request := &httpclient.Request{
		Headers: http.Header{"Openai-Beta": {WebSocketBetaHeaderValue}},
		Body:    []byte(`{"model":"gpt-5","previous_response_id":"resp_ws","stream":true}`),
	}

	prepared := PrepareHTTPTransportRequest(request, false)
	require.Empty(t, prepared.Headers.Get("OpenAI-Beta"))
	require.Equal(t, "resp_ws", gjson.GetBytes(prepared.Body, "previous_response_id").String())
	require.False(t, gjson.GetBytes(prepared.Body, "input").Exists())
}

func TestHTTPTransportExecutorKeepsNormalHTTPContinuation(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.example.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	inner := &httpTransportCaptureExecutor{}
	executor := outbound.CustomizeExecutor(inner)
	request := &httpclient.Request{
		Headers: http.Header{"Openai-Beta": {"other_beta=v1"}},
		Body:    []byte(`{"model":"gpt-5","previous_response_id":"resp_http","stream":true}`),
	}

	stream, err := executor.DoStream(context.Background(), request)
	require.NoError(t, err)
	require.NoError(t, stream.Close())
	require.Same(t, request, inner.request)
	require.Equal(t, "resp_http", gjson.GetBytes(inner.request.Body, "previous_response_id").String())
	require.Equal(t, "other_beta=v1", inner.request.Headers.Get("OpenAI-Beta"))
}

func TestWebSocketToHTTPTransportExecutorDowngradesCalllessFunctionOutputs(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.example.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		Transport:      TransportHTTP,
	})
	require.NoError(t, err)

	webSocketBody := []byte(`{
		"type":"response.create",
		"stream_id":"lane",
		"model":"gpt-5",
		"input":[
			{"type":"function_call_output","id":"fco_x","name":"create_thread","namespace":"codex_app","output":"<codex_delegation>delegate</codex_delegation>"},
			{"type":"function_call","call_id":"call_1","name":"exec","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
		]
	}`)
	body, err := AdaptRequestBody(webSocketBody, llm.APIFormatOpenAIResponseWebSocket, llm.APIFormatOpenAIResponse)
	require.NoError(t, err)

	inner := &httpTransportCaptureExecutor{}
	executor := outbound.CustomizeExecutor(inner)
	request := &httpclient.Request{Body: body}
	originalBody := append([]byte(nil), request.Body...)

	_, err = executor.Do(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, inner.request)

	callless := gjson.GetBytes(inner.request.Body, "input.0")
	require.Equal(t, "message", callless.Get("type").String())
	require.Equal(t, "user", callless.Get("role").String())
	require.Equal(t, "input_text", callless.Get("content.0.type").String())
	require.Equal(t, "<codex_delegation>delegate</codex_delegation>", callless.Get("content.0.text").String())
	for _, key := range []string{"id", "name", "namespace", "call_id", "output"} {
		require.False(t, callless.Get(key).Exists(), "downgraded item must not retain %s", key)
	}
	require.Equal(t, "function_call", gjson.GetBytes(inner.request.Body, "input.1.type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(inner.request.Body, "input.2.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(inner.request.Body, "input.2.call_id").String())
	require.Equal(t, "ok", gjson.GetBytes(inner.request.Body, "input.2.output").String())
	require.Equal(t, originalBody, request.Body, "transport cleanup must not mutate the retained request")
}

func TestPrepareHTTPTransportRequestPreservesCalllessOutputParts(t *testing.T) {
	request := &httpclient.Request{Body: []byte(`{
		"model":"gpt-5",
		"input":[{"type":"function_call_output","name":"automation_update","call_id":"  ","output":[
			{"type":"input_text","text":"a"},
			{"type":"input_image","image_url":"data:image/png;base64,AA"}
		]}]
	}`)}

	prepared := PrepareHTTPTransportRequest(request, false)
	require.NotSame(t, request, prepared)
	require.Equal(t, "message", gjson.GetBytes(prepared.Body, "input.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(prepared.Body, "input.0.role").String())
	require.JSONEq(t, `[
		{"type":"input_text","text":"a"},
		{"type":"input_image","image_url":"data:image/png;base64,AA"}
	]`, gjson.GetBytes(prepared.Body, "input.0.content").Raw)
}

func TestPrepareHTTPTransportRequestDowngradesCodexBootstrapNames(t *testing.T) {
	for _, toolName := range []string{
		"create_thread",
		"send_message_to_thread",
		"automation_update",
		"codex_apps_open_page",
		"codex_apps_selected_page_part_1_of_1",
	} {
		t.Run(toolName, func(t *testing.T) {
			request := &httpclient.Request{Body: []byte(`{"model":"gpt-5","input":[{"type":"function_call_output","name":"` + toolName + `","output":"delegate"}]}`)}
			prepared := PrepareHTTPTransportRequest(request, false)
			require.Equal(t, "message", gjson.GetBytes(prepared.Body, "input.0.type").String())
			require.Equal(t, "user", gjson.GetBytes(prepared.Body, "input.0.role").String())
			require.Equal(t, "delegate", gjson.GetBytes(prepared.Body, "input.0.content.0.text").String())
		})
	}
}

func TestPrepareHTTPTransportRequestOnlyHandlesCodexBootstrapOutputs(t *testing.T) {
	tests := map[string]string{
		"ordinary tool output":    `{"model":"gpt-5","input":[{"type":"function_call_output","name":"exec_command","output":"ok"}]}`,
		"valid Codex tool output": `{"model":"gpt-5","input":[{"type":"function_call_output","name":"create_thread","call_id":"call_1","output":"ok"}]}`,
		"valid companion output":  `{"model":"gpt-5","input":[{"type":"function_call_output","name":"codex_apps_open_page","call_id":"call_1","output":"ok"}]}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			request := &httpclient.Request{Body: []byte(body)}
			prepared := PrepareHTTPTransportRequest(request, false)
			require.Same(t, request, prepared)
			require.Equal(t, "function_call_output", gjson.GetBytes(prepared.Body, "input.0.type").String())
		})
	}
}

func TestWebSocketTransportKeepsCalllessFunctionOutputs(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.example.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		Transport:      TransportWebSocket,
	})
	require.NoError(t, err)

	for _, toolName := range []string{"create_thread", "codex_apps_open_page"} {
		request := &httpclient.Request{Body: []byte(`{"model":"gpt-5","input":[{"type":"function_call_output","name":"` + toolName + `","output":"delegate"}]}`)}
		require.Same(t, request, outbound.FinalizeTransportRequest(request))
		require.Equal(t, "function_call_output", gjson.GetBytes(request.Body, "input.0.type").String())
	}
}
