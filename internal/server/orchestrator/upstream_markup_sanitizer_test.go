package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// stubMarkupStream is a minimal in-memory streams.Stream for sanitizer tests.
type stubMarkupStream struct {
	items []*llm.Response
	idx   int
	err   error
}

// Next advances to the next queued response.
func (s *stubMarkupStream) Next() bool {
	if s.idx >= len(s.items) {
		return false
	}
	s.idx++
	return true
}

// Current returns the response delivered by the latest Next call.
func (s *stubMarkupStream) Current() *llm.Response { return s.items[s.idx-1] }

// Err returns the preset stream error, if any.
func (s *stubMarkupStream) Err() error { return s.err }

// Close releases the stub stream (no-op).
func (s *stubMarkupStream) Close() error { return nil }

// textChunk builds a streamed assistant delta carrying the given text.
func textChunk(t *testing.T, text string) *llm.Response {
	t.Helper()
	return &llm.Response{
		Choices: []llm.Choice{{
			Index: 0,
			Delta: &llm.Message{Role: "assistant", Content: llm.MessageContent{Content: &text}},
		}},
	}
}

// collectText drains the stream and concatenates all delta content fragments.
func collectText(t *testing.T, stream streams.Stream[*llm.Response]) (string, error) {
	t.Helper()
	out := ""
	for stream.Next() {
		resp := stream.Current()
		if resp == nil {
			continue
		}
		for _, c := range resp.Choices {
			msg := c.Delta
			if msg == nil {
				msg = c.Message
			}
			if msg != nil && msg.Content.Content != nil {
				out += *msg.Content.Content
			}
		}
	}
	return out, stream.Err()
}

// TestMarkupSanitizer_CleanStreamPassesThrough verifies that streams without
// leaked markup are forwarded unmodified.
func TestMarkupSanitizer_CleanStreamPassesThrough(t *testing.T) {
	inner := &stubMarkupStream{items: []*llm.Response{textChunk(t, "hello "), textChunk(t, "world")}}
	wrapped, err := withUpstreamMarkupSanitizer().(interface {
		OnOutboundLlmStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
	}).OnOutboundLlmStream(context.Background(), inner)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	text, err := collectText(t, wrapped)
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("clean stream altered: %q", text)
	}
}

// TestMarkupSanitizer_LeakWithToolCallsIsScrubbed verifies that leaked markup
// is stripped when the turn still carries tool calls.
func TestMarkupSanitizer_LeakWithToolCallsIsScrubbed(t *testing.T) {
	tag := "<\uFF5CDSML\uFF5Cparameter name=\"x\">"
	inner := &stubMarkupStream{items: []*llm.Response{
		textChunk(t, "正在"+tag),
		{Choices: []llm.Choice{{
			Index: 0,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{{ID: "call_1"}}},
		}}},
	}}
	wrapped, _ := withUpstreamMarkupSanitizer().(interface {
		OnOutboundLlmStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
	}).OnOutboundLlmStream(context.Background(), inner)

	text, err := collectText(t, wrapped)
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if text != "正在" {
		t.Fatalf("markup not stripped: %q", text)
	}
}

// TestMarkupSanitizer_LeakWithoutToolCallsAborts verifies that a leak without
// tool calls aborts the stream with a retryable 500 error.
func TestMarkupSanitizer_LeakWithoutToolCallsAborts(t *testing.T) {
	inner := &stubMarkupStream{items: []*llm.Response{
		textChunk(t, "开始"),
		textChunk(t, "</\uFF5C\uFF5CDSML\uFF5C\uFF5C parameter>残留"),
	}}
	wrapped, _ := withUpstreamMarkupSanitizer().(interface {
		OnOutboundLlmStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
	}).OnOutboundLlmStream(context.Background(), inner)

	_, err := collectText(t, wrapped)
	var respErr *llm.ResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("expected *llm.ResponseError, got %v", err)
	}
	if respErr.Detail.Code != "upstream_tool_call_markup_leak" {
		t.Fatalf("unexpected error code: %q", respErr.Detail.Code)
	}
	if respErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("abort error must carry retryable 5xx status, got %d", respErr.StatusCode)
	}
}

// A finishing chunk with an empty delta must still drain the held split-tag
// tail, otherwise the text would be emitted as an extra chunk after the one
// carrying finish_reason.
func TestMarkupSanitizer_FinishingChunkDrainsCarry(t *testing.T) {
	stop := "stop"
	inner := &stubMarkupStream{items: []*llm.Response{
		textChunk(t, "hello<"),
		{Choices: []llm.Choice{{
			Index:        0,
			Delta:        &llm.Message{},
			FinishReason: &stop,
		}}},
	}}
	wrapped, _ := withUpstreamMarkupSanitizer().(interface {
		OnOutboundLlmStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
	}).OnOutboundLlmStream(context.Background(), inner)

	var chunks []*llm.Response
	for wrapped.Next() {
		if resp := wrapped.Current(); resp != nil {
			chunks = append(chunks, resp)
		}
	}
	if err := wrapped.Err(); err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks (text + finish), got %d", len(chunks))
	}

	fin := chunks[1].Choices[0]
	if fin.FinishReason == nil {
		t.Fatalf("second chunk lost finish_reason")
	}
	if fin.Delta == nil || fin.Delta.Content.Content == nil || *fin.Delta.Content.Content != "<" {
		t.Fatalf("held tail not drained onto finishing chunk: %+v", fin.Delta)
	}
}

// TestMarkupSanitizer_SplitTagAcrossChunks verifies that a tag split across
// two chunks is still scrubbed.
func TestMarkupSanitizer_SplitTagAcrossChunks(t *testing.T) {
	inner := &stubMarkupStream{items: []*llm.Response{
		textChunk(t, "ok<"),
		textChunk(t, "\uFF5C\uFF5CDSML\uFF5C\uFF5Cparameter>"),
		{Choices: []llm.Choice{{
			Index: 0,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{{ID: "call_1"}}},
		}}},
	}}
	wrapped, _ := withUpstreamMarkupSanitizer().(interface {
		OnOutboundLlmStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
	}).OnOutboundLlmStream(context.Background(), inner)

	text, err := collectText(t, wrapped)
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if text != "ok" {
		t.Fatalf("split tag not stripped: %q", text)
	}
}

// TestScrubText_HoldsIncompleteTagPrefix verifies that incomplete tag
// prefixes are held back until they resolve or are flushed.
func TestScrubText_HoldsIncompleteTagPrefix(t *testing.T) {
	emit, hold, stripped := scrubText("text<\uFF5C\uFF5CDS", false)
	if emit != "text" || hold != "<\uFF5C\uFF5CDS" || stripped {
		t.Fatalf("emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}

	emit, hold, stripped = scrubText("text<\uFF5C\uFF5CDS", true)
	if emit != "text<\uFF5C\uFF5CDS" || hold != "" || stripped {
		t.Fatalf("flush: emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}

	emit, _, stripped = scrubText("</\uFF5C\uFF5CDSML\uFF5C\uFF5C parameter>x", false)
	if emit != "x" || !stripped {
		t.Fatalf("tag: emit=%q stripped=%v", emit, stripped)
	}
}
