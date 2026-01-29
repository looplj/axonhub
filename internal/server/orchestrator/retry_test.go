package orchestrator

import (
	"errors"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestDeriveLoadBalancerStrategy(t *testing.T) {
	defaultStrategy := "adaptive"
	retryPolicy := &biz.RetryPolicy{
		LoadBalancerStrategy: defaultStrategy,
	}

	tests := []struct {
		name     string
		apiKey   *ent.APIKey
		expected string
	}{
		{
			name:     "apiKey is nil",
			apiKey:   nil,
			expected: defaultStrategy,
		},
		{
			name: "active profile is nil",
			apiKey: &ent.APIKey{
				Profiles: nil,
			},
			expected: defaultStrategy,
		},
		{
			name: "active profile name is empty",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "",
				},
			},
			expected: defaultStrategy,
		},
		{
			name: "active profile not found in profiles list",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "non-existent",
					Profiles: []objects.APIKeyProfile{
						{Name: "other"},
					},
				},
			},
			expected: defaultStrategy,
		},
		{
			name: "load balance strategy is nil in active profile",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "default",
					Profiles: []objects.APIKeyProfile{
						{
							Name:                "default",
							LoadBalanceStrategy: nil,
						},
					},
				},
			},
			expected: defaultStrategy,
		},
		{
			name: "load balance strategy is empty in active profile",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "default",
					Profiles: []objects.APIKeyProfile{
						{
							Name:                "default",
							LoadBalanceStrategy: lo.ToPtr(""),
						},
					},
				},
			},
			expected: defaultStrategy,
		},
		{
			name: "load balance strategy is system_default in active profile",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "default",
					Profiles: []objects.APIKeyProfile{
						{
							Name:                "default",
							LoadBalanceStrategy: lo.ToPtr("system_default"),
						},
					},
				},
			},
			expected: defaultStrategy,
		},
		{
			name: "load balance strategy is set to specific value in active profile",
			apiKey: &ent.APIKey{
				Profiles: &objects.APIKeyProfiles{
					ActiveProfile: "default",
					Profiles: []objects.APIKeyProfile{
						{
							Name:                "default",
							LoadBalanceStrategy: lo.ToPtr("failover"),
						},
					},
				},
			},
			expected: "failover",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveLoadBalancerStrategy(retryPolicy, tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractStatusCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "error is nil",
			err:      nil,
			expected: 0,
		},
		{
			name: "httpclient.Error",
			err: &httpclient.Error{
				StatusCode: http.StatusTooManyRequests,
			},
			expected: http.StatusTooManyRequests,
		},
		{
			name: "llm.ResponseError",
			err: &llm.ResponseError{
				StatusCode: http.StatusInternalServerError,
			},
			expected: http.StatusInternalServerError,
		},
		{
			name:     "wrapped httpclient.Error",
			err:      errors.New("wrapped: " + (&httpclient.Error{StatusCode: 401}).Error()), // This won't work with errors.As unless we use fmt.Errorf with %w
			expected: 0,
		},
		{
			name:     "wrapped httpclient.Error with %w",
			err:      errors.Join(errors.New("error"), &httpclient.Error{StatusCode: 401}),
			expected: 401,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractStatusCodeFromError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "error is nil",
			err:      nil,
			expected: false,
		},
		{
			name: "429 Too Many Requests is retryable",
			err: &httpclient.Error{
				StatusCode: http.StatusTooManyRequests,
			},
			expected: true,
		},
		{
			name: "400 Bad Request is not retryable",
			err: &httpclient.Error{
				StatusCode: http.StatusBadRequest,
			},
			expected: false,
		},
		{
			name: "500 Internal Server Error is retryable",
			err: &llm.ResponseError{
				StatusCode: http.StatusInternalServerError,
			},
			expected: true,
		},
		{
			name:     "generic error is not retryable",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
