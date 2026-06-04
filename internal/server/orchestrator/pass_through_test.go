package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/samber/lo"
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
						PassThroughBody: lo.ToPtr(true),
					},
				},
			},
		},
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := captureRawProviderResponse(outbound, nil)
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

	mw := applyPassThroughResponse(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: nil,
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion},
		state:   state,
	}

	mw := applyPassThroughResponse(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
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

	mw := applyPassThroughResponse(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughResponse(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: nil,
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatAnthropicMessage},
		state:   state,
	}

	mw := applyPassThroughResponse(outbound, nil)
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

func TestIsPassThroughEnabled_DisablesWhenSupportedStreamParameterChanges(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"my-alias","stream":false,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	assert.False(t, outbound.isPassThroughEnabled(ctx, nil))
}

func TestIsPassThroughEnabled_DisablesWhenSupportedStreamParameterMissingButStreamingRequested(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"my-alias","messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	assert.False(t, outbound.isPassThroughEnabled(ctx, nil))
}

func TestIsPassThroughEnabled_AllowsNilAndFalseStreamToAlign(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: nil,
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(false),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"my-alias","stream":false,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	assert.True(t, outbound.isPassThroughEnabled(ctx, nil))
}

func TestIsPassThroughEnabled_DisablesWhenRequestStreamSemanticsDoNotMatchCurrentRequirement(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatGeminiContents,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatGeminiContents),
				Body:      []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatGeminiContents),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	assert.False(t, outbound.isPassThroughEnabled(ctx, nil))
}

func TestIsPassThroughEnabled_DisablesWhenOriginalRequestWasNonStreamingButExecutionIsForcedStreaming(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: lo.ToPtr(false),
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"my-alias","messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	assert.False(t, outbound.isPassThroughEnabled(ctx, nil))
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

	mw := applyPassThroughResponse(outbound, nil)
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

	mw := captureRawProviderStream(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
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

	mw := captureRawProviderStream(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
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

	mw := captureRawProviderStream(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
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

	mw := captureRawProviderStream(outbound, nil)
	result, err := mw.OnOutboundRawStream(ctx, src)
	require.NoError(t, err)

	// Drain the stream until the producer goroutine closes the channel.
	// The channel close is the happens-before barrier that makes the
	// goroutine's write to rawStreamErr visible to Err() / RawStreamErrRef.
	for result.Next() { //nolint:revive // intentional drain
	}

	assert.Equal(t, errTest, result.Err())
	assert.Equal(t, errTest, *state.RawStreamErrRef)
}

func TestCaptureRawProviderStream_CloseStopsBlockedUpstream(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
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

	src := newBlockingStream()
	mw := captureRawProviderStream(outbound, nil)
	result, err := mw.OnOutboundRawStream(ctx, src)
	require.NoError(t, err)

	select {
	case <-src.started:
	case <-time.After(time.Second):
		t.Fatal("upstream stream was not read")
	}

	require.NoError(t, result.Close())

	select {
	case <-src.closed:
	case <-time.After(time.Second):
		t.Fatal("upstream stream was not closed")
	}

	select {
	case _, ok := <-state.RawStreamCh:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("pass-through channel was not closed")
	}
}

func TestCaptureRawProviderStream_UsesRawProviderRequestAPIFormat(t *testing.T) {
	ctx := context.Background()
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
		RawProviderRequest: &httpclient.Request{
			APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		},
	}
	outbound := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{apiFormat: llm.APIFormatAnthropicMessage},
		state:   state,
	}

	mw := captureRawProviderStream(outbound, nil)
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

type blockingStream struct {
	started   chan struct{}
	closed    chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func newBlockingStream() *blockingStream {
	return &blockingStream{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (s *blockingStream) Next() bool {
	s.startOnce.Do(func() {
		close(s.started)
	})
	<-s.closed

	return false
}

func (s *blockingStream) Current() *httpclient.StreamEvent { return nil }
func (s *blockingStream) Err() error                       { return nil }
func (s *blockingStream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})

	return nil
}

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

	mw := applyPassThroughStream(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	mw := applyPassThroughStream(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	rawCh := make(chan *httpclient.StreamEvent, 8)
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		RawStreamCh:           rawCh,
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
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

	mw := applyPassThroughStream(outbound, nil)
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}
	rawCh := make(chan *httpclient.StreamEvent, 8)
	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		RawStreamCh:           rawCh,
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
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

	go func() {
		rawCh <- &httpclient.StreamEvent{Data: json.RawMessage(`{"id":"r1"}`)}
		close(rawCh)
	}()

	mw := applyPassThroughStream(outbound, nil)
	result, err := mw.OnInboundRawStream(ctx, transformed)
	require.NoError(t, err)

	for result.Next() {
	}

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

// eventCount returns the current event count under the mutex so tests can
// safely read it without racing with trackingWrapper.Next.
func (m *trackingLLM) eventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.evtCount
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

func (t *passthroughOutbound) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.Map(stream, func(ev *httpclient.StreamEvent) *llm.Response {
		return &llm.Response{Model: string(ev.Data)}
	}), nil
}

func (t *passthroughOutbound) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return nil
}

func (t *passthroughOutbound) AggregateStreamChunks(ctx context.Context, _ *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
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
				PassThroughBody: lo.ToPtr(true),
			},
		},
		Outbound: &passthroughOutbound{format: format},
	}

	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: format,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(format),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
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

	capMw := captureRawProviderStream(outbound, nil)
	pipelineStream, err := capMw.OnOutboundRawStream(ctx, srcStream)
	require.NoError(t, err)
	require.NotNil(t, state.RawStreamCh)

	llmStream, err := outbound.wrapped.TransformStream(ctx, nil, pipelineStream)
	require.NoError(t, err)

	trackedLLM, err := tracker.OnOutboundLlmStream(ctx, llmStream)
	require.NoError(t, err)
	require.True(t, tracker.called, "OnOutboundLlmStream should be called")

	inbound := &passthroughInbound{format: format}
	inboundStream, err := inbound.TransformStream(ctx, trackedLLM)
	require.NoError(t, err)

	applyMw := applyPassThroughStream(outbound, nil)
	result, err := applyMw.OnInboundRawStream(ctx, inboundStream)
	require.NoError(t, err)

	var passthroughEvents []*httpclient.StreamEvent
	for result.Next() {
		passthroughEvents = append(passthroughEvents, result.Current())
	}

	require.Len(t, passthroughEvents, 2)
	assert.Equal(t, rawEvents, passthroughEvents)

	// Wait for the applyPassThroughStream drain goroutine to finish processing.
	// Polling under the tracker mutex avoids racing with trackingWrapper.Next.
	require.Eventually(t, func() bool {
		return tracker.eventCount() == 2
	}, time.Second, 10*time.Millisecond, "tracking middleware should process 2 events")
}

func TestPassThroughStream_ErrorPropagates(t *testing.T) {
	ctx := context.Background()
	format := llm.APIFormatOpenAIChatCompletion

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}

	state := &PersistenceState{
		CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
		OriginalRequestStream: lo.ToPtr(true),
		LlmRequest: &llm.Request{
			APIFormat: format,
			Stream:    lo.ToPtr(true),
			RawRequest: &httpclient.Request{
				APIFormat: string(format),
				Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			},
		},
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

	capMw := captureRawProviderStream(outbound, nil)
	result, err := capMw.OnOutboundRawStream(ctx, src)
	require.NoError(t, err)

	// Drain the stream until the producer goroutine closes the channel.
	// The channel close is the happens-before barrier for the goroutine's
	// write to rawStreamErr.
	for result.Next() { //nolint:revive // intentional drain
	}

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
				PassThroughBody: lo.ToPtr(true),
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

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
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
				PassThroughBody: lo.ToPtr(true),
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

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
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
				PassThroughBody: lo.ToPtr(true),
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

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "jina-embeddings-v3", gjson.GetBytes(processed.Body, "model").String())
	require.Equal(t, "retrieval.query", gjson.GetBytes(processed.Body, "task").String())
	require.Equal(t, "my-embedding-alias", gjson.GetBytes(outbound.state.LlmRequest.RawRequest.Body, "model").String())
}

func TestApplyPassThroughBodySkipsPassThroughWhenSupportedStreamParameterChanges(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-stream-upgrade",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
	}

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
			LlmRequest: &llm.Request{
				Model:     "gpt-4o",
				Stream:    lo.ToPtr(true),
				APIFormat: llm.APIFormatOpenAIChatCompletion,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAIChatCompletion),
					Body:      []byte(`{"model":"my-alias","stream":false,"messages":[{"role":"user","content":"hi"}],"temperature":0.4}`),
				},
			},
		},
	}

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, string(request.Body), string(processed.Body))
	require.Equal(t, "gpt-4o", gjson.GetBytes(processed.Body, "model").String())
	require.True(t, gjson.GetBytes(processed.Body, "stream").Bool())
	require.False(t, gjson.GetBytes(processed.Body, "temperature").Exists())
}

func TestApplyPassThroughBodyPreservesAlignedStreamWithoutPatchingIt(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-aligned-stream",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate:      &ChannelModelsCandidate{Channel: channel},
			OriginalRequestStream: lo.ToPtr(true),
			LlmRequest: &llm.Request{
				Model:     "gpt-4o",
				Stream:    lo.ToPtr(true),
				APIFormat: llm.APIFormatOpenAIChatCompletion,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAIChatCompletion),
					Body:      []byte(`{"model":"my-alias","stream":true,"messages":[{"role":"user","content":"hi"}],"temperature":0.4}`),
				},
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		Body:      []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
	}

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "gpt-4o", gjson.GetBytes(processed.Body, "model").String())
	require.True(t, gjson.GetBytes(processed.Body, "stream").Bool())
	require.Equal(t, 0.4, gjson.GetBytes(processed.Body, "temperature").Float())
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

func TestApplyPassThroughBodySkipsMultipartAudio(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-multipart-audio",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}

	// The inbound multipart body uses the client boundary; the outbound transformer
	// rebuilds the multipart body with a new boundary, so the inbound bytes must not
	// replace the outbound body.
	inboundBody := []byte("--client-boundary\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nwhisper-1\r\n--client-boundary--\r\n")
	outboundBody := []byte("--new-boundary\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nmapped-whisper\r\n--new-boundary--\r\n")

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
			LlmRequest: &llm.Request{
				Model:     "mapped-whisper",
				APIFormat: llm.APIFormatOpenAITranscription,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAITranscription),
					Body:      inboundBody,
				},
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAITranscription),
		Body:      outboundBody,
	}

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, outboundBody, processed.Body)
}

func TestApplyPassThroughBodyAppliesSpeechModelPatch(t *testing.T) {
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "pass-through-speech",
			Settings: &objects.ChannelSettings{
				PassThroughBody: lo.ToPtr(true),
			},
		},
	}

	outbound := &PersistentOutboundTransformer{
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
			LlmRequest: &llm.Request{
				Model:     "tts-1-hd",
				APIFormat: llm.APIFormatOpenAISpeech,
				RawRequest: &httpclient.Request{
					APIFormat: string(llm.APIFormatOpenAISpeech),
					Body:      []byte(`{"model":"my-tts-alias","input":"hello","voice":"alloy"}`),
				},
			},
		},
	}

	request := &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAISpeech),
		Body:      []byte(`{"model":"tts-1-hd","input":"hello","voice":"alloy"}`),
	}

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, "tts-1-hd", gjson.GetBytes(processed.Body, "model").String())
	require.Equal(t, "alloy", gjson.GetBytes(processed.Body, "voice").String())
}
