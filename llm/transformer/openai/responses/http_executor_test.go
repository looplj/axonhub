package responses

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

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

func TestHTTPTransportExecutorStripsWebSocketContinuation(t *testing.T) {
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
	require.False(t, gjson.GetBytes(inner.request.Body, "previous_response_id").Exists())
	require.True(t, gjson.GetBytes(inner.request.Body, "stream").Bool())
	require.Equal(t, "hello", gjson.GetBytes(inner.request.Body, "input").String())

	// Transport cleanup must not mutate the request retained by the pipeline.
	require.Equal(t, WebSocketBetaHeaderValue+", other_beta=v1", request.Headers.Get("OpenAI-Beta"))
	require.Equal(t, "resp_ws", gjson.GetBytes(request.Body, "previous_response_id").String())
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
