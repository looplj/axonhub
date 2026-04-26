package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func testHTTPStream(events []*httpclient.StreamEvent) streams.Stream[*httpclient.StreamEvent] {
	return streams.SliceStream(events)
}

// === captureRawProviderResponse tests ===

func TestCaptureRawProviderResponse_StoresResponse(t *testing.T) {
	ctx := context.Background()
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: &biz.Channel{
				Channel: &ent.Channel{
					ID:   1,
					Name: "test",
					Settings: &objects.ChannelSettings{
						PassThroughBody: true,
					},
				},
			},
		},
		LlmRequest: &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := captureRawProviderResponse(outbound)
	resp := &httpclient.Response{StatusCode: 200, Body: []byte("ok")}

	result, err := mw.OnOutboundRawResponse(ctx, resp)
	require.NoError(t, err)
	assert.Equal(t, resp, result)
	assert.Equal(t, resp, state.RawProviderResponse)
}

// === applyPassThroughResponse tests ===

func TestApplyPassThroughResponse_Disabled(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughResponse(outbound)
	transformed := &httpclient.Response{StatusCode: 200, Body: []byte("transformed")}
	state.RawProviderResponse = &httpclient.Response{StatusCode: 200, Body: []byte("raw")}

	result, err := mw.OnInboundRawResponse(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, transformed, result)
}

func TestApplyPassThroughResponse_Enabled_ReturnsRaw(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion},
		state:   state,
	}

	mw := applyPassThroughResponse(outbound)
	transformed := &httpclient.Response{StatusCode: 200, Body: []byte("transformed")}
	rawResp := &httpclient.Response{
		StatusCode: 200,
		Body:       []byte("raw"),
	}
	state.RawProviderResponse = rawResp

	result, err := mw.OnInboundRawResponse(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, rawResp, result)
}

func TestApplyPassThroughResponse_MismatchedAPIFormat(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatAnthropicMessage),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion},
		state:   state,
	}

	mw := applyPassThroughResponse(outbound)
	transformed := &httpclient.Response{StatusCode: 200, Body: []byte("transformed")}
	rawResp := &httpclient.Response{
		StatusCode: 200,
		Body:       []byte("raw"),
	}
	state.RawProviderResponse = rawResp

	result, err := mw.OnInboundRawResponse(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, transformed, result, "return transformed when formats mismatch")
}

func TestApplyPassThroughResponse_NilLlmRequest(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughResponse(outbound)
	transformed := &httpclient.Response{StatusCode: 200, Body: []byte("transformed")}
	state.RawProviderResponse = &httpclient.Response{
		StatusCode: 200,
		Body:       []byte("raw"),
	}

	result, err := mw.OnInboundRawResponse(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, transformed, result)
}

func TestApplyPassThroughResponse_UsesRawProviderRequestAPIFormat(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatAnthropicMessage},
		state:   state,
	}

	mw := applyPassThroughResponse(outbound)
	transformed := &httpclient.Response{StatusCode: 200, Body: []byte("transformed")}
	rawResp := &httpclient.Response{
		StatusCode: 200,
		Body:       []byte("raw"),
		Request:    &httpclient.Request{APIFormat: string(llm.APIFormatAnthropicMessage)},
	}
	state.RawProviderResponse = rawResp

	result, err := mw.OnInboundRawResponse(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, rawResp, result)
}

func TestApplyPassThroughResponse_NilSettings(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:       1,
			Name:     "test",
			Settings: nil,
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughResponse(outbound)
	transformed := &httpclient.Response{StatusCode: 200, Body: []byte("transformed")}

	result, err := mw.OnInboundRawResponse(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, transformed, result)
}

// === captureRawProviderStream tests ===

func TestCaptureRawProviderStream_Disabled(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := captureRawProviderStream(outbound)
	original := testHTTPStream(nil)

	result, err := mw.OnOutboundRawStream(ctx, original)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestCaptureRawProviderStream_NilLlmRequest(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion},
		state:   state,
	}

	mw := captureRawProviderStream(outbound)
	original := testHTTPStream(nil)

	result, err := mw.OnOutboundRawStream(ctx, original)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestCaptureRawProviderStream_FansOut(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion},
		state:   state,
	}

	events := []*httpclient.StreamEvent{
		{Data: json.RawMessage(`{"id":"evt1"}`)},
		{Data: json.RawMessage(`{"id":"evt2"}`)},
		{Data: json.RawMessage(`{"id":"evt3"}`)},
	}
	src := testHTTPStream(events)

	mw := captureRawProviderStream(outbound)
	result, err := mw.OnOutboundRawStream(ctx, src)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, state.RawStreamCh)

	var (
		wg                sync.WaitGroup
		pipelineEvents    []*httpclient.StreamEvent
		passthroughEvents []*httpclient.StreamEvent
	)

	wg.Add(2)

	go func() {
		defer wg.Done()

		for result.Next() {
			pipelineEvents = append(pipelineEvents, result.Current())
		}
	}()

	go func() {
		defer wg.Done()

		for ev := range state.RawStreamCh {
			passthroughEvents = append(passthroughEvents, ev)
		}
	}()

	wg.Wait()

	assert.Len(t, pipelineEvents, 3)
	assert.Len(t, passthroughEvents, 3)
	assert.Equal(t, events, pipelineEvents)
	assert.Equal(t, events, passthroughEvents)
}

func TestCaptureRawProviderStream_PropagatesError(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion},
		state:   state,
	}

	errTest := errors.New("stream error")
	src := &errorStream{err: errTest}

	mw := captureRawProviderStream(outbound)
	result, err := mw.OnOutboundRawStream(ctx, src)
	require.NoError(t, err)

	// Wait for goroutine to finish
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, errTest, result.Err())
	assert.Equal(t, errTest, *state.RawStreamErrRef)
}

func TestCaptureRawProviderStream_UsesRawProviderRequestAPIFormat(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatAnthropicMessage},
		state:   state,
	}

	mw := captureRawProviderStream(outbound)
	original := testHTTPStream(nil)

	result, err := mw.OnOutboundRawStream(ctx, original)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEqual(t, original, result)
	assert.NotNil(t, state.RawStreamCh)
}

type errorStream struct {
	err error
}

func (s *errorStream) Next() bool                       { return false }
func (s *errorStream) Current() *httpclient.StreamEvent { return nil }
func (s *errorStream) Err() error                       { return s.err }
func (s *errorStream) Close() error                     { return nil }

// === applyPassThroughStream tests ===

func TestApplyPassThroughStream_Disabled(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughStream(outbound)
	transformed := testHTTPStream(nil)

	result, err := mw.OnInboundRawStream(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, transformed, result)
}

func TestApplyPassThroughStream_NoRawChannel(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughStream(outbound)
	transformed := testHTTPStream(nil)

	result, err := mw.OnInboundRawStream(ctx, transformed)
	require.NoError(t, err)
	assert.Equal(t, transformed, result)
}

func TestApplyPassThroughStream_ReturnsRawEvents(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	rawCh := make(chan *httpclient.StreamEvent, 8)
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		RawStreamCh:      rawCh,
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	transformed := testHTTPStream([]*httpclient.StreamEvent{
		{Data: json.RawMessage(`{"id":"t1"}`)},
		{Data: json.RawMessage(`{"id":"t2"}`)},
	})

	rawEvents := []*httpclient.StreamEvent{
		{Data: json.RawMessage(`{"id":"r1"}`)},
		{Data: json.RawMessage(`{"id":"r2"}`)},
	}

	go func() {
		for _, ev := range rawEvents {
			rawCh <- ev
		}

		close(rawCh)
	}()

	mw := applyPassThroughStream(outbound)
	result, err := mw.OnInboundRawStream(ctx, transformed)
	require.NoError(t, err)

	var passthroughEvents []*httpclient.StreamEvent
	for result.Next() {
		passthroughEvents = append(passthroughEvents, result.Current())
	}

	assert.Len(t, passthroughEvents, 2)
	assert.Equal(t, rawEvents, passthroughEvents)
}

func TestApplyPassThroughStream_DrainsInner(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}
	rawCh := make(chan *httpclient.StreamEvent, 8)
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		RawStreamCh:      rawCh,
		LlmRequest:       &llm.Request{APIFormat: llm.APIFormatOpenAIChatCompletion},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	drained := make(chan struct{})
	transformed := &doneStream{
		stream: testHTTPStream([]*httpclient.StreamEvent{
			{Data: json.RawMessage(`{"id":"t1"}`)},
		}),
		done: drained,
	}

	// Feed raw events and close
	go func() {
		rawCh <- &httpclient.StreamEvent{Data: json.RawMessage(`{"id":"r1"}`)}

		close(rawCh)
	}()

	mw := applyPassThroughStream(outbound)
	result, err := mw.OnInboundRawStream(ctx, transformed)
	require.NoError(t, err)

	for result.Next() {
	}

	// Wait for drain goroutine
	select {
	case <-drained:
	case <-time.After(2 * time.Second):
		t.Fatal("drain goroutine did not complete")
	}
}

type doneStream struct {
	stream streams.Stream[*httpclient.StreamEvent]
	done   chan struct{}
}

func (s *doneStream) Next() bool {
	ok := s.stream.Next()
	if !ok {
		close(s.done)
	}

	return ok
}

func (s *doneStream) Current() *httpclient.StreamEvent { return s.stream.Current() }
func (s *doneStream) Err() error                       { return s.stream.Err() }
func (s *doneStream) Close() error                     { return s.stream.Close() }

// === Integration: LLM stream middleware runs during stream pass-through ===

// trackingLLM is a middleware that verifies OnOutboundLlmStream is called during pass-through.
type trackingLLM struct {
	pipeline.DummyMiddleware

	called    bool
	evtCount  int
	closeCall bool
	mu        sync.Mutex
}

func (m *trackingLLM) Name() string { return "tracking-llm" }

func (m *trackingLLM) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	m.mu.Lock()
	m.called = true
	m.mu.Unlock()

	return &trackingWrapper{
		stream: stream,
		mw:     m,
	}, nil
}

type trackingWrapper struct {
	stream streams.Stream[*llm.Response]
	mw     *trackingLLM
}

func (w *trackingWrapper) Next() bool {
	ok := w.stream.Next()
	if ok {
		w.mw.mu.Lock()
		w.mw.evtCount++
		w.mw.mu.Unlock()
	}

	return ok
}

func (w *trackingWrapper) Current() *llm.Response { return w.stream.Current() }
func (w *trackingWrapper) Err() error             { return w.stream.Err() }

func (w *trackingWrapper) Close() error {
	w.mw.mu.Lock()
	w.mw.closeCall = true
	w.mw.mu.Unlock()

	return w.stream.Close()
}

// passthroughOutbound is an outbound transformer that maps raw events 1:1 to llm responses.
type passthroughOutbound struct {
	format llm.APIFormat
}

func (t *passthroughOutbound) APIFormat() llm.APIFormat { return t.format }

func (t *passthroughOutbound) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{APIFormat: t.format.String()}, nil
}

func (t *passthroughOutbound) TransformResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (t *passthroughOutbound) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.Map(stream, func(ev *httpclient.StreamEvent) *llm.Response {
		return &llm.Response{Model: string(ev.Data)}
	}), nil
}

func (t *passthroughOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return nil
}

func (t *passthroughOutbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, nil
}

// passthroughInbound is an inbound transformer that maps llm responses 1:1 to raw events.
type passthroughInbound struct {
	format llm.APIFormat
}

func (t *passthroughInbound) APIFormat() llm.APIFormat { return t.format }

func (t *passthroughInbound) TransformRequest(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
	return &llm.Request{APIFormat: t.format}, nil
}

func (t *passthroughInbound) TransformResponse(ctx context.Context, resp *llm.Response) (*httpclient.Response, error) {
	return &httpclient.Response{}, nil
}

func (t *passthroughInbound) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.Map(stream, func(llmResp *llm.Response) *httpclient.StreamEvent {
		return &httpclient.StreamEvent{Data: json.RawMessage(llmResp.Model)}
	}), nil
}

func (t *passthroughInbound) TransformError(ctx context.Context, err error) *httpclient.Error {
	return nil
}

func (t *passthroughInbound) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, nil
}

func TestPassThroughStream_LLMMiddlewareRuns(t *testing.T) {
	ctx := context.Background()
	format := llm.APIFormatOpenAIChatCompletion

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
		Outbound: &passthroughOutbound{format: format},
	}

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: format},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(format),
		},
	}

	outbound := &PersistentOutboundTransformer{
		wrapped: &passthroughOutbound{format: format},
		state:   state,
	}

	tracker := &trackingLLM{}

	rawEvents := []*httpclient.StreamEvent{
		{Data: json.RawMessage(`"evt1"`)},
		{Data: json.RawMessage(`"evt2"`)},
	}
	srcStream := testHTTPStream(rawEvents)

	// Step 1: captureRawProviderStream wraps/fans out srcStream
	capMw := captureRawProviderStream(outbound)
	pipelineStream, err := capMw.OnOutboundRawStream(ctx, srcStream)
	require.NoError(t, err)
	require.NotNil(t, state.RawStreamCh)

	// Step 2: Outbound TransformStream (raw → llm)
	llmStream, err := outbound.wrapped.TransformStream(ctx, pipelineStream)
	require.NoError(t, err)

	// Step 3: tracking middleware wraps LLM stream
	trackedLLM, err := tracker.OnOutboundLlmStream(ctx, llmStream)
	require.NoError(t, err)
	require.True(t, tracker.called, "OnOutboundLlmStream should be called")

	// Step 4: Inbound TransformStream (llm → raw)
	inbound := &passthroughInbound{format: format}
	inboundStream, err := inbound.TransformStream(ctx, trackedLLM)
	require.NoError(t, err)

	// Step 5: applyPassThroughStream drains the transformed stream
	applyMw := applyPassThroughStream(outbound)
	result, err := applyMw.OnInboundRawStream(ctx, inboundStream)
	require.NoError(t, err)

	// Consume passthrough stream
	var passthroughEvents []*httpclient.StreamEvent
	for result.Next() {
		passthroughEvents = append(passthroughEvents, result.Current())
	}

	// Passthrough client receives raw events
	require.Len(t, passthroughEvents, 2)
	assert.Equal(t, rawEvents, passthroughEvents)

	// Wait for drain to complete and tracking to process
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 2, tracker.evtCount, "tracking middleware should process 2 events")
}

func TestPassThroughStream_ErrorPropagates(t *testing.T) {
	ctx := context.Background()
	format := llm.APIFormatOpenAIChatCompletion

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest:       &llm.Request{APIFormat: format},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(format),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: format},
		state:   state,
	}

	errTest := errors.New("stream error")
	src := &errorStream{err: errTest}

	capMw := captureRawProviderStream(outbound)
	result, err := capMw.OnOutboundRawStream(ctx, src)
	require.NoError(t, err)

	// Wait for fan-out goroutine
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, errTest, result.Err())
	assert.Equal(t, errTest, *state.RawStreamErrRef)
}

func TestApplyPassThroughBodyPreservesMappedModel(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-model-mapping",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
			LlmRequest: &llm.Request{
				Model:     "gpt-4o",
				APIFormat: llm.APIFormatOpenAIChatCompletion,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAIChatCompletion),
					Body:      []byte(`{"model":"my-alias","messages":[{"role":"user","content":"hi"}],"temperature":0.4}`),
				},
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		Body:      []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
	}

	processed, err := applyPassThroughRequestBody(outbound).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "gpt-4o", gjson.GetBytes(processed.Body, "model").String())
	require.Equal(t, 0.4, gjson.GetBytes(processed.Body, "temperature").Float())
	require.Equal(t, "my-alias", gjson.GetBytes(outbound.state.LlmRequest.RawRequest.Body, "model").String())

	processed.Body[0] = '['
	require.Equal(t, `{"model":"my-alias","messages":[{"role":"user","content":"hi"}],"temperature":0.4}`, string(outbound.state.LlmRequest.RawRequest.Body))
}

func TestApplyPassThroughBodyPreservesMappedModelForJinaRerank(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-jina-rerank-model-mapping",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
			LlmRequest: &llm.Request{
				Model:     "Qwen/Qwen3-Reranker-8B",
				APIFormat: llm.APIFormatJinaRerank,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatJinaRerank),
					Body:      []byte(`{"model":"Qwen-3-Rerank-8B","query":"what is ai","documents":["a","b"],"top_n":2}`),
				},
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatJinaRerank),
		Body:      []byte(`{"model":"Qwen/Qwen3-Reranker-8B","query":"what is ai","documents":["a","b"]}`),
	}

	processed, err := applyPassThroughRequestBody(outbound).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "Qwen/Qwen3-Reranker-8B", gjson.GetBytes(processed.Body, "model").String())
	require.Equal(t, float64(2), gjson.GetBytes(processed.Body, "top_n").Float())
	require.Equal(t, "Qwen-3-Rerank-8B", gjson.GetBytes(outbound.state.LlmRequest.RawRequest.Body, "model").String())
}

func TestApplyPassThroughBodyPreservesMappedModelForJinaEmbedding(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-jina-embedding-model-mapping",
			Settings: &objects.ChannelSettings{
				PassThroughBody: true,
			},
		},
	}

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
			LlmRequest: &llm.Request{
				Model:     "jina-embeddings-v3",
				APIFormat: llm.APIFormatJinaEmbedding,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatJinaEmbedding),
					Body:      []byte(`{"model":"my-embedding-alias","input":"hello","task":"retrieval.query"}`),
				},
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatJinaEmbedding),
		Body:      []byte(`{"model":"jina-embeddings-v3","input":"hello"}`),
	}

	processed, err := applyPassThroughRequestBody(outbound).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "jina-embeddings-v3", gjson.GetBytes(processed.Body, "model").String())
	require.Equal(t, "retrieval.query", gjson.GetBytes(processed.Body, "task").String())
	require.Equal(t, "my-embedding-alias", gjson.GetBytes(outbound.state.LlmRequest.RawRequest.Body, "model").String())
}

func TestMergePassThroughBodySkipsFormatsWithoutTopLevelModel(t *testing.T) {
	rawBody := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)

	merged, err := mergePassThroughRequestBody(rawBody, llm.APIFormatGeminiContents, "gemini-2.5-pro")
	require.NoError(t, err)
	require.Equal(t, string(rawBody), string(merged))
}

// TestApplyUserAgentPassThrough tests the User-Agent pass-through middleware.
func TestApplyUserAgentPassThrough(t *testing.T) {
	tests := []struct {
		name             string
		channelUASetting *bool // Channel-level override
		globalUAEnabled  bool  // System-level setting
		clientUA         string
		wantUAHeader     string
	}{
		{
			name:             "channel_disabled_ignores_global",
			channelUASetting: new(false),
			globalUAEnabled:  true,
			clientUA:         "Client/1.0",
			wantUAHeader:     "axonhub/1.0", // Pass-through disabled: middleware sets default UA
		},
		{
			name:             "channel_enabled_ignores_global",
			channelUASetting: new(true),
			globalUAEnabled:  false,
			clientUA:         "Client/1.0",
			wantUAHeader:     "Client/1.0",
		},
		{
			name:             "channel_nil_inherits_global_disabled",
			channelUASetting: nil,
			globalUAEnabled:  false,
			clientUA:         "Client/1.0",
			wantUAHeader:     "axonhub/1.0", // Pass-through disabled: middleware sets default UA
		},
		{
			name:             "channel_nil_inherits_global_enabled",
			channelUASetting: nil,
			globalUAEnabled:  true,
			clientUA:         "Client/1.0",
			wantUAHeader:     "Client/1.0",
		},
		{
			name:             "enabled_but_no_client_ua",
			channelUASetting: new(true),
			globalUAEnabled:  true,
			clientUA:         "",
			wantUAHeader:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, client := setupTest(t)

			// Create real system service with test database
			systemService := newTestSystemService(client)

			// Set global User-Agent pass-through setting
			err := systemService.SetUserAgentPassThrough(ctx, tt.globalUAEnabled)
			require.NoError(t, err)

			// Create mock channel with optional pass-through setting
			channelSettings := &objects.ChannelSettings{}
			if tt.channelUASetting != nil {
				channelSettings.PassThroughUserAgent = tt.channelUASetting
			}

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:       1,
					Name:     "test-channel",
					Settings: channelSettings,
				},
				Outbound: &mockTransformer{},
			}

			// Create raw request with client UA - RawRequest is *httpclient.Request in llm.Request
			rawHeaders := make(http.Header)
			if tt.clientUA != "" {
				rawHeaders.Set("User-Agent", tt.clientUA)
			}

			llmRequest := &llm.Request{
				Model: "gpt-4",
				RawRequest: &httpclient.Request{
					Headers: rawHeaders,
				},
			}

			// Create outbound transformer
			outbound := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
					LlmRequest:       llmRequest,
				},
			}

			// Create middleware
			middleware := applyUserAgentPassThrough(outbound, systemService)

			// Execute middleware
			rawRequest := &httpclient.Request{
				Headers: make(http.Header),
			}
			processedRequest, err := middleware.OnOutboundRawRequest(ctx, rawRequest)

			require.NoError(t, err)
			require.NotNil(t, processedRequest)

			// Verify User-Agent header is set correctly
			if tt.wantUAHeader != "" {
				require.Equal(t, tt.wantUAHeader, processedRequest.Headers.Get("User-Agent"))
			} else {
				// When no User-Agent expected, header should be empty
				require.Empty(t, processedRequest.Headers.Get("User-Agent"))
			}
		})
	}
}

// TestApplyUserAgentPassThrough_NoChannel tests the middleware when no channel is selected.
func TestApplyUserAgentPassThrough_NoChannel(t *testing.T) {
	ctx, client := setupTest(t)

	// Create real system service with test database
	systemService := newTestSystemService(client)

	// Create outbound without a channel
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state:   &PersistenceState{},
	}

	// Create middleware
	middleware := applyUserAgentPassThrough(outbound, systemService)

	// Execute middleware
	rawRequest := &httpclient.Request{
		Headers: make(http.Header),
	}
	processedRequest, err := middleware.OnOutboundRawRequest(ctx, rawRequest)
	require.NoError(t, err)
	require.NotNil(t, processedRequest)
}
