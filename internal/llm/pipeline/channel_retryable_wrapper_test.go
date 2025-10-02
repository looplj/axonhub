package pipeline

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// ChannelRetryableWrapper wraps an outbound transformer to add same-channel retry capabilities
// with intelligent HTTP status code checking.
type ChannelRetryableWrapper struct {
	// The underlying outbound transformer
	transformer transformer.Outbound

	// Retry configuration
	maxRetries     int
	currentRetries int

	// Optional custom retry logic
	customCanRetry func(error) bool
}

// NewChannelRetryableWrapper creates a new wrapper that adds same-channel retry capability
// to any outbound transformer with intelligent HTTP status code checking.
func NewChannelRetryableWrapper(t transformer.Outbound, maxRetries int) *ChannelRetryableWrapper {
	return &ChannelRetryableWrapper{
		transformer:    t,
		maxRetries:     maxRetries,
		currentRetries: 0,
		customCanRetry: nil, // Use default HTTP status code checking
	}
}

// NewChannelRetryableWrapperWithCustomLogic creates a new wrapper with custom retry logic.
func NewChannelRetryableWrapperWithCustomLogic(t transformer.Outbound, maxRetries int, canRetryFunc func(error) bool) *ChannelRetryableWrapper {
	return &ChannelRetryableWrapper{
		transformer:    t,
		maxRetries:     maxRetries,
		currentRetries: 0,
		customCanRetry: canRetryFunc,
	}
}

// APIFormat returns the API format of the underlying transformer.
func (w *ChannelRetryableWrapper) APIFormat() llm.APIFormat {
	return w.transformer.APIFormat()
}

// TransformRequest delegates to the underlying transformer.
func (w *ChannelRetryableWrapper) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return w.transformer.TransformRequest(ctx, request)
}

// TransformResponse delegates to the underlying transformer.
func (w *ChannelRetryableWrapper) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return w.transformer.TransformResponse(ctx, response)
}

// TransformStream delegates to the underlying transformer.
func (w *ChannelRetryableWrapper) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return w.transformer.TransformStream(ctx, stream)
}

// TransformError delegates to the underlying transformer.
func (w *ChannelRetryableWrapper) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return w.transformer.TransformError(ctx, err)
}

// AggregateStreamChunks delegates to the underlying transformer.
func (w *ChannelRetryableWrapper) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return w.transformer.AggregateStreamChunks(ctx, chunks)
}

// ExtractStatusCodeFromError attempts to extract HTTP status code from various error types.
func ExtractStatusCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	// Try to extract from httpclient.Error
	var httpErr *httpclient.Error
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}

	errMsg := err.Error()

	// Try to extract status code from error messages like "HTTP error 400"
	re := regexp.MustCompile(`HTTP error (\d{3})`)

	matches := re.FindStringSubmatch(errMsg)
	if len(matches) > 1 {
		if statusCode := parseInt(matches[1]); statusCode > 0 {
			return statusCode
		}
	}

	return 0
}

// parseInt safely parses a string to int.
func parseInt(s string) int {
	var result int

	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			break
		}
	}

	return result
}

// CanRetry implements ChannelRetryable interface with intelligent HTTP status code checking.
func (w *ChannelRetryableWrapper) CanRetry(err error) bool {
	// Check if we've exhausted the maximum number of retries
	if w.currentRetries >= w.maxRetries {
		return false
	}

	// Use custom retry logic if provided
	if w.customCanRetry != nil {
		return w.customCanRetry(err)
	}

	// Default behavior: check HTTP status code
	statusCode := ExtractStatusCodeFromError(err)
	if statusCode > 0 {
		return httpclient.IsHTTPStatusCodeRetryable(statusCode)
	}

	// If we can't extract a status code, allow retry for backward compatibility
	return true
}

// PrepareForRetry implements ChannelRetryable interface.
func (w *ChannelRetryableWrapper) PrepareForRetry(ctx context.Context) error {
	w.currentRetries++
	return nil
}

// ResetRetries resets the retry counter. This can be useful when switching to a new channel.
func (w *ChannelRetryableWrapper) ResetRetries() {
	w.currentRetries = 0
}

// mockOutboundTransformer implements transformer.Outbound for testing.
type mockOutboundTransformer struct {
	apiFormat llm.APIFormat
}

func (m *mockOutboundTransformer) APIFormat() llm.APIFormat {
	return m.apiFormat
}

func (m *mockOutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	return &httpclient.Request{}, nil
}

func (m *mockOutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (m *mockOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.SliceStream([]*llm.Response{}), nil
}

func (m *mockOutboundTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return &llm.ResponseError{}
}

func (m *mockOutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

func TestChannelRetryableWrapper_CanRetry(t *testing.T) {
	underlying := &mockOutboundTransformer{apiFormat: "test/mock"}
	wrapper := NewChannelRetryableWrapper(underlying, 3)

	t.Run("should retry on 429 (rate limiting)", func(t *testing.T) {
		err := errors.New("HTTP error 429")
		require.True(t, wrapper.CanRetry(err))
	})

	t.Run("should not retry on 400 (bad request)", func(t *testing.T) {
		err := errors.New("HTTP error 400")
		require.False(t, wrapper.CanRetry(err))
	})

	t.Run("should not retry on 401 (unauthorized)", func(t *testing.T) {
		err := errors.New("HTTP error 401")
		require.False(t, wrapper.CanRetry(err))
	})

	t.Run("should not retry on 403 (forbidden)", func(t *testing.T) {
		err := errors.New("HTTP error 403")
		require.False(t, wrapper.CanRetry(err))
	})

	t.Run("should not retry on 404 (not found)", func(t *testing.T) {
		err := errors.New("HTTP error 404")
		require.False(t, wrapper.CanRetry(err))
	})

	t.Run("should retry on 500 (internal server error)", func(t *testing.T) {
		err := errors.New("HTTP error 500")
		require.True(t, wrapper.CanRetry(err))
	})

	t.Run("should retry on 502 (bad gateway)", func(t *testing.T) {
		err := errors.New("HTTP error 502")
		require.True(t, wrapper.CanRetry(err))
	})

	t.Run("should retry on 503 (service unavailable)", func(t *testing.T) {
		err := errors.New("HTTP error 503")
		require.True(t, wrapper.CanRetry(err))
	})

	t.Run("should retry on unknown errors (backward compatibility)", func(t *testing.T) {
		err := errors.New("some unknown error")
		require.True(t, wrapper.CanRetry(err))
	})

	t.Run("should not retry when max retries exhausted", func(t *testing.T) {
		wrapper.currentRetries = 3 // Set to max retries
		err := errors.New("HTTP error 500")
		require.False(t, wrapper.CanRetry(err))
	})

	t.Run("should use custom retry logic when provided", func(t *testing.T) {
		customWrapper := NewChannelRetryableWrapperWithCustomLogic(underlying, 3, func(err error) bool {
			return err.Error() == "custom retryable error"
		})

		require.True(t, customWrapper.CanRetry(errors.New("custom retryable error")))
		require.False(t, customWrapper.CanRetry(errors.New("HTTP error 500")))
	})
}

func TestChannelRetryableWrapper_PrepareForRetry(t *testing.T) {
	underlying := &mockOutboundTransformer{apiFormat: "test/mock"}
	wrapper := NewChannelRetryableWrapper(underlying, 3)

	initialRetries := wrapper.currentRetries
	err := wrapper.PrepareForRetry(nil)

	require.NoError(t, err)
	require.Equal(t, initialRetries+1, wrapper.currentRetries)
}

func TestChannelRetryableWrapper_ResetRetries(t *testing.T) {
	underlying := &mockOutboundTransformer{apiFormat: "test/mock"}
	wrapper := NewChannelRetryableWrapper(underlying, 3)

	wrapper.currentRetries = 2
	wrapper.ResetRetries()

	require.Equal(t, 0, wrapper.currentRetries)
}

func TestChannelRetryableWrapper_Delegation(t *testing.T) {
	underlying := &mockOutboundTransformer{apiFormat: "test/api"}
	wrapper := NewChannelRetryableWrapper(underlying, 3)

	// Test that all methods are properly delegated
	require.Equal(t, llm.APIFormat("test/api"), wrapper.APIFormat())

	ctx := context.Background()

	// These should not panic and should delegate properly
	_, err := wrapper.TransformRequest(ctx, &llm.Request{})
	require.NoError(t, err)

	_, err = wrapper.TransformResponse(ctx, &httpclient.Response{})
	require.NoError(t, err)

	_, err = wrapper.TransformStream(ctx, streams.SliceStream([]*httpclient.StreamEvent{}))
	require.NoError(t, err)

	result := wrapper.TransformError(ctx, &httpclient.Error{})
	require.NotNil(t, result)

	_, _, err = wrapper.AggregateStreamChunks(ctx, []*httpclient.StreamEvent{})
	require.NoError(t, err)
}

func TestExtractStatusCodeFromError(t *testing.T) {
	t.Run("extract from HTTP error message", func(t *testing.T) {
		err := errors.New("HTTP error 404")
		require.Equal(t, 404, ExtractStatusCodeFromError(err))
	})

	t.Run("extract from httpclient.Error", func(t *testing.T) {
		httpErr := &httpclient.Error{StatusCode: 422}
		require.Equal(t, 422, ExtractStatusCodeFromError(httpErr))
	})

	t.Run("return 0 for nil error", func(t *testing.T) {
		require.Equal(t, 0, ExtractStatusCodeFromError(nil))
	})

	t.Run("return 0 for unrecognized error", func(t *testing.T) {
		err := errors.New("some other error")
		require.Equal(t, 0, ExtractStatusCodeFromError(err))
	})
}
