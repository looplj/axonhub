package openai

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// finishReasonFallbackStream guarantees the client receives a terminal
// finish_reason chunk before the [DONE] marker. Some upstreams end the stream
// without ever setting Choice.FinishReason, which strict OpenAI clients
// interpret as an aborted response and surface as an interrupted turn. When
// the source stream reaches [DONE] (or ends) without a finish reason, a
// synthetic terminal chunk (finish_reason "stop") is emitted first.
type finishReasonFallbackStream struct {
	source    streams.Stream[*llm.Response]
	sawFinish bool
	sawDone   bool
	// sawChunk tracks whether any real (non-[DONE]) chunk was observed. A
	// completely empty stream is an upstream failure and must stay an error;
	// only streams that produced content get a synthesized terminal chunk.
	sawChunk bool
	// synthDoneNext marks that Current() holds a synthetic finish chunk and
	// the [DONE] marker must be delivered on the following Next() call.
	synthDoneNext bool
	current       *llm.Response
}

func newFinishReasonFallbackStream(source streams.Stream[*llm.Response]) *finishReasonFallbackStream {
	return &finishReasonFallbackStream{source: source}
}

func (s *finishReasonFallbackStream) Next() bool {
	if s.synthDoneNext {
		s.synthDoneNext = false
		// The synthetic terminal sequence is complete; never synthesize again.
		s.sawDone = true
		s.current = llm.DoneResponse
		return true
	}

	for s.source.Next() {
		chunk := s.source.Current()
		if chunk == nil {
			continue
		}

		if chunk.Object == "[DONE]" {
			s.sawDone = true
			if !s.sawFinish && s.sawChunk {
				s.current = s.synthesizeFinishChunk(nil)
				s.synthDoneNext = true
				return true
			}

			s.current = chunk
			return true
		}

		s.sawChunk = true

		for i := range chunk.Choices {
			if chunk.Choices[i].FinishReason != nil {
				s.sawFinish = true
				break
			}
		}

		s.current = chunk
		return true
	}

	// Source ended cleanly with content but without a finish reason and
	// without [DONE]: emit the synthetic finish chunk followed by the [DONE]
	// marker so clients get a well-formed terminal sequence.
	if s.source.Err() == nil && !s.sawFinish && !s.sawDone && s.sawChunk {
		s.current = s.synthesizeFinishChunk(nil)
		s.synthDoneNext = true
		return true
	}

	return false
}

func (s *finishReasonFallbackStream) synthesizeFinishChunk(prev *llm.Response) *llm.Response {
	resp := &llm.Response{
		Object:  "chat.completion.chunk",
		Choices: []llm.Choice{{Index: 0, FinishReason: lo.ToPtr("stop"), Delta: &llm.Message{}}},
	}

	if prev != nil {
		resp.ID = prev.ID
		resp.Model = prev.Model
		resp.Created = prev.Created
	}

	return resp
}

func (s *finishReasonFallbackStream) Current() *llm.Response {
	return s.current
}

func (s *finishReasonFallbackStream) Err() error {
	return s.source.Err()
}

func (s *finishReasonFallbackStream) Close() error {
	return s.source.Close()
}
