package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

const maxStreamProbeBufferedEvents = 256

type streamProbeEvent struct {
	event *httpclient.StreamEvent
	err   error
	done  bool
}

// probeStreamBeforeCommit reads the final outbound stream for a bounded window
// before any bytes are exposed to the client. If the stream errors during that
// window, the error is returned to the pipeline retry loop. On success, buffered
// events are replayed and a single reader goroutine continues owning the inner
// stream, so timeout does not create concurrent calls to inner.Next().
func probeStreamBeforeCommit(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
	duration time.Duration,
) (streams.Stream[*httpclient.StreamEvent], error) {
	if duration <= 0 {
		return stream, nil
	}

	probed := newCommitProbeStream(ctx, stream)
	timer := time.NewTimer(duration)
	defer timer.Stop()

	for {
		select {
		case event, ok := <-probed.events:
			if !ok {
				if err := probed.Err(); err != nil {
					_ = probed.Close()

					return nil, fmt.Errorf("stream probe failed before client commit: %w", err)
				}

				return probed, nil
			}

			if event.done {
				if event.err != nil {
					_ = probed.Close()

					return nil, fmt.Errorf("stream probe failed before client commit: %w", event.err)
				}

				probed.readerDone = true

				return probed, nil
			}

			probed.buffer = append(probed.buffer, event.event)
			if len(probed.buffer) >= maxStreamProbeBufferedEvents {
				return probed, nil
			}

		case <-timer.C:
			return probed, nil

		case <-ctx.Done():
			_ = probed.Close()

			return nil, ctx.Err()
		}
	}
}

type commitProbeStream struct {
	ctx    context.Context
	inner  streams.Stream[*httpclient.StreamEvent]
	events chan streamProbeEvent
	stop   chan struct{}
	once   sync.Once

	buffer      []*httpclient.StreamEvent
	bufferIndex int
	readerDone  bool
	current     *httpclient.StreamEvent
	errMu       sync.Mutex
	err         error
}

func newCommitProbeStream(ctx context.Context, inner streams.Stream[*httpclient.StreamEvent]) *commitProbeStream {
	s := &commitProbeStream{
		ctx:    ctx,
		inner:  inner,
		events: make(chan streamProbeEvent),
		stop:   make(chan struct{}),
	}
	go s.readLoop()

	return s
}

func (s *commitProbeStream) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			s.setErr(fmt.Errorf("stream probe reader panic: %v", r))
			slog.WarnContext(s.ctx, "stream probe reader recovered panic", slog.Any("panic", r))
		}
		close(s.events)
	}()

	for {
		if s.inner.Next() {
			event := s.inner.Current()
			if !s.send(streamProbeEvent{event: event}) {
				return
			}

			continue
		}

		err := s.inner.Err()
		s.setErr(err)
		s.send(streamProbeEvent{err: err, done: true})

		return
	}
}

func (s *commitProbeStream) send(event streamProbeEvent) bool {
	select {
	case s.events <- event:
		return true
	case <-s.stop:
		return false
	case <-s.ctx.Done():
		s.setErr(s.ctx.Err())
		return false
	}
}

func (s *commitProbeStream) Next() bool {
	if s.bufferIndex < len(s.buffer) {
		s.current = s.buffer[s.bufferIndex]
		s.bufferIndex++

		return true
	}

	if s.readerDone {
		return false
	}

	select {
	case event, ok := <-s.events:
		if !ok {
			s.readerDone = true

			return false
		}
		if event.done {
			s.setErr(event.err)
			s.readerDone = true

			return false
		}

		s.current = event.event

		return true

	case <-s.ctx.Done():
		s.setErr(s.ctx.Err())
		_ = s.Close()

		return false
	}
}

func (s *commitProbeStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *commitProbeStream) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()

	return s.err
}

func (s *commitProbeStream) setErr(err error) {
	if err == nil {
		return
	}

	s.errMu.Lock()
	defer s.errMu.Unlock()

	if s.err == nil {
		s.err = err
	}
}

func (s *commitProbeStream) Close() error {
	var err error
	s.once.Do(func() {
		close(s.stop)
		err = s.inner.Close()
	})

	return err
}
