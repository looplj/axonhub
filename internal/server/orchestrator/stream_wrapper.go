package orchestrator

import (
	"sync"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// onCloseStream wraps a streams.Stream and calls onClose exactly once when the stream
// is exhausted (Next returns false) or explicitly closed. The underlying stream is
// closed exactly once — inside the sync.Once block — to prevent double-close.
type onCloseStream struct {
	stream     streams.Stream[*llm.Response]
	onClose    func()
	closed     sync.Once
	closeErr   error
}

func (s *onCloseStream) Current() *llm.Response {
	return s.stream.Current()
}

func (s *onCloseStream) Next() bool {
	if !s.stream.Next() {
		s.closed.Do(func() {
			s.onClose()
			s.closeErr = s.stream.Close()
		})
		return false
	}

	return true
}

func (s *onCloseStream) Close() error {
	s.closed.Do(func() {
		s.onClose()
		s.closeErr = s.stream.Close()
	})
	return s.closeErr
}

func (s *onCloseStream) Err() error {
	return s.stream.Err()
}
