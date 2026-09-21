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

type upstreamMarkupSanitizerMiddleware struct {
	pipeline.DummyMiddleware
}

func (m *upstreamMarkupSanitizerMiddleware) Name() string { return "upstream-markup-sanitizer" }

func (m *upstreamMarkupSanitizerMiddleware) OnOutboundLlmStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*llm.Response], error) {
	return &markupSanitizerStream{ctx: ctx, inner: stream, carry: map[int]string{}}, nil
}

const maxTagHold = 128

var (
	// Complete markup tags, e.g. "<｜DSML｜parameter name=\"x\" string=\"true\">",
	// "</｜｜DSML｜｜ parameter>", "</small_placeholder>". Bars may be ASCII '|'
	// or fullwidth '｜' (U+FF5C) and are sometimes doubled.
	leakedToolCallTagRe = regexp.MustCompile(
		"</?[|\\x{FF5C}]+\\s*DSML[|\\x{FF5C}][^>]*>|</?small_placeholder>",
	)
	// Suffix that could be the beginning of a tag split across chunks: only
	// tag-ish characters after the last '<' (bars, letters, quotes, ...).
	tagPrefixRe = regexp.MustCompile(`^</?[|\x{FF5C}\w\s"=.-]*$`)
)

// scrubText removes complete markup tags from text. When flush is false and
// the text ends with what could be the start of a tag split across chunks,
// that tail is held back (returned as hold) instead of emitted.
func scrubText(text string, flush bool) (emit, hold string, stripped bool) {
	emit = leakedToolCallTagRe.ReplaceAllString(text, "")
	stripped = emit != text

	if flush {
		return emit, "", stripped
	}

	lt := strings.LastIndexByte(emit, '<')
	if lt < 0 || lt < strings.LastIndexByte(emit, '>') {
		return emit, "", stripped
	}

	if tail := emit[lt:]; len(tail) <= maxTagHold && tagPrefixRe.MatchString(tail) {
		return emit[:lt], tail, stripped
	}

	return emit, "", stripped
}

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
		stripped := s.cleanChunk(resp)

		if stripped && !s.scrubbing {
			s.scrubbing = true
			log.Warn(s.ctx, "stripped leaked tool-call markup from upstream content stream")
		}

		if s.scrubbing {
			s.pending = append(s.pending, resp)

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

func (s *markupSanitizerStream) Current() *llm.Response { return s.cur }

func (s *markupSanitizerStream) Err() error {
	if s.err != nil {
		return s.err
	}

	return s.inner.Err()
}

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
		// Markup was stripped but no tool call ever arrived: the model's tool
		// invocation was swallowed. Abort with a retryable in-stream error
		// instead of letting the client consume a silently corrupted turn.
		s.pending = nil
		s.err = &llm.ResponseError{
			// 502 semantics: retry.go classifies by StatusCode, so a zero value
			// would make the abort invisible to retryable-server-error handling.
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Type:    "server_error",
				Code:    "upstream_tool_call_markup_leak",
				Message: "upstream provider leaked internal tool-call markup into the content stream and produced no tool calls; the response was aborted and is safe to retry",
			},
		}
		log.Warn(s.ctx, "aborted stream: tool-call markup leak swallowed the turn's tool calls")

		return false
	}

	// Flush any tail text held back as a potential split tag prefix.
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
func (s *markupSanitizerStream) cleanChunk(resp *llm.Response) bool {
	strippedAny := false

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
			text, stripped := s.scrubChoiceText(choice.Index, *msg.Content.Content, flush)
			msg.Content.Content = &text
			strippedAny = strippedAny || stripped
		}

		for j := range msg.Content.MultipleContent {
			part := &msg.Content.MultipleContent[j]
			if part.Text == nil {
				continue
			}

			text, stripped := s.scrubChoiceText(choice.Index, *part.Text, flush)
			part.Text = &text
			strippedAny = strippedAny || stripped
		}

		// A finishing choice may carry no text at all (empty final delta), so the
		// scrubs above never run for it. Drain its held split-tag tail into this
		// chunk so the text is delivered with the chunk carrying finish_reason;
		// otherwise it would surface as an extra chunk after the terminal one.
		if flush && s.carry[choice.Index] != "" {
			emit, stripped := s.scrubChoiceText(choice.Index, "", true)
			strippedAny = strippedAny || stripped

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

	return strippedAny
}

func (s *markupSanitizerStream) scrubChoiceText(index int, text string, flush bool) (string, bool) {
	emit, hold, stripped := scrubText(s.carry[index]+text, flush)

	if hold != "" {
		s.carry[index] = hold
	} else {
		delete(s.carry, index)
	}

	return emit, stripped
}
