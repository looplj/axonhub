package orchestrator

import (
	"sync"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// onCloseStream wraps a streams.Stream and calls onClose exactly once when the stream
// is exhausted (Next returns false) or explicitly closed.
type onCloseStream struct {
	stream  streams.Stream[*llm.Response]
	onClose func()
	closed  sync.Once
}

func (s *onCloseStream) Current() *llm.Response {
	return s.stream.Current()
}

func (s *onCloseStream) Next() bool {
	if !s.stream.Next() {
		s.closed.Do(func() {
			s.onClose()
			_ = s.stream.Close()
		})
		return false
	}

	return true
}

func (s *onCloseStream) Close() error {
	s.closed.Do(s.onClose)
	return s.stream.Close()
}

func (s *onCloseStream) Err() error {
	return s.stream.Err()
}
