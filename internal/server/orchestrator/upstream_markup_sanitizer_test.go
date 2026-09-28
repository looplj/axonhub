package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"strings"
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

// A tag head whose attribute value carries a character the old whitelist
// omitted (a POSIX path) must be held, not released: releasing it handed the
// whole tag head to the client as content and suppressed the abort.
func TestScrubText_HoldsTagHeadWithColonInAttribute(t *testing.T) {
	head := "answer <\uFF5CDSML\uFF5Cinvoke name=\"path:C:\\tools\""
	emit, hold, stripped := scrubText(head, false)
	if emit != "answer " || hold != "<\uFF5CDSML\uFF5Cinvoke name=\"path:C:\\tools\"" || stripped {
		t.Fatalf("emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}

	// The next chunk completes the tag: all of it is dropped and reported.
	emit, hold, stripped = scrubText(hold+` extra="1">done`, false)
	if emit != "done" || hold != "" || !stripped {
		t.Fatalf("resolved: emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}
}

// A tag head longer than the old 128-byte hold limit (a parameter tag with a
// long string attribute) must still be held instead of emitted.
func TestScrubText_HoldsLongTagHead(t *testing.T) {
	head := "t<\uFF5CDSML\uFF5Cparameter name=\"p\" string=\"" + strings.Repeat("x", 600) + "\""
	emit, hold, stripped := scrubText(head, false)
	if emit != "t" || hold != head[1:] || stripped {
		t.Fatalf("emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}
	if len(hold) > maxTagHold {
		t.Fatalf("held %d bytes, over the %d limit", len(hold), maxTagHold)
	}
}

// Past the hold limit a DSML head can no longer be resolved in place, so it
// must be dropped and reported as stripped rather than handed to the client.
func TestScrubText_DiscardsOverCapDSMLHead(t *testing.T) {
	head := "<\uFF5CDSML\uFF5Cparameter string=\"" + strings.Repeat("y", maxTagHold+64) + "\""
	emit, hold, stripped := scrubText("t"+head, false)
	if emit != "t" || hold != "" || !stripped {
		t.Fatalf("emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}
}

// Prose that merely follows a stray '<' must survive untouched however long
// it runs: the hold limit is a memory bound, not a licence to drop text.
func TestScrubText_KeepsLongProseAfterStrayLt(t *testing.T) {
	prose := "if a < b then " + strings.Repeat("z", maxTagHold+64)
	emit, hold, stripped := scrubText(prose, false)
	if emit != prose || hold != "" || stripped {
		t.Fatalf("long prose lost: emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}

	// Short prose after '<' is still held (it may be a tag head) and returns
	// on flush.
	emit, hold, stripped = scrubText("2 < 3", false)
	if emit != "2 " || hold != "< 3" || stripped {
		t.Fatalf("short prose: emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}
	emit, hold, stripped = scrubText(hold, true)
	if emit != "< 3" || hold != "" || stripped {
		t.Fatalf("flush: emit=%q hold=%q stripped=%v", emit, hold, stripped)
	}
}

// End-to-end regression for the cross-chunk leak: a tag head carrying a colon
// in an attribute value used to reach the client verbatim, so the turn ended
// silently corrupted instead of aborting.
func TestMarkupSanitizer_SplitTagWithColonAttributeAborts(t *testing.T) {
	inner := &stubMarkupStream{items: []*llm.Response{
		textChunk(t, "start<\uFF5CDSML\uFF5Cinvoke name=\"path:C:\\tools\""),
		textChunk(t, ` extra="1">done`),
	}}
	wrapped, err := withUpstreamMarkupSanitizer().(interface {
		OnOutboundLlmStream(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
	}).OnOutboundLlmStream(context.Background(), inner)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	_, err = collectText(t, wrapped)

	var respErr *llm.ResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("expected leak abort, got %v", err)
	}
	if respErr.Detail.Code != "upstream_tool_call_markup_leak" {
		t.Fatalf("unexpected error code: %q", respErr.Detail.Code)
	}
}

// The same split tag with a real tool call behind it stays cosmetic: the
// turn completes and only the markup disappears.
func TestMarkupSanitizer_SplitTagWithColonAttributeKeepsToolCallTurn(t *testing.T) {
	inner := &stubMarkupStream{items: []*llm.Response{
		textChunk(t, "start<\uFF5CDSML\uFF5Cinvoke name=\"path:C:\\tools\""),
		textChunk(t, ` extra="1">done`),
		{Choices: []llm.Choice{{
			Index: 0,
			Delta: &llm.Message{ToolCalls: []llm.ToolCall{{ID: "call_1"}}},
		}}},
	}}
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
	if text != "startdone" {
		t.Fatalf("unexpected text: %q", text)
	}
}
