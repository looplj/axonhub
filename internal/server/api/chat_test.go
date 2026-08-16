package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupUpstreamErrorPolicyTest(t *testing.T, policy biz.UpstreamErrorPolicy) (context.Context, *biz.SystemService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	err := systemService.SetRetryPolicy(ctx, &biz.RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       3,
		MaxSingleChannelRetries: 2,
		RetryDelayMs:            1000,
		LoadBalancerStrategy:    biz.LoadBalancerStrategyAdaptive,
		UpstreamErrorPolicy:     policy,
	})
	require.NoError(t, err)

	return ctx, systemService
}

// errorAfterStream emits items then returns an error.
type errorAfterStream struct {
	items []*httpclient.StreamEvent
	idx   int
	err   error
}

func (s *errorAfterStream) Next() bool {
	if s.idx < len(s.items) {
		return true
	}

	return false
}

func (s *errorAfterStream) Current() *httpclient.StreamEvent {
	item := s.items[s.idx]
	s.idx++

	return item
}

func (s *errorAfterStream) Err() error {
	if s.idx >= len(s.items) {
		return s.err
	}

	return nil
}

func (s *errorAfterStream) Close() error { return nil }

type trackingStream struct {
	items   []*httpclient.StreamEvent
	idx     int
	current *httpclient.StreamEvent
}

func (s *trackingStream) Next() bool {
	if s.idx >= len(s.items) {
		return false
	}

	s.current = s.items[s.idx]
	s.idx++

	return true
}

func (s *trackingStream) Current() *httpclient.StreamEvent { return s.current }
func (s *trackingStream) Err() error                       { return nil }
func (s *trackingStream) Close() error                     { return nil }

type delayedStream struct {
	delay   time.Duration
	event   *httpclient.StreamEvent
	current *httpclient.StreamEvent
	done    bool
}

func (s *delayedStream) Next() bool {
	if s.done {
		return false
	}

	time.Sleep(s.delay)
	s.current = s.event
	s.done = true

	return true
}

func (s *delayedStream) Current() *httpclient.StreamEvent { return s.current }
func (s *delayedStream) Err() error                       { return nil }
func (s *delayedStream) Close() error                     { return nil }

type blockingStream struct {
	nextStarted  chan struct{}
	nextReleased chan struct{}
	nextReturned chan struct{}
}

func (s *blockingStream) Next() bool {
	close(s.nextStarted)
	<-s.nextReleased
	close(s.nextReturned)

	return false
}

func (s *blockingStream) Current() *httpclient.StreamEvent { return nil }
func (s *blockingStream) Err() error                       { return nil }
func (s *blockingStream) Close() error                     { return nil }

type blockingEventStream struct {
	event        *httpclient.StreamEvent
	nextStarted  chan struct{}
	nextReleased chan struct{}
	current      *httpclient.StreamEvent
	done         bool
}

func (s *blockingEventStream) Next() bool {
	if s.done {
		return false
	}

	close(s.nextStarted)
	<-s.nextReleased
	s.current = s.event
	s.done = true

	return true
}

func (s *blockingEventStream) Current() *httpclient.StreamEvent { return s.current }
func (s *blockingEventStream) Err() error                       { return nil }
func (s *blockingEventStream) Close() error                     { return nil }

type failingResponseWriter struct {
	gin.ResponseWriter

	err    error
	writes int
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	w.writes++

	return 0, w.err
}

type heartbeatFailingResponseWriter struct {
	gin.ResponseWriter

	err    error
	failed chan struct{}
}

func (w *heartbeatFailingResponseWriter) Write(_ []byte) (int, error) {
	select {
	case w.failed <- struct{}{}:
	default:
	}

	return 0, w.err
}

func (w *heartbeatFailingResponseWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func TestWriteSSEStream_Success(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Type: "", Data: []byte(`{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)},
		{Type: "", Data: []byte(`[DONE]`)},
	}
	stream := streams.SliceStream(events)

	WriteSSEStream(c, stream)

	body := w.Body.String()
	assert.Contains(t, body, `{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)
	assert.Contains(t, body, `[DONE]`)
}

func TestWriteSSEStream_OpenAIHeartbeat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := &delayedStream{
		delay: 25 * time.Millisecond,
		event: &httpclient.StreamEvent{Data: []byte(`[DONE]`)},
	}

	writeSSEStream(c, stream, FormatStreamError, SSEKeepAliveConfig{
		Enabled:  true,
		Interval: 5 * time.Millisecond,
	}, sseHeartbeatOpenAI)

	body := w.Body.String()
	require.Contains(t, body, ": keep-alive\n\n")
	require.Contains(t, body, "data: [DONE]\n\n")
}

func TestWriteSSEStream_AnthropicHeartbeat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := &delayedStream{
		delay: 25 * time.Millisecond,
		event: &httpclient.StreamEvent{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
	}

	writeSSEStream(c, stream, FormatStreamError, SSEKeepAliveConfig{
		Enabled:  true,
		Interval: 5 * time.Millisecond,
	}, sseHeartbeatAnthropic)

	body := w.Body.String()
	require.Contains(t, body, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
	require.Contains(t, body, "event:message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
}

func TestWriteSSEStream_HeartbeatWriteErrorWaitsForReader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	failingWriter := &heartbeatFailingResponseWriter{
		ResponseWriter: c.Writer,
		err:            errors.New("broken pipe"),
		failed:         make(chan struct{}, 1),
	}
	c.Writer = failingWriter

	stream := &blockingStream{
		nextStarted:  make(chan struct{}),
		nextReleased: make(chan struct{}),
		nextReturned: make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		writeSSEStreamWithHeartbeat(c, stream, FormatStreamError, time.Millisecond, sseHeartbeatOpenAI)
		close(done)
	}()

	select {
	case <-failingWriter.failed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for heartbeat write failure")
	}

	select {
	case <-stream.nextStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream reader")
	}

	select {
	case <-done:
		t.Fatal("writer returned before the stream reader stopped")
	case <-time.After(10 * time.Millisecond):
	}

	close(stream.nextReleased)

	select {
	case <-stream.nextReturned:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream reader to stop")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE writer")
	}
}

func TestWriteSSEStream_CanceledContextDrainsHeartbeatReader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	stream := &blockingEventStream{
		event:        &httpclient.StreamEvent{Data: []byte(`[DONE]`)},
		nextStarted:  make(chan struct{}),
		nextReleased: make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		writeSSEStreamWithHeartbeat(c, stream, FormatStreamError, time.Hour, sseHeartbeatOpenAI)
		close(done)
	}()

	select {
	case <-stream.nextStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stream reader")
	}

	cancel()
	close(stream.nextReleased)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canceled SSE writer")
	}

	assert.Contains(t, w.Body.String(), "data: [DONE]\n\n")
	assert.NotContains(t, w.Body.String(), "keep-alive")
}

func TestWriteSSEStream_DefaultHasNoHeartbeat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := &delayedStream{
		delay: 10 * time.Millisecond,
		event: &httpclient.StreamEvent{Data: []byte(`[DONE]`)},
	}

	WriteSSEStream(c, stream)

	require.NotContains(t, w.Body.String(), "keep-alive")
}

func TestWriteSSEStream_CanceledContextStillDrainsBufferedEvents(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Type: "", Data: []byte(`{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)},
		{Type: "", Data: []byte(`[DONE]`)},
	}
	stream := streams.SliceStream(events)

	WriteSSEStream(c, stream)

	body := w.Body.String()
	assert.Contains(t, body, `{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)
	assert.Contains(t, body, `[DONE]`)
	assert.NotContains(t, body, `"error"`)
}

func TestWriteSSEStream_ErrorFormatsAsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	streamErr := errors.New("upstream connection reset")
	stream := &errorAfterStream{
		items: []*httpclient.StreamEvent{
			{Type: "", Data: []byte(`{"id":"1","choices":[{"delta":{"content":"He"}}]}`)},
		},
		err: streamErr,
	}

	WriteSSEStream(c, stream)

	body := w.Body.String()

	// The error event should be JSON-formatted, not a plain string
	assert.Contains(t, body, "event:error")

	// Extract the data line from the error event
	lines := strings.Split(body, "\n")

	var errorData string

	foundError := false

	for i, line := range lines {
		if strings.HasPrefix(line, "event:error") {
			foundError = true
			// The next line should be the data
			if i+1 < len(lines) {
				errorData = strings.TrimPrefix(lines[i+1], "data:")
			}

			break
		}
	}

	require.True(t, foundError, "should contain an error event")
	require.NotEmpty(t, errorData, "error event should have data")

	// Parse the JSON error
	var errObj map[string]any

	err := json.Unmarshal([]byte(errorData), &errObj)
	require.NoError(t, err, "error data should be valid JSON: %s", errorData)

	// Verify structure
	errorField, ok := errObj["error"].(map[string]any)
	require.True(t, ok, "should have 'error' field")
	assert.Equal(t, "upstream connection reset", errorField["message"])
	assert.Equal(t, "server_error", errorField["type"])
	_, hasCode := errorField["code"]
	assert.True(t, hasCode)
}

func TestWriteSSEStream_HttpClientError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	httpErr := &httpclient.Error{
		StatusCode: 429,
		Body:       []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error"}}`),
	}
	stream := &errorAfterStream{err: httpErr}

	WriteSSEStream(c, stream)

	body := w.Body.String()

	// Extract error data
	lines := strings.Split(body, "\n")

	var errorData string

	for i, line := range lines {
		if strings.HasPrefix(line, "event:error") {
			if i+1 < len(lines) {
				errorData = strings.TrimPrefix(lines[i+1], "data:")
			}

			break
		}
	}

	require.NotEmpty(t, errorData)

	var errObj map[string]any

	err := json.Unmarshal([]byte(errorData), &errObj)
	require.NoError(t, err)

	errorField, ok := errObj["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Rate limit exceeded", errorField["message"])
	assert.Equal(t, "rate_limit_error", errorField["type"])
	assert.Empty(t, errorField["code"])
}

func TestWriteSSEStream_CustomErrorFormatter(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	streamErr := errors.New("custom error")
	stream := &errorAfterStream{err: streamErr}

	customFormatter := func(_ context.Context, err error) any {
		return gin.H{"custom_error": err.Error()}
	}

	WriteSSEStreamWithErrorFormatter(c, stream, customFormatter)

	body := w.Body.String()
	lines := strings.Split(body, "\n")

	var errorData string

	for i, line := range lines {
		if strings.HasPrefix(line, "event:error") {
			if i+1 < len(lines) {
				errorData = strings.TrimPrefix(lines[i+1], "data:")
			}

			break
		}
	}

	require.NotEmpty(t, errorData)

	var errObj map[string]any

	err := json.Unmarshal([]byte(errorData), &errObj)
	require.NoError(t, err)
	assert.Equal(t, "custom error", errObj["custom_error"])
}

func TestWriteSSEStream_NoError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "", Data: []byte(`[DONE]`)},
	})

	WriteSSEStream(c, stream)

	body := w.Body.String()
	assert.NotContains(t, body, "event:error")
}

func TestWriteBinaryStream(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "audio/mpeg", Data: []byte{0x01, 0x02}},
		{Type: "audio/mpeg", Data: []byte{0x03, 0x04, 0x05}},
	})

	WriteBinaryStream(c, stream)

	require.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, w.Body.Bytes())
}

func TestWriteBinaryStream_ErrorBeforeFirstChunkReturnsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := &errorAfterStream{
		err: &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Body:       []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error","code":"rate_limit"},"request_id":"req_1"}`),
		},
	}

	WriteBinaryStream(c, stream)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	errorField, ok := body["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rate_limit_error", errorField["type"])
	require.Equal(t, "rate_limit", errorField["code"])
	require.Equal(t, "req_1", body["request_id"])
}

func TestWriteBinaryStream_WriteErrorStopsConsuming(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	failingWriter := &failingResponseWriter{
		ResponseWriter: c.Writer,
		err:            errors.New("broken pipe"),
	}
	c.Writer = failingWriter

	stream := &trackingStream{
		items: []*httpclient.StreamEvent{
			{Type: "audio/mpeg", Data: []byte{0x01}},
			{Type: "audio/mpeg", Data: []byte{0x02}},
		},
	}

	WriteBinaryStream(c, stream)

	require.Equal(t, 1, failingWriter.writes)
	require.Equal(t, 1, stream.idx)
	require.Empty(t, w.Body.Bytes())
}

func TestFormatStreamError_PlainError(t *testing.T) {
	err := errors.New("something went wrong")
	result := FormatStreamError(context.Background(), err)

	data, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	errorField := parsed["error"].(map[string]any)
	assert.Equal(t, "something went wrong", errorField["message"])
	assert.Equal(t, "server_error", errorField["type"])
	assert.Equal(t, "", errorField["code"])
}

func TestFormatStreamError_HttpClientError(t *testing.T) {
	httpErr := &httpclient.Error{
		StatusCode: 500,
		Body:       []byte(`{"error":{"message":"Internal server error","type":"internal_error"}}`),
	}
	result := FormatStreamError(context.Background(), httpErr)

	data, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	errorField := parsed["error"].(map[string]any)
	assert.Equal(t, "Internal server error", errorField["message"])
	assert.Equal(t, "internal_error", errorField["type"])
	assert.Equal(t, "", errorField["code"])
}

func TestFormatStreamError_QuotaExhaustedError(t *testing.T) {
	quotaErr := orchestrator.NewQuotaExhaustedError("gpt-4")
	result := FormatStreamError(context.Background(), quotaErr)

	data, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	errorField := parsed["error"].(map[string]any)
	assert.Equal(t, "all channels quota exhausted for model gpt-4", errorField["message"])
	assert.Equal(t, "quota_exhausted", errorField["type"])
	assert.Equal(t, "quota_exhausted", errorField["code"])
}

func TestWrapQuotaExhaustedAsResponseError_QuotaError(t *testing.T) {
	quotaErr := orchestrator.NewQuotaExhaustedError("gpt-4")
	result := wrapQuotaExhaustedAsResponseError(quotaErr)

	respErr := &llm.ResponseError{}
	ok := errors.As(result, &respErr)
	require.True(t, ok, "should convert to *llm.ResponseError")
	assert.Equal(t, http.StatusServiceUnavailable, respErr.StatusCode)
	assert.Equal(t, "all channels quota exhausted for model gpt-4", respErr.Detail.Message)
	assert.Equal(t, "quota_exhausted", respErr.Detail.Type)
	assert.Equal(t, "quota_exhausted", respErr.Detail.Code)
}

func TestWrapQuotaExhaustedAsResponseError_OtherError(t *testing.T) {
	otherErr := errors.New("something else")
	result := wrapQuotaExhaustedAsResponseError(otherErr)
	assert.Equal(t, otherErr, result, "non-quota errors should pass through unchanged")
}

func TestPlaygroundHandleError_QuotaExhausted_Returns503(t *testing.T) {
	handlers := &PlaygroundHandlers{}

	quotaErr := orchestrator.NewQuotaExhaustedError("gpt-4")
	errResp := handlers.HandleError(quotaErr)

	assert.Equal(t, http.StatusServiceUnavailable, errResp.Status)
	assert.Equal(t, http.StatusServiceUnavailable, errResp.Error.Code)
	assert.Equal(t, "all channels quota exhausted for model gpt-4", errResp.Error.Message)
}

func TestPlaygroundHandleError_OtherError_Returns500(t *testing.T) {
	handlers := &PlaygroundHandlers{}

	otherErr := errors.New("something else")
	errResp := handlers.HandleError(otherErr)

	assert.Equal(t, http.StatusInternalServerError, errResp.Status)
}

func TestFormatStreamError_LlmResponseError_PassesCodeAndRequestID(t *testing.T) {
	respErr := &llm.ResponseError{
		Detail: llm.ErrorDetail{
			Code:      "1311",
			Message:   "当前订阅套餐暂未开放GPT-6权限",
			Type:      "permission_error",
			RequestID: "202603112254417d15bd26697445b0",
		},
	}

	result := FormatStreamError(context.Background(), respErr)
	data, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	errorField := parsed["error"].(map[string]any)
	assert.Equal(t, "当前订阅套餐暂未开放GPT-6权限", errorField["message"])
	assert.Equal(t, "permission_error", errorField["type"])
	assert.Equal(t, "1311", errorField["code"])
	assert.Equal(t, "202603112254417d15bd26697445b0", parsed["request_id"])
}

func TestApplyUpstreamErrorPolicy_CustomMessage(t *testing.T) {
	ctx, systemService := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{
		Mode:          biz.UpstreamErrorModeCustom,
		CustomMessage: "模型服务暂时不可用，请稍后再试",
	})

	rawErr := &httpclient.Error{
		StatusCode: http.StatusTooManyRequests,
		Body:       []byte(`{"error":{"message":"raw provider secret","type":"rate_limit_error","code":"provider_rate_limit"},"request_id":"req_123"}`),
	}

	err := applyUpstreamErrorPolicy(ctx, pipeline.WrapUpstreamError(rawErr), systemService)

	respErr := &llm.ResponseError{}
	require.True(t, errors.As(err, &respErr))
	assert.Equal(t, http.StatusTooManyRequests, respErr.StatusCode)
	assert.Equal(t, "模型服务暂时不可用，请稍后再试", respErr.Detail.Message)
	assert.Equal(t, "rate_limit_error", respErr.Detail.Type)
	assert.Equal(t, "provider_rate_limit", respErr.Detail.Code)
	assert.Equal(t, "req_123", respErr.Detail.RequestID)
	assert.NotContains(t, respErr.Error(), "raw provider secret")
}

func TestApplyUpstreamErrorPolicy_PassthroughByDefault(t *testing.T) {
	ctx, systemService := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{
		Mode: biz.UpstreamErrorModePassthrough,
	})

	rawErr := errors.New("raw upstream error")

	err := applyUpstreamErrorPolicy(ctx, rawErr, systemService)

	assert.Equal(t, rawErr, err)
}

func TestApplyUpstreamErrorPolicy_DoesNotRewriteLocalResponseError(t *testing.T) {
	ctx, systemService := setupUpstreamErrorPolicyTest(t, biz.UpstreamErrorPolicy{
		Mode:          biz.UpstreamErrorModeCustom,
		CustomMessage: "模型服务暂时不可用，请稍后再试",
	})

	localErr := &llm.ResponseError{
		StatusCode: http.StatusForbidden,
		Detail: llm.ErrorDetail{
			Code:    "quota_exceeded",
			Message: "API key quota exceeded",
			Type:    "quota_exceeded_error",
		},
	}

	err := applyUpstreamErrorPolicy(ctx, localErr, systemService)

	assert.Equal(t, localErr, err)
}

// An upstream that ends at EOF without a terminal event produces no stream error,
// so the client must be told explicitly instead of reading a truncated generation
// as a successful completion.
func TestWriteSSEStream_IncompleteStreamReportsErrorToClient(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	// Deltas only: no finish_reason, no usage, no [DONE].
	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"1","choices":[{"delta":{"role":"assistant"}}]}`)},
		{Data: []byte(`{"id":"1","choices":[{"delta":{"reasoning_content":"The"}}]}`)},
	}

	WriteSSEStream(c, streams.SliceStream(events))

	body := w.Body.String()
	require.Contains(t, body, "event:error")
	require.Contains(t, body, orchestrator.ErrStreamIncomplete.Error())
}

func TestWriteSSEStream_IncompleteStreamReportsErrorWithHeartbeat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)},
	}

	writeSSEStream(c, streams.SliceStream(events), FormatStreamError, SSEKeepAliveConfig{
		Enabled:  true,
		Interval: time.Hour,
	}, sseHeartbeatOpenAI)

	body := w.Body.String()
	require.Contains(t, body, "event:error")
	require.Contains(t, body, orchestrator.ErrStreamIncomplete.Error())
}

// Gemini generateContent streams terminate with candidates[].finishReason and
// never send [DONE]. Treating that as incomplete appends a trailing error that
// Gemini SDKs surface as StreamException.
func TestWriteSSEStream_GeminiFinishReasonIsNotIncomplete(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]}}]}`)},
		{Data: []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"world!"}]},"finishReason":"STOP"}]}`)},
	}

	WriteSSEStream(c, streams.SliceStream(events))

	body := w.Body.String()
	require.NotContains(t, body, "event:error")
	require.NotContains(t, body, orchestrator.ErrStreamIncomplete.Error())
}

// A stream that carries finish_reason but no [DONE] is already complete; clients
// commonly close right after that chunk, so it must not be flagged as incomplete.
func TestWriteSSEStream_FinishReasonWithoutDoneIsNotIncomplete(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)},
		{Data: []byte(`{"id":"1","choices":[{"delta":{},"finish_reason":"stop"}]}`)},
	}

	WriteSSEStream(c, streams.SliceStream(events))

	body := w.Body.String()
	require.NotContains(t, body, "event:error")
	require.NotContains(t, body, orchestrator.ErrStreamIncomplete.Error())
}

func TestWriteSSEStream_CompletedStreamReportsNoError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"1","choices":[{"delta":{"content":"Hi"}}]}`)},
		{Data: []byte(`[DONE]`)},
	}

	WriteSSEStream(c, streams.SliceStream(events))

	body := w.Body.String()
	require.Contains(t, body, "[DONE]")
	require.NotContains(t, body, "event:error")
}

// Anthropic streams terminate with message_stop rather than [DONE].
func TestWriteSSEStream_MessageStopIsNotIncomplete(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	events := []*httpclient.StreamEvent{
		{Type: "content_block_delta", Data: []byte(`{"type":"content_block_delta"}`)},
		{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
	}

	WriteSSEStream(c, streams.SliceStream(events))

	body := w.Body.String()
	require.NotContains(t, body, "event:error")
}
