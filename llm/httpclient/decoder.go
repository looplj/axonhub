package httpclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/tmaxmax/go-sse"
)

// decoderRegistry holds registered stream decodings.
type decoderRegistry struct {
	mu       sync.RWMutex
	decoders map[string]StreamDecoderFactory
}

// globalRegistry is the global decoder registry.
var globalRegistry = &decoderRegistry{
	decoders: make(map[string]StreamDecoderFactory),
}

// RegisterDecoder registers a stream decoder for a specific content type.
func RegisterDecoder(contentType string, factory StreamDecoderFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.decoders[contentType] = factory
}

// GetDecoder returns a decoder factory for the given content type.
func GetDecoder(contentType string) (StreamDecoderFactory, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	factory, exists := globalRegistry.decoders[contentType]

	return factory, exists
}

// recvResult carries the outcome of a single Recv() call.
type recvResult struct {
	event sse.Event
	err   error
}

// NewDefaultSSEDecoder creates a new default SSE decoder.
func NewDefaultSSEDecoder(ctx context.Context, rc io.ReadCloser) StreamDecoder {
	ctx, cancel := context.WithCancel(ctx)
	d := &defaultSSEDecoder{
		done:   ctx.Done(),
		cancel: cancel,
		sseStream: sse.NewStreamWithConfig(rc, &sse.StreamConfig{
			MaxEventSize: 32 * 1024 * 1024,
		}),
		recvCh: make(chan recvResult, 1),
	}
	go d.recvLoop()
	return d
}

// Ensure defaultSSEDecoder implements StreamDecoder.
var _ StreamDecoder = (*defaultSSEDecoder)(nil)

// defaultSSEDecoder implements streams.Stream for Server-Sent Events using go-sse Stream.
type defaultSSEDecoder struct {
	done      <-chan struct{}
	cancel    context.CancelFunc
	sseStream *sse.Stream
	recvCh    chan recvResult
	current   *StreamEvent
	err       error

	// NOT concurrency-safe: do not call Next/Close from multiple goroutines.
	// Close is made idempotent (safe to call multiple times sequentially).
	closed   bool
	closeErr error
}

// recvLoop runs in a dedicated goroutine, forwarding Recv() results to recvCh.
// It exits when s.done is cancelled or when Recv() returns an error/EOF.
func (s *defaultSSEDecoder) recvLoop() {
	defer close(s.recvCh)
	for {
		event, err := s.sseStream.Recv()
		select {
		case <-s.done:
			return
		case s.recvCh <- recvResult{event: event, err: err}:
			if err != nil {
				return
			}
		}
	}
}

// Next advances to the next event in the stream.
func (s *defaultSSEDecoder) Next() bool {
	if s.err != nil {
		return false
	}

	if s.closed {
		return false
	}

	// Check context cancellation before consuming from channel.
	select {
	case <-s.done:
		s.err = context.Canceled
		_ = s.Close()
		return false
	default:
	}

	result, ok := <-s.recvCh
	if !ok {
		// Channel closed — stream ended or context cancelled.
		if s.err == nil {
			s.err = io.EOF
		}
		return false
	}

	if result.err != nil {
		if errors.Is(result.err, io.EOF) {
			_ = s.Close()
			return false
		}
		s.err = result.err
		_ = s.Close()
		return false
	}

	s.current = &StreamEvent{
		LastEventID: result.event.LastEventID,
		Type:        result.event.Type,
		Data:        []byte(result.event.Data),
	}
	return true
}

// Current returns the current event data.
func (s *defaultSSEDecoder) Current() *StreamEvent {
	return s.current
}

// Err returns any error that occurred during streaming.
func (s *defaultSSEDecoder) Err() error {
	return s.err
}

// Close closes the stream and releases resources.
func (s *defaultSSEDecoder) Close() error {
	// NOT concurrency-safe: callers must not call Close concurrently with Next.
	if s.closed {
		return s.closeErr
	}

	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	if s.sseStream != nil {
		s.closeErr = s.sseStream.Close()
		slog.Debug("SSE stream closed")
	}

	return s.closeErr
}

// init registers the default SSE decoder.
func init() {
	RegisterDecoder("text/event-stream", NewDefaultSSEDecoder)
	RegisterDecoder("text/event-stream; charset=utf-8", NewDefaultSSEDecoder)
}
