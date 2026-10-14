package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestCodexCLIToolCallFollowupThroughResponsesGateway(t *testing.T) {
	// Given two deterministic upstream turns behind the production handler.
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIResponse, "", false)
	requests := make(chan responses.Request, 2)
	var turns atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body responses.Request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		turn := turns.Add(1)
		if turn > 2 {
			t.Error("unexpected third upstream request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		payload := codexCLIToolCallResponseStream()
		if turn == 2 {
			payload = codexCLIFollowupResponseStream()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, payload); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(upstream.Close)
	setCodexTestUpstream(t, fixture, upstream.URL)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
	ready := make(chan struct{}, 2)
	release := make(chan struct{}, 2)
	abort := make(chan struct{})
	t.Cleanup(func() { close(abort) })
	done := make(chan struct{}, 2)
	wire := make(chan []responses.StreamEvent, 2)
	h.StreamWriter = func(c *gin.Context, source streams.Stream[*httpclient.StreamEvent]) {
		observed := &codexTurnBarrier{Stream: source, ready: ready, release: release, abort: abort}
		WriteSSEStream(c, preserveStreamDelivery(observed, source))
		wire <- observed.events
	}
	engine := gin.New()
	engine.Use(fixture.testContext)
	engine.POST("/v1/responses", func(c *gin.Context) {
		h.ChatCompletion(c)
		done <- struct{}{}
	})
	server := startLoopbackServer(t, engine)

	// When the official CLI executes the requested harmless tool and follows up.
	cmd, stdout, stderr := officialCodexCLICommand(t, t.TempDir(), server.URL+"/v1", "Run printf tool-result, then report completion.")
	require.NoError(t, cmd.Start())
	for turn := range 2 {
		awaitCodexBarrier(t, ready)
		rows, err := fixture.client.Request.Query().Order(ent.Asc(request.FieldID)).All(fixture.ctx)
		require.NoError(t, err)
		require.Len(t, rows, turn+1)
		require.NotEqual(t, request.StatusCompleted, rows[turn].Status, "unwritten terminal must not complete Request")
		release <- struct{}{}
		awaitCodexBarrier(t, done)
	}
	require.NoError(t, cmd.Wait(), "stdout=%s stderr=%s", stdout, stderr)

	// Then the actual tool result, both terminal sequences and accounting agree.
	require.Equal(t, int32(2), turns.Load())
	first, second := <-requests, <-requests
	var advertised bool
	for _, tool := range first.Tools {
		advertised = advertised || tool.Name == "exec_command"
	}
	require.True(t, advertised)
	var call, output *responses.Item
	for i := range second.Input.Items {
		item := &second.Input.Items[i]
		if item.CallID != "call-tool" {
			continue
		}
		switch item.Type {
		case "function_call":
			call = item
		case "function_call_output":
			output = item
		}
	}
	require.NotNil(t, call)
	require.Equal(t, "exec_command", call.Name)
	require.NotNil(t, output)
	encodedOutput, err := json.Marshal(output.Output)
	require.NoError(t, err)
	require.Contains(t, string(encodedOutput), "tool-result")
	require.NotContains(t, string(encodedOutput), "unsupported call")
	require.Contains(t, stdout.String(), `"exit_code":0`)
	require.Equal(t, 1, strings.Count(stdout.String(), `"type":"turn.completed"`))
	for turn := range 2 {
		events := <-wire
		require.NotEmpty(t, events)
		require.Equal(t, responses.StreamEventTypeResponseCompleted, events[len(events)-1].Type)
		var terminals int
		for _, event := range events {
			if event.Type == responses.StreamEventTypeResponseCompleted {
				terminals++
			}
		}
		require.Equal(t, 1, terminals)
		want := [2][2]int64{{12, 8}, {20, 10}}[turn]
		usage := events[len(events)-1].Response.Usage
		require.NotNil(t, usage)
		require.EqualValues(t, want[0], usage.InputTokens)
		require.EqualValues(t, want[1], usage.OutputTokens)
	}
	assertCodexTurnAccounting(t, fixture, [][2]int64{{12, 8}, {20, 10}})
}

func TestStreamDeliveryE2E_2616ConvertedResponsesOrdering(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, streamDeliveryChat2616(), false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
	h.streamAdapterFactory = newResponsesStreamAdapter
	engine := gin.New()
	engine.Use(fixture.testContext)
	done := make(chan struct{})
	engine.POST("/v1/responses", func(c *gin.Context) {
		defer close(done)
		h.ChatCompletion(c)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(fixture.ctx, http.MethodPost, server.URL+"/v1/responses", strings.NewReader(`{"model":"gpt-test","input":"hello","stream":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("converted Responses handler did not return")
	}

	wire := string(body)
	require.Equal(t, 1, strings.Count(wire, "event:response.completed"))
	require.Contains(t, wire, `"input_tokens":12`)
	require.Contains(t, wire, `"output_tokens":34`)
	require.Contains(t, wire, `"total_tokens":46`)
	require.NotContains(t, wire, `"input_tokens":0,"output_tokens":0`)
	fixture.assertDatabase(t, request.StatusCompleted, true)
	usage, err := fixture.client.UsageLog.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.EqualValues(t, 12, usage.PromptTokens)
	require.EqualValues(t, 34, usage.CompletionTokens)
}

func TestStreamDeliveryE2E_2616PrematureTerminalGuardSensitivity(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, streamDeliveryChat2616PrematureTerminalVariant(), false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = responses.NewInboundTransformer()
	h.streamAdapterFactory = newResponsesStreamAdapter
	engine := gin.New()
	engine.Use(fixture.testContext)
	engine.POST("/v1/responses", h.ChatCompletion)
	server := startLoopbackServer(t, engine)

	req, err := http.NewRequestWithContext(fixture.ctx, http.MethodPost, server.URL+"/v1/responses", strings.NewReader(`{"model":"gpt-test","input":"hello","stream":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	wire := string(body)
	require.Equal(t, 1, strings.Count(wire, "event:response.completed"))
	terminal := wire[strings.LastIndex(wire, "event:response.completed"):]
	require.Contains(t, terminal, `"input_tokens":0`)
	require.Contains(t, terminal, `"output_tokens":0`)
	logs, err := fixture.client.UsageLog.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Empty(t, logs)
}

func TestStreamDeliveryE2E_2616ConvertedAnthropicOrdering(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, streamDeliveryChat2616(), false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
	engine := gin.New()
	engine.Use(fixture.testContext)
	engine.POST("/v1/messages", h.ChatCompletion)
	server := startLoopbackServer(t, engine)

	req, err := http.NewRequestWithContext(fixture.ctx, http.MethodPost, server.URL+"/v1/messages", strings.NewReader(`{"model":"gpt-test","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	wire := string(body)
	require.Equal(t, 1, strings.Count(wire, "event:message_delta"))
	require.Equal(t, 1, strings.Count(wire, "event:message_stop"))
	terminal := wire[strings.LastIndex(wire, "event:message_delta"):]
	require.Contains(t, terminal, `"input_tokens":12`)
	require.Contains(t, terminal, `"output_tokens":34`)
	require.NotContains(t, terminal, `"input_tokens":0,"output_tokens":0`)
	require.Less(t, strings.Index(terminal, "event:message_delta"), strings.Index(terminal, "event:message_stop"))
	fixture.assertDatabase(t, request.StatusCompleted, true)
}

func TestStreamDeliveryE2E_2616ConvertedAnthropicGuardSensitivity(t *testing.T) {
	fixture := newStreamDeliveryE2EFixture(t, llm.APIFormatOpenAIChatCompletion, streamDeliveryChat2616PrematureTerminalVariant(), false)
	h := fixture.handler(t)
	h.ChatCompletionOrchestrator.Inbound = anthropic.NewInboundTransformer()
	engine := gin.New()
	engine.Use(fixture.testContext)
	engine.POST("/v1/messages", h.ChatCompletion)
	server := startLoopbackServer(t, engine)

	req, err := http.NewRequestWithContext(fixture.ctx, http.MethodPost, server.URL+"/v1/messages", strings.NewReader(`{"model":"gpt-test","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":true}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	wire := string(body)
	require.Equal(t, 1, strings.Count(wire, "event:message_delta"))
	require.Contains(t, wire, `"input_tokens":0`)
	require.Contains(t, wire, `"output_tokens":0`)
	logs, err := fixture.client.UsageLog.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Empty(t, logs)
}

type codexTurnBarrier struct {
	streams.Stream[*httpclient.StreamEvent]
	ready   chan<- struct{}
	release <-chan struct{}
	abort   <-chan struct{}
	events  []responses.StreamEvent
}

func (s *codexTurnBarrier) Current() *httpclient.StreamEvent {
	event := s.Stream.Current()
	var parsed responses.StreamEvent
	if json.Unmarshal(event.Data, &parsed) == nil {
		s.events = append(s.events, parsed)
	}
	if event.Type == "response.completed" {
		s.ready <- struct{}{}
		select {
		case <-s.release:
		case <-s.abort:
		}
	}
	return event
}

func awaitCodexBarrier(t *testing.T, barrier <-chan struct{}) {
	t.Helper()
	select {
	case <-barrier:
	case <-time.After(20 * time.Second):
		t.Fatal("Codex scenario barrier watchdog expired")
	}
}

func setCodexTestUpstream(t *testing.T, fixture *streamDeliveryE2EFixture, url string) {
	t.Helper()
	row, err := fixture.channel.Update().SetBaseURL(url).SetEndpoints([]objects.ChannelEndpoint{{APIFormat: fixture.format.String(), BaseURL: url, Transport: objects.ChannelEndpointTransportHTTP}}).Save(fixture.ctx)
	require.NoError(t, err)
	fixture.channel = row
}

func assertCodexTurnAccounting(t *testing.T, fixture *streamDeliveryE2EFixture, counts [][2]int64) {
	t.Helper()
	rows, err := fixture.client.Request.Query().Order(ent.Asc(request.FieldID)).All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, rows, len(counts))
	executions, err := fixture.client.RequestExecution.Query().Order(ent.Asc(requestexecution.FieldID)).All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, executions, len(counts))
	logs, err := fixture.client.UsageLog.Query().All(fixture.ctx)
	require.NoError(t, err)
	require.Len(t, logs, len(counts))
	for i, row := range rows {
		require.Equal(t, request.StatusCompleted, row.Status)
		execution := executions[i]
		require.Equal(t, row.ID, execution.RequestID)
		require.Equal(t, requestexecution.StatusCompleted, execution.Status)
		var matched int
		for _, usage := range logs {
			if usage.RequestID != row.ID {
				continue
			}
			matched++
			require.Equal(t, row.ID, usage.RequestID)
			require.Equal(t, fixture.channel.ID, usage.ChannelID)
			require.Equal(t, row.ProjectID, usage.ProjectID)
			require.Equal(t, counts[i][0], usage.PromptTokens)
			require.Equal(t, counts[i][1], usage.CompletionTokens)
			require.NotNil(t, usage.TotalCost)
			want := decimal.NewFromInt(counts[i][0] + counts[i][1]).Mul(decimal.RequireFromString("0.00000001"))
			require.True(t, want.Equal(decimal.NewFromFloat(*usage.TotalCost)), "cost got=%s want=%s", usage.TotalCost, want)
			t.Logf("Request=%d Execution=%d UsageLog=%d completed usage=%d/%d cost=%f", row.ID, execution.ID, usage.ID, usage.PromptTokens, usage.CompletionTokens, *usage.TotalCost)
		}
		require.Equal(t, 1, matched)
	}
}
