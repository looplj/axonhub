package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

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
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

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
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

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
