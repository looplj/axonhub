package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/pkg/chunkbuffer"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

type livePreviewMiddleware struct {
	pipeline.DummyMiddleware

	state              *PersistenceState
	systemService      *biz.SystemService
	liveStreamRegistry *biz.LiveStreamRegistry
	enabled            bool
	initialized        bool
}

func withLivePreview(state *PersistenceState, systemService *biz.SystemService, liveStreamRegistry *biz.LiveStreamRegistry) pipeline.Middleware {
	return &livePreviewMiddleware{
		state:              state,
		systemService:      systemService,
		liveStreamRegistry: liveStreamRegistry,
	}
}

func (m *livePreviewMiddleware) Name() string {
	return "live-preview"
}

func (m *livePreviewMiddleware) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if m.liveStreamRegistry == nil {
		m.enabled = false
		return request, nil
	}

	if request == nil || request.Stream == nil || !*request.Stream {
		m.enabled = false
		return request, nil
	}

	if !m.initialized {
		m.enabled = m.systemService != nil && m.systemService.StoragePolicyOrDefault(ctx).LivePreview
		m.initialized = true
	}

	return request, nil
}

func (m *livePreviewMiddleware) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	if !m.enabled {
		return request, nil
	}

	if m.state.Request != nil && m.liveStreamRegistry.GetRequestBuffer(m.state.Request.ID) == nil {
		m.liveStreamRegistry.RegisterRequest(m.state.Request.ID, chunkbuffer.New())
	}

	if m.state.RequestExec != nil && m.liveStreamRegistry.GetExecutionBuffer(m.state.RequestExec.ID) == nil {
		m.liveStreamRegistry.RegisterExecution(m.state.RequestExec.ID, chunkbuffer.New())
	}

	return request, nil
}

func (m *livePreviewMiddleware) OnOutboundRawError(ctx context.Context, err error) {
	if !m.enabled || m.state == nil || m.liveStreamRegistry == nil {
		return
	}

	if m.state.RequestExec != nil {
		if buffer := m.liveStreamRegistry.GetExecutionBuffer(m.state.RequestExec.ID); buffer != nil {
			buffer.Close()
			m.liveStreamRegistry.UnregisterExecution(m.state.RequestExec.ID)
		}
	}

	if m.state.Request != nil {
		if buffer := m.liveStreamRegistry.GetRequestBuffer(m.state.Request.ID); buffer != nil {
			buffer.Close()
			m.liveStreamRegistry.UnregisterRequest(m.state.Request.ID)
		}
	}
}

func (m *livePreviewMiddleware) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	if !m.enabled {
		return stream, nil
	}

	if m.state == nil || m.state.RequestExec == nil {
		return stream, nil
	}

	buffer := m.liveStreamRegistry.GetExecutionBuffer(m.state.RequestExec.ID)
	if buffer == nil {
		return stream, nil
	}

	return &liveRequestExecutionStream{
		stream:             stream,
		buffer:             buffer,
		liveStreamRegistry: m.liveStreamRegistry,
		executionID:        m.state.RequestExec.ID,
	}, nil
}

func (m *livePreviewMiddleware) OnInboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	if !m.enabled {
		return stream, nil
	}

	if m.state == nil || m.state.Request == nil {
		return stream, nil
	}

	buffer := m.liveStreamRegistry.GetRequestBuffer(m.state.Request.ID)
	if buffer == nil {
		return stream, nil
	}

	return &liveRequestStream{
		stream:             stream,
		buffer:             buffer,
		liveStreamRegistry: m.liveStreamRegistry,
		requestID:          m.state.Request.ID,
	}, nil
}

type liveRequestExecutionStream struct {
	stream             streams.Stream[*httpclient.StreamEvent]
	buffer             *chunkbuffer.Buffer
	liveStreamRegistry *biz.LiveStreamRegistry
	executionID        int
	closed             bool
	current            *httpclient.StreamEvent
}

func (s *liveRequestExecutionStream) Next() bool {
	if !s.stream.Next() {
		s.current = nil
		return false
	}

	s.current = s.stream.Current()
	if s.current != nil {
		s.buffer.Append(s.current)
	}

	return true
}

func (s *liveRequestExecutionStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *liveRequestExecutionStream) Err() error {
	return s.stream.Err()
}

func (s *liveRequestExecutionStream) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	s.buffer.Close()
	s.liveStreamRegistry.UnregisterExecution(s.executionID)

	return s.stream.Close()
}

type liveRequestStream struct {
	stream             streams.Stream[*httpclient.StreamEvent]
	buffer             *chunkbuffer.Buffer
	liveStreamRegistry *biz.LiveStreamRegistry
	requestID          int
	closed             bool
	current            *httpclient.StreamEvent
}

func (s *liveRequestStream) Next() bool {
	if !s.stream.Next() {
		s.current = nil
		return false
	}

	s.current = s.stream.Current()
	if s.current != nil {
		s.buffer.Append(s.current)
	}

	return true
}

func (s *liveRequestStream) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *liveRequestStream) Err() error {
	return s.stream.Err()
}

func (s *liveRequestStream) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	s.buffer.Close()
	s.liveStreamRegistry.UnregisterRequest(s.requestID)

	return s.stream.Close()
}

var (
	_ streams.Stream[*httpclient.StreamEvent] = (*liveRequestExecutionStream)(nil)
	_ streams.Stream[*httpclient.StreamEvent] = (*liveRequestStream)(nil)
)
