package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

type failingResponseWriter struct {
	gin.ResponseWriter

	err    error
	writes int
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	w.writes++

	return 0, w.err
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
