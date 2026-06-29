package pipeline

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestPipeline_Process_StreamProbeErrorRetriesBeforeClientCommit(t *testing.T) {
	ctx := context.Background()
	streamErr := errors.New("stream error: stream ID 7; INTERNAL_ERROR; received from peer")
	streamFlag := true

	attempts := 0
	executor := &mockExecutor{
		doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			attempts++
			if attempts == 1 {
				return newErrorAfterEventsStream(
					[]*httpclient.StreamEvent{{Data: []byte("partial")}},
					streamErr,
				), nil
			}

			return streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("ok")}}), nil
		},
	}

	prepareCalls := 0
	outbound := &mockOutbound{
		transformStream: rawToLlmObjectStream,
		canRetry: func(err error) bool {
			return errors.Is(err, streamErr)
		},
		prepareForRetry: func(ctx context.Context) error {
			prepareCalls++
			return nil
		},
	}
	inbound := &mockInbound{
		transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
			return &llm.Request{Stream: &streamFlag}, nil
		},
		transformStream: llmObjectToRawStream,
	}

	p := &pipeline{
		Executor:              executor,
		Inbound:               inbound,
		Outbound:              outbound,
		maxSameChannelRetries: 1,
		streamProbeDuration:   time.Second,
	}

	res, err := p.Process(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.Stream)
	require.Equal(t, 2, attempts)
	require.Equal(t, 1, prepareCalls)

	events := collectStreamEvents(t, res.EventStream)
	require.Equal(t, []string{"ok"}, streamEventData(events))
}

func TestPipeline_Process_StreamProbeExhaustionMarksUpstreamError(t *testing.T) {
	ctx := context.Background()
	streamErr := errors.New("stream error: stream ID 7; INTERNAL_ERROR; received from peer")
	streamFlag := true

	executor := &mockExecutor{
		doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			return newErrorAfterEventsStream(
				[]*httpclient.StreamEvent{{Data: []byte("partial")}},
				streamErr,
			), nil
		},
	}
	outbound := &mockOutbound{
		transformStream: rawToLlmObjectStream,
	}
	inbound := &mockInbound{
		transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
			return &llm.Request{Stream: &streamFlag}, nil
		},
		transformStream: llmObjectToRawStream,
	}

	p := &pipeline{
		Executor:            executor,
		Inbound:             inbound,
		Outbound:            outbound,
		streamProbeDuration: time.Second,
	}

	res, err := p.Process(ctx, &httpclient.Request{})
	require.Error(t, err)
	require.Nil(t, res)
	require.True(t, errors.Is(err, streamErr))
	require.True(t, IsUpstreamError(err))
}

func TestProbeStreamBeforeCommit_ReplaysBufferedEventsAfterProbeWindow(t *testing.T) {
	stream := streams.SliceStream([]*httpclient.StreamEvent{
		{Data: []byte("one")},
		{Data: []byte("two")},
	})

	probed, err := probeStreamBeforeCommit(context.Background(), stream, time.Second)
	require.NoError(t, err)

	events := collectStreamEvents(t, probed)
	require.Equal(t, []string{"one", "two"}, streamEventData(events))
}

func TestProbeStreamBeforeCommit_TimeoutKeepsSingleInnerReader(t *testing.T) {
	stream := newControlledStream([]*httpclient.StreamEvent{{Data: []byte("late")}})

	resultCh := make(chan struct {
		stream streams.Stream[*httpclient.StreamEvent]
		err    error
	}, 1)
	go func() {
		defer recoverTestGoroutine(t)

		probed, err := probeStreamBeforeCommit(context.Background(), stream, 50*time.Millisecond)
		resultCh <- struct {
			stream streams.Stream[*httpclient.StreamEvent]
			err    error
		}{stream: probed, err: err}
	}()

	require.Eventually(t, func() bool {
		return stream.nextCalls.Load() > 0
	}, time.Second, 10*time.Millisecond)

	var result struct {
		stream streams.Stream[*httpclient.StreamEvent]
		err    error
	}
	select {
	case result = <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("probe did not return after duration")
	}
	require.NoError(t, result.err)
	require.NotNil(t, result.stream)

	nextResult := make(chan bool, 1)
	go func() {
		defer recoverTestGoroutine(t)

		nextResult <- result.stream.Next()
	}()

	require.Never(t, func() bool {
		return stream.maxActive.Load() > 1
	}, 50*time.Millisecond, 5*time.Millisecond)

	stream.releaseReads()

	select {
	case ok := <-nextResult:
		require.True(t, ok)
	case <-time.After(time.Second):
		t.Fatal("probed stream did not deliver released event")
	}
	require.Equal(t, "late", string(result.stream.Current().Data))
	require.NoError(t, result.stream.Close())
}

func TestProbeStreamBeforeCommit_StopsBufferingAtEventCap(t *testing.T) {
	const expectedMaxBufferedEvents = 256

	events := make([]*httpclient.StreamEvent, expectedMaxBufferedEvents+1)
	for i := range events {
		events[i] = &httpclient.StreamEvent{Data: []byte("event")}
	}

	stream := newBlockingAfterEventsStream(events)

	resultCh := make(chan struct {
		stream streams.Stream[*httpclient.StreamEvent]
		err    error
	}, 1)
	go func() {
		defer recoverTestGoroutine(t)

		probed, err := probeStreamBeforeCommit(context.Background(), stream, time.Hour)
		resultCh <- struct {
			stream streams.Stream[*httpclient.StreamEvent]
			err    error
		}{stream: probed, err: err}
	}()

	var result struct {
		stream streams.Stream[*httpclient.StreamEvent]
		err    error
	}
	select {
	case result = <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("probe did not stop buffering after reaching the event cap")
	}
	require.NoError(t, result.err)
	require.NotNil(t, result.stream)
	defer result.stream.Close()

	for i := 0; i < expectedMaxBufferedEvents+1; i++ {
		require.True(t, result.stream.Next(), "event %d should be replayed or delivered live", i)
		require.Equal(t, "event", string(result.stream.Current().Data))
	}
}

func rawToLlmObjectStream(
	ctx context.Context,
	req *httpclient.Request,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return streams.Map(stream, func(event *httpclient.StreamEvent) *llm.Response {
		return &llm.Response{Object: string(event.Data)}
	}), nil
}

func llmObjectToRawStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.Map(stream, func(resp *llm.Response) *httpclient.StreamEvent {
		return &httpclient.StreamEvent{Data: []byte(resp.Object)}
	}), nil
}

func collectStreamEvents(t *testing.T, stream streams.Stream[*httpclient.StreamEvent]) []*httpclient.StreamEvent {
	t.Helper()
	defer stream.Close()

	var events []*httpclient.StreamEvent
	for stream.Next() {
		events = append(events, stream.Current())
	}
	require.NoError(t, stream.Err())

	return events
}

func streamEventData(events []*httpclient.StreamEvent) []string {
	data := make([]string, 0, len(events))
	for _, event := range events {
		data = append(data, string(event.Data))
	}

	return data
}

func recoverTestGoroutine(t *testing.T) {
	t.Helper()

	if r := recover(); r != nil {
		t.Errorf("test goroutine panic: %v", r)
	}
}

type blockingAfterEventsStream struct {
	events []*httpclient.StreamEvent
	index  int
	closed chan struct{}
	once   sync.Once
}

func newBlockingAfterEventsStream(events []*httpclient.StreamEvent) *blockingAfterEventsStream {
	return &blockingAfterEventsStream{
		events: events,
		closed: make(chan struct{}),
	}
}

func (s *blockingAfterEventsStream) Next() bool {
	if s.index < len(s.events) {
		return true
	}

	<-s.closed
	return false
}

func (s *blockingAfterEventsStream) Current() *httpclient.StreamEvent {
	event := s.events[s.index]
	s.index++

	return event
}

func (s *blockingAfterEventsStream) Err() error {
	return nil
}

func (s *blockingAfterEventsStream) Close() error {
	s.once.Do(func() {
		close(s.closed)
	})

	return nil
}

type errorAfterEventsStream struct {
	events []*httpclient.StreamEvent
	err    error
	index  int
}

func newErrorAfterEventsStream(events []*httpclient.StreamEvent, err error) *errorAfterEventsStream {
	return &errorAfterEventsStream{
		events: events,
		err:    err,
	}
}

func (s *errorAfterEventsStream) Next() bool {
	return s.index < len(s.events)
}

func (s *errorAfterEventsStream) Current() *httpclient.StreamEvent {
	event := s.events[s.index]
	s.index++

	return event
}

func (s *errorAfterEventsStream) Err() error {
	if s.index >= len(s.events) {
		return s.err
	}

	return nil
}

func (s *errorAfterEventsStream) Close() error {
	return nil
}

type controlledStream struct {
	events    []*httpclient.StreamEvent
	index     int
	release   chan struct{}
	closed    chan struct{}
	closeOnce sync.Once

	active    atomic.Int32
	maxActive atomic.Int32
	nextCalls atomic.Int32
}

func newControlledStream(events []*httpclient.StreamEvent) *controlledStream {
	return &controlledStream{
		events:  events,
		release: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (s *controlledStream) Next() bool {
	active := s.active.Add(1)
	defer s.active.Add(-1)

	for {
		maxActive := s.maxActive.Load()
		if active <= maxActive || s.maxActive.CompareAndSwap(maxActive, active) {
			break
		}
	}
	s.nextCalls.Add(1)

	select {
	case <-s.release:
	case <-s.closed:
		return false
	}

	return s.index < len(s.events)
}

func (s *controlledStream) Current() *httpclient.StreamEvent {
	event := s.events[s.index]
	s.index++

	return event
}

func (s *controlledStream) Err() error {
	return nil
}

func (s *controlledStream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})

	return nil
}

func (s *controlledStream) releaseReads() {
	close(s.release)
}
