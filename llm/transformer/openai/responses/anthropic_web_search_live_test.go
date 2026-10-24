package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

// Opt in with AXON_SEARCH_LIVE_URL, AXON_SEARCH_LIVE_KEY_FILE and AXON_SEARCH_LIVE_MODEL. This uses
// a temporary gateway with the checkout's transformers, not a deployed binary.
func TestClaudeCodeWebSearchLive(t *testing.T) {
	base := os.Getenv("AXON_SEARCH_LIVE_URL")
	if base == "" {
		t.Skip("live Claude Code / AxonHub test is opt-in")
	}
	model := os.Getenv("AXON_SEARCH_LIVE_MODEL")
	require.NotEmpty(t, model, "AXON_SEARCH_LIVE_MODEL must select a search-capable model")
	raw, err := os.ReadFile(os.Getenv("AXON_SEARCH_LIVE_KEY_FILE"))
	require.NoError(t, err)
	var key struct {
		Key string `json:"key"`
	}
	require.NoError(t, json.Unmarshal(raw, &key))
	require.NotEmpty(t, key.Key)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	in := anthropic.NewInboundTransformer()
	out, err := NewOutboundTransformer(base, key.Key)
	require.NoError(t, err)
	var searches atomic.Int32
	var completedSearch atomic.Bool
	var serverUses, serverResults, resultLinks atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		isSearch := false
		for _, tool := range gjson.GetBytes(body, "tools").Array() {
			if tool.Get("type").String() == "web_search_20250305" {
				isSearch = true
			}
		}
		t.Logf("gateway request path=%s search=%t", r.URL.Path, isSearch)
		target := base + r.URL.RequestURI()
		var converted *httpclient.Request
		if isSearch {
			unified, e := in.TransformRequest(r.Context(), &httpclient.Request{Body: body, Headers: r.Header})
			if e != nil {
				t.Logf("gateway transform error: %v", e)
				http.Error(w, e.Error(), http.StatusBadRequest)
				return
			}
			converted, e = out.TransformRequest(r.Context(), unified)
			if e != nil {
				t.Logf("gateway transform error: %v", e)
				http.Error(w, e.Error(), http.StatusBadRequest)
				return
			}
			forcedSearch := gjson.GetBytes(body, "tool_choice.type").String() == "tool" && gjson.GetBytes(body, "tool_choice.name").String() == "web_search"
			body = converted.Body
			target = base + "/v1/responses"
			if forcedSearch && gjson.GetBytes(body, "tool_choice.type").String() != "web_search" {
				t.Logf("choice: %s", gjson.GetBytes(body, "tool_choice").Raw)
				http.Error(w, "incorrect search choice", http.StatusBadRequest)
				return
			}
			searches.Add(1)
		}
		upstream, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		upstream.Header.Set("Content-Type", "application/json")
		upstream.Header.Set("Authorization", "Bearer "+key.Key)
		upstream.Header.Set("Anthropic-Version", "2023-06-01")
		response, err := http.DefaultClient.Do(upstream)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		if !isSearch || response.StatusCode != http.StatusOK {
			w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
			w.WriteHeader(response.StatusCode)
			// Flush streamed main-agent responses promptly.
			buf := make([]byte, 8192)
			for {
				n, e := response.Body.Read(buf)
				if n > 0 {
					_, _ = w.Write(buf[:n])
					w.(http.Flusher).Flush()
				}
				if e != nil {
					break
				}
			}
			return
		}
		if !gjson.GetBytes(body, "stream").Bool() {
			data, e := io.ReadAll(response.Body)
			if e != nil {
				t.Logf("gateway transform error: %v", e)
				http.Error(w, e.Error(), http.StatusBadRequest)
				return
			}
			unified, e := out.TransformResponse(r.Context(), &httpclient.Response{StatusCode: 200, Body: data})
			if e != nil {
				t.Logf("gateway transform error: %v", e)
				http.Error(w, e.Error(), http.StatusBadRequest)
				return
			}
			result, e := in.TransformResponse(r.Context(), unified)
			if e != nil {
				t.Logf("gateway transform error: %v", e)
				http.Error(w, e.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(result.Body)
			return
		}
		observed := streams.Map(httpclient.NewDefaultSSEDecoder(r.Context(), response.Body), func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
			if gjson.GetBytes(event.Data, "type").String() == "response.web_search_call.completed" || (gjson.GetBytes(event.Data, "item.type").String() == "web_search_call" && gjson.GetBytes(event.Data, "item.status").String() == "completed") {
				completedSearch.Store(true)
			}
			for _, item := range gjson.GetBytes(event.Data, "response.output").Array() {
				if item.Get("type").String() == "web_search_call" && item.Get("status").String() == "completed" {
					completedSearch.Store(true)
				}
			}
			return event
		})
		stream, e := out.TransformStream(r.Context(), converted, observed)
		if e != nil {
			t.Logf("gateway transform error: %v", e)
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}
		events, e := in.TransformStream(r.Context(), stream)
		if e != nil {
			t.Logf("gateway transform error: %v", e)
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}
		defer events.Close()
		w.Header().Set("Content-Type", "text/event-stream")
		for events.Next() {
			event := events.Current()
			if gjson.GetBytes(event.Data, "type").String() == "content_block_start" {
				switch gjson.GetBytes(event.Data, "content_block.type").String() {
				case "server_tool_use":
					serverUses.Add(1)
				case "web_search_tool_result":
					serverResults.Add(1)
					resultLinks.Add(int32(len(gjson.GetBytes(event.Data, "content_block.content").Array())))
				}
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
			w.(http.Flusher).Flush()
		}
		if e := events.Err(); e != nil {
			t.Errorf("search stream: %v", e)
		}
	}))
	defer server.Close()
	cmd := exec.CommandContext(ctx, "claude", "-p", "Use WebSearch exactly once to search for the current stable Debian release on the official Debian website. Return the resulting title and URL. Do not use other tools.", "--model", model, "--tools", "WebSearch", "--allowedTools", "WebSearch", "--output-format", "stream-json", "--verbose", "--max-turns", "3", "--no-session-persistence", "--setting-sources", "", "--settings", `{"env":{"ANTHROPIC_BASE_URL":"`+server.URL+`"}}`)
	cmd.Dir = t.TempDir()
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "ANTHROPIC_") && !strings.HasPrefix(env, "CLAUDECODE=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "ANTHROPIC_BASE_URL="+server.URL, "ANTHROPIC_AUTH_TOKEN="+key.Key, "ANTHROPIC_DEFAULT_HAIKU_MODEL="+model, "ANTHROPIC_DEFAULT_SONNET_MODEL="+model, "ANTHROPIC_DEFAULT_OPUS_MODEL="+model)
	var captured bytes.Buffer
	cmd.Stdout = &captured
	cmd.Stderr = &captured
	err = cmd.Run()
	output := captured.Bytes()
	// Print only tool results and final result, never environment or headers.
	var successful bool
	for line := range bytes.SplitSeq(output, []byte("\n")) {
		if gjson.GetBytes(line, "type").String() == "user" {
			for _, b := range gjson.GetBytes(line, "message.content").Array() {
				if b.Get("type").String() == "tool_result" {
					t.Log(b.Raw)
					if !b.Get("is_error").Bool() && strings.Contains(b.Get("content").String(), "debian.org") {
						successful = true
					}
				}
			}
		}
		if gjson.GetBytes(line, "type").String() == "result" {
			t.Log(gjson.GetBytes(line, "result").String())
		}
	}
	require.NoError(t, err)
	require.Positive(t, searches.Load(), "Claude must actually call native web search")
	require.Positive(t, serverUses.Load(), "Claude must receive search progress blocks")
	require.Equal(t, serverUses.Load(), serverResults.Load(), "each search must have one result block")
	t.Logf("Anthropic search progress/result pairs: %d; source links: %d", serverResults.Load(), resultLinks.Load())
	require.Positive(t, resultLinks.Load(), "the successful search must expose structured source links")
	require.True(t, completedSearch.Load(), "upstream must report a completed search, not just generate a URL")
	require.True(t, successful, "Claude must receive a successful search tool result with a source URL")
}
