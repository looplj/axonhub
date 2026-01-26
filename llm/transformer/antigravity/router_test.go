package antigravity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineQuotaPreference(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected QuotaPreference
	}{
		// Explicit suffix tests
		{
			name:     "explicit antigravity suffix",
			model:    "gemini-2.5-pro:antigravity",
			expected: QuotaAntigravity,
		},
		{
			name:     "explicit gemini-cli suffix",
			model:    "claude-sonnet-4-5:gemini-cli",
			expected: QuotaGeminiCLI,
		},
		{
			name:     "suffix overrides default",
			model:    "gemini-2.5-flash:antigravity",
			expected: QuotaAntigravity,
		},

		// Explicit prefix tests
		{
			name:     "antigravity prefix on gemini model",
			model:    "antigravity-gemini-2.5-flash",
			expected: QuotaAntigravity,
		},
		{
			name:     "antigravity prefix on claude model",
			model:    "antigravity-claude-sonnet-4-5",
			expected: QuotaAntigravity,
		},

		// Claude models (always antigravity)
		{
			name:     "claude-sonnet-4-5",
			model:    "claude-sonnet-4-5",
			expected: QuotaAntigravity,
		},
		{
			name:     "claude-sonnet-4-5-thinking",
			model:    "claude-sonnet-4-5-thinking",
			expected: QuotaAntigravity,
		},
		{
			name:     "claude-opus-4-5-thinking",
			model:    "claude-opus-4-5-thinking",
			expected: QuotaAntigravity,
		},
		{
			name:     "claude with uppercase",
			model:    "Claude-Sonnet-4-5",
			expected: QuotaAntigravity,
		},

		// GPT models (antigravity)
		{
			name:     "gpt-oss-120b-medium",
			model:    "gpt-oss-120b-medium",
			expected: QuotaAntigravity,
		},
		{
			name:     "gpt model",
			model:    "gpt-4",
			expected: QuotaAntigravity,
		},

		// Image models (antigravity)
		{
			name:     "image generation model",
			model:    "gemini-3-pro-image",
			expected: QuotaAntigravity,
		},
		{
			name:     "imagen model",
			model:    "imagen-3",
			expected: QuotaAntigravity,
		},

		// Legacy Gemini 3 models (antigravity)
		{
			name:     "gemini-3-pro-low",
			model:    "gemini-3-pro-low",
			expected: QuotaAntigravity,
		},
		{
			name:     "gemini-3-pro-high",
			model:    "gemini-3-pro-high",
			expected: QuotaAntigravity,
		},
		{
			name:     "gemini-3-pro-medium",
			model:    "gemini-3-pro-medium",
			expected: QuotaAntigravity,
		},
		{
			name:     "gemini-3-flash",
			model:    "gemini-3-flash",
			expected: QuotaAntigravity,
		},
		{
			name:     "gemini-3-flash-low",
			model:    "gemini-3-flash-low",
			expected: QuotaAntigravity,
		},

		// Standard Gemini models (gemini-cli)
		{
			name:     "gemini-2.5-pro",
			model:    "gemini-2.5-pro",
			expected: QuotaGeminiCLI,
		},
		{
			name:     "gemini-2.5-flash",
			model:    "gemini-2.5-flash",
			expected: QuotaGeminiCLI,
		},
		{
			name:     "gemini-2.5-flash-lite",
			model:    "gemini-2.5-flash-lite",
			expected: QuotaGeminiCLI,
		},
		{
			name:     "gemini-1.5-pro",
			model:    "gemini-1.5-pro",
			expected: QuotaGeminiCLI,
		},
		{
			name:     "gemini-3-pro-preview",
			model:    "gemini-3-pro-preview",
			expected: QuotaGeminiCLI,
		},
		{
			name:     "gemini-3-flash-preview",
			model:    "gemini-3-flash-preview",
			expected: QuotaGeminiCLI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineQuotaPreference(tt.model)
			assert.Equal(t, tt.expected, result, "Model: %s", tt.model)
		})
	}
}

func TestGetInitialEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		preference QuotaPreference
		expected   string
	}{
		{
			name:       "antigravity quota uses daily endpoint",
			preference: QuotaAntigravity,
			expected:   EndpointDaily,
		},
		{
			name:       "gemini-cli quota uses daily endpoint",
			preference: QuotaGeminiCLI,
			expected:   EndpointDaily, // All models now start with Daily
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInitialEndpoint(tt.preference)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFallbackEndpoints(t *testing.T) {
	endpoints := GetFallbackEndpoints()
	expected := []string{
		EndpointDaily,
		EndpointAutopush,
		EndpointProd,
	}
	assert.Equal(t, expected, endpoints)
}

func TestStripModelSuffix(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "strip antigravity suffix",
			model:    "gemini-2.5-pro:antigravity",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "strip gemini-cli suffix",
			model:    "claude-sonnet-4-5:gemini-cli",
			expected: "claude-sonnet-4-5",
		},
		{
			name:     "no suffix to strip",
			model:    "gemini-2.5-pro",
			expected: "gemini-2.5-pro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripModelSuffix(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStripAntigravityPrefix(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "strip antigravity prefix",
			model:    "antigravity-gemini-2.5-pro",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "no prefix to strip",
			model:    "gemini-2.5-pro",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "case sensitive prefix",
			model:    "Antigravity-gemini-2.5-pro",
			expected: "Antigravity-gemini-2.5-pro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripAntigravityPrefix(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "strip both suffix and prefix",
			model:    "antigravity-gemini-2.5-pro:gemini-cli",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "strip only suffix",
			model:    "gemini-2.5-pro:antigravity",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "strip only prefix",
			model:    "antigravity-claude-sonnet-4-5",
			expected: "claude-sonnet-4-5",
		},
		{
			name:     "no changes needed",
			model:    "gemini-2.5-pro",
			expected: "gemini-2.5-pro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeModelName(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldRetryWithDifferentEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{name: "429 rate limit should retry", statusCode: 429, expected: true},
		{name: "403 forbidden should retry", statusCode: 403, expected: true},
		{name: "404 not found should retry", statusCode: 404, expected: true},
		{name: "500 server error should retry", statusCode: 500, expected: true},
		{name: "502 bad gateway should retry", statusCode: 502, expected: true},
		{name: "503 service unavailable should retry", statusCode: 503, expected: true},
		{name: "504 gateway timeout should retry", statusCode: 504, expected: true},
		{name: "200 ok should not retry", statusCode: 200, expected: false},
		{name: "400 bad request should not retry", statusCode: 400, expected: false},
		{name: "401 unauthorized should not retry", statusCode: 401, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetryWithDifferentEndpoint(tt.statusCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}
