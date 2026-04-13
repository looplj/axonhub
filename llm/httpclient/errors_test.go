package httpclient

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "404 error",
			err:      &Error{StatusCode: http.StatusNotFound},
			expected: true,
		},
		{
			name:     "500 error",
			err:      &Error{StatusCode: http.StatusInternalServerError},
			expected: false,
		},
		{
			name:     "429 error",
			err:      &Error{StatusCode: http.StatusTooManyRequests},
			expected: false,
		},
		{
			name:     "other error",
			err:      assert.AnError,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsNotFoundErr(tt.err))
		})
	}
}

func TestIsRateLimitErr(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "429 error",
			err:      &Error{StatusCode: http.StatusTooManyRequests},
			expected: true,
		},
		{
			name:     "500 error",
			err:      &Error{StatusCode: http.StatusInternalServerError},
			expected: false,
		},
		{
			name:     "404 error",
			err:      &Error{StatusCode: http.StatusNotFound},
			expected: false,
		},
		{
			name:     "other error",
			err:      assert.AnError,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRateLimitErr(tt.err))
		})
	}
}

func TestHasRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "429 with Retry-After header",
			err:      &Error{StatusCode: http.StatusTooManyRequests, Headers: http.Header{"Retry-After": []string{"30"}}},
			expected: true,
		},
		{
			name:     "429 without Retry-After header",
			err:      &Error{StatusCode: http.StatusTooManyRequests, Headers: http.Header{}},
			expected: false,
		},
		{
			name:     "429 with nil headers",
			err:      &Error{StatusCode: http.StatusTooManyRequests},
			expected: false,
		},
		{
			name:     "500 with Retry-After header",
			err:      &Error{StatusCode: http.StatusInternalServerError, Headers: http.Header{"Retry-After": []string{"30"}}},
			expected: false,
		},
		{
			name:     "other error",
			err:      assert.AnError,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasRetryAfterHeader(tt.err))
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Run("not a 429 error", func(t *testing.T) {
		err := &Error{StatusCode: http.StatusInternalServerError}
		duration, ok := ParseRetryAfter(err)
		assert.False(t, ok)
		assert.Equal(t, time.Duration(0), duration)
	})

	t.Run("other error type", func(t *testing.T) {
		duration, ok := ParseRetryAfter(assert.AnError)
		assert.False(t, ok)
		assert.Equal(t, time.Duration(0), duration)
	})
}

func TestParseRetryAfter_IntegerSeconds(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{
			name:     "30 seconds",
			value:    "30",
			expected: 30 * time.Second,
		},
		{
			name:     "60 seconds",
			value:    "60",
			expected: 60 * time.Second,
		},
		{
			name:     "120 seconds",
			expected: 120 * time.Second,
			value:    "120",
		},
		{
			name:     "0 seconds - retry immediately",
			value:    "0",
			expected: 0,
		},
		{
			name:     "negative seconds - retry immediately",
			value:    "-30",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{
				StatusCode: http.StatusTooManyRequests,
				Headers:    http.Header{"Retry-After": []string{tt.value}},
			}
			result, ok := ParseRetryAfter(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	// Use a future date
	futureTime := time.Now().Add(45 * time.Second)
	httpDate := futureTime.UTC().Format(http.TimeFormat)

	err := &Error{
		StatusCode: http.StatusTooManyRequests,
		Headers:    http.Header{"Retry-After": []string{httpDate}},
	}
	result, ok := ParseRetryAfter(err)

	assert.True(t, ok)
	// Should be approximately 45 seconds (allow 1 second tolerance)
	assert.WithinDuration(t, time.Now().Add(45*time.Second), time.Now().Add(result), 1*time.Second)
}

func TestParseRetryAfter_HTTPDate_Past(t *testing.T) {
	// Use a past date
	pastTime := time.Now().Add(-10 * time.Second)
	httpDate := pastTime.UTC().Format(http.TimeFormat)

	err := &Error{
		StatusCode: http.StatusTooManyRequests,
		Headers:    http.Header{"Retry-After": []string{httpDate}},
	}
	result, ok := ParseRetryAfter(err)

	assert.True(t, ok)
	// Past date means retry immediately
	assert.Equal(t, time.Duration(0), result)
}

func TestParseRetryAfter_Empty(t *testing.T) {
	err := &Error{
		StatusCode: http.StatusTooManyRequests,
		Headers:    http.Header{},
	}
	result, ok := ParseRetryAfter(err)

	assert.True(t, ok)
	// Should use default
	assert.Equal(t, DefaultRetryAfterSeconds*time.Second, result)
}

func TestParseRetryAfter_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "invalid string",
			value: "invalid",
		},
		{
			name:  "malformed date",
			value: "not-a-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &Error{
				StatusCode: http.StatusTooManyRequests,
				Headers:    http.Header{"Retry-After": []string{tt.value}},
			}
			result, ok := ParseRetryAfter(err)

			assert.True(t, ok)
			// Should use default
			assert.Equal(t, DefaultRetryAfterSeconds*time.Second, result)
		})
	}
}

func TestParseRetryAfter_MaxCooldownCap(t *testing.T) {
	// Test with value exceeding max cooldown (5 minutes)
	err := &Error{
		StatusCode: http.StatusTooManyRequests,
		Headers:    http.Header{"Retry-After": []string{"86400"}}, // 24 hours
	}
	result, ok := ParseRetryAfter(err)

	assert.True(t, ok)
	// Should be capped at MaxRetryAfterDuration
	assert.Equal(t, MaxRetryAfterDuration, result)
}

func TestParseRetryAfter_HTTPDate_MaxCooldownCap(t *testing.T) {
	// Use a date far in the future (1 year)
	futureTime := time.Now().Add(365 * 24 * time.Hour)
	httpDate := futureTime.UTC().Format(http.TimeFormat)

	err := &Error{
		StatusCode: http.StatusTooManyRequests,
		Headers:    http.Header{"Retry-After": []string{httpDate}},
	}
	result, ok := ParseRetryAfter(err)

	assert.True(t, ok)
	// Should be capped at MaxRetryAfterDuration
	assert.Equal(t, MaxRetryAfterDuration, result)
}

func TestError_Error(t *testing.T) {
	err := &Error{
		Method:     "POST",
		URL:        "https://api.example.com/v1/chat",
		StatusCode: http.StatusTooManyRequests,
		Status:     "429 Too Many Requests",
	}
	assert.Equal(t, "POST - https://api.example.com/v1/chat with status 429 Too Many Requests", err.Error())
}