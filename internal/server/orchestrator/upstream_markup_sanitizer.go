package orchestrator

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

// Some providers (notably DeepSeek V4.x) occasionally leak their internal
// tool-call markup (DSML tags such as "<｜DSML｜parameter ...>") into the
// assistant content stream instead of emitting structured tool calls.
// Downstream clients then render those tags as garbage text. This middleware
// sanitizes the unified outbound stream:
//
//   - markup fragments are stripped from message content;
//   - if the response still produced real tool calls, the turn completes
//     normally (the leak was cosmetic);
//   - if markup was stripped but no tool call ever arrived, the model's tool
//     invocation was swallowed: the stream is aborted with a retryable
//     in-stream error so clients fail fast / retry instead of consuming a
//     silently corrupted turn.
//
// Clean streams pass through untouched.
func withUpstreamMarkupSanitizer() pipeline.Middleware {
	return &upstreamMarkupSanitizerMiddleware{}
}

// upstreamMarkupSanitizerMiddleware registers the markup sanitizer in the
// outbound LLM stream pipeline.
type upstreamMarkupSanitizerMiddleware struct {
	pipeline.DummyMiddleware
}

// Name identifies the middleware in pipeline diagnostics.
func (m *upstreamMarkupSanitizerMiddleware) Name() string { return "upstream-markup-sanitizer" }

// OnOutboundLlmStream wraps the upstream response stream with the sanitizer.
func (m *upstreamMarkupSanitizerMiddleware) OnOutboundLlmStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*llm.Response], error) {
	return &markupSanitizerStream{ctx: ctx, inner: stream, carry: map[int]string{}}, nil
}

// maxTagHold is the maximum amount of an already recognizable, incomplete
// DSML tag that the stream will wait for. Past this point the malformed output
// is aborted so callers receive a retryable error without an excessive delay.
const maxTagHold = 1024

// maxPendingChunks bounds how long stripped markup may wait for a real tool
// call before the turn is treated as irrecoverably corrupted.
const maxPendingChunks = 256

var (
	// Complete markup tags, e.g. "<｜DSML｜parameter name=\"x\" string=\"true\">",
	// "</｜｜DSML｜｜ parameter>", "</small_placeholder>". Bars may be ASCII '|'
	// or fullwidth '｜' (U+FF5C) and are sometimes doubled.
	leakedToolCallTagRe = regexp.MustCompile(
		"</?[|\\x{FF5C}]+\\s*DSML[|\\x{FF5C}][^>]*>|</?small_placeholder>",
	)
	// Suffix that could be the beginning of a tag split across chunks: "<" or
	// "</" plus whatever has not reached its closing '>' yet. Attribute values
	// may hold almost any character (POSIX paths, JSON, ':' inside
	// name="path:C:\tools"), so this deliberately does not whitelist
	// characters: the previous whitelist missed ':' and silently released
	// unresolvable markup as content instead of holding it.
	tagPrefixRe = regexp.MustCompile(`^</?[^>]*$`)

	// Head of a DSML tag, used to tell an over-long markup fragment (drop) from
	// over-long prose that merely follows a stray '<' (keep).
	dsmlHeadRe = regexp.MustCompile("^</?[|\\x{FF5C}]+\\s*DSML[|\\x{FF5C}]")
)

// scrubText removes complete markup tags from text. When flush is false and
// the text ends with what could be the start of a tag split across chunks,
// that tail is held back (returned as hold) instead of emitted.
func scrubText(text string, flush bool) (emit, hold string, stripped bool) {
	emit, hold, stripped, _ = scrubTextChecked(text, flush)
	return emit, hold, stripped
}

// scrubTextChecked additionally reports an incomplete, confirmed internal tag
// that must abort the stream instead of being released as content.
func scrubTextChecked(text string, flush bool) (emit, hold string, stripped, fatal bool) {
	emit = leakedToolCallTagRe.ReplaceAllString(text, "")
	stripped = emit != text

	lt := incompleteTagStart(emit)
	if lt < 0 {
		return emit, "", stripped, false
	}

	tail := emit[lt:]
	confirmed := dsmlHeadRe.MatchString(tail) || strings.HasPrefix(tail, "<small_placeholder") || strings.HasPrefix(tail, "</small_placeholder")
	if flush {
		if confirmed {
			return emit[:lt], "", true, true
		}
		return emit, "", stripped, false
	}

	if len(tail) <= maxTagHold {
		return emit[:lt], tail, stripped, false
	}

	if confirmed {
		return emit[:lt], "", true, true
	}
	return emit, "", stripped, false
}

// incompleteTagStart selects the first confirmed internal-tag head after the
// last closing bracket. If none is confirmed yet, the last '<' is retained so
// a short prefix split across chunks can resolve without pinning earlier prose.
func incompleteTagStart(text string) int {
	start := strings.LastIndexByte(text, '>') + 1
	for offset := start; offset < len(text); {
		rel := strings.IndexByte(text[offset:], '<')
		if rel < 0 {
			break
		}
		candidate := offset + rel
		tail := text[candidate:]
		if dsmlHeadRe.MatchString(tail) || strings.HasPrefix(tail, "<small_placeholder") || strings.HasPrefix(tail, "</small_placeholder") {
			return candidate
		}
		offset = candidate + 1
	}
	rel := strings.LastIndexByte(text[start:], '<')
	if rel < 0 {
		return -1
	}
	return start + rel
}

// markupSanitizerStream filters a unified LLM response stream. Chunks are
// held back while scrubbed markup may still resolve into tool calls; if the
// leak swallowed the tool call, the stream aborts with a retryable error.
type markupSanitizerStream struct {
	ctx   context.Context
	inner streams.Stream[*llm.Response]

	carry map[int]string // per-choice held tail that may be a split tag prefix

	pending    []*llm.Response // held back while scrubbing, awaiting the verdict
	flushQueue []*llm.Response // salvaged chunks waiting to be emitted

	scrubbing    bool
	sawToolCalls bool

	cur  *llm.Response
	last *llm.Response
	err  error
}

// Next advances the stream to the next sanitized response chunk.
func (s *markupSanitizerStream) Next() bool {
	if s.err != nil {
		return false
	}

	for {
		if len(s.flushQueue) > 0 {
			s.cur = s.flushQueue[0]
			s.flushQueue = s.flushQueue[1:]
			return true
		}

		if !s.inner.Next() {
			return s.finish()
		}

		resp := s.inner.Current()
		if resp == nil {
			s.cur = nil
			return true
		}

		s.last = resp
		s.trackToolCalls(resp)
		stripped, fatal := s.cleanChunk(resp)
		if fatal {
			return s.abortMarkupLeak()
		}

		if stripped && !s.scrubbing {
			s.scrubbing = true
			log.Warn(s.ctx, "stripped leaked tool-call markup from upstream content stream")
		}

		if s.scrubbing {
			s.pending = append(s.pending, resp)
			if len(s.pending) > maxPendingChunks {
				return s.abortMarkupLeak()
			}

			if s.sawToolCalls {
				// The turn produced real tool calls: the leak was cosmetic.
				// Flush everything held back and resume pass-through.
				s.scrubbing = false
				s.flushQueue = s.pending
				s.pending = nil
			}

			continue
		}

		s.cur = resp

		return true
	}
}

// Current returns the current sanitized response chunk.
func (s *markupSanitizerStream) Current() *llm.Response { return s.cur }

// Err returns the stream error, preferring an abort raised by the sanitizer.
func (s *markupSanitizerStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.inner.Err()
}

// Close releases the underlying stream.
func (s *markupSanitizerStream) Close() error { return s.inner.Close() }

// finish is called when the inner stream is exhausted.
func (s *markupSanitizerStream) finish() bool {
	// A genuine upstream transport error is the root cause and always wins.
	if err := s.inner.Err(); err != nil {
		s.pending = nil
		s.err = err
		return false
	}

	if s.scrubbing && !s.sawToolCalls {
		return s.abortMarkupLeak()
	}

	// Flush ordinary tail text, but never release a confirmed internal-tag
	// fragment merely because the upstream stream ended.
	for _, held := range s.carry {
		if _, _, _, fatal := scrubTextChecked(held, true); fatal {
			return s.abortMarkupLeak()
		}
	}

	if len(s.carry) > 0 && s.last != nil {
		tail := *s.last
		tail.Usage = nil
		tail.Error = nil
		tail.Choices = make([]llm.Choice, 0, len(s.carry))

		indexes := make([]int, 0, len(s.carry))
		for index := range s.carry {
			indexes = append(indexes, index)
		}
		sort.Ints(indexes)

		for _, index := range indexes {
			t := s.carry[index]
			tail.Choices = append(tail.Choices, llm.Choice{
				Index: index,
				Delta: &llm.Message{Content: llm.MessageContent{Content: &t}},
			})
		}

		s.carry = map[int]string{}
		s.cur = &tail

		return true
	}

	return false
}

// abortMarkupLeak stops a corrupted turn with the existing retryable error.
func (s *markupSanitizerStream) abortMarkupLeak() bool {
	s.pending = nil
	s.carry = map[int]string{}
	s.err = &llm.ResponseError{
		StatusCode: http.StatusInternalServerError,
		Detail: llm.ErrorDetail{
			Type:    "server_error",
			Code:    "upstream_tool_call_markup_leak",
			Message: "upstream provider leaked internal tool-call markup into the content stream and produced no tool calls; the response was aborted and is safe to retry",
		},
	}
	_ = s.inner.Close()
	log.Warn(s.ctx, "aborted stream: tool-call markup leak swallowed the turn's tool calls")
	return false
}

// trackToolCalls records whether any real tool call appeared in the stream.
func (s *markupSanitizerStream) trackToolCalls(resp *llm.Response) {
	if s.sawToolCalls {
		return
	}

	for i := range resp.Choices {
		msg := resp.Choices[i].Delta
		if msg == nil {
			msg = resp.Choices[i].Message
		}

		if msg != nil && len(msg.ToolCalls) > 0 {
			s.sawToolCalls = true
			return
		}
	}
}

// cleanChunk strips markup from every content field of resp in place and
// reports whether anything was stripped.
func (s *markupSanitizerStream) cleanChunk(resp *llm.Response) (bool, bool) {
	strippedAny := false
	fatalAny := false

	for i := range resp.Choices {
		choice := &resp.Choices[i]

		msg := choice.Delta
		if msg == nil {
			msg = choice.Message
		}

		if msg == nil {
			continue
		}

		flush := choice.FinishReason != nil

		if msg.Content.Content != nil {
			text, stripped, fatal := s.scrubChoiceText(choice.Index, *msg.Content.Content, flush)
			msg.Content.Content = &text
			strippedAny = strippedAny || stripped
			fatalAny = fatalAny || fatal
		}

		for j := range msg.Content.MultipleContent {
			part := &msg.Content.MultipleContent[j]
			if part.Text == nil {
				continue
			}

			text, _, stripped, fatal := scrubTextChecked(*part.Text, true)
			part.Text = &text
			strippedAny = strippedAny || stripped
			fatalAny = fatalAny || fatal
		}

		// A finishing choice may carry no text at all (empty final delta), so the
		// scrubs above never run for it. Drain its held split-tag tail into this
		// chunk so the text is delivered with the chunk carrying finish_reason;
		// otherwise it would surface as an extra chunk after the terminal one.
		if flush && s.carry[choice.Index] != "" {
			emit, stripped, fatal := s.scrubChoiceText(choice.Index, "", true)
			strippedAny = strippedAny || stripped
			fatalAny = fatalAny || fatal

			if emit != "" {
				if msg.Content.Content == nil && len(msg.Content.MultipleContent) == 0 {
					msg.Content.Content = &emit
				} else {
					msg.Content.MultipleContent = append(msg.Content.MultipleContent, llm.MessageContentPart{
						Type: "text",
						Text: &emit,
					})
				}
			}
		}
	}

	return strippedAny, fatalAny
}

// scrubChoiceText scrubs a text fragment together with any held split-tag
// tail for the same choice index; when flush is true the tail is released.
func (s *markupSanitizerStream) scrubChoiceText(index int, text string, flush bool) (string, bool, bool) {
	emit, hold, stripped, fatal := scrubTextChecked(s.carry[index]+text, flush)

	if hold != "" {
		s.carry[index] = hold
	} else {
		delete(s.carry, index)
	}

	return emit, stripped, fatal
}
