package httpclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/tmaxmax/go-sse"
)

// decoderRegistry holds registered stream decoders.
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
	return &defaultSSEDecoder{
		ctx:    ctx,
		reader: rc,
		// sseStream: sse.NewStream(rc),
		// 图片生成需要大量数据，设置最大事件大小
		sseStream: sse.NewStreamWithConfig(rc, &sse.StreamConfig{
			MaxEventSize: 32 * 1024 * 1024,
		}),
	}
}

// Ensure defaultSSEDecoder implements StreamDecoder.
var _ StreamDecoder = (*defaultSSEDecoder)(nil)

// defaultSSEDecoder implements streams.Stream for Server-Sent Events backed by
// go-sse Stream.
//
// Concurrency model:
//   - Next / Current / Err must be called from a single goroutine (the reader).
//   - Close is idempotent (sync.Once) and safe to call concurrently with Next
//     from any goroutine. The reader (typically *http.Response.Body) is closed
//     directly, which is documented to be safe to call concurrently with Read
//     and is what unblocks an in-flight Recv.
//   - The `closed` flag is the only field written by Close and read by Next,
//     hence it must be atomic. All other state mutations (s.err, s.current)
//     happen only on the reader goroutine.
//
// Why we close the underlying reader instead of sse.Stream.Close:
//   - sse.Stream.Close() is essentially `s.closed = true; reader.Close()`. The
//     `closed` field on sse.Stream is a plain bool with no synchronization and
//     is read inside Recv, so calling sse.Stream.Close from another goroutine
//     while Recv is running causes a data race. Closing the reader directly
//     avoids touching sse.Stream's non-thread-safe state entirely; only the
//     reader's Read returns an error and Recv unwinds naturally.
//
// Why the `closed` flag is necessary even though Recv would surface the
// reader-Close error on its own:
//   - The decoder API contract is "after Close, Next returns false". We cannot
//     enforce this by writing to s.err from Close, because s.err is read by
//     Next on the reader goroutine and writing it from another goroutine would
//     be a data race. An atomic flag is the cheapest way to honor the contract
//     regardless of whether the underlying reader actually fails Read after
//     being closed (e.g., test mocks backed by bytes.Reader do not).
//
// Why there is no recover() in Next:
//   - The historical panic in #1634 came from the race on sse.Stream described
//     above. With sse.Stream's internal state no longer touched concurrently,
//     that race is gone at the source. The pass-through producer goroutine
//     still has its own top-level recover (per project rule), which is the
//     proper place to catch any unforeseen panic from this stack.
//
// Why context cancellation has no dedicated watcher goroutine:
//   - When the decoder wraps an *http.Response.Body produced via
//     http.NewRequestWithContext, the http.Transport already aborts the
//     in-flight Read on ctx.Done(), unblocking Recv naturally. Callers that
//     pass a Reader not bound to ctx are responsible for calling Close.
//
//nolint:containedctx // Checked.
type defaultSSEDecoder struct {
	ctx       context.Context
	reader    io.ReadCloser
	sseStream *sse.Stream
	current   *StreamEvent
	err       error

	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

// Next advances to the next event in the stream.
func (s *defaultSSEDecoder) Next() bool {
	if s.err != nil {
		return false
	}

	// Honor the "Close stops Next" contract without writing to s.err from
	// the closer goroutine (which would race with this read).
	if s.closed.Load() {
		return false
	}

	// Pre-check ctx so we don't enter Recv after explicit cancellation.
	select {
	case <-s.ctx.Done():
		s.err = s.ctx.Err()

		return false
	default:
	}

	event, err := s.sseStream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			slog.DebugContext(s.ctx, "SSE stream closed")

			return false
		}

		// If the error surfaced because ctx was canceled (or the reader was
		// closed by an external Close), surface ctx.Err() to callers so they
		// can distinguish cancellation from genuine transport errors.
		if ctxErr := s.ctx.Err(); ctxErr != nil {
			s.err = ctxErr
		} else {
			s.err = err
		}

		return false
	}

	slog.DebugContext(s.ctx, "SSE event received", slog.Any("event", event))

	s.current = &StreamEvent{
		LastEventID: event.LastEventID,
		Type:        event.Type,
		Data:        []byte(event.Data),
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

// Close closes the underlying reader. It is idempotent and safe to call from
// any goroutine, including concurrently with Next.
func (s *defaultSSEDecoder) Close() error {
	s.closeOnce.Do(func() {
		// Set the closed flag before touching the reader so a concurrent
		// Next observes the closed state on its next entry.
		s.closed.Store(true)

		if s.reader != nil {
			s.closeErr = s.reader.Close()
		}

		slog.DebugContext(s.ctx, "SSE stream closed")
	})

	return s.closeErr
}

// init registers the default SSE decoder.
func init() {
	RegisterDecoder("text/event-stream", NewDefaultSSEDecoder)
	RegisterDecoder("text/event-stream; charset=utf-8", NewDefaultSSEDecoder)
}
