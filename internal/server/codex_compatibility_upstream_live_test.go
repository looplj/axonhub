package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

type task6Result struct {
	status      int
	completed   bool
	overloaded  bool
	compactions []json.RawMessage
	events      []string
}

func task6Surface(t *testing.T, h *codexRouteHarness, baseURL, model string) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/codex/responses"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": []string{"Bearer " + h.key.Key}})
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(90*time.Second)))
	message, err := json.Marshal(map[string]any{
		"type":         "response.create",
		"model":        model,
		"instructions": "You are a concise assistant.",
		"input":        []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"input_text","text":"Reply exactly OK."}]}`)},
		"stream":       true,
		"store":        false,
	})
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, message))
	completed := false
	for !completed {
		_, raw, readErr := conn.ReadMessage()
		require.NoError(t, readErr)
		typ := gjson.GetBytes(raw, "type").String()
		require.NotEqual(t, "error", typ, string(raw))
		completed = typ == "response.completed"
	}
	t.Log("GATEWAY WS completed")
}

func task6CLI(t *testing.T, h *codexRouteHarness, baseURL, model string) {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"personal_access_token":"ah-codex-test"}`), 0o600))
	config := "model_provider = \"openai\"\nopenai_base_url = \"" + baseURL + "/codex\"\nchatgpt_base_url = \"" + baseURL + "/codex\"\nmodel = \"" + model + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o600))
	binary, err := exec.LookPath("codex")
	require.NoError(t, err)
	cmd := exec.CommandContext(t.Context(), binary, "exec", "--skip-git-repo-check", "--json", "Reply exactly OK.")
	cmd.Env = append(os.Environ(), "CODEX_HOME="+home, "CODEX_AUTHAPI_BASE_URL="+baseURL+"/codex")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Codex CLI failed: %s", strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return r
		}
		if r >= 32 && r < 127 {
			return r
		}
		return '?'
	}, string(output)))
	t.Log("CODEX CLI completed")
}

func task6Body(model string, input []json.RawMessage) []byte {
	body, err := json.Marshal(struct {
		Model        string            `json:"model"`
		Instructions string            `json:"instructions"`
		Input        []json.RawMessage `json:"input"`
		Stream       bool              `json:"stream"`
		Store        bool              `json:"store"`
	}{model, "You are a concise assistant.", input, true, false})
	if err != nil {
		panic(err)
	}
	return body
}

func task6Parse(status int, raw []byte) task6Result {
	result := task6Result{status: status, overloaded: bytes.Contains(raw, []byte("server_is_overloaded"))}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		event := strings.TrimPrefix(line, "data: ")
		typ := gjson.Get(event, "type").String()
		result.events = append(result.events, typ)
		if typ == "response.completed" {
			result.completed = true
		}
		if typ == "response.output_item.done" && gjson.Get(event, "item.type").String() == "compaction" {
			result.compactions = append(result.compactions, json.RawMessage(gjson.Get(event, "item").Raw))
		}
	}
	return result
}

func task6HTTP(t *testing.T, client *http.Client, req *http.Request) task6Result {
	t.Helper()
	response, err := client.Do(req)
	require.True(t, err == nil, "HTTP transport failed (details suppressed)")
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return task6Parse(response.StatusCode, raw)
}

func task6Post(t *testing.T, destination string, headers http.Header, body []byte) task6Result {
	t.Helper()
	for attempt := 1; attempt <= 6; attempt++ {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, destination, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header = headers.Clone()
		result := task6HTTP(t, &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, req)
		t.Logf("attempt=%d status=%d completed=%t overloaded=%t compaction_count=%d events=%v", attempt, result.status, result.completed, result.overloaded, len(result.compactions), result.events)
		if !result.overloaded || result.completed || attempt == 6 {
			return result
		}
		select {
		case <-time.After(3 * time.Second):
		case <-t.Context().Done():
			t.Fatal("canceled")
		}
	}
	panic("unreachable")
}

func task6Provenance(t *testing.T, h *codexRouteHarness) {
	t.Helper()
	requests := h.db.Request.Query().WithExecutions().AllX(h.ctx)
	for _, req := range requests {
		t.Logf("persisted request id=%d requested_model=%s status=%s selected_channel=%d executions=%d", req.ID, req.ModelID, req.Status, req.ChannelID, len(req.Edges.Executions))
		for _, execution := range req.Edges.Executions {
			statusCode := 0
			if execution.ResponseStatusCode != nil {
				statusCode = *execution.ResponseStatusCode
			}
			t.Logf("persisted execution id=%d channel=%d mapped_model=%s wire_model=%s upstream_reported_model=%s status=%s provider_http=%d request_url_path=%s trigger_count=%d", execution.ID, execution.ChannelID, execution.ModelID, gjson.GetBytes(execution.RequestBody, "model").String(), execution.UpstreamModelID, execution.Status, statusCode, task6URLPath(execution.RequestURL), len(gjson.GetBytes(execution.RequestBody, `input.#(type=="compaction_trigger")#`).Array()))
		}
	}
}

func task6URLPath(raw string) string {
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil {
		return parsed.Path
	}
	return "<invalid>"
}

func TestTask6LiveAcceptance(t *testing.T) {
	if os.Getenv("AXONHUB_CODEX_LIVE_QA") != "1" {
		t.Skip("authorized live QA only")
	}
	h := newCodexRouteHarness(t)
	sources := task6Sources(t, h)
	input := []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"input_text","text":"Reply exactly OK."}]}`)}
	for _, source := range sources {
		t.Run(source.Name, func(t *testing.T) {
			defer task6Provenance(t, h)
			built, err := h.channels.GetChannel(h.ctx, source.ID)
			require.NoError(t, err)
			h.channels.SetEnabledChannelsForTest([]*biz.Channel{built})
			catalog, err := h.channels.FetchCodexCatalog(h.ctx, source.ID, "0.162.1")
			require.NoError(t, err)
			encoded, err := json.Marshal(catalog)
			require.NoError(t, err)
			_ = encoded
			nativeReq, err := codex.ModelsRequest(t.Context(), built.Outbound.(*codex.OutboundTransformer).TokenProvider(), source.BaseURL)
			require.NoError(t, err)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, nativeReq.URL, nil)
			require.NoError(t, err)
			request.Header = nativeReq.Headers
			response, err := built.HTTPClient.GetNativeClient().Do(request)
			require.True(t, err == nil, "catalog transport failed")
			raw, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, 200, response.StatusCode)
			models := gjson.GetBytes(raw, "models").Array()
			require.NotEmpty(t, models)
			selected := models[0].Get("slug").String()
			for _, m := range models {
				if m.Get("slug").String() == "gpt-6.1-sol" {
					selected = m.Get("slug").String()
				}
			}
			headers := nativeReq.Headers.Clone()
			headers.Set("Content-Type", "application/json")
			headers.Set("Originator", "codex_cli_rs")
			headers.Set("User-Agent", "codex_cli_rs/0.162.1")
			t.Logf("DIRECT source=%s candidate_model=%s", source.Name, selected)
			direct := task6Post(t, strings.TrimRight(source.BaseURL, "/#")+"/responses", headers, task6Body(selected, input))
			require.True(t, direct.completed, "direct synthetic inference failed; not a gateway defect")
			source = h.db.Channel.UpdateOneID(source.ID).SetStatus(channel.StatusEnabled).SetSupportedModels([]string{selected}).SetDefaultTestModel(selected).SaveX(h.ctx)
			built, err = h.channels.GetChannel(h.ctx, source.ID)
			require.NoError(t, err)
			h.channels.SetEnabledChannelsForTest([]*biz.Channel{built})
			require.NoError(t, h.system.SetCodexCompatibilitySettings(h.ctx, biz.CodexCompatibilitySettings{Enabled: true, ChannelID: &source.ID}))
			t.Logf("CONFIG source=%s channel_id=%d type=codex enabled_inference_count=1 supported_model=%s mapping=identity catalog_id=%d", source.Name, source.ID, selected, source.ID)
			surface := httptest.NewServer(h.router)
			defer surface.Close()
			gatewayHeaders := http.Header{"Authorization": []string{"Bearer " + h.key.Key}, "Content-Type": []string{"application/json"}, "Originator": []string{"codex_cli_rs"}}
			task6Whoami(t, surface.URL, h.key.Key, source.Name)
			t.Log("GATEWAY SSE")
			sse := task6Post(t, surface.URL+"/codex/responses", gatewayHeaders, task6Body(selected, input))
			require.True(t, sse.completed, "gateway SSE missing completed")
			require.Positive(t, h.db.RequestExecution.Query().Where(requestexecution.ChannelIDEQ(source.ID)).CountX(h.ctx))
			task6Surface(t, h, surface.URL, selected)
			t.Log("GATEWAY V2")
			compact := task6Post(t, surface.URL+"/codex/responses", gatewayHeaders, task6Body(selected, append(input, json.RawMessage(`{"role":"assistant","content":[{"type":"output_text","text":"OK."}]}`), json.RawMessage(`{"type":"compaction_trigger"}`))))
			require.True(t, compact.completed, "gateway v2 missing completed")
			require.Len(t, compact.compactions, 1)
			continuation := append(compact.compactions, input...)
			t.Log("GATEWAY V2 CONTINUATION")
			continued := task6Post(t, surface.URL+"/codex/responses", gatewayHeaders, task6Body(selected, continuation))
			require.True(t, continued.completed, "compaction continuation failed")
			task6CLI(t, h, surface.URL, selected)
			h.db.Channel.UpdateOneID(source.ID).SetStatus(channel.StatusDisabled).SaveX(h.ctx)
			h.channels.SetEnabledChannelsForTest(nil)
		})
	}
}
