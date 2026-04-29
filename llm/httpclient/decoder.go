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

// NewDefaultSSEDecoder creates a new default SSE decoder.
func NewDefaultSSEDecoder(ctx context.Context, rc io.ReadCloser) StreamDecoder {
	ctx, cancel := context.WithCancel(ctx)
	return &defaultSSEDecoder{
		done: ctx.Done(),
		cancel: cancel,
		sseStream: sse.NewStreamWithConfig(rc, &sse.StreamConfig{
			MaxEventSize: 32 * 1024 * 1024,
		}),
	}
}

// Ensure defaultSSEDecoder implements StreamDecoder.
var _ StreamDecoder = (*defaultSSEDecoder)(nil)

// defaultSSEDecoder implements streams.Stream for Server-Sent Events using go-sse Stream.
type defaultSSEDecoder struct {
	done      <-chan struct{}
	cancel    context.CancelFunc
	sseStream *sse.Stream
	current   *StreamEvent
	err       error

	// NOT concurrency-safe: do not call Next/Close from multiple goroutines.
	// Close is made idempotent (safe to call multiple times sequentially).
	closed   bool
	closeErr error
}

// Next advances to the next event in the stream.
func (s *defaultSSEDecoder) Next() bool {
	if s.err != nil {
		return false
	}

	if s.closed {
		return false
	}

	// Check context cancellation
	select {
	case <-s.done:
		s.err = context.Canceled
		_ = s.Close()

		return false
	default:
	}

	// Receive next event from go-sse Stream.
	// Recv() blocks on I/O, so we run it in a goroutine and select
	// against s.done to allow context cancellation to interrupt.
	type recvResult struct {
		event sse.Event
		err   error
	}
	ch := make(chan recvResult, 1)
	go func() {
		event, err := s.sseStream.Recv()
		ch <- recvResult{event: event, err: err}
	}()

	select {
	case <-s.done:
		s.err = context.Canceled
		_ = s.Close()
		return false
	case result := <-ch:
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
