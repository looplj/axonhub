package api

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestResponsesProtocolIntegrationPartialOutputDoesNotReplay(t *testing.T) {
	var dispatches atomic.Int32
	upstream := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatches.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, err := io.WriteString(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"partial-provider\"}}\n\n")
		require.NoError(t, err)
		_, err = io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"partial-item\",\"delta\":\"partial-canary\"}\n\n")
		require.NoError(t, err)
		flusher.Flush()
	}))
	client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL)
	gateway := responsesIntegrationGateway(t, client, handlers, project)

	status, body := responsesIntegrationPost(t, gateway, "/v1/responses", `{"model":"synthetic-model","input":"partial-input","stream":true}`)

	// Then: an exposed partial stream is not replayed through another upstream attempt.
	require.Equal(t, http.StatusOK, status, string(body))
	require.Equal(t, int32(1), dispatches.Load())
	require.Contains(t, string(body), "partial-canary")
}

func TestResponsesProtocolIntegrationDownstreamCancellationDrainsUpstream(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	upstream := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		require.NoError(t, err)
		close(started)
		select {
		case <-r.Context().Done():
			close(canceled)
		case <-time.After(2 * time.Second):
		}
	}))
	client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL)
	gateway := responsesIntegrationGateway(t, client, handlers, project)
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/v1/responses", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"cancel-lane","model":"synthetic-model","input":"cancel-canary"}`)))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream request did not start")
	}
	require.NoError(t, conn.Close())

	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("downstream cancellation did not drain upstream request")
	}
}

func TestResponsesProtocolIntegrationRotatedAuthAndEndpointReachProductionListener(t *testing.T) {
	var path atomic.Value
	var auth atomic.Value
	upstream := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path.Store(r.URL.Path)
		auth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, `{"id":"rotated-response","object":"response","status":"completed","output":[{"type":"message","id":"rotated-item"}]}`)
		require.NoError(t, err)
	}))
	client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL, func(create *ent.ChannelCreate) {
		create.SetCredentials(objects.ChannelCredentials{APIKey: "rotated-key"})
		create.SetEndpoints([]objects.ChannelEndpoint{{APIFormat: "openai/responses", Path: "/custom/responses"}})
	})
	gateway := responsesIntegrationGateway(t, client, handlers, project)

	status, body := responsesIntegrationPost(t, gateway, "/v1/responses", `{"model":"synthetic-model","input":"rotation-canary"}`)

	// Then: the selected channel's current credentials and endpoint path reach the local provider.
	require.Equal(t, http.StatusOK, status, string(body))
	require.Equal(t, "/custom/responses", path.Load())
	require.Equal(t, "Bearer rotated-key", auth.Load())
	require.Equal(t, "rotated-response", gjson.GetBytes(body, "id").String())
	require.Equal(t, "rotated-item", gjson.GetBytes(body, "output.0.id").String())
}

func TestResponsesProtocolIntegrationAdjacentImagesStayNativeHTTP(t *testing.T) {
	type observedRequest struct {
		path string
		body []byte
	}
	requests := make(chan observedRequest, 2)
	upstream := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := readRequestBody(r)
		require.NoError(t, err)
		requests <- observedRequest{path: r.URL.Path, body: body}
		w.Header().Set("Content-Type", "application/json")
		response := `{"created":1700000000,"data":[{"b64_json":"synthetic-image"}]}`
		if r.URL.Path == "/v1/images/edits" {
			response = `{"created":1700000001,"data":[{"b64_json":"synthetic-edited-image"}]}`
		}
		_, err = io.WriteString(w, response)
		require.NoError(t, err)
	}))
	client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL, func(create *ent.ChannelCreate) {
		create.SetEndpoints([]objects.ChannelEndpoint{
			{APIFormat: "openai/responses"},
			{APIFormat: "openai/image_generation"},
			{APIFormat: "openai/image_edit"},
		})
	})
	gateway := responsesIntegrationGateway(t, client, handlers, project)

	generationStatus, generationBody := responsesIntegrationPost(t, gateway, "/v1/images/generations", `{"model":"synthetic-model","prompt":"red square"}`)
	editStatus, editBody := responsesIntegrationPost(t, gateway, "/v1/images/edits", `{"model":"synthetic-model","image":"data:image/png;base64,c3ludGhldGljLWltYWdl","prompt":"blue square"}`)

	// Then: both image operations use their native HTTP paths and preserve their own artifacts.
	require.Equal(t, http.StatusOK, generationStatus, string(generationBody))
	require.Equal(t, http.StatusOK, editStatus, string(editBody))
	first := <-requests
	second := <-requests
	require.Equal(t, "/v1/images/generations", first.path)
	require.Equal(t, "/v1/images/edits", second.path)
	require.Contains(t, string(first.body), "red square")
	require.Contains(t, string(second.body), "synthetic-image")
	require.NotContains(t, string(first.body), "response.create")
	require.NotContains(t, string(second.body), "response.create")
	require.Equal(t, "synthetic-image", gjson.GetBytes(generationBody, "data.0.b64_json").String())
	require.Equal(t, "synthetic-edited-image", gjson.GetBytes(editBody, "data.0.b64_json").String())
}
