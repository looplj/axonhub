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

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func init() {
	gin.SetMode(gin.TestMode)
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
