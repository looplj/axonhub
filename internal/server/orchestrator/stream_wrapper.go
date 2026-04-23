package orchestrator

import (
	"context"
	"sync"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// onCloseStream wraps a streams.Stream, calling onClose exactly once on exhaustion
// or explicit close. The onClose callback MUST NOT re-enter any onCloseStream method
// (deadlock). The underlying stream.Close runs even if onClose panics.
type onCloseStream struct {
	stream   streams.Stream[*llm.Response]
	onClose  func()
	closed   sync.Once
	closeErr error
	mu       sync.Mutex
	done     chan struct{}
}

func newOnCloseStream(stream streams.Stream[*llm.Response], onClose func()) *onCloseStream {
	return &onCloseStream{
		stream:  stream,
		onClose: onClose,
		done:    make(chan struct{}),
	}
}

func (s *onCloseStream) Current() *llm.Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stream.Current()
}

func (s *onCloseStream) Next() bool {
	s.mu.Lock()
	next := s.stream.Next()
	s.mu.Unlock()

	if !next {
		s.closeOnce()
		return false
	}

	return true
}

func (s *onCloseStream) Close() error {
	s.closeOnce()
	return s.closeErr
}

func (s *onCloseStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stream.Err()
}

// Done returns a channel that is closed when the stream has been fully consumed
// or explicitly closed. Callers can select on this to detect stream termination
// without blocking indefinitely.
func (s *onCloseStream) Done() <-chan struct{} {
	return s.done
}

//nolint:unused // called from Next and Close
func (s *onCloseStream) closeOnce() {
	s.closed.Do(func() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Debug(context.Background(), "onClose callback panicked",
						log.Any("panic", r),
				)
				}
			}()
			s.onClose()
		}()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closeErr = s.stream.Close()
		close(s.done)
	})
}
