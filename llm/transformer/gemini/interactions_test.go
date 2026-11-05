package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestInteractionsOutboundStreamRejectsMalformedKnownEvent(t *testing.T) {
	stream, err := (&InteractionsOutboundTransformer{}).TransformStream(context.Background(), nil, streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "interaction.completed", Data: []byte(`{"interaction":{"status":`)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if stream.Next() {
		t.Fatal("malformed terminal event was accepted")
	}
	if stream.Err() == nil {
		t.Fatal("malformed terminal event did not return an error")
	}
}

func TestInteractionsOutboundStreamRejectsTruncatedStream(t *testing.T) {
	stream, err := (&InteractionsOutboundTransformer{}).TransformStream(context.Background(), nil, streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "step.delta", Data: []byte(`{"delta":{"type":"text","text":"partial"}}`)},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !stream.Next() {
		t.Fatal("text delta was not emitted")
	}
	if stream.Next() {
		t.Fatal("truncated stream produced an extra event")
	}
	if !errors.Is(stream.Err(), ErrInteractionsIncomplete) {
		t.Fatalf("stream error = %v, want %v", stream.Err(), ErrInteractionsIncomplete)
	}
}

func TestInteractionsOutboundStreamsDoNotShareQueryState(t *testing.T) {
	first := interactionStream(t, `{"id":"first","model":"gemini-first","status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"one"}]}]}`)
	second := interactionStream(t, `{"id":"second","model":"gemini-second","status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"two"}]}]}`)

	if !first.Next() || first.Current().Model != "gemini-first" {
		t.Fatalf("first stream leaked model state: %#v", first.Current())
	}
	if !second.Next() || second.Current().Model != "gemini-second" {
		t.Fatalf("second stream leaked model state: %#v", second.Current())
	}
	if !first.Next() || first.Current() != llm.DoneResponse {
		t.Fatalf("first stream did not emit its own done sentinel: %#v", first.Current())
	}
}

func TestInteractionsInboundRejectsStatefulFields(t *testing.T) {
	request := &httpclient.Request{Body: []byte(`{"model":"gemini-test","input":"hello","store":true}`)}
	_, err := (&InteractionsInboundTransformer{}).TransformRequest(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "store=true") {
		t.Fatalf("error = %v, want deterministic store rejection", err)
	}
}

func TestInteractionsInboundAcceptsStatelessStepContent(t *testing.T) {
	request := &httpclient.Request{Body: []byte(`{"model":"gemini-test","input":[{"type":"user_input","content":"hello"},{"type":"model_output","content":[{"type":"text","text":"hi"}]},{"type":"thought","content":"reason"}]}`)}
	result, err := (&InteractionsInboundTransformer{}).TransformRequest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 3 || result.Messages[0].Role != "user" || result.Messages[1].Role != "assistant" || result.Messages[2].ReasoningContent == nil {
		t.Fatalf("messages = %#v", result.Messages)
	}
}

func TestInteractionsInboundParsesOfficialContentAndFunctionTools(t *testing.T) {
	request := &httpclient.Request{Body: []byte(`{
		"model":"gemini-test",
		"input":[{"type":"text","text":"What is the weather?"}],
		"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}}],
		"generation_config":{"max_output_tokens":64,"stop_sequences":["END"]}
	}`)}
	parsed, err := (&InteractionsInboundTransformer{}).TransformRequest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.APIFormat != llm.APIFormatGeminiInteractions || parsed.Model != "gemini-test" {
		t.Fatalf("request identity = %#v", parsed)
	}
	if len(parsed.Messages) != 1 || parsed.Messages[0].Content.Content == nil || *parsed.Messages[0].Content.Content != "What is the weather?" {
		t.Fatalf("messages = %#v", parsed.Messages)
	}
	if len(parsed.Tools) != 1 || parsed.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("tools = %#v", parsed.Tools)
	}
	if parsed.MaxCompletionTokens == nil || *parsed.MaxCompletionTokens != 64 || parsed.Stop == nil {
		t.Fatalf("generation config was not converted: %#v", parsed)
	}
}

func TestInteractionsOutboundUsesContentArrayAndStoreFalse(t *testing.T) {
	transformer, err := NewInteractionsOutboundTransformer("https://example.invalid", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	stream := false
	inputText := "hello"
	request, err := transformer.TransformRequest(context.Background(), &llm.Request{
		Model:    "gemini-test",
		Stream:   &stream,
		Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: &inputText}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatal(err)
	}
	if string(body["store"]) != "false" || string(body["input"]) != `[{"type":"text","text":"hello"}]` {
		t.Fatalf("body = %s", request.Body)
	}
	if request.URL != "https://example.invalid/v1beta/interactions" || !request.SkipInboundQueryMerge {
		t.Fatalf("request transport = %#v", request)
	}
	if request.Auth == nil || request.Auth.HeaderKey != "x-goog-api-key" || request.Auth.APIKey != "test-key" {
		t.Fatalf("request auth = %#v", request.Auth)
	}
}

func TestInteractionsInboundStreamUsesOfficialEventNames(t *testing.T) {
	text := "hello"
	stream, err := (&InteractionsInboundTransformer{}).TransformStream(context.Background(), streams.SliceStream([]*llm.Response{
		{ID: "id", Model: "gemini-test", Object: "interaction.created"},
		{ID: "id", Model: "gemini-test", Object: "interaction.status_update"},
		{Model: "gemini-test", Choices: []llm.Choice{{Delta: &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: &text}}}}},
		{ID: "id", Model: "gemini-test", Object: "interaction.completed", Usage: &llm.Usage{TotalTokens: 1}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var eventTypes []string
	for stream.Next() {
		eventTypes = append(eventTypes, stream.Current().Type)
	}
	if stream.Err() != nil {
		t.Fatal(stream.Err())
	}
	want := []string{"interaction.created", "interaction.status_update", "step.start", "step.delta", "step.stop", "interaction.completed", "done"}
	if len(eventTypes) != len(want) {
		t.Fatalf("event types = %#v, want %#v", eventTypes, want)
	}
	for i := range want {
		if eventTypes[i] != want[i] {
			t.Fatalf("event types = %#v, want %#v", eventTypes, want)
		}
	}
}

func TestInteractionsEventPayloadsUseOfficialEnvelopes(t *testing.T) {
	var created map[string]json.RawMessage
	if err := json.Unmarshal(interactionEventJSON("interaction.created", interactionsResponse{ID: "i1", Status: "in_progress", EventID: "e1"}), &created); err != nil {
		t.Fatal(err)
	}
	if _, ok := created["interaction"]; !ok || string(created["event_type"]) != `"interaction.created"` {
		t.Fatalf("created payload = %s", created)
	}

	var status map[string]json.RawMessage
	if err := json.Unmarshal(interactionEventJSON("interaction.status_update", interactionsResponse{ID: "i1", Status: "in_progress", EventID: "e2"}), &status); err != nil {
		t.Fatal(err)
	}
	if _, ok := status["interaction"]; ok || string(status["interaction_id"]) != `"i1"` || string(status["status"]) != `"in_progress"` {
		t.Fatalf("status payload = %s", status)
	}

	var failure map[string]json.RawMessage
	if err := json.Unmarshal(interactionEventJSON("error", interactionsResponse{Error: &interactionsError{Code: "gateway_timeout", Message: "timed out"}}), &failure); err != nil {
		t.Fatal(err)
	}
	if _, ok := failure["interaction"]; ok || string(failure["error"]) != `{"message":"timed out","code":"gateway_timeout"}` {
		t.Fatalf("error payload = %s", failure)
	}
}

func TestInteractionsSyntheticUpstreamJSONAndSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/interactions" || r.Header.Get("X-Goog-Api-Key") != "test-key" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || string(body["store"]) != "false" {
			http.Error(w, "store must be false", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			_, _ = fmt.Fprint(w, "event: interaction.created\ndata: {\"interaction\":{\"id\":\"i1\",\"model\":\"gemini-test\",\"status\":\"in_progress\"},\"event_type\":\"interaction.created\"}\n\n")
			_, _ = fmt.Fprint(w, "event: interaction.status_update\ndata: {\"interaction_id\":\"i1\",\"status\":\"in_progress\",\"event_type\":\"interaction.status_update\"}\n\n")
			_, _ = fmt.Fprint(w, "event: step.start\ndata: {\"index\":0,\"step\":{\"type\":\"model_output\"},\"event_type\":\"step.start\"}\n\n")
			_, _ = fmt.Fprint(w, "event: step.delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text\",\"text\":\"hello\"},\"event_type\":\"step.delta\"}\n\n")
			_, _ = fmt.Fprint(w, "event: step.stop\ndata: {\"index\":0,\"event_type\":\"step.stop\"}\n\n")
			_, _ = fmt.Fprint(w, "event: interaction.completed\ndata: {\"interaction\":{\"id\":\"i1\",\"model\":\"gemini-test\",\"status\":\"completed\",\"usage\":{\"total_tokens\":1}}}\n\n")
			_, _ = fmt.Fprint(w, "event: done\ndata: [DONE]\n\n")
			flusher.Flush()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"i1","model":"gemini-test","status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"hello"}]}]}`)
	}))
	defer server.Close()

	transformer, err := NewInteractionsOutboundTransformer(server.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	stream := false
	request, err := transformer.TransformRequest(context.Background(), &llm.Request{Model: "gemini-test", Stream: &stream, Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: new(string)}}}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpclient.NewHttpClientWithClient(server.Client()).Do(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := transformer.TransformResponse(context.Background(), response)
	if err != nil || parsed.ID != "i1" || len(parsed.Choices) != 1 {
		t.Fatalf("json response = %#v, err = %v", parsed, err)
	}

	stream = true
	request, err = transformer.TransformRequest(context.Background(), &llm.Request{Model: "gemini-test", Stream: &stream, Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: new(string)}}}})
	if err != nil {
		t.Fatal(err)
	}
	rawStream, err := httpclient.NewHttpClientWithClient(server.Client()).DoStream(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	parsedStream, err := transformer.TransformStream(context.Background(), request, rawStream)
	if err != nil {
		t.Fatal(err)
	}
	var events []string
	for parsedStream.Next() {
		events = append(events, parsedStream.Current().Object)
	}
	if parsedStream.Err() != nil {
		t.Fatal(parsedStream.Err())
	}
	if len(events) != 7 || events[0] != "interaction.created" || events[len(events)-1] != "[DONE]" {
		t.Fatalf("stream objects = %#v", events)
	}
}

func interactionStream(t *testing.T, terminal string) streams.Stream[*llm.Response] {
	t.Helper()
	transformer := &InteractionsOutboundTransformer{}
	stream, err := transformer.TransformStream(context.Background(), nil, streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "interaction.completed", Data: []byte(`{"interaction":` + terminal + `}`)},
		{Type: "done", Data: []byte("[DONE]")},
	}))
	if err != nil {
		t.Fatal(err)
	}
	return stream
}
