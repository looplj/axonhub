package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const codexFixtureEvents = "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\",\"object\":\"response\",\"model\":\"gpt-test\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"type\":\"compaction\",\"id\":\"cmp_test\",\"encrypted_content\":\"synthetic\"}}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"object\":\"response\",\"model\":\"gpt-test\",\"status\":\"completed\",\"output\":[{\"type\":\"compaction\",\"id\":\"cmp_test\",\"encrypted_content\":\"synthetic\"}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"

func setupCodexResponsesUpstream(t *testing.T, h *codexRouteHarness) *atomic.Int32 {
	t.Helper()
	calls := new(atomic.Int32)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "Bearer upstream", r.Header.Get("Authorization"))
		require.Equal(t, "compaction_trigger", gjson.GetBytes(body, "input.0.type").String())
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, codexFixtureEvents)
	}))
	t.Cleanup(upstream.Close)
	source := h.db.Channel.Create().SetName("responses").SetType(channel.TypeCodex).SetBaseURL(upstream.URL).SetCredentials(objects.ChannelCredentials{APIKey: "upstream"}).SetDefaultTestModel("gpt-test").SetSupportedModels([]string{"gpt-test"}).SaveX(h.ctx)
	built, err := h.channels.GetChannel(h.ctx, source.ID)
	require.NoError(t, err)
	h.channels.SetEnabledChannelsForTest([]*biz.Channel{built})
	require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &h.source.ID}))
	return calls
}

func TestCodexRoutes_HTTPParity(t *testing.T) {
	h := newCodexRouteHarness(t)
	calls := setupCodexResponsesUpstream(t, h)
	var bodies []string
	for _, path := range []string{"/v1/responses", "/codex/responses"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{"model":"gpt-test","instructions":"synthetic","stream":true,"input":[{"type":"compaction_trigger"}]}`))
		req.Header.Set("Authorization", "Bearer "+h.key.Key)
		req.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		h.router.ServeHTTP(result, req)
		require.Equal(t, 200, result.Code, result.Body.String())
		require.Contains(t, result.Body.String(), "response.completed")
		require.Contains(t, result.Body.String(), `"type":"compaction"`)
		bodies = append(bodies, result.Body.String())
	}
	require.Equal(t, bodies[0], bodies[1])
	require.Equal(t, int32(2), calls.Load())
}

func TestCodexRoutes_WebSocketParity(t *testing.T) {
	h := newCodexRouteHarness(t)
	calls := setupCodexResponsesUpstream(t, h)
	surface := httptest.NewServer(h.router)
	defer surface.Close()
	var terminals []string
	for _, path := range []string{"/v1/responses", "/codex/responses"} {
		conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(surface.URL, "http")+path, http.Header{"Authorization": []string{"Bearer " + h.key.Key}})
		require.NoError(t, err)
		require.Equal(t, 101, resp.StatusCode)
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		for range 2 {
			require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","instructions":"synthetic","input":[{"type":"compaction_trigger"}]}`)))
			for {
				_, event, err := conn.ReadMessage()
				require.NoError(t, err)
				eventType := gjson.GetBytes(event, "type").String()
				require.NotEqual(t, "error", eventType, string(event))
				if eventType == "response.completed" {
					terminals = append(terminals, gjson.GetBytes(event, "response.output.0.type").String())
					break
				}
			}

			timer := time.NewTimer(1100 * time.Millisecond)
			select {
			case <-timer.C:
			case <-t.Context().Done():
				timer.Stop()
				t.Fatal("test canceled")
			}
		}
		require.NoError(t, conn.Close())
	}
	require.Equal(t, []string{"compaction", "compaction", "compaction", "compaction"}, terminals)
	require.Equal(t, int32(4), calls.Load())
}

func TestCodexRoutes_CompactionV2(t *testing.T) {
	h := newCodexRouteHarness(t)
	setupCodexResponsesUpstream(t, h)
	surface := httptest.NewServer(h.router)
	defer surface.Close()
	req, err := http.NewRequest("POST", surface.URL+"/codex/responses", strings.NewReader(`{"model":"gpt-test","instructions":"synthetic","stream":true,"input":[{"type":"compaction_trigger"}]}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+h.key.Key)
	req.Header.Set("Content-Type", "application/json")
	response, err := surface.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	done, completed := 0, 0
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]json.RawMessage
		if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) != nil {
			continue
		}
		typ := gjson.Get(line[6:], "type").String()
		switch typ {
		case "response.output_item.done":
			if gjson.Get(line[6:], "item.type").String() == "compaction" {
				done++
			}
		case "response.completed":
			completed++
		}
	}
	require.Equal(t, 1, done)
	require.Equal(t, 1, completed)
	t.Logf("real HTTP handler driver: one compaction item and completed terminal (%d bytes)", len(body))
}

func TestCodexRoutes_DisabledWebSocketUpgrade(t *testing.T) {
	h := newCodexRouteHarness(t)
	surface := httptest.NewServer(h.router)
	defer surface.Close()
	for _, key := range []string{"", "bad", h.key.Key} {
		headers := http.Header{}
		if key != "" {
			headers.Set("Authorization", "Bearer "+key)
		}
		conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(surface.URL, "http")+"/codex/responses", headers)
		require.Error(t, err)
		require.Nil(t, conn)
		require.Equal(t, 404, response.StatusCode)
		require.NoError(t, response.Body.Close())
	}
}
